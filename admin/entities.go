package admin

import "time"

// Seeded counts the rows a seed run inserted, by the consumer's own names.
type Seeded map[string]int

// Diagnostics is one read of the database's health: the dialect, the ping
// latency, the server's version when the dialect supplies the statement,
// the pool's counters, and the pattern namespaces the catalog registered.
type Diagnostics struct {
	Dialect       string        `json:"dialect"`
	Ping          time.Duration `json:"ping"`
	ServerVersion string        `json:"server_version,omitempty"`
	Pool          Pool          `json:"pool"`
	Namespaces    []string      `json:"namespaces"`
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

// Status is the schema's state against the migration set: the applied head,
// whether it is dirty, the versions still pending, whether the service
// reports ready (a clean, complete history), and every migration of the
// set with whether it is applied.
type Status struct {
	Version    int             `json:"version"`
	Dirty      bool            `json:"dirty"`
	Pending    []int           `json:"pending"`
	Ready      bool            `json:"ready"`
	Migrations []MigrationInfo `json:"migrations"`
}

// MigrationInfo describes one migration of the set.
type MigrationInfo struct {
	Version       int    `json:"version"`
	Name          string `json:"name"`
	Transactional bool   `json:"transactional"`
	Applied       bool   `json:"applied"`
}

// Catalog is the pattern catalog as an operator reads it: every namespace
// the composition root registered and every pattern under them, in
// namespace then name order. It is build-time state; the read has no
// write.
type Catalog struct {
	Namespaces []string  `json:"namespaces"`
	Patterns   []Pattern `json:"patterns"`
}

// Pattern is one catalog entry: its namespace and name, its tier and
// native note, the slots its body declares, and the body as the library
// composes or splices it.
type Pattern struct {
	Namespace string   `json:"namespace"`
	Name      string   `json:"name"`
	Tier      string   `json:"tier"`
	Native    string   `json:"native,omitempty"`
	Slots     []string `json:"slots"`
	Text      string   `json:"text"`
}

// Inventory is the statements registry as an operator reads it: every
// domain that registered its compiled statements, in the registry's order,
// each statement as the library holds it. Build-time state; no write.
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
