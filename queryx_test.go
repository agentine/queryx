package queryx

import (
	"context"
	"database/sql"
	"errors"
	"testing"

	_ "github.com/mattn/go-sqlite3"
)

type testUser struct {
	ID    int    `db:"id"`
	Name  string `db:"name"`
	Email string `db:"email"`
}

type testEmbedded struct {
	testUser
	Phone string `db:"phone"`
}

func setupDB(t *testing.T) *DB {
	t.Helper()
	db, err := Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })

	ctx := context.Background()
	_, err = db.ExecContext(ctx, `CREATE TABLE users (
		id INTEGER PRIMARY KEY,
		name TEXT NOT NULL,
		email TEXT NOT NULL
	)`)
	if err != nil {
		t.Fatalf("CREATE TABLE: %v", err)
	}
	_, err = db.ExecContext(ctx, `INSERT INTO users (id, name, email) VALUES
		(1, 'alice', 'alice@example.com'),
		(2, 'bob', 'bob@example.com'),
		(3, 'charlie', 'charlie@example.com')`)
	if err != nil {
		t.Fatalf("INSERT: %v", err)
	}
	return db
}

func TestOpen(t *testing.T) {
	db, err := Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer func() { _ = db.Close() }()
	if db.DriverName() != "sqlite3" {
		t.Errorf("DriverName = %q, want sqlite3", db.DriverName())
	}
	if db.BindType() != QUESTION {
		t.Errorf("BindType = %d, want QUESTION(%d)", db.BindType(), QUESTION)
	}
}

func TestConnect(t *testing.T) {
	ctx := context.Background()
	db, err := Connect(ctx, "sqlite3", ":memory:")
	if err != nil {
		t.Fatalf("Connect: %v", err)
	}
	defer func() { _ = db.Close() }()
}

func TestMustConnect(t *testing.T) {
	ctx := context.Background()
	db := MustConnect(ctx, "sqlite3", ":memory:")
	defer func() { _ = db.Close() }()
}

func TestMustConnectPanics(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Fatal("expected panic")
		}
	}()
	MustConnect(context.Background(), "sqlite3", "/nonexistent/path/to/db")
}

func TestDB_GetContext(t *testing.T) {
	db := setupDB(t)
	ctx := context.Background()

	var user testUser
	err := db.GetContext(ctx, &user, "SELECT * FROM users WHERE id = ?", 1)
	if err != nil {
		t.Fatalf("GetContext: %v", err)
	}
	if user.Name != "alice" {
		t.Errorf("Name = %q, want alice", user.Name)
	}
	if user.Email != "alice@example.com" {
		t.Errorf("Email = %q, want alice@example.com", user.Email)
	}
}

func TestDB_GetContext_NoRows(t *testing.T) {
	db := setupDB(t)
	ctx := context.Background()

	var user testUser
	err := db.GetContext(ctx, &user, "SELECT * FROM users WHERE id = ?", 999)
	if !errors.Is(err, sql.ErrNoRows) {
		t.Errorf("expected sql.ErrNoRows, got %v", err)
	}
}

func TestDB_GetContext_Scalar(t *testing.T) {
	db := setupDB(t)
	ctx := context.Background()

	var count int
	err := db.GetContext(ctx, &count, "SELECT COUNT(*) FROM users")
	if err != nil {
		t.Fatalf("GetContext scalar: %v", err)
	}
	if count != 3 {
		t.Errorf("count = %d, want 3", count)
	}
}

func TestDB_SelectContext(t *testing.T) {
	db := setupDB(t)
	ctx := context.Background()

	var users []testUser
	err := db.SelectContext(ctx, &users, "SELECT * FROM users ORDER BY id")
	if err != nil {
		t.Fatalf("SelectContext: %v", err)
	}
	if len(users) != 3 {
		t.Fatalf("len = %d, want 3", len(users))
	}
	if users[0].Name != "alice" {
		t.Errorf("users[0].Name = %q, want alice", users[0].Name)
	}
	if users[2].Name != "charlie" {
		t.Errorf("users[2].Name = %q, want charlie", users[2].Name)
	}
}

