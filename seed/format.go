package seed

import (
	"io"
)

// Format decodes one seed file into the shape a Step expects. A format owns
// one or more file extensions, and the Runner selects it by the extension of
// the step's path. Formats are passed at construction ([New]); there is no
// registry.
type Format interface {
	// Extensions lists the file extensions this format owns, leading dot
	// included, as ".json".
	Extensions() []string
	// Decode reads a whole seed file from r into v.
	Decode(r io.Reader, v any) error
}
