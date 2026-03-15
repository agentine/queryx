# queryx — Modern SQL Extensions for Go

**Replaces:** [jmoiron/sqlx](https://github.com/jmoiron/sqlx) (17.5k stars, 24,898 importers)
**Package:** `github.com/agentine/queryx`
**Language:** Go (requires Go 1.22+)
**License:** MIT

## Why

jmoiron/sqlx has been effectively unmaintained since April 2024 — no commits in nearly 2 years, 314 open issues, 72 unmerged PRs, and the maintainer is unresponsive. Multiple issues (#793, #799, #878, #883, #969) ask whether the project is dead. Known bugs with Postgres cast syntax (`:foo::text`) and SQL comment parsing remain unfixed. The only fork (vinovest/sqlx) is a tiny company-specific effort with no community traction.

sqlx occupies a unique niche: a thin `database/sql` wrapper that adds struct scanning and named parameters without ORM complexity. GORM, sqlc, and pgx serve different paradigms and are not drop-in replacements.

## Scope

queryx provides extensions to Go's standard `database/sql` package:

- Struct row scanning (with embedded struct support)
- Named parameter binding
- `Get` / `Select` convenience methods
- `In` clause expansion
- Multiple bind var formats (`$1`, `?`, `@p1`, `:name`)
- Transaction wrappers
- Prepared statement support

## Architecture

### Core Types

```
queryx.DB      — wraps *sql.DB, adds Get/Select/NamedExec/Preparex
queryx.Tx      — wraps *sql.Tx, same extended methods
queryx.Stmt    — wraps *sql.Stmt, adds Get/Select
queryx.NamedStmt — prepared statement with named parameters
queryx.Rows    — wraps *sql.Rows, adds StructScan/SliceScan/MapScan
```

All types embed the standard library types and implement their interfaces.

### Key Packages

```
queryx/          — main package: DB, Tx, Stmt, Rows, named params, bind vars
queryx/reflectx/ — struct field mapping cache, tag parsing, embedded struct support
```

### Improvements Over sqlx

1. **Generics:** `Get[T]()` and `Select[T]()` functions (Go 1.18+) for type-safe queries without manual type assertion.
2. **Context-first:** All methods require `context.Context` as first parameter. No non-context variants.
3. **Fixed Postgres parsing:** Correctly handle `::type` casts and SQL comments in queries.
4. **Improved error messages:** Wrap errors with query context for easier debugging.
5. **Zero `reflect` in hot path:** Cache struct field mappings aggressively, minimize allocations.

## Deliverables

1. `queryx/` package with DB, Tx, Stmt, NamedStmt, Rows types
2. `queryx/reflectx/` package for struct field mapping
3. Generic helper functions: `Get[T]`, `Select[T]`
4. Named parameter support with all bind var formats
5. `In` clause expansion
6. Comprehensive test suite (PostgreSQL, MySQL, SQLite)
7. Migration guide from jmoiron/sqlx
8. README with examples and API documentation

## Non-Goals

- ORM features (auto-migration, associations, query builder)
- Connection pooling (use database/sql's built-in pool)
- Schema management
- Code generation
