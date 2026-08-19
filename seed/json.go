package seed

import (
	"encoding/json"
	"errors"
	"io"
)

// JSON is the Format for .json seed files: one JSON document per file,
// decoded strictly. An unknown field or content after the document is a data
// defect in a curated seed file, so both fail the decode rather than pass
// silently.
type JSON struct{}

// Extensions reports ".json".
func (JSON) Extensions() []string {
	return []string{".json"}
}

// Decode reads one JSON document from r into v.
func (JSON) Decode(r io.Reader, v any) error {
	dec := json.NewDecoder(r)
	dec.DisallowUnknownFields()
	if err := dec.Decode(v); err != nil {
		return err
	}
	if _, err := dec.Token(); !errors.Is(err, io.EOF) {
		return errors.New("trailing content after document")
	}
	return nil
}
