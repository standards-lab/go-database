# Providers

A provider is an implementation of the standard the `database` package establishes for one engine.
It constructs the connection pool over that engine's driver, supplies the engine's `Dialect`, and
hands both to `database.New`, which returns the `*database.DB` a consumer uses. That constructor
call is the whole contract between a provider and the base. `postgres` is the first provider.

## Selected by direct typed construction

Each provider exposes a `Provider` constant and a typed constructor: `postgres.Provider` and
`postgres.New(cfg database.Config) (*database.DB, error)`. A consumer selects its provider in its
composition root (`tiers.md`) with a typed switch over the constant and a direct import of the
chosen provider. There is no runtime registry, no `Register()` call, and no `init()` side effect;
importing a package never registers anything. Adding a provider is one new import and one new
switch case in the composition root, with no change to the `database` package.

## One provider per engine

A provider covers an engine wherever it runs. The `postgres` provider serves a local container and a
managed PostgreSQL service alike; the difference between them is configuration: the connection
string, the credentials, and the `Options` keys.

## Changing a provider

Changing the provider changes infrastructure, not only configuration. The consumer's schema,
migrations, seed SQL, and domain SQL are written for its engine and are rewritten for the new one,
along with the import and switch case in the composition root. Code written against the `database`
package does not change.

## A provider for another engine

A provider for another engine is defined where it is owned and maintained: as a sub-module of this
repository (`postgres`) or as its own library (`go-database-mssql`, for instance). Either way, it
satisfies the requirements of the base `database` package.
