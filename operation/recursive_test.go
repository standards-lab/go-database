package operation_test

import (
	"strings"
	"testing"

	"github.com/standards-lab/go-database/operation"
)

func orgPath(rooted bool) operation.RecursivePath {
	return operation.RecursivePath{
		Table:     "organization",
		Key:       "id",
		Parent:    "parent_id",
		Segment:   "code",
		Field:     "path",
		Separator: "/",
		Rooted:    rooted,
		Columns:   []string{"id", "code"},
	}
}

func TestRecursivePathRooted(t *testing.T) {
	list, err := orgPath(true).Projection().List(stub{}, operation.Directives{Page: pageOne()})
	if err != nil {
		t.Fatalf("List() error: %v", err)
	}
	for _, fragment := range []string{
		"WITH RECURSIVE organization_path (id, code, path) AS (",
		"SELECT o.id, o.code, '/' || o.code FROM organization o WHERE o.parent_id IS NULL",
		"UNION ALL",
		"SELECT o.id, o.code, l.path || '/' || o.code FROM organization o INNER JOIN organization_path l ON o.parent_id = l.id",
		"FROM organization_path",
	} {
		if !strings.Contains(list.Page.Text, fragment) {
			t.Errorf("page sql = %q\nwant fragment %q", list.Page.Text, fragment)
		}
	}
}

func TestRecursivePathUnrooted(t *testing.T) {
	unrooted := orgPath(false)
	unrooted.Separator = "."
	list, err := unrooted.Projection().List(stub{}, operation.Directives{Page: pageOne()})
	if err != nil {
		t.Fatalf("List() error: %v", err)
	}
	if !strings.Contains(list.Page.Text, "SELECT o.id, o.code, o.code FROM organization o WHERE o.parent_id IS NULL") {
		t.Errorf("page sql = %q, want the unrooted anchor without a leading separator", list.Page.Text)
	}
	if !strings.Contains(list.Page.Text, "l.path || '.' || o.code") {
		t.Errorf("page sql = %q, want the dotted step composition", list.Page.Text)
	}
}

func TestRecursivePathProjectionShape(t *testing.T) {
	p := orgPath(true).Projection()
	if p.Key.Name != "id" {
		t.Errorf("key = %q, want id", p.Key.Name)
	}
	names := make([]string, len(p.Fields))
	for i, f := range p.Fields {
		names[i] = f.Name
	}
	want := []string{"code", "path"}
	if len(names) != len(want) || names[0] != want[0] || names[1] != want[1] {
		t.Errorf("fields = %v, want %v (carried columns minus key, computed field last)", names, want)
	}
}

func TestRecursivePathRequiresSeparator(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Error("Projection() with no Separator did not panic")
		}
	}()
	p := orgPath(true)
	p.Separator = ""
	p.Projection()
}
