# The SQL architecture for v0.4

Direction for the `query` and `migrate` mechanisms and the domain code that consumes them,
settled to the depth a prototype can start from. Recorded by the `v1.data.sql.plan` session
(2026-09-01) from a reviewed API design over the strategy
(`standards-lab/context/design/dsl-driven-services.md`) and the v0.3.0 findings
(`v0.4-findings.md`). It is a concept, not design: the experiment `v1.data.sql.prototype`
supplies the evidence, and what proves out is promoted at its close. The sessions that follow
(`v1.data.sql.query`, `v1.data.sql.migrate`) extract the proven shape into this module.

## Decisions the plan session took

These are settled inputs to the prototype, not questions it re-opens.

1. **`database.go` is the domain's sole interface to SQL infrastructure.** It is the domain's
   SQL client, what an auth or storage SDK hands a consumer out of the box: the typed statement
   handles and the operations as its methods. `service.go` calls it and never imports `query`.
2. **Named parameters, `:name` only.** Resolved to dialect positions once at load. The scanner
   tracks single-quoted strings (`''` escape), double-quoted identifiers, line comments, and
   block comments; nothing inside them is a parameter. In normal text a parameter is `:`
   followed by `[A-Za-z_][A-Za-z0-9_]*` where the `:` is preceded by neither `:` nor an
   identifier character, so `::uuid` is a cast and `:=` is untouched. Each distinct name takes
   the position of its first occurrence; later occurrences rebind to it. Arguments bind from
   `query.Args`, a `map[string]any`; a missing name returns `ArgumentError`; an extra name is
   ignored, which lets one map serve a guard's command and its narrower check. `?` is not kept
   as a second grammar. The stdlib `sql.Named` is not used: pgx's `database/sql` adapter
   ignores names, so resolution has to be in the library to be hermetic across dialects.
3. **Typed handles, bound once at wiring, no reflection.** A statement is fetched by name only
   in the domain's wiring function. `query.Scan(stmt, scanFn)` yields `Rows[T]` with `One`,
   `All`, and `Each` (an `iter.Seq2[T, error]` that closes on break); `query.Project(base,
   scanFn)` yields `Projection[T]` with `List` and `One`; `query.Guarded(command, check,
   "version")` yields `Guard` with `Run`; `Statement` carries `Exec`. Constructors are package
   functions because Go has no generic methods, and `T` is inferred from the scan function. A
   reflection-populated statement struct was evaluated and rejected: the typed handles need a
   scan function only code supplies, so reflection saves a few lines per domain for a
   `reflect` import and a runtime mismatch.
4. **`Session` is the stdlib method set.** `ExecContext`, `QueryContext`, `PrepareContext`,
   implemented by `*DB` and `*Tx`, every error mapped through the dialect inside the seam.
   `Dialect()` and `QueryRowContext` leave the seam: runners never need the dialect at request
   time, and `*sql.Row` defers its error to `Scan` where nothing can map it. Every runner takes
   a `Session`, so the same handle runs in either scope.
5. **Transactions.** `database.Transact[T](ctx, db, fn func(*Tx) (T, error), opts
   ...TxOption) (T, error)` for a unit with a result; `ExecTx` for the void case. Both recover,
   roll back, and re-panic; a rollback failure is joined onto the unit's error; the commit error
   is mapped. `database.Isolation(level)` and `ReadOnly()` are the options; the common call has
   none. The guard's serializable-isolation claim thereby has a code path.
6. **Writes are not enforced by type.** A single statement is atomic under autocommit; the
   guard's happy path is one statement; a `*Tx`-only rule guaranteed nothing the compiler could
   check while doubling the API shape. Multi-statement units use `Transact`. The one real hazard,
   a transaction-scoped advisory lock run outside a transaction, is a silent no-op; a file headed
   `-- transaction: required` makes the runner return `ErrTransactionRequired` unless the
   session is a `*Tx`.
