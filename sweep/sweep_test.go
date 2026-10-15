package sweep_test

import (
	"bytes"
	"context"
	"database/sql"
	"database/sql/driver"
	"errors"
	"log/slog"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/standards-lab/go-core/reactor"
	"github.com/standards-lab/go-database/sweep"
	"github.com/standards-lab/sqlate"
	"github.com/standards-lab/sqlate/sqltest"
)

// t0 is the first delivery's time in the tests that drive the handler.
var t0 = time.Date(2026, 10, 15, 12, 0, 0, 0, time.UTC)

// flag scripts the one boolean row a try-lock or unlock statement reads.
func flag(b bool) sqltest.Response {
	return sqltest.Response{Columns: []string{"ok"}, Rows: [][]driver.Value{{b}}}
}

// claimed scripts one cycle whose claim is granted and released.
func claimed() []sqltest.Response { return []sqltest.Response{flag(true), flag(true)} }

// result is one scripted pass outcome.
type result struct {
	more bool
	err  error
}

// reclaimer is a fake Reclaimer: it returns its scripted results in order,
// then (false, nil), and records each budget it was given. block, when
// set, runs inside each pass before it returns.
type reclaimer struct {
	mu      sync.Mutex
	results []result
	budgets []int
	block   func(ctx context.Context) error
	log     *[]string // shared order log, when set
	name    string
}

func (r *reclaimer) Pass(ctx context.Context, budget int) (bool, error) {
	r.mu.Lock()
	r.budgets = append(r.budgets, budget)
	if r.log != nil {
		*r.log = append(*r.log, "pass "+r.name)
	}
	var res result
	if len(r.results) > 0 {
		res, r.results = r.results[0], r.results[1:]
	}
	block := r.block
	r.mu.Unlock()
	if block != nil {
		if err := block(ctx); err != nil {
			return false, err
		}
	}
	return res.more, res.err
}

func (r *reclaimer) passes() int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return len(r.budgets)
}

// passes collects the hook's reports.
type passes struct {
	mu  sync.Mutex
	got []sweep.Pass
}

func (p *passes) hook(_ context.Context, pass sweep.Pass) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.got = append(p.got, pass)
}

func (p *passes) all() []sweep.Pass {
	p.mu.Lock()
	defer p.mu.Unlock()
	return slices.Clone(p.got)
}

// logs is a logger whose output a test reads, safe for the reactor's
// goroutine to write.
type logs struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (l *logs) Write(p []byte) (int, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.buf.Write(p)
}

func (l *logs) String() string {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.buf.String()
}

func (l *logs) logger() *slog.Logger { return slog.New(slog.NewTextHandler(l, nil)) }

// fixture is a sweeper over a scripted locking pool.
type fixture struct {
	s    *sweep.Sweeper
	pool *sql.DB
	rec  *sqltest.Recorder
	logs *logs
}

func reg(name string, r sweep.Reclaimer, interval time.Duration, budget int) sweep.Registration {
	return sweep.Registration{Name: name, Reclaimer: r, Policy: sweep.Policy{Interval: interval, Budget: budget}}
}

func newFixture(t *testing.T, dialect sqlate.Dialect, regs []sweep.Registration, shutdown time.Duration, opts ...sweep.Option) fixture {
	t.Helper()
	pool, rec := sqltest.Open(t)
	l := &logs{}
	s, err := sweep.New(sqlate.Wrap(pool, dialect), regs, shutdown, l.logger(), opts...)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	return fixture{s: s, pool: pool, rec: rec, logs: l}
}

// locks returns the lock statements recorded, each with its lock's name.
func locks(rec *sqltest.Recorder) []string {
	var out []string
	for _, c := range rec.Calls() {
		if len(c.Args) == 1 {
			out = append(out, c.SQL+" "+c.Args[0].(string))
		}
	}
	return out
}

func wantLocks(t *testing.T, rec *sqltest.Recorder, want ...string) {
	t.Helper()
	if got := locks(rec); !slices.Equal(got, want) {
		t.Errorf("lock calls = %q, want %q", got, want)
	}
	if rec.Pending() != 0 || rec.RowsLeaked() != 0 {
		t.Errorf("pending = %d, leaked = %d; want 0, 0", rec.Pending(), rec.RowsLeaked())
	}
}

// await receives from ch, failing the test if nothing arrives in time, so a
// sweeper that never gets there fails rather than hangs.
func await[T any](t *testing.T, ch <-chan T, what string) T {
	t.Helper()
	select {
	case v := <-ch:
		return v
	case <-time.After(5 * time.Second):
		t.Fatalf("timed out waiting for %s", what)
		var zero T
		return zero
	}
}

