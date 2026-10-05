package postgres_test

import (
	"context"
	"errors"
	"maps"
	"net"
	"os"
	"os/user"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgproto3"

	"github.com/standards-lab/go-database"
	"github.com/standards-lab/go-database/postgres"
)

func finalizedConfig(t *testing.T) database.Config {
	t.Helper()
	cfg := database.Config{Name: "app", User: "app"}
	if err := cfg.Finalize(""); err != nil {
		t.Fatalf("finalize config: %v", err)
	}
	return cfg
}

// session is what a client sent the test server: its startup parameters
// and, answering a cleartext-password request, its password.
type session struct {
	params   map[string]string
	password string
	err      error
}

// serve answers every connection on ln as a server that asks for a
// cleartext password and then refuses it, reporting each session it sees.
func serve(t *testing.T, ln net.Listener) <-chan session {
	t.Helper()
	sessions := make(chan session, 4)
	t.Cleanup(func() { _ = ln.Close() })
	go func() {
		for {
			conn, err := ln.Accept()
			if err != nil {
				return
			}
			s := handshake(conn)
			select {
			case sessions <- s:
			default:
			}
		}
	}()
	return sessions
}

func handshake(conn net.Conn) session {
	defer func() { _ = conn.Close() }()
	_ = conn.SetDeadline(time.Now().Add(10 * time.Second))
	be := pgproto3.NewBackend(conn, conn)
	msg, err := be.ReceiveStartupMessage()
	if err != nil {
		return session{err: err}
	}
	startup, ok := msg.(*pgproto3.StartupMessage)
	if !ok {
		return session{err: errors.New("first message is not a startup message")}
	}
	s := session{params: maps.Clone(startup.Parameters)}
	be.Send(&pgproto3.AuthenticationCleartextPassword{})
	if err := be.Flush(); err != nil {
		return session{err: err}
	}
	if err := be.SetAuthType(pgproto3.AuthTypeCleartextPassword); err != nil {
		return session{err: err}
	}
	msg, err = be.Receive()
	if err != nil {
		return session{err: err}
	}
	if pw, ok := msg.(*pgproto3.PasswordMessage); ok {
		s.password = pw.Password
	}
	be.Send(&pgproto3.ErrorResponse{Severity: "FATAL", Code: "28P01", Message: "the test server refuses every login"})
	_ = be.Flush()
	return s
}

// connect starts a pool built from cfg against the test server and returns
// the session the server saw; the server's refusal fails Start.
func connect(t *testing.T, cfg database.Config, sessions <-chan session) session {
	t.Helper()
	if err := cfg.Finalize(""); err != nil {
		t.Fatalf("finalize config: %v", err)
	}
	db, err := postgres.New(cfg)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	t.Cleanup(func() { _ = db.Shutdown(context.Background()) })
	if err := db.Start(context.Background()); !errors.Is(err, database.ErrConnectionFailed) {
		t.Fatalf("Start = %v, want the refused login as ErrConnectionFailed", err)
	}
	select {
	case s := <-sessions:
		if s.err != nil {
			t.Fatalf("test server: %v", s.err)
		}
		return s
	case <-time.After(10 * time.Second):
		t.Fatal("no connection reached the test server")
		return session{}
	}
}

// noTLS keeps the session in cleartext, so the test server reads it.
var noTLS = map[string]string{"sslmode": "disable"}

// socketDir makes a short directory for a Unix socket, whose path is
// limited to about a hundred bytes.
func socketDir(t *testing.T) string {
	t.Helper()
	dir, err := os.MkdirTemp("", "pg")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(dir) })
	return dir
}

// New only builds the pool: a server listening at the configured address
// has no connection waiting once New returns, and readiness stays false
// until Start.
func TestNew_ConstructsWithoutIO(t *testing.T) {
	ln, err := net.ListenTCP("tcp", &net.TCPAddr{IP: net.IPv4(127, 0, 0, 1)})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = ln.Close() })
	cfg := database.Config{Name: "app", User: "app", Host: "127.0.0.1", Port: new(ln.Addr().(*net.TCPAddr).Port)}
	if err := cfg.Finalize(""); err != nil {
		t.Fatalf("finalize config: %v", err)
	}

	db, err := postgres.New(cfg)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	t.Cleanup(func() { _ = db.Conn().Close() })

	// A dial completes its handshake into the listener's backlog before it
	// returns, so an Accept that times out finds that New never dialed.
	_ = ln.SetDeadline(time.Now().Add(50 * time.Millisecond))
	if conn, err := ln.Accept(); err == nil {
		_ = conn.Close()
		t.Error("New connected to the server")
	}
	if db.Ready() {
		t.Error("Ready() = true on a freshly constructed database, want false")
	}
}

