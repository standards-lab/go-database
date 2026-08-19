package seed_test

import (
	"strings"
	"testing"

	"github.com/standards-lab/go-database/seed"
)

func TestJSON_ExtensionsReportsJSON(t *testing.T) {
	got := seed.JSON{}.Extensions()

	if len(got) != 1 || got[0] != ".json" {
		t.Errorf("Extensions() = %v, want [.json]", got)
	}
}

func TestJSON_DecodeReadsDocument(t *testing.T) {
	type row struct {
		Code string `json:"code"`
		Name string `json:"name"`
	}
	input := `[{"code":"hq","name":"Headquarters"},{"code":"ops","name":"Operations"}]`

	var rows []row
	if err := (seed.JSON{}).Decode(strings.NewReader(input), &rows); err != nil {
		t.Fatalf("Decode() error = %v", err)
	}

	if len(rows) != 2 {
		t.Fatalf("decoded %d rows, want 2", len(rows))
	}
	if rows[0].Code != "hq" || rows[1].Name != "Operations" {
		t.Errorf("rows = %+v, want hq and Operations preserved", rows)
	}
}

func TestJSON_DecodeRejectsUnknownField(t *testing.T) {
	type row struct {
		Code string `json:"code"`
	}
	input := `[{"code":"hq","surprise":true}]`

	var rows []row
	err := (seed.JSON{}).Decode(strings.NewReader(input), &rows)

	if err == nil {
		t.Fatal("Decode() = nil, want unknown-field error")
	}
	if !strings.Contains(err.Error(), "surprise") {
		t.Errorf("Decode() error = %v, want it to name the unknown field", err)
	}
}

func TestJSON_DecodeRejectsTrailingContent(t *testing.T) {
	type row struct {
		Code string `json:"code"`
	}
	input := `[{"code":"hq"}] [{"code":"ops"}]`

	var rows []row
	err := (seed.JSON{}).Decode(strings.NewReader(input), &rows)

	if err == nil {
		t.Fatal("Decode() = nil, want trailing-content error")
	}
	if !strings.Contains(err.Error(), "trailing content") {
		t.Errorf("Decode() error = %v, want trailing-content error", err)
	}
}
