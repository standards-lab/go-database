// Package exec is the base module's execution layer — the only layer that
// touches database/sql at runtime. The query runners [List], [One], and
// [Query] take a database.Session, so the same reads run against the pool
// or inside a transaction; the command runners [Insert], [Update], and
// [Delete] take a *database.Tx, so the compiler enforces the
// consumer-owned transaction boundary. Every runner renders the SQL its
// operation would be hand-written as — never another operation's
// scaffolding — and every driver error routes through the session
// dialect's MapError, so constraint violations reach the caller as the
// database package's classified sentinels.
//
// # Guard outcomes
//
// The guarded runners map the operation.Guarded pair to one outcome:
// success returns the deterministic new version with no extra round trip;
// a miss runs the check, returning sql.ErrNoRows when the row is gone and
// database.ErrVersionMismatch — carrying the expected and current versions
// — when it moved. Under concurrent writers the split is best-effort (the
// check observes a later snapshot); it is exact under serializable
// isolation, and both outcomes are conflicts either way.
package exec
