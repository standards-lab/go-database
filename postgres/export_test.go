package postgres

// ConnConfig exposes the parsed connection config New opens the pool over,
// so a test can inspect what reaches pgx.
var ConnConfig = connConfig