func TestDB_SelectContext_Empty(t *testing.T) {
	db := setupDB(t)
	ctx := context.Background()

	var users []testUser
	err := db.SelectContext(ctx, &users, "SELECT * FROM users WHERE id > 100")
	if err != nil {
		t.Fatalf("SelectContext: %v", err)
	}
	if users != nil {
		t.Errorf("expected nil slice, got %v", users)
	}
}

func TestDB_NamedExecContext(t *testing.T) {
	db := setupDB(t)
	ctx := context.Background()

	result, err := db.NamedExecContext(ctx,
		"INSERT INTO users (id, name, email) VALUES (:id, :name, :email)",
		map[string]interface{}{"id": 4, "name": "diana", "email": "diana@example.com"},
	)
	if err != nil {
		t.Fatalf("NamedExecContext: %v", err)
	}
	rows, _ := result.RowsAffected()
	if rows != 1 {
		t.Errorf("RowsAffected = %d, want 1", rows)
	}
}

func TestDB_NamedExecContext_Struct(t *testing.T) {
	db := setupDB(t)
	ctx := context.Background()

	user := testUser{ID: 5, Name: "eve", Email: "eve@example.com"}
	_, err := db.NamedExecContext(ctx,
		"INSERT INTO users (id, name, email) VALUES (:id, :name, :email)",
		user,
	)
	if err != nil {
		t.Fatalf("NamedExecContext struct: %v", err)
	}

	var got testUser
	err = db.GetContext(ctx, &got, "SELECT * FROM users WHERE id = ?", 5)
	if err != nil {
		t.Fatalf("GetContext: %v", err)
	}
	if got.Name != "eve" {
		t.Errorf("Name = %q, want eve", got.Name)
	}
}

func TestDB_NamedQueryContext(t *testing.T) {
	db := setupDB(t)
	ctx := context.Background()

	rows, err := db.NamedQueryContext(ctx,
		"SELECT * FROM users WHERE name = :name",
		map[string]interface{}{"name": "bob"},
	)
	if err != nil {
		t.Fatalf("NamedQueryContext: %v", err)
	}
	defer func() { _ = rows.Close() }()

	if !rows.Next() {
		t.Fatal("expected a row")
	}
	var user testUser
	if err := rows.StructScan(&user); err != nil {
		t.Fatalf("StructScan: %v", err)
	}
	if user.Name != "bob" {
		t.Errorf("Name = %q, want bob", user.Name)
	}
}

func TestDB_MustExecContext(t *testing.T) {
	db := setupDB(t)
	ctx := context.Background()
	db.MustExecContext(ctx, "INSERT INTO users (id, name, email) VALUES (?, ?, ?)", 10, "test", "test@test.com")
}

func TestDB_MustExecContext_Panics(t *testing.T) {
	db := setupDB(t)
	ctx := context.Background()
	defer func() {
		if r := recover(); r == nil {
			t.Fatal("expected panic")
		}
	}()
	db.MustExecContext(ctx, "INSERT INTO nonexistent (id) VALUES (?)", 1)
}

func TestDB_Rebind(t *testing.T) {
	db := setupDB(t)
	// sqlite3 uses QUESTION, so Rebind should be a no-op.
	q := db.Rebind("SELECT * FROM users WHERE id = ?")
	if q != "SELECT * FROM users WHERE id = ?" {
		t.Errorf("Rebind = %q", q)
	}
}

func TestDB_Preparex(t *testing.T) {
	db := setupDB(t)
	ctx := context.Background()

	stmt, err := db.Preparex(ctx, "SELECT * FROM users WHERE id = ?")
	if err != nil {
		t.Fatalf("Preparex: %v", err)
	}
	defer func() { _ = stmt.Close() }()

	var user testUser
	err = stmt.GetContext(ctx, &user, 1)
	if err != nil {
		t.Fatalf("Stmt.GetContext: %v", err)
	}
	if user.Name != "alice" {
		t.Errorf("Name = %q, want alice", user.Name)
	}
}

