package query_test

import (
	"errors"
	"strings"
	"testing"

	"github.com/standards-lab/go-database/query"
)

func peopleProjection() query.Projection {
	return query.Projection{
		From: query.Table("t"),
		Key:  query.Field{Name: "id", Expr: query.Col("t.id")},
		Fields: []query.Field{
			{Name: "name", Expr: query.Col("t.name")},
			{Name: "age", Expr: query.Col("t.age")},
		},
	}
}

func pageOne() query.Page { return query.Page{Number: 1, Size: 10} }

func TestStatementsPair(t *testing.T) {
	stmts, err := peopleProjection().Statements(stub{}, query.Directives{
		Page:    query.Page{Number: 2, Size: 10},
		Sort:    []query.Sort{{Field: "name", Descending: true}},
		Filters: []query.Filter{{Field: "age", Op: query.OpGe, Value: 21}},
	})
	if err != nil {
		t.Fatalf("Statements() error: %v", err)
	}
	wantSQL(t, stmts.Count.SQL, "SELECT COUNT(*) FROM t WHERE t.age >= $1")
	wantArgs(t, stmts.Count.Args, 21)
	wantSQL(t, stmts.Page.SQL,
		"SELECT t.id AS id, t.name AS name, t.age AS age FROM t WHERE t.age >= $1 "+
			"ORDER BY t.name DESC, t.id OFFSET $2 ROWS FETCH NEXT $3 ROWS ONLY")
	wantArgs(t, stmts.Page.Args, 21, 10, 10)
}

func TestTieBreakerSkippedWhenKeySorted(t *testing.T) {
	stmts, err := peopleProjection().Statements(stub{}, query.Directives{
		Page: pageOne(),
		Sort: []query.Sort{{Field: "id", Descending: true}},
	})
	if err != nil {
		t.Fatalf("Statements() error: %v", err)
	}
	if !strings.Contains(stmts.Page.SQL, "ORDER BY t.id DESC OFFSET") {
		t.Errorf("page sql = %q, want ORDER BY t.id DESC without an appended tie-breaker", stmts.Page.SQL)
	}
}

func TestDefaultOrderIsKey(t *testing.T) {
	stmts, err := peopleProjection().Statements(stub{}, query.Directives{Page: pageOne()})
	if err != nil {
		t.Fatalf("Statements() error: %v", err)
	}
	if !strings.Contains(stmts.Page.SQL, "ORDER BY t.id OFFSET") {
		t.Errorf("page sql = %q, want ORDER BY t.id", stmts.Page.SQL)
	}
}

func TestFilterOperatorLowering(t *testing.T) {
	cases := map[query.Op]struct {
		value any
		want  string
		args  []any
	}{
		query.OpEq:        {21, "t.age = $1", []any{21}},
		query.OpNe:        {21, "t.age <> $1", []any{21}},
		query.OpGt:        {21, "t.age > $1", []any{21}},
		query.OpGe:        {21, "t.age >= $1", []any{21}},
		query.OpLt:        {21, "t.age < $1", []any{21}},
		query.OpLe:        {21, "t.age <= $1", []any{21}},
		query.OpLike:      {"2%", "t.age LIKE $1", []any{"2%"}},
		query.OpIsNull:    {nil, "t.age IS NULL", nil},
		query.OpIsNotNull: {nil, "t.age IS NOT NULL", nil},
		query.OpIn:        {[]any{21, 22}, "t.age IN ($1, $2)", []any{21, 22}},
	}
	for op, c := range cases {
		stmts, err := peopleProjection().Statements(stub{}, query.Directives{
			Page:    pageOne(),
			Filters: []query.Filter{{Field: "age", Op: op, Value: c.value}},
		})
		if err != nil {
			t.Fatalf("%s: Statements() error: %v", op, err)
		}
		wantSQL(t, stmts.Count.SQL, "SELECT COUNT(*) FROM t WHERE "+c.want)
		wantArgs(t, stmts.Count.Args, c.args...)
	}
}

func TestInFilterRequiresAnySlice(t *testing.T) {
	_, err := peopleProjection().Statements(stub{}, query.Directives{
		Page:    pageOne(),
		Filters: []query.Filter{{Field: "age", Op: query.OpIn, Value: 21}},
	})
	if !errors.Is(err, query.ErrInvalidStatement) {
		t.Errorf("error = %v, want ErrInvalidStatement", err)
	}
}

