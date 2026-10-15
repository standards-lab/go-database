package sweep

import (
	"context"
	"database/sql/driver"
	"errors"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/standards-lab/go-core/reactor"
	"github.com/standards-lab/sqlate"
)

// Reclaimer is one library's bounded reclaim: a pass removes at most budget
// units of its garbage and reports whether more may remain. A pass is
// idempotent and finds its work in the database, so a pass that fails is
// retried by a later cycle. A pass that reports more is followed by
// another in the same cycle, whether or not it failed, until the cycle has
// run for its interval, when the reclaimer yields to the others due and
// stays due. A reclaimer therefore reports more only while a further pass
// can make progress: one that reports more on every pass spends each of
// its cycles, an interval at a time, on passes that remove nothing. blobfs's
// sweep and go-messaging's retention satisfy it structurally; this package
// imports neither.
type Reclaimer interface {
	Pass(ctx context.Context, budget int) (more bool, err error)
}

// Policy is a reclaimer's schedule: the least time between the starts of
// two of its cycles, and the budget each of its passes is given.
type Policy struct {
	// Interval is how long after a cycle the reclaimer is due again; it
	// must be positive.
	Interval time.Duration
	// Budget is the bound handed to each Pass; it must be positive.
	Budget int
}

// Registration is one reclaimer in the list [New] builds a Sweeper from:
// its name, unique in the list, which [Sweeper.Nudge] takes and the
// advisory lock "sweep.<name>" is named for; the reclaimer; and its policy.
type Registration struct {
	Name      string
	Reclaimer Reclaimer
	Policy    Policy
}

// ErrNoTryLocker reports a dialect without the sqlate.TryLocker capability,
// which a Sweeper needs to claim each reclaimer's lock without waiting;
// [Unlocked] opts out.
var ErrNoTryLocker = errors.New("sweep: dialect has no try-lock capability")

// Gate is a quiesce gate each pass holds shared, so an operation that holds
// it exclusively runs with the passes paused: go-core's [reactor.Gate] is
// one. Shared waits until the gate admits a shared holder and returns its
// release, or fails, holding nothing; a wait ctx ends returns ctx's error.
type Gate interface {
	Shared(ctx context.Context) (release func(), err error)
}

var _ Gate = (*reactor.Gate)(nil)

// Pass is one reclaimer pass, reported to the [Hook] once the pass returns.
type Pass struct {
	// Name is the reclaimer's registered name.
	Name string
	// More is what the pass reported: whether more may remain.
	More bool
	// Err is the pass's error, or, under [Gated], the gate's failure other
	// than its wait ending, in which case no pass ran.
	Err error
	// Duration is how long the pass took, its wait on the gate included.
	Duration time.Duration
}

// Hook receives each [Pass], with the context it ran under. It must not
// block for long: the sweeper waits on it before the next pass.
type Hook func(ctx context.Context, p Pass)

// Option configures a [Sweeper].
type Option func(*options)

type options struct {
	unlocked bool
	gate     Gate
	hook     Hook
}

// Unlocked lets the sweeper run on a dialect without the sqlate.TryLocker
// capability, or with it, without claiming the locks. Its intended shape is
// a single process, or a consumer that serializes the sweep across
// processes some other way; concurrent sweepers without one each run every
// reclaimer, which their idempotent passes tolerate but no budget bounds.
func Unlocked() Option {
	return func(o *options) { o.unlocked = true }
}

// Gated sets the gate each pass holds shared, from before the pass begins
// until it returns. Without it, or with a nil gate, passes hold none. A
// wait on g that the reactor's drain or the handler's context ends runs no
// pass and reports nothing. Any other failure of g is reported as the
// pass's error, with no pass run, and ends the cycle.
func Gated(g Gate) Option {
	return func(o *options) {
		if g != nil {
			o.gate = g
		}
	}
}

// OnPass sets the hook each [Pass] is reported to. Without it, or with a
// nil hook, the sweeper logs each pass that returned an error at warn on
// its logger and nothing else.
func OnPass(h Hook) Option {
	return func(o *options) {
		if h != nil {
			o.hook = h
		}
	}
}