7. **The field contract sits beside the SQL it constrains.** Under the derived-table wrap
   (`SELECT * FROM (<base>) q`) the only names filters and sorts can reference are the base's
   output columns, which the base controls by aliasing, so a Go-side `Column` mapping was
   redundant. The base file's header declares `-- key: <name>` and one `-- field: <name>
   <kind>` line per contract field; kinds are `text`, `integer`, `boolean`, `uuid`,
   `timestamp`. A string value parses per kind (`strconv`, `uuid.Parse` then bound as the
   canonical string, RFC 3339); a typed value passes; a parse failure is `InvalidValueError`.
   Held as the starting point; the prototype may move it (question 2).
8. **Errors.** One request sentinel, `query.ErrDirectives`, that `UnknownFieldError`,
   `UnknownOperatorError`, `InvalidValueError`, and the bad-page errors unwrap to, so the
   consumer's matcher is one `errors.Is` for 400. `sql.ErrNoRows` is never mapped. The guard's
   outcomes are unchanged from `exec/command.go`: rows affected returns `version+1` with no
   second round trip; zero rows runs the check, no row is `sql.ErrNoRows`, a row is
   `ErrVersionMismatch` wrapping expected and current.
9. **`seed` retires** to a documented pattern. Under `ExecTx` and `encoding/json` a loader is
   about twenty-five consumer lines in the domain, disabled by config outside dev and test; the
   template scaffolds it. Under the sufficiency rule the package does not justify itself.
10. **`migrate`.** `New(db, migrations []Migration, opts)` takes SQL text plus versions;
    `migrate.Files(fsys, dir)` packages the `NNNN_name.{up,down}.sql` convention as a helper,
    never the contract. A history table (`version`, `name`, `applied_at`, `dirty`), one row per
    applied version, `Version` as `MAX(version)`, `Down` deleting rows; the table name is an
    option defaulting to `schema_version`. One transaction per file; a file headed
    `-- transaction: none` runs outside one and holds a single statement by convention, so
    engines without transactional DDL need no statement splitter. The history row is written
    dirty before a non-transactional migration and cleared after; a failure leaves it dirty,
    and only `Force(version)` clears it. Concurrent starters take a `database.Locker` dialect
    capability on a dedicated `*sql.Conn` held for the run; postgres implements it with
    `pg_advisory_lock`. A provider without `Locker` fails `Up` with a typed error unless the
    consumer opts into an unlocked run. Verbs: `Version`, `Up`, `Down`, `Steps`, `Force`,
    `Verify`, `Migrations`. Checksums are held, not built.
11. **Verification is explicit composition.** `Source.Verify` prepares every statement;
    `Projection.Verify` prepares a probe naming every contract field and the key; `query.Verify
    (ctx, db, verifiers...)` joins them over a `Verifier` interface. `Source` is the inventory
    (name, tier, native note, parameters, fields, text), not a registry. Startup and the
    management surface call the same functions.
12. **Row mapping stays scan-function-only**; the struct-tag mapper waits for a fourth domain.
    Entity identifiers stay `string`.
13. **Universal patterns as templates are admitted under one split.** A pattern carries only
    protocol: the guard frame (`WHERE <key> = :key AND <version> = :version`, `<version> =
    <version> + 1`), the version check, the collection wrap, the identity-returning insert
    frame, the guarded delete. The domain carries all expressive content: SET lists, base
    queries, columns, CTEs. The library instantiates a protocol frame around the consumer's
    text, which is what `List` already does in Go. How it lands is question 1.

## The API sketch the prototype starts from

```go
// database
type Session interface {
	ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error)
	QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error)
	PrepareContext(ctx context.Context, query string) (*sql.Stmt, error)
}
func (d *DB) Begin(ctx context.Context, opts ...TxOption) (*Tx, error)
func Transact[T any](ctx context.Context, db *DB, fn func(*Tx) (T, error), opts ...TxOption) (T, error)
func ExecTx(ctx context.Context, db *DB, fn func(*Tx) error, opts ...TxOption) error
type Dialect interface { Name() string; Placeholder(n int) string; MapError(err error) error }
type Locker interface {
	Lock(ctx context.Context, conn *sql.Conn, key int64) error
	Unlock(ctx context.Context, conn *sql.Conn, key int64) error
}