func TestUnknownFieldErrors(t *testing.T) {
	p := peopleProjection()

	_, err := p.Statements(stub{}, query.Directives{
		Page:    pageOne(),
		Filters: []query.Filter{{Field: "nope", Op: query.OpEq, Value: 1}},
	})
	var fieldErr *query.UnknownFieldError
	if !errors.As(err, &fieldErr) || fieldErr.Use != query.FieldUseFilter || fieldErr.Field != "nope" {
		t.Errorf("filter error = %v, want UnknownFieldError{nope, filter}", err)
	}

	_, err = p.Statements(stub{}, query.Directives{
		Page: pageOne(),
		Sort: []query.Sort{{Field: "nope"}},
	})
	if !errors.As(err, &fieldErr) || fieldErr.Use != query.FieldUseSort || fieldErr.Field != "nope" {
		t.Errorf("sort error = %v, want UnknownFieldError{nope, sort}", err)
	}
}

func TestUnknownOperatorError(t *testing.T) {
	_, err := peopleProjection().Statements(stub{}, query.Directives{
		Page:    pageOne(),
		Filters: []query.Filter{{Field: "age", Op: "between", Value: 1}},
	})
	var opErr *query.UnknownOperatorError
	if !errors.As(err, &opErr) || opErr.Op != "between" {
		t.Errorf("error = %v, want UnknownOperatorError{between}", err)
	}
}

func TestPageBounds(t *testing.T) {
	for name, page := range map[string]query.Page{
		"zero number": {Number: 0, Size: 10},
		"zero size":   {Number: 1, Size: 0},
	} {
		_, err := peopleProjection().Statements(stub{}, query.Directives{Page: page})
		if !errors.Is(err, query.ErrInvalidStatement) {
			t.Errorf("%s: error = %v, want ErrInvalidStatement", name, err)
		}
	}
}

func TestProjectionValidation(t *testing.T) {
	base := peopleProjection()
	missingKey := base
	missingKey.Key = query.Field{}
	duplicate := base
	duplicate.Fields = append(duplicate.Fields, query.Field{Name: "name", Expr: query.Col("t.other")})
	unnamed := base
	unnamed.Fields = append(unnamed.Fields, query.Field{Expr: query.Col("t.other")})

	for name, p := range map[string]query.Projection{
		"missing key":     missingKey,
		"duplicate field": duplicate,
		"unnamed field":   unnamed,
	} {
		_, err := p.Statements(stub{}, query.Directives{Page: pageOne()})
		if !errors.Is(err, query.ErrInvalidStatement) {
			t.Errorf("%s: error = %v, want ErrInvalidStatement", name, err)
		}
	}
}

func TestWhereDoorComposesWithFilters(t *testing.T) {
	stmts, err := peopleProjection().Statements(stub{}, query.Directives{
		Page:    pageOne(),
		Filters: []query.Filter{{Field: "age", Op: query.OpGe, Value: 21}},
		Where:   query.Col("t.name").Like("a%"),
	})
	if err != nil {
		t.Fatalf("Statements() error: %v", err)
	}
	wantSQL(t, stmts.Count.SQL, "SELECT COUNT(*) FROM t WHERE (t.age >= $1 AND t.name LIKE $2)")
	wantArgs(t, stmts.Count.Args, 21, "a%")
}

func TestCountAndPageShareWhere(t *testing.T) {
	stmts, err := peopleProjection().Statements(stub{}, query.Directives{
		Page:    pageOne(),
		Filters: []query.Filter{{Field: "name", Op: query.OpLike, Value: "a%"}},
	})
	if err != nil {
		t.Fatalf("Statements() error: %v", err)
	}
	where := "WHERE t.name LIKE $1"
	if !strings.Contains(stmts.Count.SQL, where) || !strings.Contains(stmts.Page.SQL, where) {
		t.Errorf("count = %q, page = %q; want both to contain %q", stmts.Count.SQL, stmts.Page.SQL, where)
	}
}

func TestProjectionCTEHoistsToBothStatements(t *testing.T) {
	lineage := query.CTE{
		Name:  "lineage",
		Query: selectOf(col(query.Col("id"))),
	}
	p := query.Projection{
		With: []query.CTE{lineage},
		From: query.Table("t").Join(query.Table("lineage").As("l"), query.Col("l.id").Eq(query.Col("t.id"))),
		Key:  query.Field{Name: "id", Expr: query.Col("t.id")},
	}
	stmts, err := p.Statements(stub{}, query.Directives{Page: pageOne()})
	if err != nil {
		t.Fatalf("Statements() error: %v", err)
	}
	prefix := "WITH lineage AS (SELECT id FROM t) "
	if !strings.HasPrefix(stmts.Count.SQL, prefix) || !strings.HasPrefix(stmts.Page.SQL, prefix) {
		t.Errorf("count = %q, page = %q; want both to open with %q", stmts.Count.SQL, stmts.Page.SQL, prefix)
	}
}
