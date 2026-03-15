// Package queryx provides extensions to Go's standard database/sql package.
//
// queryx adds struct scanning, named parameter binding, and convenience methods
// on top of database/sql. It is a modern, maintained replacement for jmoiron/sqlx
// with generics support, context-first API, and fixed Postgres parsing.
//
// Core types wrap the standard library:
//
//   - DB wraps *sql.DB with extended query methods
//   - Tx wraps *sql.Tx with the same extended methods
//   - Stmt wraps *sql.Stmt with Get/Select
//   - NamedStmt provides prepared statements with named parameters
//   - Rows wraps *sql.Rows with StructScan/SliceScan/MapScan
//
// Bind variable formats are supported for all major databases:
//
//   - DOLLAR: $1, $2 (PostgreSQL)
//   - QUESTION: ?, ? (MySQL, SQLite)
//   - AT: @p1, @p2 (MSSQL)
//   - NAMED: :name (Oracle)
package queryx