func TestStmt_SelectContext(t *testing.T) {
	db := setupDB(t)
	ctx := context.Background()

	stmt, err := db.Preparex(ctx, "SELECT * FROM users ORDER BY id")
	if err != nil {
		t.Fatalf("Preparex: %v", err)
	}
	defer func() { _ = stmt.Close() }()

	var users []testUser
	err = stmt.SelectContext(ctx, &users)
	if err != nil {
		t.Fatalf("Stmt.SelectContext: %v", err)
	}
	if len(users) != 3 {
		t.Errorf("len = %d, want 3", len(users))
	}
}

func TestTx_GetContext(t *testing.T) {
	db := setupDB(t)
	ctx := context.Background()

	tx, err := db.BeginTxx(ctx, nil)
	if err != nil {
		t.Fatalf("BeginTxx: %v", err)
	}
	defer func() { _ = tx.Rollback() }()

	var user testUser
	err = tx.GetContext(ctx, &user, "SELECT * FROM users WHERE id = ?", 2)
	if err != nil {
		t.Fatalf("Tx.GetContext: %v", err)
	}
	if user.Name != "bob" {
		t.Errorf("Name = %q, want bob", user.Name)
	}
}

func TestTx_SelectContext(t *testing.T) {
	db := setupDB(t)
	ctx := context.Background()

	tx, err := db.BeginTxx(ctx, nil)
	if err != nil {
		t.Fatalf("BeginTxx: %v", err)
	}
	defer func() { _ = tx.Rollback() }()

	var users []testUser
	err = tx.SelectContext(ctx, &users, "SELECT * FROM users ORDER BY id")
	if err != nil {
		t.Fatalf("Tx.SelectContext: %v", err)
	}
	if len(users) != 3 {
		t.Errorf("len = %d, want 3", len(users))
	}
}

func TestTx_NamedExecContext(t *testing.T) {
	db := setupDB(t)
	ctx := context.Background()

	tx, err := db.BeginTxx(ctx, nil)
	if err != nil {
		t.Fatalf("BeginTxx: %v", err)
	}

	_, err = tx.NamedExecContext(ctx,
		"INSERT INTO users (id, name, email) VALUES (:id, :name, :email)",
		map[string]interface{}{"id": 6, "name": "frank", "email": "frank@example.com"},
	)
	if err != nil {
		t.Fatalf("Tx.NamedExecContext: %v", err)
	}

	if err := tx.Commit(); err != nil {
		t.Fatalf("Commit: %v", err)
	}

	var user testUser
	err = db.GetContext(ctx, &user, "SELECT * FROM users WHERE id = ?", 6)
	if err != nil {
		t.Fatalf("GetContext after commit: %v", err)
	}
	if user.Name != "frank" {
		t.Errorf("Name = %q, want frank", user.Name)
	}
}

func TestTx_Preparex(t *testing.T) {
	db := setupDB(t)
	ctx := context.Background()

	tx, err := db.BeginTxx(ctx, nil)
	if err != nil {
		t.Fatalf("BeginTxx: %v", err)
	}
	defer func() { _ = tx.Rollback() }()

	stmt, err := tx.Preparex(ctx, "SELECT * FROM users WHERE id = ?")
	if err != nil {
		t.Fatalf("Tx.Preparex: %v", err)
	}
	defer func() { _ = stmt.Close() }()

	var user testUser
	err = stmt.GetContext(ctx, &user, 1)
	if err != nil {
		t.Fatalf("Stmt.GetContext: %v", err)
	}
	if user.Name != "alice" {
		t.Errorf("Name = %q, want alice", user.Name)
	}
}