// Sweeper runs a fixed list of reclaimers as one go-core reactor. Each
// delivery runs every reclaimer that is due, one at a time in the list's
// order: a reclaimer is due when its interval has passed since its last
// cycle, when [Sweeper.Nudge] named it since its last cycle, and at start.
//
// The reactor's source is a [reactor.Wake] whose interval is the smallest
// policy interval, so each delivery is a tick of that interval or a nudge.
// A reclaimer counts as due once its interval, less half a tick, has passed
// since the delivery that last ran it, which absorbs the drift between a
// nudge's time and the ticks; a reclaimer whose interval is not a multiple
// of the smallest therefore runs on the first tick at or after its
// interval, up to one tick late.
//
// The Sweeper is a lifecycle Starter, Stopper, ReadinessChecker, and
// Monitored through its reactor's methods; it keeps no backlog state, and
// its readiness is the reactor's alone.
type Sweeper struct {
	db     *sqlate.DB
	locker sqlate.TryLocker // nil when unlocked
	regs   []Registration
	names  map[string]bool
	tick   time.Duration
	opts   options
	logger *slog.Logger
	wake   *reactor.Waker
	r      *reactor.Reactor[time.Time]
	now    func() time.Time // the clock that bounds a cycle; time.Now outside tests

	// mu guards the due state: each reclaimer's last cycle, by its
	// delivery's time, and the reclaimers nudged since their last cycle.
	mu     sync.Mutex
	last   map[string]time.Time
	nudged map[string]bool
}

// New builds a Sweeper over db from the reclaimers in regs, logging to
// logger. shutdown is the process's drain timeout: the reactor cancels a
// pass still running at half of it ([reactor.CancelWithin]). New refuses,
// with an error and before any I/O, an empty list; a registration with an
// empty name, a name another uses, a nil reclaimer, or a non-positive
// interval or budget; a non-positive shutdown; and a dialect without
// sqlate.TryLocker, as [ErrNoTryLocker], unless [Unlocked]. A nil db or
// logger is a wiring defect and panics.
//
// Every reclaimer is due at start: New nudges the source, which delivers
// as soon as the reactor starts.
func New(db *sqlate.DB, regs []Registration, shutdown time.Duration, logger *slog.Logger, opts ...Option) (*Sweeper, error) {
	if db == nil {
		panic("sweep: nil db")
	}
	if logger == nil {
		panic("sweep: nil logger")
	}
	if len(regs) == 0 {
		return nil, errors.New("sweep: no reclaimers")
	}
	if shutdown <= 0 {
		return nil, fmt.Errorf("sweep: shutdown must be positive, got %v", shutdown)
	}
	s := &Sweeper{
		db:     db,
		regs:   append([]Registration(nil), regs...),
		names:  make(map[string]bool, len(regs)),
		logger: logger,
		now:    time.Now,
		last:   make(map[string]time.Time, len(regs)),
		nudged: make(map[string]bool, len(regs)),
	}
	s.opts.hook = logPasses(logger)
	for _, opt := range opts {
		opt(&s.opts)
	}
	for _, reg := range s.regs {
		switch {
		case reg.Name == "":
			return nil, errors.New("sweep: a reclaimer has no name")
		case s.names[reg.Name]:
			return nil, fmt.Errorf("sweep: reclaimer %q is registered twice", reg.Name)
		case reg.Reclaimer == nil:
			return nil, fmt.Errorf("sweep: reclaimer %q is nil", reg.Name)
		case reg.Policy.Interval <= 0:
			return nil, fmt.Errorf("sweep: reclaimer %q: interval must be positive, got %v", reg.Name, reg.Policy.Interval)
		case reg.Policy.Budget <= 0:
			return nil, fmt.Errorf("sweep: reclaimer %q: budget must be positive, got %d", reg.Name, reg.Policy.Budget)
		}
		s.names[reg.Name] = true
		if s.tick == 0 || reg.Policy.Interval < s.tick {
			s.tick = reg.Policy.Interval
		}
	}
	if !s.opts.unlocked {
		locker, ok := db.Dialect().(sqlate.TryLocker)
		if !ok {
			return nil, ErrNoTryLocker
		}
		s.locker = locker
	}
	s.wake = reactor.Wake(s.tick)
	s.wake.Nudge()
	s.r = reactor.New(s.wake, s.handle, reactor.CancelWithin(shutdown))
	return s, nil
}

