# Dependencies

What the base module may depend on, where a driver lives, and what the base takes from `go-core`.

## The dependency rule

The base module depends on the standard library and `go-core`, and at most on packages as idiomatic
and stable as the standard library itself (`golang.org/x/…`, `google/uuid`, and the like). Drivers
never enter the base; each lives in its provider's `go.mod`. Importing `database` or `seed`
therefore compiles no driver and pulls nothing beyond the standard library and `go-core`.

This rule is about dependency weight. Who may import a provider at all is a separate rule, the
import boundary in `tiers.md`.

## Interface in the base, driver in a provider

The `database` package defines the standard-tier interface and the shared types in the base module.
Each provider is a module of its own whose `go.mod` pins its driver, so a consumer that needs only
the interface never pulls a driver. `postgres` is a nested sub-module of this repository; a provider
for another engine is a sub-module here or its own library (`providers.md`). The module layout is in
`topology-and-naming.md`.

## What the base takes from go-core

`database` imports `go-core`'s `config` package for the configuration contract: `config.Duration`
for the timeout fields, and `config.EnvName` and `config.SetDurationFromEnv` for the environment
overrides `Finalize` composes. It does not import `lifecycle`: `DB.Start` and `DB.Shutdown` carry
the lifecycle hook signature and register in a consumer's composition root as bare method values,
and `DB.Ready` satisfies `lifecycle.ReadinessChecker` structurally. The signatures are the contract.

`seed` imports the standard library alone; it takes a `*sql.DB` and a `*slog.Logger` directly.