// query
func Load(fsys fs.FS, dir string, d database.Dialect) (*Source, error)
func MustLoad(fsys fs.FS, dir string, d database.Dialect) *Source
func (s *Source) Statement(name string) Statement      // file base name; panics if missing
func (s *Source) Statements() []Statement              // the inventory
func (s *Source) Verify(ctx context.Context, db database.Session) error
type Verifier interface{ Verify(context.Context, database.Session) error }
func Verify(ctx context.Context, db database.Session, vs ...Verifier) error

type Statement struct{ /* name, text, tier, native, params, key, fields, txRequired */ }
func (st Statement) Exec(ctx context.Context, s database.Session, args Args) (int64, error)
type Args map[string]any

type ScanFunc[T any] func(*sql.Rows) (T, error)
func Scalar[T any](rows *sql.Rows) (T, error)
type Rows[T any] struct{ /* statement, scan */ }
func Scan[T any](st Statement, scan ScanFunc[T]) Rows[T]
func (r Rows[T]) One(ctx context.Context, s database.Session, args Args) (T, error)
func (r Rows[T]) All(ctx context.Context, s database.Session, args Args) ([]T, error)
func (r Rows[T]) Each(ctx context.Context, s database.Session, args Args) iter.Seq2[T, error]

type Projection[T any] struct{ /* base, key, fields, scan */ }
func Project[T any](base Statement, scan ScanFunc[T]) Projection[T]
func (p Projection[T]) List(ctx context.Context, s database.Session, d Directives) ([]T, int, error)
func (p Projection[T]) One(ctx context.Context, s database.Session, field string, value any) (T, error)
func (p Projection[T]) Verify(ctx context.Context, s database.Session) error

type Guard struct{ /* command, check, version parameter name */ }
func Guarded(command, check Statement, version string) Guard
func (g Guard) Run(ctx context.Context, s database.Session, version int64, args Args) (int64, error)

// Directives, Page, Sort, Op (ten operators), Filter: carried over from operation/directives.go
// minus the ast.Predicate escape. ErrDirectives; UnknownFieldError, UnknownOperatorError,
// InvalidValueError, ArgumentError; ErrTransactionRequired.