// Nudge marks the reclaimer registered as name due, and asks the reactor
// for a delivery as soon as it is free; nudges before a delivery coalesce
// into it, and a nudge while the reclaimer's cycle runs earns it one more
// cycle after. It never blocks. A name the Sweeper was not built with is a
// defect in the caller and panics.
func (s *Sweeper) Nudge(name string) {
	if !s.names[name] {
		panic(fmt.Sprintf("sweep: Nudge of unregistered reclaimer %q", name))
	}
	s.nudge(name)
}

// nudge is [Sweeper.Nudge] for a name known to be registered: it marks
// name due and asks the reactor for a delivery.
func (s *Sweeper) nudge(name string) {
	s.mu.Lock()
	s.nudged[name] = true
	s.mu.Unlock()
	s.wake.Nudge()
}

// Start starts the reactor; see [reactor.Reactor.Start].
func (s *Sweeper) Start(ctx context.Context) error { return s.r.Start(ctx) }

// Shutdown drains the reactor: no further pass starts, the pass in flight
// finishes, and one still running when the CancelWithin bound passes is
// cancelled. See [reactor.Reactor.Shutdown].
func (s *Sweeper) Shutdown(ctx context.Context) error { return s.r.Shutdown(ctx) }

// Ready reports the reactor's readiness; see [reactor.Reactor.Ready].
func (s *Sweeper) Ready() bool { return s.r.Ready() }

// Err yields the reactor's failure while running; see [reactor.Reactor.Err].
func (s *Sweeper) Err() <-chan error { return s.r.Err() }

// handle is the reactor's Func: it runs the cycle of each reclaimer due at
// now, in the list's order, until the drain begins. A cycle that yields
// leaves its reclaimer due and the waker nudged, so the reclaimers after it
// run in this delivery and every due reclaimer runs again, in order, in the
// next: each gets at most one interval in turn. It returns an error
// only when ctx ends, which is the drain's cancellation, so that error is
// the reactor's; every other failure is reported and left to the next
// cycle, and the reactor continues.
func (s *Sweeper) handle(ctx context.Context, now time.Time) error {
	drain := reactor.Draining(ctx)
	for _, reg := range s.regs {
		if err := ctx.Err(); err != nil {
			return err
		}
		if closed(drain) {
			return nil
		}
		if !s.due(reg, now) {
			continue
		}
		if err := s.cycle(ctx, drain, reg); err != nil {
			return err
		}
	}
	return nil
}

// due reports whether reg is due at now, and if so consumes its nudge and
// records now as its last cycle.
func (s *Sweeper) due(reg Registration, now time.Time) bool {
	s.mu.Lock()
	nudged := s.nudged[reg.Name]
	last, ran := s.last[reg.Name]
	if !nudged && ran && now.Sub(last) < reg.Policy.Interval-s.tick/2 {
		s.mu.Unlock()
		return false
	}
	delete(s.nudged, reg.Name)
	s.last[reg.Name] = now
	s.mu.Unlock()
	return true
}

// cycle claims reg's lock and runs its passes while one reports more,
// whatever its error, which is reported and never ends the cycle, for at
// most reg's interval from the claim. A cycle still reporting more once
// the interval has passed yields: it marks reg nudged, as [Sweeper.Nudge]
// does, so reg stays due and the delivery that nudge earns resumes its
// backlog without waiting out an interval. A lock held elsewhere runs no
// pass and reports nothing; a failure to claim it is logged at warn and
// leaves the work to the next cycle. The claim is released when the cycle
// ends, on every path. It returns ctx's error once ctx ends, and nil
// otherwise.
func (s *Sweeper) cycle(ctx context.Context, drain <-chan struct{}, reg Registration) error {
	if s.locker != nil {
		release, claimed, err := s.claim(ctx, reg.Name)
		if err != nil || !claimed {
			return err
		}
		defer release()
	}
	start := s.now()
	for {
		if err := ctx.Err(); err != nil {
			return err
		}
		if closed(drain) {
			return nil
		}
		p, ran := s.pass(ctx, drain, reg)
		if err := ctx.Err(); err != nil {
			// The drain's cancellation ended the pass: its outcome is the
			// cancellation's, which the reactor reports, not the pass's.
			return err
		}
		if !ran {
			return nil
		}
		s.opts.hook(ctx, p)
		// more alone decides: a failed pass that reports more is followed
		// by another, since a reclaimer's passes are idempotent. A gate
		// failure ran no pass, so it reports no more and ends the cycle.
		if !p.More {
			return nil
		}
		if s.now().Sub(start) >= reg.Policy.Interval {
			// Yield: reg stays due, and the delivery the nudge earns
			// resumes its backlog after the reclaimers behind it.
			s.nudge(reg.Name)
			return nil
		}
	}
}

