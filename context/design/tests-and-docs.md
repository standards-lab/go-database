# Tests and documentation

## Tests: co-located and black-box

Tests are `{file}_test.go` files co-located with the source they cover, in an external test package
(`package <pkg>_test`). They exercise only the public API; private infrastructure is covered
transitively through the public entry points that use it.

They run without a database. The `database` tests drive the wrapper through a `database/sql/driver`
stub whose connectivity can be broken and restored mid-test; the `seed` tests build their seed file
system with `testing/fstest` and count transactions through a fake connector; the `postgres` tests
assert construction, which performs no I/O, and never dial. `go test -race ./...` is green in each
module with no service running, so CI needs no database container.

## doc.go and godoc

Production source is written without doc comments; the agent writes godoc. Each package has exactly
one `doc.go` holding only the package comment, and the package comment is the authoritative
description of the package's API and the reasoning behind it.
