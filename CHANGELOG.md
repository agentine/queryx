# Changelog

## v0.1.0

Initial release of queryx — modern database/sql extensions for Go.

### Features

- Core types: `DB`, `Tx`, `Stmt`, `NamedStmt`, `Rows` wrapping `database/sql`
- `reflectx` package: struct field mapping with caching, embedded struct support, `db` tag parsing
- Named parameter binding: `:name` syntax with all bind var formats (`$1`, `?`, `@p1`, `:name`)
- In-clause expansion for slice arguments
- Generic helpers: `Get[T]`, `Select[T]` for type-safe queries
- `QueryError` wrapping with query context
- Context-first API throughout
- Fixed upstream sqlx bugs: Postgres `::type` cast parsing, SQL comment handling
- Zero external dependencies (only `mattn/go-sqlite3` for tests)
- Go 1.22+ required
