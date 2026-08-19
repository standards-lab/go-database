# The standard and native tiers

How the capability layers its API so a consumer can interface with a SQL engine at the resolution
its purpose requires, and how native use is kept from spreading through a consumer's code.

## Two tiers

The capability presents two tiers, and a consumer targets one or the other per use.

The **standard tier** is the interface every provider implements, and it is exactly the technology's
common standard: ISO/IEC 9075 SQL. The `database` package is that tier: the wrapper, the
configuration block, the sentinels, and the `Dialect` interface. It stays compliant with the standard
and does not grow past it; a feature beyond the standard is native, however many engines share it.

The **native tier** is the provider's own API, reached through the handle the capability exposes:
`database.DB.Conn()` returns the pool a consumer can drive with the driver's own features. The
capability never asks a consumer to forgo an engine's strengths for a common interface.

A consumer interfaces at the resolution its purpose requires, as a Go package uses the lowest layer
it needs and no lower. A package that reaches the native tier wraps that use and presents the
standard tier upward: the domain package that owns engine-specific SQL exposes plain queries and
commands to its callers. That is what keeps a provider from spreading through a consumer's code,
and the import boundary below enforces it.

## The native tier at configuration and error level

`Config`'s typed fields are the standard tier; its free-form `Options` map is the native tier at
configuration level, passing provider-specific connection keys through to the provider untouched.

Errors are dual-wrapped, `fmt.Errorf("%w: %w", sentinel, err)`: `errors.Is` classifies against the
base sentinel while `errors.As` reaches the driver's own error, so the native detail is never lost
behind the standard classification. `sql.ErrNoRows` is never mapped; it flows to the caller
unchanged.

## The import boundary

A program's **composition root** is the package where it assembles its dependencies: it constructs
the providers, the pools, the loggers, and the configuration, and hands them to the packages that
use them. Only a consumer's composition root, its `cmd/*` binaries, and packages that declare native
use import a provider. Every other package works against the standard tier and stays provider-free.

The boundary is a consumer-side rule; this repository never names its consumers. A consumer checks
it with a lint step that allows the provider import only in the declared packages, so a port to
another provider is a list of packages rather than a search.
