# Changelog

All notable changes to the PostgreSQL provider (`github.com/standards-lab/go-database/postgres`)
are documented here. The format follows [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and the module adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html). This
changelog covers this sub-module only; the base module keeps its own.

## [Unreleased]

### Changed

- **Breaking:** the provider supplies no dialect. The `dialect` type, its `MapError`
  classification into the base module's constraint classes, and its `ast.ReturningRenderer`
  capability are removed; the `sqlate/postgres` package owns the PostgreSQL dialect, and a
  composition root wraps the pool it constructs with `sqlate.Wrap`. `New` keeps its signature
  and calls the base module's `New(conn, cfg)`.
- **Breaking:** the `Provider` constant is removed with the base module's `Provider` type.
- Requires `github.com/standards-lab/go-database` v0.4.0.

## [v0.2.0] - 2026-08-28

### Added

- `MapError` classifies SQLSTATE class-23 constraint violations into the base package's
  sentinels — unique (23505), foreign-key (23503), check (23514), not-null (23502) — through
  a `ConstraintError` carrying the constraint name pgx exposes structurally; every other
  error, `sql.ErrNoRows` included, passes through unchanged.
- The dialect implements `ast.ReturningRenderer`, the declared-native returning capability,
  emitting the postgres `RETURNING` form for the write statements.

### Changed

- The go-database pin moves to v0.3.0, the release carrying the `ast`/`operation`/`exec`
  layer ontology this provider's capabilities plug into.

## [v0.1.1] - 2026-08-24

### Changed

- The go-database pin moves to v0.1.1, carrying go-core v0.3.0, and the module builds on
  Go 1.27 (from 1.26), aligning it with the rest of the standard's modules. No provider
  changes.

## [v0.1.0] - 2026-08-19

The first release of the PostgreSQL provider, against `github.com/standards-lab/go-database v0.1.0`.

### Added

- **Open and ping**: `postgres.New` constructs the connection pool from a finalized
  `database.Config` over pgx's `database/sql` adapter. `pgx.ParseConfig` runs eagerly, so a
  malformed config is a construction error; the URL is composed with `net/url` and the password
  set post-parse as a field, never entering the string; `Options` keys that name connection
  fields are rejected; the connect timeout is bound to the config's `conn_timeout`. The postgres
  dialect names itself, renders `$n` placeholders, and returns driver errors unchanged;
  classifying constraint violations arrives with the write path. `postgres.Provider` types the
  selection constant.

[Unreleased]: https://github.com/standards-lab/go-database/compare/postgres/v0.2.0...HEAD
[v0.2.0]: https://github.com/standards-lab/go-database/compare/postgres/v0.1.1...postgres/v0.2.0
[v0.1.1]: https://github.com/standards-lab/go-database/compare/postgres/v0.1.0...postgres/v0.1.1
[v0.1.0]: https://github.com/standards-lab/go-database/releases/tag/postgres/v0.1.0
