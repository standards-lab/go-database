package operation_test

import (
	"errors"
	"strings"
	"testing"

	"github.com/standards-lab/go-database/ast"
	"github.com/standards-lab/go-database/operation"
)

func peopleProjection() operation.Projection {
	return operation.Projection{
		From: ast.Table("t"),
		Key:  operation.Field{Name: "id", Expr: ast.Col("t.id")},
		Fields: []operation.Field{
			{Name: "name", Expr: ast.Col("t.name")},
			{Name: "age", Expr: ast.Col("t.age")},
		},
	}
}

func pageOne() operation.Page { return operation.Page{Number: 1, Size: 10} }

func TestListPair(t *testing.T) {
	list, err := peopleProjection().List(stub{}, operation.Directives{
		Page:    operation.Page{Number: 2, Size: 10},
		Sort:    []operation.Sort{{Field: "name", Descending: true}},
		Filters: []operation.Filter{{Field: "age", Op: operation.OpGe, Value: 21}},
	})
	if err != nil {
		t.Fatalf("List() error: %v", err)
	}
	wantSQL(t, list.Count.Text, "SELECT COUNT(*) FROM t WHERE t.age >= $1")
	wantArgs(t, list.Count.Args, 21)
	wantSQL(t, list.Page.Text,
		"SELECT t.id AS id, t.name AS name, t.age AS age FROM t WHERE t.age >= $1 "+
			"ORDER BY t.name DESC, t.id OFFSET $2 ROWS FETCH NEXT $3 ROWS ONLY")
	wantArgs(t, list.Page.Args, 21, 10, 10)
}

func TestTieBreakerSkippedWhenKeySorted(t *testing.T) {
	list, err := peopleProjection().List(stub{}, operation.Directives{
		Page: pageOne(),
		Sort: []operation.Sort{{Field: "id", Descending: true}},
	})
	if err != nil {
		t.Fatalf("List() error: %v", err)
	}
	if !strings.Contains(list.Page.Text, "ORDER BY t.id DESC OFFSET") {
		t.Errorf("page sql = %q, want ORDER BY t.id DESC without an appended tie-breaker", list.Page.Text)
	}
}

func TestDefaultOrderIsKey(t *testing.T) {
	list, err := peopleProjection().List(stub{}, operation.Directives{Page: pageOne()})
	if err != nil {
		t.Fatalf("List() error: %v", err)
	}
	if !strings.Contains(list.Page.Text, "ORDER BY t.id OFFSET") {
		t.Errorf("page sql = %q, want ORDER BY t.id", list.Page.Text)
	}
}

func TestFilterOperatorLowering(t *testing.T) {
	cases := map[operation.Op]struct {
		value any
		want  string
		args  []any
	}{
		operation.OpEq:        {21, "t.age = $1", []any{21}},
		operation.OpNe:        {21, "t.age <> $1", []any{21}},
		operation.OpGt:        {21, "t.age > $1", []any{21}},
		operation.OpGe:        {21, "t.age >= $1", []any{21}},
		operation.OpLt:        {21, "t.age < $1", []any{21}},
		operation.OpLe:        {21, "t.age <= $1", []any{21}},
		operation.OpLike:      {"2%", "t.age LIKE $1", []any{"2%"}},
		operation.OpIsNull:    {nil, "t.age IS NULL", nil},
		operation.OpIsNotNull: {nil, "t.age IS NOT NULL", nil},
		operation.OpIn:        {[]any{21, 22}, "t.age IN ($1, $2)", []any{21, 22}},
	}
	for op, c := range cases {
		list, err := peopleProjection().List(stub{}, operation.Directives{
			Page:    pageOne(),
			Filters: []operation.Filter{{Field: "age", Op: op, Value: c.value}},
		})
		if err != nil {
			t.Fatalf("%s: List() error: %v", op, err)
		}
		wantSQL(t, list.Count.Text, "SELECT COUNT(*) FROM t WHERE "+c.want)
		wantArgs(t, list.Count.Args, c.args...)
	}
}

func TestInFilterRequiresAnySlice(t *testing.T) {
	_, err := peopleProjection().List(stub{}, operation.Directives{
		Page:    pageOne(),
		Filters: []operation.Filter{{Field: "age", Op: operation.OpIn, Value: 21}},
	})
	if !errors.Is(err, ast.ErrInvalidStatement) {
		t.Errorf("error = %v, want ErrInvalidStatement", err)
	}
}

func TestUnknownFieldErrors(t *testing.T) {
	p := peopleProjection()

	_, err := p.List(stub{}, operation.Directives{
		Page:    pageOne(),
		Filters: []operation.Filter{{Field: "nope", Op: operation.OpEq, Value: 1}},
	})
	var fieldErr *operation.UnknownFieldError
	if !errors.As(err, &fieldErr) || fieldErr.Use != operation.FieldUseFilter || fieldErr.Field != "nope" {
		t.Errorf("filter error = %v, want UnknownFieldError{nope, filter}", err)
	}

	_, err = p.List(stub{}, operation.Directives{
		Page: pageOne(),
		Sort: []operation.Sort{{Field: "nope"}},
	})
	if !errors.As(err, &fieldErr) || fieldErr.Use != operation.FieldUseSort || fieldErr.Field != "nope" {
		t.Errorf("sort error = %v, want UnknownFieldError{nope, sort}", err)
	}
}

