# Changelog

All notable changes to the pgvector module (`github.com/standards-lab/go-database/postgres/pgvector`)
are documented here. The format follows [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and the module adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html). This
changelog covers this sub-module only; the base module and the `postgres` provider keep their own.

## [Unreleased]

### Added

- `ParseMetric` resolves a distance metric name, matched exactly, to its entry in the metric
  table of sqlate's `postgres/pgvector` v0.1.0, without I/O. `Metric` is an alias of sqlate's
  type. A name outside the table returns an error that wraps `ErrUnknownMetric` and quotes the
  name.

[Unreleased]: https://github.com/standards-lab/go-database/commits/HEAD/postgres/pgvector
