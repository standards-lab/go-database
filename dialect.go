package database

// Provider types the constant each provider sub-module declares for typed
// selection at the composition root; database/postgres declares
// postgres.Provider.
type Provider string

// Dialect is the seam a provider implements: the whole surface the base
// package needs from an engine. Query surfaces in later slices route SQL
// rendering and driver errors through it, so consumers import their provider
// only at the composition root.
type Dialect interface {
	// Name identifies the dialect; it matches the provider's Provider
	// constant.
	Name() string

	// Placeholder renders the 1-based nth bind parameter ("$1" for
	// postgres, "@p1" for a future mssql).
	Placeholder(n int) string

	// MapError translates a driver error into the package's sentinels. At
	// the connectivity slice it is the identity: nothing is classified and
	// sql.ErrNoRows always flows through unchanged.
	MapError(err error) error
}