// migrate
type Migration struct{ Version int; Name string; Up, Down string; Transactional bool }
func Files(fsys fs.FS, dir string) ([]Migration, error)
type Options struct{ Table string; LockKey int64; Unlocked bool; Logger *slog.Logger }
func New(db *database.DB, migrations []Migration, opts Options) (*Migrator, error)
func (m *Migrator) Version(ctx context.Context) (Version, error)
func (m *Migrator) Up(ctx context.Context) error
func (m *Migrator) Down(ctx context.Context, n int) error
func (m *Migrator) Steps(ctx context.Context, n int) error
func (m *Migrator) Force(ctx context.Context, version int) error
func (m *Migrator) Verify(ctx context.Context) error
func (m *Migrator) Migrations() []Migration
```

The header grammar `Load` parses, as `-- key: value` lines before the first SQL token:

```
tier: standard | native        required
native: <free text>            required when tier is native: the reach and the port
transaction: required | none   optional; the runner check, or migrate's opt-out
key: <name>                    projection bases; must also be a field
field: <name> <kind>           one per line
```

The collection composition, unchanged from the strategy, now over names the header declared:

```
SELECT * FROM (<base>) q WHERE <f> <op> $n ... ORDER BY <s> [DESC] ..., <key> OFFSET $n ROWS FETCH NEXT $n ROWS ONLY
SELECT COUNT(*) FROM (<base>) q WHERE ...
```

The paging fragment is the one library-generated text with a known engine divergence
(MySQL and SQLite lack `OFFSET … FETCH`), so the override direction of the capability pattern
survives for it alone as an optional dialect interface; the declared-native direction retires
because `RETURNING` is now the consumer's native-tier file.

## The worked example

The organization domain under the sketch. The read model declares its contract where it
defines it; its SELECT list is the scan order.

```sql
-- tier: standard
-- key: id
-- field: id uuid
-- field: parent_id uuid
-- field: code text
-- field: name text
-- field: version integer
-- field: created_at timestamp
-- field: updated_at timestamp
-- field: path text
WITH RECURSIVE lineage (id, parent_id, code, name, version, created_at, updated_at, path) AS (
    SELECT o.id, o.parent_id, o.code, o.name, o.version, o.created_at, o.updated_at,
           '/' || o.code
    FROM organization o
    WHERE o.parent_id IS NULL
  UNION ALL
    SELECT o.id, o.parent_id, o.code, o.name, o.version, o.created_at, o.updated_at,
           l.path || '/' || o.code
    FROM organization o
    JOIN lineage l ON l.id = o.parent_id
)
SELECT id, parent_id, code, name, version, created_at, updated_at, path
FROM lineage
```

```sql
-- tier: standard
UPDATE organization
SET code = :code, name = :name, updated_at = CURRENT_TIMESTAMP, version = version + 1
WHERE id = :id AND version = :version
```

```sql
-- tier: native
-- native: postgres — pg_advisory_xact_lock. Ports: sp_getapplock, GET_LOCK, DBMS_LOCK, or a FOR UPDATE mutex row.
-- transaction: required
SELECT pg_advisory_xact_lock(:key)
```

`version.sql` is `SELECT version FROM organization WHERE id = :id`; `in_subtree.sql` is the
six-line recursive ancestor count over `:candidate` and `:node`; `insert.sql` is native
(`RETURNING id, version`); `transfer.sql` and `delete.sql` follow `edit.sql`'s shape.

`database.go`, the domain's SQL client: wiring, scans, and the operations.

```go
//go:embed sql/*.sql
var files embed.FS

const treeLock int64 = 1

type store struct {
	db        *database.DB
	src       *query.Source
	view      query.Projection[Organization]
	insert    query.Rows[Identity]
	inSubtree query.Rows[int64]
	lockTree  query.Statement
	edit      query.Guard
	transfer  query.Guard
	remove    query.Guard
}

func newStore(db *database.DB) *store {
	src := query.MustLoad(files, "sql", db.Dialect())
	check := src.Statement("version")
	return &store{
		db:        db,
		src:       src,
		view:      query.Project(src.Statement("organization_view"), scanOrganization),
		insert:    query.Scan(src.Statement("insert"), scanIdentity),
		inSubtree: query.Scan(src.Statement("in_subtree"), query.Scalar[int64]),
		lockTree:  src.Statement("lock_tree"),
		edit:      query.Guarded(src.Statement("edit"), check, "version"),
		transfer:  query.Guarded(src.Statement("transfer"), check, "version"),
		remove:    query.Guarded(src.Statement("delete"), check, "version"),
	}
}

func (s *store) Verify(ctx context.Context) error { return query.Verify(ctx, s.db, s.src, s.view) }

func (s *store) list(ctx context.Context, d query.Directives) ([]Organization, int, error) {
	return s.view.List(ctx, s.db, d)
}

func (s *store) create(ctx context.Context, c Create) (Identity, error) {
	return s.insert.One(ctx, s.db, query.Args{"parent_id": c.ParentID, "code": c.Code, "name": c.Name})
}

func (s *store) edit(ctx context.Context, id string, version int64, e Edit) (int64, error) {
	return s.edit.Run(ctx, s.db, version, query.Args{"id": id, "code": e.Code, "name": e.Name})
}

