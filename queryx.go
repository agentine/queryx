// Package queryx provides extensions to Go's standard database/sql package.
//
// queryx adds struct scanning, named parameter binding, and convenience methods
// on top of database/sql. It is a modern, maintained replacement for jmoiron/sqlx
// with generics support, context-first API, and fixed Postgres parsing.
package queryx

import (
	"context"
	"database/sql"
	"fmt"
	"reflect"

	"github.com/agentine/queryx/reflectx"
)

// BindType represents a placeholder style for SQL queries.
type BindType int

const (
	UNKNOWN  BindType = iota
	DOLLAR            // $1, $2 — PostgreSQL
	QUESTION          // ?, ? — MySQL, SQLite
	AT                // @p1, @p2 — SQL Server
	NAMED             // :name — Oracle
)

// defaultMapper is the package-level mapper used for struct scanning.
var defaultMapper = reflectx.NewMapper("db")

// driverBindTypes maps driver names to their default bind type.
var driverBindTypes = map[string]BindType{
	"postgres":  DOLLAR,
	"pgx":       DOLLAR,
	"pq-timeouts": DOLLAR,
	"cloudsqlpostgres": DOLLAR,
	"ql":        QUESTION,
	"mysql":     QUESTION,
	"sqlite3":   QUESTION,
	"sqlite":    QUESTION,
	"nrpostgres": DOLLAR,
	"nrmysql":   QUESTION,
	"nrsqlite3": QUESTION,
	"mssql":     AT,
	"sqlserver": AT,
	"azuresql":  AT,
	"oci8":      NAMED,
	"ora":       NAMED,
	"godror":    NAMED,
}

// BindDriver registers a bind type for a driver name.
func BindDriver(driverName string, bindType BindType) {
	driverBindTypes[driverName] = bindType
}

// Ext is an interface for types that can execute queries (DB or Tx).
type Ext interface {
	QueryContext(ctx context.Context, query string, args ...interface{}) (*sql.Rows, error)
	QueryRowContext(ctx context.Context, query string, args ...interface{}) *sql.Row
	ExecContext(ctx context.Context, query string, args ...interface{}) (sql.Result, error)
	PrepareContext(ctx context.Context, query string) (*sql.Stmt, error)
}

// ExtContext is Ext plus driver name and mapper access.
type ExtContext interface {
	Ext
	DriverName() string
	Mapper() *reflectx.Mapper
}

// DB wraps *sql.DB with extended query methods.
type DB struct {
	*sql.DB
	driverName string
	mapper     *reflectx.Mapper
	bindType   BindType
}

// Open is the same as sql.Open, but returns a *queryx.DB.
func Open(driverName, dataSourceName string) (*DB, error) {
	db, err := sql.Open(driverName, dataSourceName)
	if err != nil {
		return nil, err
	}
	return NewDB(db, driverName), nil
}

// Connect opens a database and verifies with a ping.
func Connect(ctx context.Context, driverName, dataSourceName string) (*DB, error) {
	db, err := Open(driverName, dataSourceName)
	if err != nil {
		return nil, err
	}
	if err := db.PingContext(ctx); err != nil {
		_ = db.Close()
		return nil, err
	}
	return db, nil
}

// MustConnect is like Connect but panics on error.
func MustConnect(ctx context.Context, driverName, dataSourceName string) *DB {
	db, err := Connect(ctx, driverName, dataSourceName)
	if err != nil {
		panic(err)
	}
	return db
}

// NewDB wraps an existing *sql.DB with a known driver name.
func NewDB(db *sql.DB, driverName string) *DB {
	bt := driverBindTypes[driverName]
	return &DB{
		DB:         db,
		driverName: driverName,
		mapper:     defaultMapper,
		bindType:   bt,
	}
}

// DriverName returns the driver name used to open this database.
func (db *DB) DriverName() string {
	return db.driverName
}

// Mapper returns the struct field mapper.
func (db *DB) Mapper() *reflectx.Mapper {
	return db.mapper
}

