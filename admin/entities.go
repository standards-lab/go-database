package admin

import "time"

// Seeded counts the rows a seed run inserted, by the consumer's own names.
type Seeded map[string]int

// Transition is the result of a reset: the state, the schema's status, and
// the rows seeded.
type Transition struct {
	State  string `json:"state"`
	Schema Status `json:"schema"`
	Seeded Seeded `json:"seeded"`
}

// Diagnostics is one read of the database's health.
type Diagnostics struct {
	Dialect       string        `json:"dialect"`
	Ping          time.Duration `json:"ping"`                     // the ping's latency
	ServerVersion string        `json:"server_version,omitempty"` // empty without a Versioner
	Pool          Pool          `json:"pool"`
	Namespaces    []string      `json:"namespaces"` // the catalog's pattern namespaces
}

// Pool is the connection pool's counters.
type Pool struct {
	Open         int           `json:"open"`
	InUse        int           `json:"in_use"`
	Idle         int           `json:"idle"`
	MaxOpen      int           `json:"max_open"`
	WaitCount    int64         `json:"wait_count"`
	WaitDuration time.Duration `json:"wait_duration"`
}

// Status is the schema's state: Ready when every set is clean and current
// and every extension declaration matches the database, each set's state,
// and each extension declaration's state, in declared order.
type Status struct {
	Ready      bool              `json:"ready"`
	Sets       []SetStatus       `json:"sets"`
	Extensions []ExtensionStatus `json:"extensions"`
}

// SetStatus is one migration set's state.
type SetStatus struct {
	Name       string          `json:"name"`
	Table      string          `json:"table"`   // the history table
	Version    int             `json:"version"` // the applied head
	Latest     int             `json:"latest"`  // the set's last migration
	Dirty      bool            `json:"dirty"`
	Pending    []int           `json:"pending"`
	Migrations []MigrationInfo `json:"migrations"`
}

// ExtensionStatus is one extension declaration's state. A declaration
// matches the database when its extension is installed at the default
// version the database offers, or, when Removed, is not installed; Pending
// marks a mismatch Up resolves, so an extension the database does not offer,
// or one with no update path to the default, is neither matched nor
// pending.
type ExtensionStatus struct {
	Name      string `json:"name"`
	Removed   bool   `json:"removed"`
	Installed string `json:"installed"` // empty when not installed
	Default   string `json:"default"`   // empty when the database does not offer it
	Pending   bool   `json:"pending"`
}

// MigrationInfo describes one migration of a set.
type MigrationInfo struct {
	Version       int    `json:"version"`
	Name          string `json:"name"`
	Transactional bool   `json:"transactional"`
	Applied       bool   `json:"applied"`
}

// Catalog is the pattern catalog as an operator reads it, in namespace then
// name order.
type Catalog struct {
	Namespaces []string  `json:"namespaces"`
	Patterns   []Pattern `json:"patterns"`
}

// Pattern is one catalog entry: its namespace and name, its tier and
// native note, the slots its body declares, and the body itself, as the
// library composes or splices it.
type Pattern struct {
	Namespace string   `json:"namespace"`
	Name      string   `json:"name"`
	Tier      string   `json:"tier"`
	Native    string   `json:"native,omitempty"`
	Slots     []string `json:"slots"`
	Text      string   `json:"text"`
}

// Inventory is the statements registry as an operator reads it, in the
// registry's order.
type Inventory struct {
	Domains []DomainStatements `json:"domains"`
}

// DomainStatements is one domain's compiled inventory.
type DomainStatements struct {
	Name       string          `json:"name"`
	Statements []StatementInfo `json:"statements"`
}

// StatementInfo is one compiled statement: its declarations, its
// parameters in position order, and the text the engine receives.
type StatementInfo struct {
	Name                string   `json:"name"`
	Tier                string   `json:"tier"`
	Native              string   `json:"native,omitempty"`
	TransactionRequired bool     `json:"transaction_required"`
	Params              []string `json:"params"`
	Key                 string   `json:"key,omitempty"`
	Fields              []string `json:"fields,omitempty"`
	Text                string   `json:"text"`
}
