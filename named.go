package queryx

import (
	"context"
	"database/sql"

	"github.com/agentine/queryx/reflectx"
)

// NamedStmt wraps a prepared statement with named parameter support.
type NamedStmt struct {
	Stmt     *sql.Stmt
	query    string
	params   []string
	bindType BindType
	mapper   *reflectx.Mapper
}

func prepareNamedContext(ctx context.Context, ext Ext, bindType BindType, query string, mapper *reflectx.Mapper) (*NamedStmt, error) {
	compiled, names, err := compileNamedQuery(bindType, query)
	if err != nil {
		return nil, err
	}
	stmt, err := ext.PrepareContext(ctx, compiled)
	if err != nil {
		return nil, err
	}
	return &NamedStmt{
		Stmt:     stmt,
		query:    compiled,
		params:   names,
		bindType: bindType,
		mapper:   mapper,
	}, nil
}

// Close closes the underlying prepared statement.
func (ns *NamedStmt) Close() error {
	return ns.Stmt.Close()
}

// ExecContext executes the named statement with the given argument.
func (ns *NamedStmt) ExecContext(ctx context.Context, arg interface{}) (sql.Result, error) {
	args, err := bindArgs(ns.params, arg, ns.mapper)
	if err != nil {
		return nil, err
	}
	return ns.Stmt.ExecContext(ctx, args...)
}

// QueryContext executes the named statement and returns *Rows.
func (ns *NamedStmt) QueryContext(ctx context.Context, arg interface{}) (*Rows, error) {
	args, err := bindArgs(ns.params, arg, ns.mapper)
	if err != nil {
		return nil, err
	}
	rows, err := ns.Stmt.QueryContext(ctx, args...)
	if err != nil {
		return nil, err
	}
	return &Rows{Rows: rows, mapper: ns.mapper}, nil
}

// QueryRowContext executes the named statement and returns a single *sql.Row.
func (ns *NamedStmt) QueryRowContext(ctx context.Context, arg interface{}) *sql.Row {
	args, err := bindArgs(ns.params, arg, ns.mapper)
	if err != nil {
		// Return a row that will error on Scan.
		return nil
	}
	return ns.Stmt.QueryRowContext(ctx, args...)
}

// GetContext executes the named statement and scans a single row into dest.
func (ns *NamedStmt) GetContext(ctx context.Context, dest interface{}, arg interface{}) error {
	r, err := ns.QueryContext(ctx, arg)
	if err != nil {
		return err
	}
	defer r.Close()
	if !r.Next() {
		if err := r.Err(); err != nil {
			return err
		}
		return sql.ErrNoRows
	}
	return r.StructScan(dest)
}

// SelectContext executes the named statement and scans all rows into dest.
func (ns *NamedStmt) SelectContext(ctx context.Context, dest interface{}, arg interface{}) error {
	r, err := ns.QueryContext(ctx, arg)
	if err != nil {
		return err
	}
	return scanAll(r.Rows, ns.mapper, dest)
}