// shutdown drains s within a bound, so a drain that never ends fails the
// test rather than hangs it.
func shutdown(s *sweep.Sweeper) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	return s.Shutdown(ctx)
}

const (
	try    = "SELECT try_lock($1) "
	unlock = "SELECT unlock($1) "
)

func TestNew_RefusesBeforeAnyIO(t *testing.T) {
	r := &reclaimer{}
	cases := []struct {
		name     string
		dialect  sqlate.Dialect
		regs     []sweep.Registration
		shutdown time.Duration
		opts     []sweep.Option
		want     string
		is       error
	}{
		{name: "no reclaimers", want: "no reclaimers"},
		{name: "empty name", regs: []sweep.Registration{reg("", r, time.Minute, 1)}, want: "no name"},
		{name: "duplicate name", regs: []sweep.Registration{reg("a", r, time.Minute, 1), reg("a", r, time.Minute, 1)}, want: `"a" is registered twice`},
		{name: "nil reclaimer", regs: []sweep.Registration{reg("a", nil, time.Minute, 1)}, want: `"a" is nil`},
		{name: "zero interval", regs: []sweep.Registration{reg("a", r, 0, 1)}, want: "interval must be positive"},
		{name: "negative interval", regs: []sweep.Registration{reg("a", r, -time.Second, 1)}, want: "interval must be positive"},
		{name: "zero budget", regs: []sweep.Registration{reg("a", r, time.Minute, 0)}, want: "budget must be positive"},
		{name: "negative budget", regs: []sweep.Registration{reg("a", r, time.Minute, -1)}, want: "budget must be positive"},
		{name: "zero shutdown", regs: []sweep.Registration{reg("a", r, time.Minute, 1)}, shutdown: -1, want: "shutdown must be positive"},
		{name: "no try-lock", dialect: sqltest.Dialect{}, regs: []sweep.Registration{reg("a", r, time.Minute, 1)}, is: sweep.ErrNoTryLocker},
		{name: "no try-lock, gated", dialect: sqltest.Dialect{}, regs: []sweep.Registration{reg("a", r, time.Minute, 1)}, opts: []sweep.Option{sweep.Gated(&reactor.Gate{})}, is: sweep.ErrNoTryLocker},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			pool, rec := sqltest.Open(t)
			dialect := tc.dialect
			if dialect == nil {
				dialect = sqltest.LockingDialect{}
			}
			shutdown := tc.shutdown
			if shutdown == 0 {
				shutdown = time.Second
			} else if shutdown < 0 {
				shutdown = 0
			}
			s, err := sweep.New(sqlate.Wrap(pool, dialect), tc.regs, shutdown, slog.New(slog.DiscardHandler), tc.opts...)
			if s != nil || err == nil {
				t.Fatalf("New = %v, %v; want a refusal", s, err)
			}
			if tc.is != nil && !errors.Is(err, tc.is) {
				t.Errorf("err = %v, want %v", err, tc.is)
			}
			if tc.want != "" && !strings.Contains(err.Error(), tc.want) {
				t.Errorf("err = %v, want it to mention %q", err, tc.want)
			}
			if calls := rec.Calls(); len(calls) != 0 {
				t.Errorf("calls = %+v, want none", calls)
			}
		})
	}
}

func TestNew_UnlockedAcceptsADialectWithoutTryLock(t *testing.T) {
	f := newFixture(t, sqltest.Dialect{}, []sweep.Registration{reg("a", &reclaimer{}, time.Minute, 1)}, time.Second, sweep.Unlocked())
	if f.s == nil {
		t.Fatal("New returned no sweeper")
	}
}

func TestNew_WiringDefectsPanic(t *testing.T) {
	pool, _ := sqltest.Open(t)
	db := sqlate.Wrap(pool, sqltest.LockingDialect{})
	regs := []sweep.Registration{reg("a", &reclaimer{}, time.Minute, 1)}
	for name, call := range map[string]func(){
		"nil db":     func() { _, _ = sweep.New(nil, regs, time.Second, slog.New(slog.DiscardHandler)) },
		"nil logger": func() { _, _ = sweep.New(db, regs, time.Second, nil) },
	} {
		t.Run(name, func(t *testing.T) {
			defer func() {
				if recover() == nil {
					t.Error("no panic")
				}
			}()
			call()
		})
	}
}