// A TCP host and port reach the server, carrying the user and database,
// and the password arrives intact whatever characters it holds, since it
// never passes through the connection URL.
func TestNew_ConnectsOverTCPWithThePassword(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	sessions := serve(t, ln)
	password := `sp ace:sl/ash@at?q&amp'quote%20`
	cfg := database.Config{
		Name: "app", User: "app", Password: password,
		Host: "127.0.0.1", Port: new(ln.Addr().(*net.TCPAddr).Port), Options: noTLS,
	}

	s := connect(t, cfg, sessions)
	if s.params["user"] != "app" || s.params["database"] != "app" {
		t.Errorf("startup parameters = %v, want user and database app", s.params)
	}
	if s.password != password {
		t.Errorf("password = %q, want %q", s.password, password)
	}
}

// A host that is a path is a Unix-socket directory, and the port, 5432
// unless set, names the socket file in it.
func TestNew_UnixSocketHost(t *testing.T) {
	cases := []struct {
		name string
		port *int
		file string
	}{
		{"default port", nil, ".s.PGSQL.5432"},
		{"set port", new(5433), ".s.PGSQL.5433"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			dir := socketDir(t)
			ln, err := net.Listen("unix", filepath.Join(dir, tc.file))
			if err != nil {
				t.Fatal(err)
			}
			sessions := serve(t, ln)
			cfg := database.Config{Name: "app", User: "app", Host: dir, Port: tc.port, Options: noTLS}

			if s := connect(t, cfg, sessions); s.params["database"] != "app" {
				t.Errorf("startup parameters = %v, want database app", s.params)
			}
		})
	}
}

// Whether a user is required varies by provider and auth mode, so the
// config leaves it optional and the connection carries the OS user.
func TestNew_EmptyUserFallsBackToTheOSUser(t *testing.T) {
	osUser, err := user.Current()
	if err != nil {
		t.Skipf("no OS user to fall back to: %v", err)
	}
	t.Setenv("PGUSER", "")
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	sessions := serve(t, ln)
	cfg := database.Config{Name: "app", Host: "127.0.0.1", Port: new(ln.Addr().(*net.TCPAddr).Port), Options: noTLS}

	if s := connect(t, cfg, sessions); s.params["user"] != osUser.Username {
		t.Errorf("user = %q, want the OS user %q", s.params["user"], osUser.Username)
	}
}

// Finalized is the one check: a config missing any field New reads panics,
// though its ConnTimeout is set.
func TestNew_PanicsOnPartlyFinalizedConfig(t *testing.T) {
	cfg := finalizedConfig(t)
	cfg.MaxOpenConns = nil
	defer func() {
		msg, ok := recover().(string)
		if !ok || !strings.HasPrefix(msg, "postgres:") {
			t.Fatalf("panic = %q, want the provider's finalized-config panic", msg)
		}
	}()
	_, _ = postgres.New(cfg)
}

func TestNew_RejectsReservedOptions(t *testing.T) {
	reserved := []string{
		"host", "port", "user", "password", "dbname", "database", "connect_timeout",
	}
	for _, key := range reserved {
		t.Run(key, func(t *testing.T) {
			cfg := database.Config{
				Name:    "app",
				Options: map[string]string{key: "x"},
			}
			if err := cfg.Finalize(""); err != nil {
				t.Fatalf("finalize config: %v", err)
			}

			_, err := postgres.New(cfg)
			if err == nil {
				t.Fatalf("New accepted reserved option %q", key)
			}
			if !strings.Contains(err.Error(), "conflicts with a connection field") {
				t.Errorf("error = %v, want the reserved-option message", err)
			}
		})
	}
}

func TestNew_RejectsUnparseableOptions(t *testing.T) {
	cfg := database.Config{
		Name:    "app",
		Options: map[string]string{"sslmode": "bogus"},
	}
	if err := cfg.Finalize(""); err != nil {
		t.Fatalf("finalize config: %v", err)
	}

	_, err := postgres.New(cfg)
	if err == nil {
		t.Fatal("New accepted an invalid sslmode")
	}
	if !strings.Contains(err.Error(), "parse connection config") {
		t.Errorf("error = %v, want the parse wrap", err)
	}
}

func TestNew_PanicsOnUnfinalizedConfig(t *testing.T) {
	defer func() {
		r := recover()
		if r == nil {
			t.Fatal("no panic on an unfinalized config")
		}
		msg, ok := r.(string)
		if !ok || !strings.Contains(msg, "Config not finalized") {
			t.Errorf("panic = %v, want the finalize guidance", r)
		}
	}()
	_, _ = postgres.New(database.Config{Name: "app"})
}
