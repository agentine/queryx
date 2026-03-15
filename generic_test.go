package queryx

import (
	"context"
	"database/sql"
	"errors"
	"testing"
)

func TestGenericGet(t *testing.T) {
	db := setupDB(t)
	ctx := context.Background()

	user, err := Get[testUser](ctx, db, "SELECT * FROM users WHERE id = ?", 1)
	if err != nil {
		t.Fatalf("Get[testUser]: %v", err)
	}
	if user.Name != "alice" {
		t.Errorf("Name = %q, want alice", user.Name)
	}
	if user.Email != "alice@example.com" {
		t.Errorf("Email = %q, want alice@example.com", user.Email)
	}
}

func TestGenericGet_NoRows(t *testing.T) {
	db := setupDB(t)
	ctx := context.Background()

	_, err := Get[testUser](ctx, db, "SELECT * FROM users WHERE id = ?", 999)
	if !errors.Is(err, sql.ErrNoRows) {
		t.Errorf("expected sql.ErrNoRows, got %v", err)
	}
}

func TestGenericGet_Scalar(t *testing.T) {
	db := setupDB(t)
	ctx := context.Background()

	count, err := Get[int](ctx, db, "SELECT COUNT(*) FROM users")
	if err != nil {
		t.Fatalf("Get[int]: %v", err)
	}
	if count != 3 {
		t.Errorf("count = %d, want 3", count)
	}
}

func TestGenericSelect(t *testing.T) {
	db := setupDB(t)
	ctx := context.Background()

	users, err := Select[testUser](ctx, db, "SELECT * FROM users ORDER BY id")
	if err != nil {
		t.Fatalf("Select[testUser]: %v", err)
	}
	if len(users) != 3 {
		t.Fatalf("len = %d, want 3", len(users))
	}
	if users[0].Name != "alice" {
		t.Errorf("users[0].Name = %q, want alice", users[0].Name)
	}
}

func TestGenericSelect_Scalar(t *testing.T) {
	db := setupDB(t)
	ctx := context.Background()

	names, err := Select[string](ctx, db, "SELECT name FROM users ORDER BY id")
	if err != nil {
		t.Fatalf("Select[string]: %v", err)
	}
	if len(names) != 3 {
		t.Fatalf("len = %d, want 3", len(names))
	}
	if names[0] != "alice" {
		t.Errorf("names[0] = %q, want alice", names[0])
	}
}

func TestGenericSelect_Empty(t *testing.T) {
	db := setupDB(t)
	ctx := context.Background()

	users, err := Select[testUser](ctx, db, "SELECT * FROM users WHERE id > 100")
	if err != nil {
		t.Fatalf("Select[testUser]: %v", err)
	}
	if users != nil {
		t.Errorf("expected nil slice, got %v", users)
	}
}

func TestGenericGet_InTx(t *testing.T) {
	db := setupDB(t)
	ctx := context.Background()

	tx, err := db.BeginTxx(ctx, nil)
	if err != nil {
		t.Fatalf("BeginTxx: %v", err)
	}
	defer func() { _ = tx.Rollback() }()

	user, err := Get[testUser](ctx, tx, "SELECT * FROM users WHERE id = ?", 2)
	if err != nil {
		t.Fatalf("Get[testUser] in tx: %v", err)
	}
	if user.Name != "bob" {
		t.Errorf("Name = %q, want bob", user.Name)
	}
}

func TestQueryError_Unwrap(t *testing.T) {
	db := setupDB(t)
	ctx := context.Background()

	_, err := Get[testUser](ctx, db, "SELECT * FROM nonexistent WHERE id = ?", 1)
	if err == nil {
		t.Fatal("expected error")
	}

	var qe *QueryError
	if !errors.As(err, &qe) {
		t.Fatalf("expected QueryError, got %T", err)
	}
	if qe.Op != "Get" {
		t.Errorf("Op = %q, want Get", qe.Op)
	}
	if qe.Query != "SELECT * FROM nonexistent WHERE id = ?" {
		t.Errorf("Query = %q", qe.Query)
	}
}

func TestQueryError_LongQueryTruncated(t *testing.T) {
	longQuery := "SELECT * FROM users WHERE " + string(make([]byte, 200))
	qe := &QueryError{Op: "Get", Query: longQuery, Err: sql.ErrNoRows}
	msg := qe.Error()
	if len(msg) > 200 {
		// Should be truncated to ~100 chars of query + prefix.
		if len(qe.Query) <= 100 {
			t.Error("query should not have been truncated")
		}
	}
}
