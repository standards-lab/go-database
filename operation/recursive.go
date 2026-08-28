package operation

import (
	"slices"

	"github.com/standards-lab/go-database/ast"
)

// RecursivePath is a computed-field pattern: a self-referencing table whose
// rows compose a path by walking the parent chain — Segment joined by
// Separator, prefixed with it when Rooted ("/acme/engineering"; unrooted,
// "v1.data.reads"). Parent names the self-reference, Key the column it
// joins, and Columns the carried set, Key included, in scan order.
type RecursivePath struct {
	Table     string
	Key       string
	Parent    string
	Segment   string
	Field     string
	Separator string
	Rooted    bool
	Columns   []string
}

// Projection builds the pattern's whole read model: a recursive CTE
// (standard SQL:1999) re-presenting the table with Field as one extra
// column, and the projection reading from it — Key first, the carried
// columns, Field last. The pattern owns the full projection because the CTE
// replaces the FROM and already requires every input the projection needs;
// a consumer composing several computed fields would decompose this into
// projection transformers each contributing a CTE and a field, a seam
// recorded in the design notes and not yet built. An empty Separator panics
// as a wiring mistake.
func (r RecursivePath) Projection() Projection {
	if r.Separator == "" {
		panic("operation: RecursivePath requires a Separator")
	}

	name := r.Table + "_" + r.Field
	sep := "'" + r.Separator + "'"

	root := ast.Column{Expr: ast.Col("o." + r.Segment)}
	if r.Rooted {
		root = ast.Column{Expr: ast.Raw(sep + " || o." + r.Segment)}
	}

	cte := ast.CTE{
		Name:      name,
		Columns:   append(slices.Clone(r.Columns), r.Field),
		Recursive: true,
		Query: ast.UnionAll(
			ast.Select{
				Columns: Columns("o", r.Columns, root),
				From:    ast.Table(r.Table).As("o"),
				Where:   ast.Col("o." + r.Parent).IsNull(),
			},
			ast.Select{
				Columns: Columns("o", r.Columns, ast.Column{
					Expr: ast.Raw("l." + r.Field + " || " + sep + " || o." + r.Segment),
				}),
				From: ast.Table(r.Table).As("o").Join(
					ast.Table(name).As("l"),
					ast.Col("o."+r.Parent).Eq(ast.Col("l."+r.Key)),
				),
			},
		),
	}

	fields := make([]string, 0, len(r.Columns))
	for _, c := range r.Columns {
		if c != r.Key {
			fields = append(fields, c)
		}
	}
	fields = append(fields, r.Field)

	return Projection{
		With:   []ast.CTE{cte},
		From:   ast.Table(name),
		Key:    Field{Name: r.Key, Expr: ast.Col(r.Key)},
		Fields: Fields(fields),
	}
}
