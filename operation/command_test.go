package operation_test

import (
	"errors"
	"strings"
	"testing"

	"github.com/standards-lab/go-database/ast"
	"github.com/standards-lab/go-database/operation"
)

func organizationInsertion() operation.Insertion {
	return operation.Insertion{
		Into: "organization",
		Values: []ast.Assignment{
			{Column: "parent_id", Value: nil},
			{Column: "code", Value: "acme"},
			{Column: "name", Value: "Acme"},
		},
		Identity: operation.Field{Name: "id", Expr: ast.Col("id")},
		Version:  operation.Field{Name: "version", Expr: ast.Col("version")},
	}
}

func guardedUpdate() operation.GuardedUpdate {
	return operation.GuardedUpdate{
		Table: "organization",
		Key:   operation.Field{Name: "id", Expr: ast.Col("id")},
		ID:    "x",
		Guard: operation.Guard{Column: "version", Version: 3},
		Set:   []ast.Assignment{{Column: "name", Value: "Acme"}},
	}
}

func TestInsertionSQL(t *testing.T) {
	stmt, err := organizationInsertion().SQL(returningStub{})
	if err != nil {
		t.Fatalf("SQL() error: %v", err)
	}
	wantSQL(t, stmt.Text,
		"INSERT INTO organization (parent_id, code, name) VALUES ($1, $2, $3) "+
			"RETURNING id AS id, version AS version")
	wantArgs(t, stmt.Args, nil, "acme", "Acme")
}

func TestInsertionRequiresReturningCapability(t *testing.T) {
	_, err := organizationInsertion().SQL(stub{})
	var unsupported *ast.UnsupportedFeatureError
	if !errors.As(err, &unsupported) || unsupported.Feature != ast.FeatureReturning {
		t.Errorf("error = %v, want UnsupportedFeatureError for %s", err, ast.FeatureReturning)
	}
}

func TestInsertionValidation(t *testing.T) {
	base := organizationInsertion()
	noTable := base
	noTable.Into = ""
	noValues := base
	noValues.Values = nil
	noIdentity := base
	noIdentity.Identity = operation.Field{}
	noVersion := base
	noVersion.Version = operation.Field{}

	cases := map[string]struct {
		i    operation.Insertion
		want string
	}{
		"missing table":    {noTable, "requires a table"},
		"missing values":   {noValues, "requires values"},
		"missing identity": {noIdentity, "requires an identity field"},
		"missing version":  {noVersion, "requires a version field"},
	}
	for name, c := range cases {
		_, err := c.i.SQL(returningStub{})
		if !errors.Is(err, ast.ErrInvalidStatement) || !strings.Contains(err.Error(), c.want) {
			t.Errorf("%s: error = %v, want ErrInvalidStatement containing %q", name, err, c.want)
		}
	}
}

func TestGuardedUpdateSQL(t *testing.T) {
	g, err := guardedUpdate().SQL(stub{})
	if err != nil {
		t.Fatalf("SQL() error: %v", err)
	}
	wantSQL(t, g.Command.Text,
		"UPDATE organization SET name = $1, version = version + 1 WHERE (id = $2 AND version = $3)")
	wantArgs(t, g.Command.Args, "Acme", "x", int64(3))
	wantSQL(t, g.Check.Text, "SELECT version FROM organization WHERE id = $1")
	wantArgs(t, g.Check.Args, "x")
}

func TestGuardedUpdateRejectsGuardAssignment(t *testing.T) {
	u := guardedUpdate()
	u.Set = append(u.Set, ast.Assignment{Column: "version", Value: 9})
	_, err := u.SQL(stub{})
	if !errors.Is(err, ast.ErrInvalidStatement) || !strings.Contains(err.Error(), "the guard manages") {
		t.Errorf("error = %v, want ErrInvalidStatement about the guarded column", err)
	}
}

func TestGuardedUpdateValidation(t *testing.T) {
	base := guardedUpdate()
	noSet := base
	noSet.Set = nil
	noTable := base
	noTable.Table = ""
	noKey := base
	noKey.Key = operation.Field{}
	noColumn := base
	noColumn.Guard = operation.Guard{Version: 3}

	cases := map[string]struct {
		u    operation.GuardedUpdate
		want string
	}{
		"missing SET":    {noSet, "requires a SET list"},
		"missing table":  {noTable, "requires a table"},
		"missing key":    {noKey, "requires a key field"},
		"missing column": {noColumn, "requires a version column"},
	}
	for name, c := range cases {
		_, err := c.u.SQL(stub{})
		if !errors.Is(err, ast.ErrInvalidStatement) || !strings.Contains(err.Error(), c.want) {
			t.Errorf("%s: error = %v, want ErrInvalidStatement containing %q", name, err, c.want)
		}
	}
}

func TestGuardedUpdateDoesNotMutateSet(t *testing.T) {
	u := guardedUpdate()
	u.Set = make([]ast.Assignment, 1, 4)
	u.Set[0] = ast.Assignment{Column: "name", Value: "Acme"}
	if _, err := u.SQL(stub{}); err != nil {
		t.Fatalf("SQL() error: %v", err)
	}
	if len(u.Set) != 1 {
		t.Errorf("Set length = %d after lowering, want the caller's slice untouched", len(u.Set))
	}
}

func TestGuardedDeleteSQL(t *testing.T) {
	g, err := operation.GuardedDelete{
		Table: "organization",
		Key:   operation.Field{Name: "id", Expr: ast.Col("id")},
		ID:    "x",
		Guard: operation.Guard{Column: "version", Version: 3},
	}.SQL(stub{})
	if err != nil {
		t.Fatalf("SQL() error: %v", err)
	}
	wantSQL(t, g.Command.Text, "DELETE FROM organization WHERE (id = $1 AND version = $2)")
	wantArgs(t, g.Command.Args, "x", int64(3))
	wantSQL(t, g.Check.Text, "SELECT version FROM organization WHERE id = $1")
	wantArgs(t, g.Check.Args, "x")
}

func TestGuardedDeleteValidation(t *testing.T) {
	_, err := operation.GuardedDelete{
		Key:   operation.Field{Name: "id", Expr: ast.Col("id")},
		Guard: operation.Guard{Column: "version"},
	}.SQL(stub{})
	if !errors.Is(err, ast.ErrInvalidStatement) || !strings.Contains(err.Error(), "requires a table") {
		t.Errorf("error = %v, want ErrInvalidStatement containing %q", err, "requires a table")
	}
}
