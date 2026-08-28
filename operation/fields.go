package operation

import "github.com/standards-lab/go-database/ast"

// Field is the contract layer's shared field vocabulary: a contract name
// and the expression that computes it. Both CQRS sides speak it — a
// projection's key and fields on the query side, a command's identity on
// the command side.
type Field struct {
	Name string
	Expr ast.Expression
}

// Columns builds a select list from column names under one table prefix,
// with extra appended for computed columns. An empty prefix renders bare
// names.
func Columns(prefix string, names []string, extra ...ast.Column) []ast.Column {
	cols := make([]ast.Column, 0, len(names)+len(extra))
	for _, name := range names {
		col := name
		if prefix != "" {
			col = prefix + "." + name
		}
		cols = append(cols, ast.Column{Expr: ast.Col(col)})
	}
	return append(cols, extra...)
}

// Fields builds projected fields whose contract names are their column
// names.
func Fields(names []string) []Field {
	fields := make([]Field, len(names))
	for i, name := range names {
		fields[i] = Field{Name: name, Expr: ast.Col(name)}
	}
	return fields
}