func TestHandle_TicksAtTheSmallestInterval(t *testing.T) {
	// The tick is the smallest interval, 1m, so a reclaimer is due half a
	// tick, 30s, before its own interval has passed: slow's 5m makes it due
	// from 4m30s. A tick of the largest interval would make it due from
	// 4m-30m, and one of the middle interval from 2m30s.
	var order []string
	slow := &reclaimer{name: "slow", log: &order}
	f := newFixture(t, sqltest.LockingDialect{}, []sweep.Registration{
		reg("slow", slow, 5*time.Minute, 1),
		reg("fast", &reclaimer{}, time.Minute, 1),
		reg("rare", &reclaimer{}, time.Hour, 1),
	}, time.Second, sweep.Unlocked())
	handle := sweep.Handle(f.s)

	for _, at := range []time.Duration{0, 4*time.Minute + 29*time.Second, 4*time.Minute + 31*time.Second} {
		if err := handle(context.Background(), t0.Add(at)); err != nil {
			t.Fatalf("handle at %v: %v", at, err)
		}
	}
	if want := []string{"pass slow", "pass slow"}; !slices.Equal(order, want) {
		t.Errorf("slow's passes = %q, want %q: due at start and from 4m30s", order, want)
	}
}

func TestHandle_ClaimsTheLockAndPassesAtTheBudget(t *testing.T) {
	var p passes
	r := &reclaimer{}
	f := newFixture(t, sqltest.LockingDialect{}, []sweep.Registration{reg("files", r, time.Minute, 7)}, time.Second, sweep.OnPass(p.hook))
	f.rec.Queue(claimed()...)

	if err := sweep.Handle(f.s)(context.Background(), t0); err != nil {
		t.Fatalf("handle: %v", err)
	}
	if !slices.Equal(r.budgets, []int{7}) {
		t.Errorf("budgets = %v, want [7]", r.budgets)
	}
	got := p.all()
	if len(got) != 1 || got[0].Name != "files" || got[0].More || got[0].Err != nil || got[0].Duration < 0 {
		t.Errorf("reports = %+v, want one finished pass of files", got)
	}
	wantLocks(t, f.rec, try+"sweep.files", unlock+"sweep.files")
	if st := f.pool.Stats(); st.OpenConnections != 1 || st.Idle != 1 {
		t.Errorf("pool = %d open, %d idle; want the released connection pooled", st.OpenConnections, st.Idle)
	}
}

func TestHandle_ALockHeldElsewhereRunsNoPassAndReportsNothing(t *testing.T) {
	var p passes
	r := &reclaimer{}
	f := newFixture(t, sqltest.LockingDialect{}, []sweep.Registration{reg("files", r, time.Minute, 1)}, time.Second, sweep.OnPass(p.hook))
	f.rec.Queue(flag(false))

	if err := sweep.Handle(f.s)(context.Background(), t0); err != nil {
		t.Fatalf("handle: %v", err)
	}
	if r.passes() != 0 || len(p.all()) != 0 {
		t.Errorf("passes = %d, reports = %+v; want none", r.passes(), p.all())
	}
	if f.logs.String() != "" {
		t.Errorf("logged %q, want nothing", f.logs.String())
	}
	wantLocks(t, f.rec, try+"sweep.files")
}

func TestHandle_PassesWhileMoreUnderOneClaim(t *testing.T) {
	var p passes
	r := &reclaimer{results: []result{{more: true}, {more: true}, {more: false}}}
	f := newFixture(t, sqltest.LockingDialect{}, []sweep.Registration{reg("files", r, time.Minute, 3)}, time.Second, sweep.OnPass(p.hook))
	f.rec.Queue(claimed()...)

	if err := sweep.Handle(f.s)(context.Background(), t0); err != nil {
		t.Fatalf("handle: %v", err)
	}
	if r.passes() != 3 {
		t.Errorf("passes = %d, want 3", r.passes())
	}
	var more []bool
	for _, got := range p.all() {
		more = append(more, got.More)
	}
	if !slices.Equal(more, []bool{true, true, false}) {
		t.Errorf("reported more = %v, want [true true false]", more)
	}
	wantLocks(t, f.rec, try+"sweep.files", unlock+"sweep.files")
}

// clock is a fake clock for a sweeper's cycle bound, which a reclaimer's
// pass advances so a cycle runs past its interval without sleeping.
type clock struct {
	mu sync.Mutex
	t  time.Time
}

func (c *clock) now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.t
}

func (c *clock) advance(d time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.t = c.t.Add(d)
}

// endless is a reclaimer that reports more on every pass, each pass
// advancing c by step.
func endless(name string, log *[]string, c *clock, step time.Duration) *reclaimer {
	results := make([]result, 100)
	for i := range results {
		results[i].more = true
	}
	return &reclaimer{name: name, log: log, results: results, block: func(context.Context) error {
		c.advance(step)
		return nil
	}}
}