func TestTx_MustExecContext_Panics(t *testing.T) {
	db := setupDB(t)
	ctx := context.Background()

	tx, err := db.BeginTxx(ctx, nil)
	if err != nil {
		t.Fatalf("BeginTxx: %v", err)
	}
	defer func() { _ = tx.Rollback() }()

	defer func() {
		if r := recover(); r == nil {
			t.Fatal("expected panic")
		}
	}()
	tx.MustExecContext(ctx, "INSERT INTO nonexistent (id) VALUES (?)", 1)
}

func TestMustBeginTx(t *testing.T) {
	db := setupDB(t)
	ctx := context.Background()

	tx := db.MustBeginTx(ctx, nil)
	_ = tx.Rollback()
}

func TestRows_StructScan(t *testing.T) {
	db := setupDB(t)
	ctx := context.Background()

	rows, err := db.QueryContext(ctx, "SELECT * FROM users ORDER BY id")
	if err != nil {
		t.Fatalf("QueryContext: %v", err)
	}
	defer func() { _ = rows.Close() }()

	r := &Rows{Rows: rows, mapper: db.mapper}
	var users []testUser
	for r.Next() {
		var u testUser
		if err := r.StructScan(&u); err != nil {
			t.Fatalf("StructScan: %v", err)
		}
		users = append(users, u)
	}
	if len(users) != 3 {
		t.Errorf("len = %d, want 3", len(users))
	}
}

func TestRows_SliceScan(t *testing.T) {
	db := setupDB(t)
	ctx := context.Background()

	rows, err := db.QueryContext(ctx, "SELECT id, name FROM users WHERE id = ?", 1)
	if err != nil {
		t.Fatalf("QueryContext: %v", err)
	}
	defer func() { _ = rows.Close() }()

	r := &Rows{Rows: rows, mapper: db.mapper}
	if !r.Next() {
		t.Fatal("expected a row")
	}
	values, err := r.SliceScan()
	if err != nil {
		t.Fatalf("SliceScan: %v", err)
	}
	if len(values) != 2 {
		t.Errorf("len = %d, want 2", len(values))
	}
}

func TestRows_MapScan(t *testing.T) {
	db := setupDB(t)
	ctx := context.Background()

	rows, err := db.QueryContext(ctx, "SELECT id, name FROM users WHERE id = ?", 1)
	if err != nil {
		t.Fatalf("QueryContext: %v", err)
	}
	defer func() { _ = rows.Close() }()

	r := &Rows{Rows: rows, mapper: db.mapper}
	if !r.Next() {
		t.Fatal("expected a row")
	}
	m := make(map[string]interface{})
	if err := r.MapScan(m); err != nil {
		t.Fatalf("MapScan: %v", err)
	}
	if m["name"] != "alice" {
		t.Errorf("name = %v, want alice", m["name"])
	}
}

func TestRows_StructScan_NilPointerError(t *testing.T) {
	db := setupDB(t)
	ctx := context.Background()

	rows, err := db.QueryContext(ctx, "SELECT * FROM users WHERE id = ?", 1)
	if err != nil {
		t.Fatalf("QueryContext: %v", err)
	}
	defer func() { _ = rows.Close() }()

	r := &Rows{Rows: rows, mapper: db.mapper}
	if !r.Next() {
		t.Fatal("expected a row")
	}
	err = r.StructScan((*testUser)(nil))
	if err == nil {
		t.Fatal("expected error for nil pointer")
	}
}

func TestDB_PrepareNamedContext(t *testing.T) {
	db := setupDB(t)
	ctx := context.Background()

	ns, err := db.PrepareNamedContext(ctx, "SELECT * FROM users WHERE name = :name")
	if err != nil {
		t.Fatalf("PrepareNamedContext: %v", err)
	}
	defer func() { _ = ns.Close() }()

	var user testUser
	err = ns.GetContext(ctx, &user, map[string]interface{}{"name": "alice"})
	if err != nil {
		t.Fatalf("NamedStmt.GetContext: %v", err)
	}
	if user.Name != "alice" {
		t.Errorf("Name = %q, want alice", user.Name)
	}
}