// BindType returns the bind variable type for this database.
func (db *DB) BindType() BindType {
	return db.bindType
}

// Rebind transforms a query from QUESTION format to the DB's bind type.
func (db *DB) Rebind(query string) string {
	return Rebind(db.bindType, query)
}

// GetContext queries a single row and scans it into dest.
func (db *DB) GetContext(ctx context.Context, dest interface{}, query string, args ...interface{}) error {
	return getContext(ctx, db, db.mapper, dest, query, args...)
}

// SelectContext queries multiple rows and scans them into dest.
func (db *DB) SelectContext(ctx context.Context, dest interface{}, query string, args ...interface{}) error {
	return selectContext(ctx, db, db.mapper, dest, query, args...)
}

// Preparex returns a prepared Stmt.
func (db *DB) Preparex(ctx context.Context, query string) (*Stmt, error) {
	s, err := db.DB.PrepareContext(ctx, query)
	if err != nil {
		return nil, err
	}
	return &Stmt{Stmt: s, mapper: db.mapper}, nil
}

// MustExecContext executes a query or panics.
func (db *DB) MustExecContext(ctx context.Context, query string, args ...interface{}) sql.Result {
	res, err := db.ExecContext(ctx, query, args...)
	if err != nil {
		panic(err)
	}
	return res
}

// NamedExecContext executes a named query.
func (db *DB) NamedExecContext(ctx context.Context, query string, arg interface{}) (sql.Result, error) {
	q, args, err := bindNamedMapper(db.bindType, query, arg, db.mapper)
	if err != nil {
		return nil, err
	}
	return db.ExecContext(ctx, q, args...)
}

// NamedQueryContext executes a named query and returns Rows.
func (db *DB) NamedQueryContext(ctx context.Context, query string, arg interface{}) (*Rows, error) {
	q, args, err := bindNamedMapper(db.bindType, query, arg, db.mapper)
	if err != nil {
		return nil, err
	}
	rows, err := db.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	return &Rows{Rows: rows, mapper: db.mapper}, nil
}

// PrepareNamedContext returns a prepared NamedStmt.
func (db *DB) PrepareNamedContext(ctx context.Context, query string) (*NamedStmt, error) {
	return prepareNamedContext(ctx, db, db.bindType, query, db.mapper)
}

// BeginTxx begins a transaction and returns a *Tx.
func (db *DB) BeginTxx(ctx context.Context, opts *sql.TxOptions) (*Tx, error) {
	tx, err := db.DB.BeginTx(ctx, opts)
	if err != nil {
		return nil, err
	}
	return &Tx{Tx: tx, driverName: db.driverName, mapper: db.mapper, bindType: db.bindType}, nil
}

// MustBeginTx is like BeginTxx but panics on error.
func (db *DB) MustBeginTx(ctx context.Context, opts *sql.TxOptions) *Tx {
	tx, err := db.BeginTxx(ctx, opts)
	if err != nil {
		panic(err)
	}
	return tx
}

// Tx wraps *sql.Tx with extended query methods.
type Tx struct {
	*sql.Tx
	driverName string
	mapper     *reflectx.Mapper
	bindType   BindType
}

// DriverName returns the driver name.
func (tx *Tx) DriverName() string {
	return tx.driverName
}

// Mapper returns the struct field mapper.
func (tx *Tx) Mapper() *reflectx.Mapper {
	return tx.mapper
}

// BindType returns the bind variable type.
func (tx *Tx) BindType() BindType {
	return tx.bindType
}

// Rebind transforms a query from QUESTION format to this Tx's bind type.
func (tx *Tx) Rebind(query string) string {
	return Rebind(tx.bindType, query)
}

// GetContext queries a single row and scans it into dest.
func (tx *Tx) GetContext(ctx context.Context, dest interface{}, query string, args ...interface{}) error {
	return getContext(ctx, tx, tx.mapper, dest, query, args...)
}