func TestHandle_ACycleYieldsAtItsIntervalAndStaysDue(t *testing.T) {
	c := &clock{t: t0}
	// Each pass takes 20s of a 1m interval, so the cycle yields after the
	// third, still reporting more.
	r := endless("files", nil, c, 20*time.Second)
	f := newFixture(t, sqltest.LockingDialect{}, []sweep.Registration{reg("files", r, time.Minute, 1)}, time.Second)
	sweep.SetClock(f.s, c.now)
	handle := sweep.Handle(f.s)

	f.rec.Queue(claimed()...)
	if err := handle(context.Background(), t0); err != nil {
		t.Fatalf("handle: %v", err)
	}
	if r.passes() != 3 {
		t.Fatalf("passes = %d, want the cycle to end at its interval, after 3", r.passes())
	}
	wantLocks(t, f.rec, try+"sweep.files", unlock+"sweep.files")

	// The next delivery, a second later and well inside the interval, runs
	// the yielded reclaimer again.
	f.rec.Queue(claimed()...)
	if err := handle(context.Background(), t0.Add(time.Second)); err != nil {
		t.Fatalf("second handle: %v", err)
	}
	if r.passes() != 6 {
		t.Errorf("passes = %d, want the yielded reclaimer due again for 3 more", r.passes())
	}
	wantLocks(t, f.rec,
		try+"sweep.files", unlock+"sweep.files", try+"sweep.files", unlock+"sweep.files")
}

func TestHandle_AYieldingCycleLetsTheReclaimersAfterItRun(t *testing.T) {
	var order []string
	c := &clock{t: t0}
	a := endless("a", &order, c, 30*time.Second)
	b := &reclaimer{name: "b", log: &order}
	f := newFixture(t, sqltest.LockingDialect{}, []sweep.Registration{
		reg("a", a, time.Minute, 1),
		reg("b", b, time.Minute, 1),
	}, time.Second)
	sweep.SetClock(f.s, c.now)
	f.rec.Queue(append(claimed(), claimed()...)...)

	if err := sweep.Handle(f.s)(context.Background(), t0); err != nil {
		t.Fatalf("handle: %v", err)
	}
	if want := []string{"pass a", "pass a", "pass b"}; !slices.Equal(order, want) {
		t.Errorf("passes = %q, want %q: a yields at its interval and b runs after it", order, want)
	}
	wantLocks(t, f.rec, try+"sweep.a", unlock+"sweep.a", try+"sweep.b", unlock+"sweep.b")
}

func TestHandle_ACycleWhoseLastPassReportsNoMoreIsNotLeftDue(t *testing.T) {
	c := &clock{t: t0}
	// The one pass outlasts the 1m interval but reports no more, so the
	// cycle ends rather than yields, and the reclaimer waits out its
	// interval.
	r := &reclaimer{block: func(context.Context) error {
		c.advance(2 * time.Minute)
		return nil
	}}
	f := newFixture(t, sqltest.LockingDialect{}, []sweep.Registration{reg("files", r, time.Minute, 1)}, time.Second)
	sweep.SetClock(f.s, c.now)
	handle := sweep.Handle(f.s)

	f.rec.Queue(claimed()...)
	if err := handle(context.Background(), t0); err != nil {
		t.Fatalf("handle: %v", err)
	}
	if err := handle(context.Background(), t0.Add(time.Second)); err != nil {
		t.Fatalf("second handle: %v", err)
	}
	if r.passes() != 1 {
		t.Errorf("passes = %d, want 1: a cycle that ran out of work is not left due", r.passes())
	}
	wantLocks(t, f.rec, try+"sweep.files", unlock+"sweep.files")
}

func TestHandle_AFailedPassThatReportsMoreIsFollowedByAnother(t *testing.T) {
	boom := errors.New("boom")
	var p passes
	r := &reclaimer{results: []result{{more: true, err: boom}, {more: false}}}
	f := newFixture(t, sqltest.LockingDialect{}, []sweep.Registration{reg("files", r, time.Minute, 1)}, time.Second, sweep.OnPass(p.hook))
	f.rec.Queue(claimed()...)

	if err := sweep.Handle(f.s)(context.Background(), t0); err != nil {
		t.Fatalf("handle returned the pass's error: %v", err)
	}
	if r.passes() != 2 {
		t.Fatalf("passes = %d, want the failed pass's more to run a second", r.passes())
	}
	got := p.all()
	if len(got) != 2 || !errors.Is(got[0].Err, boom) || !got[0].More || got[1].Err != nil || got[1].More {
		t.Errorf("reports = %+v, want the failed pass with its more, then the finished one", got)
	}
	wantLocks(t, f.rec, try+"sweep.files", unlock+"sweep.files")
}