func TestUnknownOperatorError(t *testing.T) {
	_, err := peopleProjection().List(stub{}, operation.Directives{
		Page:    pageOne(),
		Filters: []operation.Filter{{Field: "age", Op: "between", Value: 1}},
	})
	var opErr *operation.UnknownOperatorError
	if !errors.As(err, &opErr) || opErr.Op != "between" {
		t.Errorf("error = %v, want UnknownOperatorError{between}", err)
	}
}

func TestPageBounds(t *testing.T) {
	for name, page := range map[string]operation.Page{
		"zero number": {Number: 0, Size: 10},
		"zero size":   {Number: 1, Size: 0},
	} {
		_, err := peopleProjection().List(stub{}, operation.Directives{Page: page})
		if !errors.Is(err, ast.ErrInvalidStatement) {
			t.Errorf("%s: error = %v, want ErrInvalidStatement", name, err)
		}
	}
}

func TestProjectionValidation(t *testing.T) {
	base := peopleProjection()
	missingKey := base
	missingKey.Key = operation.Field{}
	duplicate := base
	duplicate.Fields = append(duplicate.Fields, operation.Field{Name: "name", Expr: ast.Col("t.other")})
	unnamed := base
	unnamed.Fields = append(unnamed.Fields, operation.Field{Expr: ast.Col("t.other")})

	for name, p := range map[string]operation.Projection{
		"missing key":     missingKey,
		"duplicate field": duplicate,
		"unnamed field":   unnamed,
	} {
		_, err := p.List(stub{}, operation.Directives{Page: pageOne()})
		if !errors.Is(err, ast.ErrInvalidStatement) {
			t.Errorf("%s: error = %v, want ErrInvalidStatement", name, err)
		}
	}
}

func TestWhereDoorComposesWithFilters(t *testing.T) {
	list, err := peopleProjection().List(stub{}, operation.Directives{
		Page:    pageOne(),
		Filters: []operation.Filter{{Field: "age", Op: operation.OpGe, Value: 21}},
		Where:   ast.Col("t.name").Like("a%"),
	})
	if err != nil {
		t.Fatalf("List() error: %v", err)
	}
	wantSQL(t, list.Count.Text, "SELECT COUNT(*) FROM t WHERE (t.age >= $1 AND t.name LIKE $2)")
	wantArgs(t, list.Count.Args, 21, "a%")
}

func TestCountAndPageShareWhere(t *testing.T) {
	list, err := peopleProjection().List(stub{}, operation.Directives{
		Page:    pageOne(),
		Filters: []operation.Filter{{Field: "name", Op: operation.OpLike, Value: "a%"}},
	})
	if err != nil {
		t.Fatalf("List() error: %v", err)
	}
	where := "WHERE t.name LIKE $1"
	if !strings.Contains(list.Count.Text, where) || !strings.Contains(list.Page.Text, where) {
		t.Errorf("count = %q, page = %q; want both to contain %q", list.Count.Text, list.Page.Text, where)
	}
}

func TestProjectionCTEHoistsToBothStatements(t *testing.T) {
	lineage := ast.CTE{
		Name:  "lineage",
		Query: ast.Select{Columns: []ast.Column{{Expr: ast.Col("id")}}, From: ast.Table("t")},
	}
	p := operation.Projection{
		With: []ast.CTE{lineage},
		From: ast.Table("t").Join(ast.Table("lineage").As("l"), ast.Col("l.id").Eq(ast.Col("t.id"))),
		Key:  operation.Field{Name: "id", Expr: ast.Col("t.id")},
	}
	list, err := p.List(stub{}, operation.Directives{Page: pageOne()})
	if err != nil {
		t.Fatalf("List() error: %v", err)
	}
	prefix := "WITH lineage AS (SELECT id FROM t) "
	if !strings.HasPrefix(list.Count.Text, prefix) || !strings.HasPrefix(list.Page.Text, prefix) {
		t.Errorf("count = %q, page = %q; want both to open with %q", list.Count.Text, list.Page.Text, prefix)
	}
}

func TestOne(t *testing.T) {
	stmt, err := peopleProjection().One(stub{}, "name", "Ada")
	if err != nil {
		t.Fatalf("One() error: %v", err)
	}
	wantSQL(t, stmt.Text, "SELECT t.id AS id, t.name AS name, t.age AS age FROM t WHERE t.name = $1")
	wantArgs(t, stmt.Args, "Ada")
}

func TestOneByKey(t *testing.T) {
	stmt, err := peopleProjection().One(stub{}, "id", "x")
	if err != nil {
		t.Fatalf("One() error: %v", err)
	}
	if !strings.Contains(stmt.Text, "WHERE t.id = $1") {
		t.Errorf("sql = %q, want the key field resolved", stmt.Text)
	}
	if strings.Contains(stmt.Text, "ORDER BY") || strings.Contains(stmt.Text, "OFFSET") {
		t.Errorf("sql = %q, want no ORDER BY and no paging", stmt.Text)
	}
}

func TestOneUnknownField(t *testing.T) {
	_, err := peopleProjection().One(stub{}, "nope", 1)
	var fieldErr *operation.UnknownFieldError
	if !errors.As(err, &fieldErr) || fieldErr.Field != "nope" || fieldErr.Use != operation.FieldUseFilter {
		t.Errorf("error = %v, want UnknownFieldError{nope, filter}", err)
	}
}
