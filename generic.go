package queryx

import (
	"context"
	"database/sql"
	"fmt"
	"reflect"

	"github.com/agentine/queryx/reflectx"
)

// Get queries a single row and returns a value of type T.
// T must be a struct type with db tags for column mapping.
func Get[T any](ctx context.Context, ext ExtContext, query string, args ...interface{}) (_ T, retErr error) {
	var zero T
	rows, err := ext.QueryContext(ctx, query, args...)
	if err != nil {
		return zero, wrapQueryError("Get", query, err)
	}
	defer func() {
		if closeErr := rows.Close(); closeErr != nil && retErr == nil {
			retErr = closeErr
		}
	}()

	if !rows.Next() {
		if err := rows.Err(); err != nil {
			return zero, wrapQueryError("Get", query, err)
		}
		return zero, sql.ErrNoRows
	}

	var result T
	v := reflect.ValueOf(&result).Elem()

	// Handle scalar types.
	if v.Kind() != reflect.Struct {
		if err := rows.Scan(&result); err != nil {
			return zero, wrapQueryError("Get", query, err)
		}
		return result, nil
	}

	cols, err := rows.Columns()
	if err != nil {
		return zero, wrapQueryError("Get", query, err)
	}
	fields := ext.Mapper().TraversalsByName(v.Type(), cols)
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
		return zero, wrapQueryError("Get", query, err)
	}
	return result, nil
}

// Select queries multiple rows and returns a slice of type T.
// T must be a struct type with db tags for column mapping.
func Select[T any](ctx context.Context, ext ExtContext, query string, args ...interface{}) (_ []T, retErr error) {
	rows, err := ext.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, wrapQueryError("Select", query, err)
	}
	defer func() {
		if closeErr := rows.Close(); closeErr != nil && retErr == nil {
			retErr = closeErr
		}
	}()

	var result []T
	v := reflect.ValueOf(new(T)).Elem()
	isScalar := v.Kind() != reflect.Struct

	cols, err := rows.Columns()
	if err != nil {
		return nil, wrapQueryError("Select", query, err)
	}

	var fields [][]int
	if !isScalar {
		fields = ext.Mapper().TraversalsByName(v.Type(), cols)
	}

	for rows.Next() {
		var item T
		iv := reflect.ValueOf(&item).Elem()

		if isScalar {
			if err := rows.Scan(&item); err != nil {
				return nil, wrapQueryError("Select", query, err)
			}
		} else {
			values := make([]interface{}, len(cols))
			for i, idx := range fields {
				if idx == nil {
					values[i] = new(interface{})
				} else {
					f := reflectx.FieldByIndexes(iv, idx)
					values[i] = f.Addr().Interface()
				}
			}
			if err := rows.Scan(values...); err != nil {
				return nil, wrapQueryError("Select", query, err)
			}
		}
		result = append(result, item)
	}
	if err := rows.Err(); err != nil {
		return nil, wrapQueryError("Select", query, err)
	}
	return result, nil
}

// QueryError wraps a database error with query context.
type QueryError struct {
	Op    string
	Query string
	Err   error
}

func (e *QueryError) Error() string {
	q := e.Query
	if len(q) > 100 {
		q = q[:100] + "..."
	}
	return fmt.Sprintf("queryx.%s: %v [query: %s]", e.Op, e.Err, q)
}

func (e *QueryError) Unwrap() error {
	return e.Err
}

func wrapQueryError(op, query string, err error) error {
	return &QueryError{Op: op, Query: query, Err: err}
}