func TestHandle_AFailedPassWithoutMoreEndsTheCycleAndTheNextIntervalRetries(t *testing.T) {
	boom := errors.New("boom")
	var p passes
	r := &reclaimer{results: []result{{more: false, err: boom}, {more: true}, {more: false}}}
	f := newFixture(t, sqltest.LockingDialect{}, []sweep.Registration{reg("files", r, time.Minute, 1)}, time.Second, sweep.OnPass(p.hook))
	f.rec.Queue(claimed()...)
	handle := sweep.Handle(f.s)

	if err := handle(context.Background(), t0); err != nil {
		t.Fatalf("handle returned the pass's error: %v", err)
	}
	if r.passes() != 1 {
		t.Fatalf("passes = %d, want the cycle to end after a pass without more", r.passes())
	}
	if got := p.all(); len(got) != 1 || !errors.Is(got[0].Err, boom) || got[0].More {
		t.Errorf("reports = %+v, want the failed pass", got)
	}
	wantLocks(t, f.rec, try+"sweep.files", unlock+"sweep.files")

	f.rec.Queue(claimed()...)
	if err := handle(context.Background(), t0.Add(time.Minute)); err != nil {
		t.Fatalf("second handle: %v", err)
	}
	if r.passes() != 3 {
		t.Errorf("passes = %d, want the next interval to run a cycle of two", r.passes())
	}
}

func TestHandle_TheDefaultHookLogsOnlyAFailedPass(t *testing.T) {
	boom := errors.New("boom")
	ok := &reclaimer{results: []result{{more: false}}}
	bad := &reclaimer{results: []result{{err: boom}}}
	f := newFixture(t, sqltest.LockingDialect{}, []sweep.Registration{
		reg("ok", ok, time.Minute, 1),
		reg("bad", bad, time.Minute, 1),
	}, time.Second)
	f.rec.Queue(append(claimed(), claimed()...)...)

	if err := sweep.Handle(f.s)(context.Background(), t0); err != nil {
		t.Fatalf("handle: %v", err)
	}
	out := f.logs.String()
	if strings.Count(out, "\n") != 1 || !strings.Contains(out, "level=WARN") ||
		!strings.Contains(out, `msg="sweep pass failed"`) || !strings.Contains(out, "reclaimer=bad") ||
		!strings.Contains(out, "error=boom") {
		t.Errorf("log = %q, want one warn line for bad's pass", out)
	}
}

func TestHandle_NudgeRunsOnlyTheNamedReclaimer(t *testing.T) {
	var order []string
	a := &reclaimer{name: "a", log: &order}
	b := &reclaimer{name: "b", log: &order}
	f := newFixture(t, sqltest.LockingDialect{}, []sweep.Registration{
		reg("a", a, time.Minute, 1),
		reg("b", b, time.Minute, 1),
	}, time.Second)
	handle := sweep.Handle(f.s)
	f.rec.Queue(append(claimed(), claimed()...)...)
	if err := handle(context.Background(), t0); err != nil {
		t.Fatalf("handle: %v", err)
	}

	f.s.Nudge("b")
	f.rec.Queue(claimed()...)
	if err := handle(context.Background(), t0.Add(time.Second)); err != nil {
		t.Fatalf("handle: %v", err)
	}
	if want := []string{"pass a", "pass b", "pass b"}; !slices.Equal(order, want) {
		t.Errorf("passes = %q, want %q", order, want)
	}

	// The nudge was consumed: a delivery before the interval runs nothing.
	if err := handle(context.Background(), t0.Add(2*time.Second)); err != nil {
		t.Fatalf("handle: %v", err)
	}
	if len(order) != 3 {
		t.Errorf("passes = %q, want no further pass", order)
	}
	wantLocks(t, f.rec,
		try+"sweep.a", unlock+"sweep.a", try+"sweep.b", unlock+"sweep.b",
		try+"sweep.b", unlock+"sweep.b")
}

func TestNudge_AnUnregisteredNamePanics(t *testing.T) {
	f := newFixture(t, sqltest.LockingDialect{}, []sweep.Registration{reg("a", &reclaimer{}, time.Minute, 1)}, time.Second)
	defer func() {
		if recover() == nil {
			t.Error("Nudge of an unregistered name did not panic")
		}
	}()
	f.s.Nudge("b")
}