func (s *store) transfer(ctx context.Context, id string, version int64, t Transfer) (int64, error) {
	return database.Transact(ctx, s.db, func(tx *database.Tx) (int64, error) {
		if _, err := s.lockTree.Exec(ctx, tx, query.Args{"key": treeLock}); err != nil {
			return 0, err
		}
		if t.ParentID != nil {
			n, err := s.inSubtree.One(ctx, tx, query.Args{"node": id, "candidate": *t.ParentID})
			if err != nil {
				return 0, err
			}
			if n > 0 {
				return 0, fmt.Errorf("%w: %s is in the subtree of %s", ErrCycle, *t.ParentID, id)
			}
		}
		return s.transfer.Run(ctx, tx, version, query.Args{"id": id, "parent_id": t.ParentID})
	})
}
```

`service.go` keeps validation and calls the store. The handler's matcher collapses its two
`errors.As` cases to `errors.Is(err, query.ErrDirectives)`; the 404, 409, and 412 rows are
unchanged. The directive translation from `web.Query` lives in the service's `sdk` package,
shared by every domain, since go-web-sdk cannot import go-database under the dependency line.

## What the prototype settles

Each question names the decision its answer changes; a spike is worth running only if one does.

1. **Catalog composition.** Two shapes are tried, judged against the convention-plus-lint
   baseline (complete statements per domain, a rule checking the protocol clauses):
   load-time stitching, where pattern files ship in the library, the domain supplies fragments
   and header declarations, `Load` composes complete statements, and `Verify` prepares them;
   and build-time generation, where `go generate` composes the same templates and fragments
   into committed complete `.sql` files and the runtime sees plain SQL. The trade is workflow
   simplicity against editor and lint reach over the final text. Decides the pattern catalog's
   mechanism and the meta-language concept's phase one.
2. **The file grammar.** Whether the `:name` scanner and the header declarations hold up under
   editor tooling and Verify, and whether the field contract stays in the header or moves to
   a Go map beside the scan. Decides the grammar `Load` parses and the harness lints.
3. **The exports.** What a domain's `database.go` needs from the library once written against
   real handlers with two domains present. Fixes `query`'s API by extraction rather than
   design.
4. **Migrate's protocol** against a live engine: dirty state, non-transactional DDL, concurrent
   starters. These are the session-time acceptance proofs the testing hierarchy requires
   (`standards-lab/context/design/testing-hierarchy.md`), recorded in the experiment's notes.
5. **The split.** What promotes into this module and what stays service-side. Rewrites the
   `v1.data.sql.query`, `v1.data.sql.migrate`, and `v1.data.sql.organization` task breakdown.

## The prototype's shape

For the experiment's own settling to start from; it may adjust.

- Home: `experiments/sql-dsl/` in this repository, a nested Go module generated from the
  template (`gonew github.com/standards-lab/go-web-sdk-template/template@latest
  github.com/standards-lab/go-database/experiments/sql-dsl`). The root module and the CI matrix
  ignore it; it is not added to `go.work`.
- Dependencies: go-core and go-web-sdk at their releases; this module's v0.3.0 root and
  postgres v0.2.0 only for what stays (`Config`, the `DB` lifecycle, the sentinels, the pgx
  pool). The new seam, `query`, `migrate`, and the pattern layer are built inside the module
  over `db.Conn()` and `db.Dialect()`, so the whole SQL architecture is redesigned in one place
  with nothing published.
- Two domains, so the pattern-reuse question is tested rather than assumed: organization
  (recursive CTE, guard, advisory lock) and people (the person anchor, record status, unit FK,
  activate, deactivate, and transfer-unit commands; the claim is
  `go-web-service/context/concepts/data-layer.md`). The compose stack copies from
  go-web-service for the live-engine proofs.

## Assumptions

- `database/sql` in this Go release does not scan into `encoding.TextUnmarshaler`, so stdlib
  `uuid.UUID` does not bind or scan transparently; identifiers stay strings.
- pgx's `database/sql` adapter ignores `sql.Named`; name resolution has to be in the library.
- The derived-table wrap makes the base's output column names the only names a filter or sort
  can reference; a base that needs different contract names aliases them.
- The pattern templates hold only protocol; if a domain needs expressive content inside a
  frame, the frame is wrong, not the split.