// claim pins a connection and tries the lock "sweep.<name>" on it. It
// returns the release, which unlocks and then closes the connection, and
// whether the lock was taken; a lock not taken leaves nothing held. Its
// error is ctx's alone: any other failure is logged and reported as not
// claimed.
func (s *Sweeper) claim(ctx context.Context, name string) (release func(), claimed bool, err error) {
	lock := "sweep." + name
	conn, err := s.db.Conn(ctx)
	if err != nil {
		return nil, false, s.claimFailed(ctx, name, err)
	}
	ok, err := s.locker.TryLock(ctx, conn, lock)
	if err != nil || !ok {
		_ = conn.Close()
		if err != nil {
			return nil, false, s.claimFailed(ctx, name, s.db.MapError(err))
		}
		return nil, false, nil
	}
	return func() {
		// The unlock outlives ctx, so a cycle the drain cancelled still
		// releases its claim before the connection returns to the pool.
		if err := s.locker.Unlock(context.WithoutCancel(ctx), conn, lock); err != nil {
			s.logger.WarnContext(ctx, "sweep release failed", "reclaimer", name, "error", s.db.MapError(err))
			// A session whose unlock failed may still hold the lock;
			// discard it rather than return it to the pool, so the
			// session's end releases it.
			_ = conn.Raw(func(any) error { return driver.ErrBadConn })
		}
		_ = conn.Close()
	}, true, nil
}

// claimFailed logs a failure to claim name's lock, unless ctx ended, in
// which case it returns ctx's error.
func (s *Sweeper) claimFailed(ctx context.Context, name string, err error) error {
	if cerr := ctx.Err(); cerr != nil {
		return cerr
	}
	s.logger.WarnContext(ctx, "sweep claim failed", "reclaimer", name, "error", err)
	return nil
}

// pass runs one pass of reg, holding the gate shared when the sweeper has
// one. ran is false when the wait on the gate was ended by ctx or the
// drain, which runs no pass and reports nothing.
func (s *Sweeper) pass(ctx context.Context, drain <-chan struct{}, reg Registration) (p Pass, ran bool) {
	p.Name = reg.Name
	start := time.Now()
	if s.opts.gate != nil {
		release, ended, err := s.share(ctx, drain)
		if ended {
			return p, false
		}
		if err != nil {
			p.Err, p.Duration = err, time.Since(start)
			return p, true
		}
		defer release()
	}
	p.More, p.Err = reg.Reclaimer.Pass(ctx, reg.Policy.Budget)
	p.Duration = time.Since(start)
	return p, true
}

// share holds the gate shared, waiting until ctx ends or the drain begins.
// ended reports a wait either ended, which holds nothing.
func (s *Sweeper) share(ctx context.Context, drain <-chan struct{}) (release func(), ended bool, err error) {
	wait, cancel := context.WithCancel(ctx)
	defer cancel()
	if drain != nil {
		go func() {
			select {
			case <-drain:
				cancel()
			case <-wait.Done():
			}
		}()
	}
	release, err = s.opts.gate.Shared(wait)
	if err != nil {
		if wait.Err() != nil && errors.Is(err, wait.Err()) {
			return nil, true, nil
		}
		return nil, false, fmt.Errorf("sweep: gate: %w", err)
	}
	return release, false, nil
}

// closed reports whether ch is closed; a nil ch never is.
func closed(ch <-chan struct{}) bool {
	select {
	case <-ch:
		return true
	default:
		return false
	}
}

// logPasses returns the default hook: a pass that returned an error logged
// at warn on logger as "sweep pass failed", with the reclaimer, whether it
// reported more, its duration, and the error; any other pass not at all.
func logPasses(logger *slog.Logger) Hook {
	return func(ctx context.Context, p Pass) {
		if p.Err == nil {
			return
		}
		logger.WarnContext(ctx, "sweep pass failed",
			"reclaimer", p.Name, "more", p.More, "duration", p.Duration, "error", p.Err)
	}
}