func TestHandle_EachReclaimerRunsOnItsOwnInterval(t *testing.T) {
	var order []string
	fast := &reclaimer{name: "fast", log: &order}
	slow := &reclaimer{name: "slow", log: &order}
	f := newFixture(t, sqltest.LockingDialect{}, []sweep.Registration{
		reg("fast", fast, time.Minute, 1),
		reg("slow", slow, 3*time.Minute, 1),
	}, time.Second, sweep.Unlocked())
	handle := sweep.Handle(f.s)

	steps := []struct {
		at   time.Duration
		want []string
	}{
		{0, []string{"fast", "slow"}}, // every reclaimer is due at start
		{29 * time.Second, nil},       // under the interval less half a tick
		// A tick a little short of the interval, as a tick after a
		// nudge's delivery is, still counts.
		{time.Minute - time.Second, []string{"fast"}},
		{2 * time.Minute, []string{"fast"}},
		{3*time.Minute - time.Second, []string{"fast", "slow"}},
		{3 * time.Minute, nil},
	}
	for _, step := range steps {
		order = order[:0]
		if err := handle(context.Background(), t0.Add(step.at)); err != nil {
			t.Fatalf("handle at %v: %v", step.at, err)
		}
		var want []string
		for _, n := range step.want {
			want = append(want, "pass "+n)
		}
		if !slices.Equal(order, want) {
			t.Errorf("at +%v passes = %q, want %q", step.at, order, want)
		}
	}
}

func TestHandle_UnlockedTakesNoLock(t *testing.T) {
	r := &reclaimer{results: []result{{more: true}, {more: false}}}
	f := newFixture(t, sqltest.LockingDialect{}, []sweep.Registration{reg("files", r, time.Minute, 1)}, time.Second, sweep.Unlocked())

	if err := sweep.Handle(f.s)(context.Background(), t0); err != nil {
		t.Fatalf("handle: %v", err)
	}
	if r.passes() != 2 {
		t.Errorf("passes = %d, want 2", r.passes())
	}
	if calls := f.rec.Calls(); len(calls) != 0 {
		t.Errorf("calls = %+v, want none", calls)
	}
}

func TestHandle_AFailedClaimIsLoggedAndRetriedNextInterval(t *testing.T) {
	r := &reclaimer{}
	f := newFixture(t, sqltest.LockingDialect{}, []sweep.Registration{reg("files", r, time.Minute, 1)}, time.Second)
	f.rec.Queue(sqltest.Response{Err: errors.New("lock table full")})
	handle := sweep.Handle(f.s)

	if err := handle(context.Background(), t0); err != nil {
		t.Fatalf("handle: %v", err)
	}
	if r.passes() != 0 {
		t.Errorf("passes = %d, want none without the claim", r.passes())
	}
	if out := f.logs.String(); !strings.Contains(out, `msg="sweep claim failed"`) || !strings.Contains(out, "lock table full") {
		t.Errorf("log = %q, want the claim failure", out)
	}

	f.rec.Queue(claimed()...)
	if err := handle(context.Background(), t0.Add(time.Minute)); err != nil {
		t.Fatalf("handle: %v", err)
	}
	if r.passes() != 1 {
		t.Errorf("passes = %d, want the next interval to claim and pass", r.passes())
	}
}

func TestHandle_AFailedReleaseIsLogged(t *testing.T) {
	f := newFixture(t, sqltest.LockingDialect{}, []sweep.Registration{reg("files", &reclaimer{}, time.Minute, 1)}, time.Second)
	f.rec.Queue(flag(true), flag(false))

	if err := sweep.Handle(f.s)(context.Background(), t0); err != nil {
		t.Fatalf("handle: %v", err)
	}
	if out := f.logs.String(); !strings.Contains(out, `msg="sweep release failed"`) || !strings.Contains(out, "lock not held") {
		t.Errorf("log = %q, want the release failure", out)
	}
	wantLocks(t, f.rec, try+"sweep.files", unlock+"sweep.files")
	if st := f.pool.Stats(); st.OpenConnections != 0 {
		t.Errorf("pool = %d open; want the connection whose unlock failed discarded", st.OpenConnections)
	}
}

func TestHandle_AnEndedContextReturnsItsErrorAndReleasesTheClaim(t *testing.T) {
	var p passes
	ctx, cancel := context.WithCancel(context.Background())
	r := &reclaimer{results: []result{{more: true}}, block: func(context.Context) error {
		cancel()
		return nil
	}}
	next := &reclaimer{}
	f := newFixture(t, sqltest.LockingDialect{}, []sweep.Registration{
		reg("files", r, time.Minute, 1),
		reg("next", next, time.Minute, 1),
	}, time.Second, sweep.OnPass(p.hook))
	f.rec.Queue(claimed()...)

	if err := sweep.Handle(f.s)(ctx, t0); !errors.Is(err, context.Canceled) {
		t.Fatalf("handle = %v, want the context's error", err)
	}
	if r.passes() != 1 || next.passes() != 0 {
		t.Errorf("passes = %d and %d, want 1 and none", r.passes(), next.passes())
	}
	if got := p.all(); len(got) != 0 {
		t.Errorf("reports = %+v, want none for a cancelled pass", got)
	}
	wantLocks(t, f.rec, try+"sweep.files", unlock+"sweep.files")
}

