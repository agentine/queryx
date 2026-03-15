# Migration Guide: jmoiron/sqlx to agentine/queryx

queryx is a drop-in replacement for jmoiron/sqlx. This guide covers the differences and how to migrate.

## Import Change

```diff
-import "github.com/jmoiron/sqlx"
+import "github.com/agentine/queryx"
```

Replace all `sqlx.` references with `queryx.`.

## Context-First API

The biggest change: all methods require `context.Context` as the first parameter. queryx does not provide non-context variants.

```go
// Before (sqlx)
db.Get(&user, "SELECT * FROM users WHERE id = ?", 1)
db.Select(&users, "SELECT * FROM users")
db.Exec("DELETE FROM users WHERE id = ?", 1)
db.NamedExec("INSERT INTO users (name) VALUES (:name)", user)

// After (queryx)
db.GetContext(ctx, &user, "SELECT * FROM users WHERE id = ?", 1)
db.SelectContext(ctx, &users, "SELECT * FROM users")
db.ExecContext(ctx, "DELETE FROM users WHERE id = ?", 1)
db.NamedExecContext(ctx, "INSERT INTO users (name) VALUES (:name)", user)
```

### Quick Migration

For a quick migration, add `ctx := context.Background()` at the top of functions and append `Context` to method names:

| sqlx Method       | queryx Method           |
|-------------------|-------------------------|
| `Get`             | `GetContext`            |
| `Select`          | `SelectContext`         |
| `Exec`            | `ExecContext`           |
| `NamedExec`       | `NamedExecContext`      |
| `NamedQuery`      | `NamedQueryContext`     |
| `Preparex`        | `Preparex` (takes ctx)  |
| `PrepareNamed`    | `PrepareNamedContext`   |
| `MustExec`        | `MustExecContext`       |
| `Beginx`          | `BeginTxx`              |
| `MustBegin`       | `MustBeginTx`           |

## New Generic Helpers

queryx adds type-safe generic functions that eliminate the need for destination variables:

```go
// Before (sqlx)
var user User
err := db.Get(&user, "SELECT * FROM users WHERE id = $1", 1)

// After (queryx) — option 1: same as sqlx
var user User
err := db.GetContext(ctx, &user, "SELECT * FROM users WHERE id = $1", 1)

// After (queryx) — option 2: generics
user, err := queryx.Get[User](ctx, db, "SELECT * FROM users WHERE id = $1", 1)
users, err := queryx.Select[User](ctx, db, "SELECT * FROM users")
```

## Connect / MustConnect

```go
// Before (sqlx)
db := sqlx.MustConnect("postgres", dsn)
db, err := sqlx.Connect("postgres", dsn)

// After (queryx) — requires context
db := queryx.MustConnect(ctx, "postgres", dsn)
db, err := queryx.Connect(ctx, "postgres", dsn)
```

## Transactions

```go
// Before (sqlx)
tx := db.MustBegin()
tx, err := db.Beginx()

// After (queryx) — requires context
tx := db.MustBeginTx(ctx, nil)
tx, err := db.BeginTxx(ctx, nil)
```

## Named Statements

```go
// Before (sqlx)
stmt, err := db.PrepareNamed("SELECT * FROM users WHERE name = :name")

// After (queryx) — requires context
stmt, err := db.PrepareNamedContext(ctx, "SELECT * FROM users WHERE name = :name")
```

## Error Wrapping

queryx wraps errors from `Get[T]` and `Select[T]` with `QueryError`, which includes the operation name and query text:

```go
user, err := queryx.Get[User](ctx, db, "SELECT * FROM users WHERE id = $1", 1)
if err != nil {
    // err.Error() → "queryx.Get: sql: no rows in result set [query: SELECT * FROM users WHERE id = $1]"
    var qe *queryx.QueryError
    if errors.As(err, &qe) {
        fmt.Println(qe.Op)    // "Get"
        fmt.Println(qe.Query) // "SELECT * FROM users WHERE id = $1"
    }
    // errors.Is(err, sql.ErrNoRows) still works via Unwrap
}
```

## Bug Fixes

queryx fixes two known sqlx bugs:

### Postgres Cast Syntax

sqlx incorrectly parses `::type` casts as named parameters:

```sql
-- sqlx would try to bind ":text" as a named parameter
SELECT :name::text FROM users

-- queryx correctly handles :: as a Postgres cast
SELECT $1::text FROM users
```

### SQL Comments

sqlx would parse named parameters inside SQL comments:

```sql
-- sqlx would try to bind ":ignored" inside comments
SELECT * FROM users -- WHERE :ignored
/* :also_ignored */
WHERE name = :name

-- queryx correctly skips comments
```

## Unchanged

These work identically in queryx and sqlx:

- Struct tags: `db:"column_name"`
- `Rebind()` / `In()` functions
- `BindDriver()` registration
- Bind type constants: `DOLLAR`, `QUESTION`, `AT`, `NAMED`
- `StructScan` / `SliceScan` / `MapScan` on Rows
- `reflectx.Mapper` field mapping