// SelectContext queries multiple rows and scans them into dest.
func (tx *Tx) SelectContext(ctx context.Context, dest interface{}, query string, args ...interface{}) error {
	return selectContext(ctx, tx, tx.mapper, dest, query, args...)
}

// Preparex returns a prepared Stmt.
func (tx *Tx) Preparex(ctx context.Context, query string) (*Stmt, error) {
	s, err := tx.Tx.PrepareContext(ctx, query)
	if err != nil {
		return nil, err
	}
	return &Stmt{Stmt: s, mapper: tx.mapper}, nil
}

// MustExecContext executes a query or panics.
func (tx *Tx) MustExecContext(ctx context.Context, query string, args ...interface{}) sql.Result {
	res, err := tx.ExecContext(ctx, query, args...)
	if err != nil {
		panic(err)
	}
	return res
}

// NamedExecContext executes a named query.
func (tx *Tx) NamedExecContext(ctx context.Context, query string, arg interface{}) (sql.Result, error) {
	q, args, err := bindNamedMapper(tx.bindType, query, arg, tx.mapper)
	if err != nil {
		return nil, err
	}
	return tx.ExecContext(ctx, q, args...)
}

// NamedQueryContext executes a named query and returns Rows.
func (tx *Tx) NamedQueryContext(ctx context.Context, query string, arg interface{}) (*Rows, error) {
	q, args, err := bindNamedMapper(tx.bindType, query, arg, tx.mapper)
	if err != nil {
		return nil, err
	}
	rows, err := tx.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	return &Rows{Rows: rows, mapper: tx.mapper}, nil
}

// Stmt wraps *sql.Stmt with extended query methods.
type Stmt struct {
	*sql.Stmt
	mapper *reflectx.Mapper
}

// GetContext queries a single row and scans it into dest.
func (s *Stmt) GetContext(ctx context.Context, dest interface{}, args ...interface{}) error {
	rows, err := s.QueryContext(ctx, args...)
	if err != nil {
		return err
	}
	return scanOne(rows, s.mapper, dest)
}

// SelectContext queries multiple rows and scans them into dest.
func (s *Stmt) SelectContext(ctx context.Context, dest interface{}, args ...interface{}) error {
	rows, err := s.QueryContext(ctx, args...)
	if err != nil {
		return err
	}
	return scanAll(rows, s.mapper, dest)
}

// Rows wraps *sql.Rows with struct scanning methods.
type Rows struct {
	*sql.Rows
	mapper  *reflectx.Mapper
	columns []string
	fields  [][]int
	started bool
}

// StructScan scans the current row into dest.
func (r *Rows) StructScan(dest interface{}) error {
	if !r.started {
		cols, err := r.Columns()
		if err != nil {
			return err
		}
		r.columns = cols
		r.started = true
	}

	v := reflect.ValueOf(dest)
	if v.Kind() != reflect.Ptr || v.IsNil() {
		return fmt.Errorf("queryx: dest must be a non-nil pointer, got %T", dest)
	}
	v = v.Elem()

	if r.fields == nil {
		r.fields = r.mapper.TraversalsByName(v.Type(), r.columns)
	}

	values := make([]interface{}, len(r.columns))
	for i, idx := range r.fields {
		if idx == nil {
			values[i] = new(interface{})
		} else {
			f := reflectx.FieldByIndexes(v, idx)
			values[i] = f.Addr().Interface()
		}
	}
	return r.Scan(values...)
}

// SliceScan scans the current row into a []interface{}.
func (r *Rows) SliceScan() ([]interface{}, error) {
	cols, err := r.Columns()
	if err != nil {
		return nil, err
	}
	values := make([]interface{}, len(cols))
	ptrs := make([]interface{}, len(cols))
	for i := range values {
		ptrs[i] = &values[i]
	}
	if err := r.Scan(ptrs...); err != nil {
		return nil, err
	}
	return values, nil
}