// gateLog is a Gate that records each acquisition and release in a shared
// log, or fails with err.
type gateLog struct {
	log *[]string
	err error
}

func (g gateLog) Shared(context.Context) (func(), error) {
	if g.err != nil {
		return nil, g.err
	}
	*g.log = append(*g.log, "hold")
	return func() { *g.log = append(*g.log, "release") }, nil
}

func TestHandle_EachPassHoldsTheGateShared(t *testing.T) {
	var order []string
	r := &reclaimer{name: "files", log: &order, results: []result{{more: true}, {more: false}}}
	f := newFixture(t, sqltest.LockingDialect{}, []sweep.Registration{reg("files", r, time.Minute, 1)}, time.Second,
		sweep.Gated(gateLog{log: &order}))
	f.rec.Queue(claimed()...)

	if err := sweep.Handle(f.s)(context.Background(), t0); err != nil {
		t.Fatalf("handle: %v", err)
	}
	if want := []string{"hold", "pass files", "release", "hold", "pass files", "release"}; !slices.Equal(order, want) {
		t.Errorf("order = %q, want %q", order, want)
	}
}

func TestHandle_AGateFailureIsReportedAndRunsNoPass(t *testing.T) {
	refused := errors.New("gate broken")
	var p passes
	r := &reclaimer{}
	f := newFixture(t, sqltest.LockingDialect{}, []sweep.Registration{reg("files", r, time.Minute, 1)}, time.Second,
		sweep.Gated(gateLog{err: refused}), sweep.OnPass(p.hook))
	f.rec.Queue(claimed()...)

	if err := sweep.Handle(f.s)(context.Background(), t0); err != nil {
		t.Fatalf("handle: %v", err)
	}
	if r.passes() != 0 {
		t.Errorf("passes = %d, want none", r.passes())
	}
	if got := p.all(); len(got) != 1 || !errors.Is(got[0].Err, refused) {
		t.Errorf("reports = %+v, want the gate's failure", got)
	}
	wantLocks(t, f.rec, try+"sweep.files", unlock+"sweep.files")
}

// waiting is a Gate over a reactor.Gate that signals each wait it begins.
type waiting struct {
	g     *reactor.Gate
	began chan struct{}
}

func (w waiting) Shared(ctx context.Context) (func(), error) {
	w.began <- struct{}{}
	return w.g.Shared(ctx)
}

func TestHandle_AGateWaitItsContextEndsRunsNoPass(t *testing.T) {
	var p passes
	g := &reactor.Gate{}
	hold, err := g.Exclusive(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	defer hold()
	w := waiting{g: g, began: make(chan struct{}, 1)}
	r := &reclaimer{}
	f := newFixture(t, sqltest.LockingDialect{}, []sweep.Registration{reg("files", r, time.Minute, 1)}, time.Second,
		sweep.Gated(w), sweep.OnPass(p.hook))
	f.rec.Queue(claimed()...)

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- sweep.Handle(f.s)(ctx, t0) }()
	await(t, w.began, "the gate wait")
	cancel()
	if err := await(t, done, "the handler"); !errors.Is(err, context.Canceled) {
		t.Errorf("handle = %v, want the context's error", err)
	}
	if r.passes() != 0 || len(p.all()) != 0 {
		t.Errorf("passes = %d, reports = %+v; want none", r.passes(), p.all())
	}
	wantLocks(t, f.rec, try+"sweep.files", unlock+"sweep.files")
}

func TestSweeper_EveryReclaimerIsDueAtStart(t *testing.T) {
	ran := make(chan string, 2)
	mark := func(name string) *reclaimer {
		return &reclaimer{block: func(context.Context) error { ran <- name; return nil }}
	}
	f := newFixture(t, sqltest.LockingDialect{}, []sweep.Registration{
		reg("a", mark("a"), time.Hour, 1),
		reg("b", mark("b"), time.Hour, 1),
	}, time.Second, sweep.Unlocked())
	if f.s.Ready() {
		t.Error("Ready before Start")
	}
	if err := f.s.Start(context.Background()); err != nil {
		t.Fatal(err)
	}
	got := []string{await(t, ran, "a pass"), await(t, ran, "a pass")}
	if err := shutdown(f.s); err != nil {
		t.Errorf("Shutdown: %v", err)
	}
	if !slices.Equal(got, []string{"a", "b"}) {
		t.Errorf("ran = %q, want both at start, in order", got)
	}
	if f.s.Ready() {
		t.Error("Ready after Shutdown")
	}
}

