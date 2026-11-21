package postgres_test

import (
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"maps"
	"net"
	"os"
	"os/user"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgproto3"
	"github.com/jackc/pgx/v5/pgtype"

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

type session struct {
	params   map[string]string
	password string
	err      error
}

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

var noTLS = map[string]string{"sslmode": "disable"}

func socketDir(t *testing.T) string {
	t.Helper()
	dir, err := os.MkdirTemp("", "pg")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(dir) })
	return dir
}

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

	_ = ln.SetDeadline(time.Now().Add(50 * time.Millisecond))
	if conn, err := ln.Accept(); err == nil {
		_ = conn.Close()
		t.Error("New connected to the server")
	}
	if db.Ready() {
		t.Error("Ready() = true on a freshly constructed database, want false")
	}
}

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

func serveTimestamptz(t *testing.T, ln net.Listener, at time.Time) <-chan error {
	t.Helper()
	errs := make(chan error, 4)
	t.Cleanup(func() { _ = ln.Close() })
	go func() {
		for {
			conn, err := ln.Accept()
			if err != nil {
				return
			}
			go func() {
				if err := answerTimestamptz(conn, at); err != nil {
					select {
					case errs <- err:
					default:
					}
				}
			}()
		}
	}()
	return errs
}

func answerTimestamptz(conn net.Conn, at time.Time) error {
	defer func() { _ = conn.Close() }()
	_ = conn.SetDeadline(time.Now().Add(10 * time.Second))
	be := pgproto3.NewBackend(conn, conn)
	if _, err := be.ReceiveStartupMessage(); err != nil {
		return err
	}
	be.Send(&pgproto3.AuthenticationOk{})
	be.Send(&pgproto3.ParameterStatus{Name: "standard_conforming_strings", Value: "on"})
	be.Send(&pgproto3.ParameterStatus{Name: "client_encoding", Value: "UTF8"})
	be.Send(&pgproto3.BackendKeyData{ProcessID: 1, SecretKey: []byte{0, 0, 0, 1}})
	be.Send(&pgproto3.ReadyForQuery{TxStatus: 'I'})
	if err := be.Flush(); err != nil {
		return err
	}

	column := func(format int16) *pgproto3.RowDescription {
		return &pgproto3.RowDescription{Fields: []pgproto3.FieldDescription{{
			Name: []byte("at"), DataTypeOID: pgtype.TimestamptzOID, DataTypeSize: 8,
			TypeModifier: -1, Format: format,
		}}}
	}
	value := func(format int16) [][]byte {
		if format == pgtype.BinaryFormatCode {
			y2k := time.Date(2000, 1, 1, 0, 0, 0, 0, time.UTC)
			return [][]byte{binary.BigEndian.AppendUint64(nil, uint64(at.Sub(y2k).Microseconds()))}
		}
		return [][]byte{[]byte(at.UTC().Format("2006-01-02 15:04:05.999999") + "+00")}
	}
	var format int16
	for {
		msg, err := be.Receive()
		if err != nil {
			return err
		}
		switch msg := msg.(type) {
		case *pgproto3.Query:
			if strings.HasPrefix(strings.ToUpper(msg.String), "SELECT") {
				be.Send(column(pgtype.TextFormatCode))
				be.Send(&pgproto3.DataRow{Values: value(pgtype.TextFormatCode)})
				be.Send(&pgproto3.CommandComplete{CommandTag: []byte("SELECT 1")})
			} else {
				be.Send(&pgproto3.EmptyQueryResponse{})
			}
			be.Send(&pgproto3.ReadyForQuery{TxStatus: 'I'})
		case *pgproto3.Parse:
			be.Send(&pgproto3.ParseComplete{})
		case *pgproto3.Describe:
			if msg.ObjectType == 'S' {
				be.Send(&pgproto3.ParameterDescription{})
				be.Send(column(pgtype.TextFormatCode))
			} else {
				be.Send(column(format))
			}
		case *pgproto3.Bind:
			format = pgtype.TextFormatCode
			if len(msg.ResultFormatCodes) > 0 {
				format = msg.ResultFormatCodes[0]
			}
			be.Send(&pgproto3.BindComplete{})
		case *pgproto3.Execute:
			be.Send(&pgproto3.DataRow{Values: value(format)})
			be.Send(&pgproto3.CommandComplete{CommandTag: []byte("SELECT 1")})
		case *pgproto3.Close:
			be.Send(&pgproto3.CloseComplete{})
		case *pgproto3.Sync:
			be.Send(&pgproto3.ReadyForQuery{TxStatus: 'I'})
		case *pgproto3.Terminate:
			return nil
		default:
			return fmt.Errorf("unexpected message %T", msg)
		}
		if err := be.Flush(); err != nil {
			return err
		}
	}
}

func TestNew_ReadsTimestamptzInUTC(t *testing.T) {
	want := time.Date(2026, 7, 1, 12, 30, 15, 123456000, time.UTC)
	cases := []struct {
		name    string
		options map[string]string
	}{
		{"text, simple protocol", map[string]string{"sslmode": "disable", "default_query_exec_mode": "simple_protocol"}},
		{"binary, extended protocol", noTLS},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ln, err := net.Listen("tcp", "127.0.0.1:0")
			if err != nil {
				t.Fatal(err)
			}
			errs := serveTimestamptz(t, ln, want)
			cfg := database.Config{
				Name: "app", User: "app",
				Host: "127.0.0.1", Port: new(ln.Addr().(*net.TCPAddr).Port), Options: tc.options,
			}
			if err := cfg.Finalize(""); err != nil {
				t.Fatalf("finalize config: %v", err)
			}
			db, err := postgres.New(cfg)
			if err != nil {
				t.Fatalf("New: %v", err)
			}
			t.Cleanup(func() { _ = db.Shutdown(context.Background()) })
			if err := db.Start(context.Background()); err != nil {
				t.Fatalf("Start: %v", err)
			}

			var got time.Time
			if err := db.Conn().QueryRowContext(context.Background(), "SELECT at FROM t").Scan(&got); err != nil {
				select {
				case serr := <-errs:
					t.Fatalf("query: %v (test server: %v)", err, serr)
				default:
					t.Fatalf("query: %v", err)
				}
			}
			if got != want {
				t.Errorf("timestamptz = %v, want %v", got, want)
			}
			if got.Location() != time.UTC {
				t.Errorf("Location() = %v, want UTC", got.Location())
			}
		})
	}
}