// MapScan scans the current row into a map[string]interface{}.
func (r *Rows) MapScan(dest map[string]interface{}) error {
	cols, err := r.Columns()
	if err != nil {
		return err
	}
	values := make([]interface{}, len(cols))
	ptrs := make([]interface{}, len(cols))
	for i := range values {
		ptrs[i] = &values[i]
	}
	if err := r.Scan(ptrs...); err != nil {
		return err
	}
	for i, col := range cols {
		dest[col] = values[i]
	}
	return nil
}

// internal helpers for Get/Select scanning

func getContext(ctx context.Context, ext Ext, mapper *reflectx.Mapper, dest interface{}, query string, args ...interface{}) error {
	rows, err := ext.QueryContext(ctx, query, args...)
	if err != nil {
		return err
	}
	return scanOne(rows, mapper, dest)
}

func selectContext(ctx context.Context, ext Ext, mapper *reflectx.Mapper, dest interface{}, query string, args ...interface{}) error {
	rows, err := ext.QueryContext(ctx, query, args...)
	if err != nil {
		return err
	}
	return scanAll(rows, mapper, dest)
}

func scanOne(rows *sql.Rows, mapper *reflectx.Mapper, dest interface{}) (retErr error) {
	defer func() {
		if closeErr := rows.Close(); closeErr != nil && retErr == nil {
			retErr = closeErr
		}
	}()
	if !rows.Next() {
		if err := rows.Err(); err != nil {
			return err
		}
		return sql.ErrNoRows
	}

	v := reflect.ValueOf(dest)
	if v.Kind() != reflect.Ptr || v.IsNil() {
		return fmt.Errorf("queryx: dest must be a non-nil pointer, got %T", dest)
	}
	v = v.Elem()

	// Check if dest is a scalar type (not a struct).
	if v.Kind() != reflect.Struct {
		return rows.Scan(dest)
	}

	cols, err := rows.Columns()
	if err != nil {
		return err
	}
	fields := mapper.TraversalsByName(v.Type(), cols)
	values := make([]interface{}, len(cols))
	for i, idx := range fields {
		if idx == nil {
			values[i] = new(interface{})
		} else {
			f := reflectx.FieldByIndexes(v, idx)
			values[i] = f.Addr().Interface()
		}
	}
	if err := rows.Scan(values...); err != nil {
		return err
	}
	return rows.Close()
}

func scanAll(rows *sql.Rows, mapper *reflectx.Mapper, dest interface{}) error {
	defer rows.Close()

	dv := reflect.ValueOf(dest)
	if dv.Kind() != reflect.Ptr || dv.IsNil() {
		return fmt.Errorf("queryx: dest must be a non-nil pointer to a slice, got %T", dest)
	}
	sliceVal := dv.Elem()
	if sliceVal.Kind() != reflect.Slice {
		return fmt.Errorf("queryx: dest must be a pointer to a slice, got %T", dest)
	}

	elemType := sliceVal.Type().Elem()
	isPtr := elemType.Kind() == reflect.Ptr
	baseType := elemType
	if isPtr {
		baseType = elemType.Elem()
	}

	isScalar := baseType.Kind() != reflect.Struct

	cols, err := rows.Columns()
	if err != nil {
		return err
	}

	var fields [][]int
	if !isScalar {
		fields = mapper.TraversalsByName(baseType, cols)
	}

	for rows.Next() {
		elemVal := reflect.New(baseType).Elem()

		if isScalar {
			if err := rows.Scan(elemVal.Addr().Interface()); err != nil {
				return err
			}
		} else {
			values := make([]interface{}, len(cols))
			for i, idx := range fields {
				if idx == nil {
					values[i] = new(interface{})
				} else {
					f := reflectx.FieldByIndexes(elemVal, idx)
					values[i] = f.Addr().Interface()
				}
			}
			if err := rows.Scan(values...); err != nil {
				return err
			}
		}

		if isPtr {
			p := reflect.New(baseType)
			p.Elem().Set(elemVal)
			sliceVal.Set(reflect.Append(sliceVal, p))
		} else {
			sliceVal.Set(reflect.Append(sliceVal, elemVal))
		}
	}
	return rows.Err()
}