func TestSweeper_AYieldingCycleNudgesTheReactor(t *testing.T) {
	// The interval is an hour, so a second delivery that comes at all comes
	// from the yield's nudge, not a tick.
	c := &clock{t: t0}
	ran := make(chan struct{}, 2)
	r := &reclaimer{results: []result{{more: true}}, block: func(context.Context) error {
		c.advance(time.Hour)
		ran <- struct{}{}
		return nil
	}}
	f := newFixture(t, sqltest.LockingDialect{}, []sweep.Registration{reg("files", r, time.Hour, 1)}, time.Second)
	sweep.SetClock(f.s, c.now)
	f.rec.Queue(append(claimed(), claimed()...)...)
	if err := f.s.Start(context.Background()); err != nil {
		t.Fatal(err)
	}
	await(t, ran, "the first pass")
	await(t, ran, "the pass the yield's delivery runs")
	if err := shutdown(f.s); err != nil {
		t.Errorf("Shutdown: %v", err)
	}
	wantLocks(t, f.rec,
		try+"sweep.files", unlock+"sweep.files", try+"sweep.files", unlock+"sweep.files")
}

func TestSweeper_TheDrainStartsNoFurtherPassAndLetsTheOneInFlightFinish(t *testing.T) {
	entered := make(chan struct{})
	var readyInPass bool
	var s *sweep.Sweeper
	// The pass in flight returns, reporting more, only once the drain has
	// begun: the cycle must not start another.
	r := &reclaimer{results: []result{{more: true}, {more: true}}, block: func(ctx context.Context) error {
		readyInPass = s.Ready()
		close(entered)
		<-reactor.Draining(ctx)
		return ctx.Err()
	}}
	next := &reclaimer{}
	f := newFixture(t, sqltest.LockingDialect{}, []sweep.Registration{
		reg("files", r, time.Hour, 1),
		reg("next", next, time.Hour, 1),
	}, time.Hour)
	s = f.s
	f.rec.Queue(claimed()...)
	if err := f.s.Start(context.Background()); err != nil {
		t.Fatal(err)
	}
	await(t, entered, "the pass")
	if err := shutdown(f.s); err != nil {
		t.Errorf("Shutdown: %v", err)
	}
	if !readyInPass {
		t.Error("not Ready while a pass ran")
	}
	if r.passes() != 1 || next.passes() != 0 {
		t.Errorf("passes = %d and %d, want 1 and none after the drain", r.passes(), next.passes())
	}
	wantLocks(t, f.rec, try+"sweep.files", unlock+"sweep.files")
}

func TestSweeper_TheDrainsCancellationIsTheReactorsError(t *testing.T) {
	var p passes
	entered := make(chan struct{})
	r := &reclaimer{block: func(ctx context.Context) error {
		close(entered)
		<-ctx.Done()
		return ctx.Err()
	}}
	// CancelWithin cancels the pass at half the shutdown timeout.
	f := newFixture(t, sqltest.LockingDialect{}, []sweep.Registration{reg("files", r, time.Hour, 1)}, 20*time.Millisecond,
		sweep.OnPass(p.hook))
	f.rec.Queue(claimed()...)
	if err := f.s.Start(context.Background()); err != nil {
		t.Fatal(err)
	}
	await(t, entered, "the pass")
	err := shutdown(f.s)
	if err == nil || !strings.Contains(err.Error(), "handlers cancelled") {
		t.Errorf("Shutdown = %v, want the handlers' cancellation", err)
	}
	if got := p.all(); len(got) != 0 {
		t.Errorf("reports = %+v, want none for the cancelled pass", got)
	}
	wantLocks(t, f.rec, try+"sweep.files", unlock+"sweep.files")
}

func TestSweeper_TheDrainEndsAGateWaitWithNoPass(t *testing.T) {
	var p passes
	g := &reactor.Gate{}
	hold, err := g.Exclusive(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	defer hold()
	w := waiting{g: g, began: make(chan struct{}, 1)}
	r := &reclaimer{}
	f := newFixture(t, sqltest.LockingDialect{}, []sweep.Registration{reg("files", r, time.Hour, 1)}, time.Hour,
		sweep.Gated(w), sweep.OnPass(p.hook))
	f.rec.Queue(claimed()...)
	if err := f.s.Start(context.Background()); err != nil {
		t.Fatal(err)
	}
	await(t, w.began, "the gate wait")
	if err := shutdown(f.s); err != nil {
		t.Errorf("Shutdown = %v, want nil: the drain, not a cancellation, ended the wait", err)
	}
	if r.passes() != 0 || len(p.all()) != 0 {
		t.Errorf("passes = %d, reports = %+v; want none", r.passes(), p.all())
	}
	wantLocks(t, f.rec, try+"sweep.files", unlock+"sweep.files")
}