func TestNamedStmt_SelectContext(t *testing.T) {
	db := setupDB(t)
	ctx := context.Background()

	ns, err := db.PrepareNamedContext(ctx, "SELECT * FROM users WHERE id > :min_id ORDER BY id")
	if err != nil {
		t.Fatalf("PrepareNamedContext: %v", err)
	}
	defer func() { _ = ns.Close() }()

	var users []testUser
	err = ns.SelectContext(ctx, &users, map[string]interface{}{"min_id": 1})
	if err != nil {
		t.Fatalf("NamedStmt.SelectContext: %v", err)
	}
	if len(users) != 2 {
		t.Errorf("len = %d, want 2", len(users))
	}
}

func TestNamedStmt_ExecContext(t *testing.T) {
	db := setupDB(t)
	ctx := context.Background()

	ns, err := db.PrepareNamedContext(ctx,
		"INSERT INTO users (id, name, email) VALUES (:id, :name, :email)")
	if err != nil {
		t.Fatalf("PrepareNamedContext: %v", err)
	}
	defer func() { _ = ns.Close() }()

	_, err = ns.ExecContext(ctx, map[string]interface{}{
		"id": 7, "name": "grace", "email": "grace@example.com",
	})
	if err != nil {
		t.Fatalf("NamedStmt.ExecContext: %v", err)
	}
}

func TestNamedStmt_QueryContext(t *testing.T) {
	db := setupDB(t)
	ctx := context.Background()

	ns, err := db.PrepareNamedContext(ctx, "SELECT * FROM users WHERE name = :name")
	if err != nil {
		t.Fatalf("PrepareNamedContext: %v", err)
	}
	defer func() { _ = ns.Close() }()

	rows, err := ns.QueryContext(ctx, map[string]interface{}{"name": "charlie"})
	if err != nil {
		t.Fatalf("NamedStmt.QueryContext: %v", err)
	}
	defer func() { _ = rows.Close() }()

	if !rows.Next() {
		t.Fatal("expected a row")
	}
	var user testUser
	if err := rows.StructScan(&user); err != nil {
		t.Fatalf("StructScan: %v", err)
	}
	if user.Name != "charlie" {
		t.Errorf("Name = %q, want charlie", user.Name)
	}
}

func TestSelectContext_PointerSlice(t *testing.T) {
	db := setupDB(t)
	ctx := context.Background()

	var users []*testUser
	err := db.SelectContext(ctx, &users, "SELECT * FROM users ORDER BY id")
	if err != nil {
		t.Fatalf("SelectContext: %v", err)
	}
	if len(users) != 3 {
		t.Fatalf("len = %d, want 3", len(users))
	}
	if users[0].Name != "alice" {
		t.Errorf("users[0].Name = %q, want alice", users[0].Name)
	}
}

func TestSelectContext_ScalarSlice(t *testing.T) {
	db := setupDB(t)
	ctx := context.Background()

	var names []string
	err := db.SelectContext(ctx, &names, "SELECT name FROM users ORDER BY id")
	if err != nil {
		t.Fatalf("SelectContext scalar: %v", err)
	}
	if len(names) != 3 {
		t.Fatalf("len = %d, want 3", len(names))
	}
	if names[0] != "alice" {
		t.Errorf("names[0] = %q, want alice", names[0])
	}
}

func TestInWithRebind(t *testing.T) {
	db := setupDB(t)
	ctx := context.Background()

	query, args, err := In("SELECT * FROM users WHERE id IN (?)", []int{1, 3})
	if err != nil {
		t.Fatalf("In: %v", err)
	}
	query = db.Rebind(query)

	var users []testUser
	err = db.SelectContext(ctx, &users, query, args...)
	if err != nil {
		t.Fatalf("SelectContext: %v", err)
	}
	if len(users) != 2 {
		t.Fatalf("len = %d, want 2", len(users))
	}
}

func TestMapper_Accessor(t *testing.T) {
	db := setupDB(t)
	m := db.Mapper()
	if m == nil {
		t.Fatal("Mapper() returned nil")
	}
}
