package queryx

import (
	"reflect"
	"testing"
)

func TestRebind(t *testing.T) {
	tests := []struct {
		name     string
		bindType BindType
		query    string
		want     string
	}{
		{"question passthrough", QUESTION, "SELECT * FROM t WHERE a = ? AND b = ?", "SELECT * FROM t WHERE a = ? AND b = ?"},
		{"dollar", DOLLAR, "SELECT * FROM t WHERE a = ? AND b = ?", "SELECT * FROM t WHERE a = $1 AND b = $2"},
		{"at", AT, "SELECT * FROM t WHERE a = ? AND b = ?", "SELECT * FROM t WHERE a = @p1 AND b = @p2"},
		{"no placeholders", DOLLAR, "SELECT 1", "SELECT 1"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := Rebind(tt.bindType, tt.query)
			if got != tt.want {
				t.Errorf("Rebind(%d, %q) = %q, want %q", tt.bindType, tt.query, got, tt.want)
			}
		})
	}
}

func TestCompileNamedQuery(t *testing.T) {
	tests := []struct {
		name      string
		bindType  BindType
		query     string
		wantQuery string
		wantNames []string
	}{
		{
			"simple question",
			QUESTION,
			"SELECT * FROM t WHERE name = :name AND age = :age",
			"SELECT * FROM t WHERE name = ? AND age = ?",
			[]string{"name", "age"},
		},
		{
			"simple dollar",
			DOLLAR,
			"SELECT * FROM t WHERE name = :name AND age = :age",
			"SELECT * FROM t WHERE name = $1 AND age = $2",
			[]string{"name", "age"},
		},
		{
			"postgres cast not confused with named param",
			DOLLAR,
			"SELECT :name::text FROM t WHERE id = :id",
			"SELECT $1::text FROM t WHERE id = $2",
			[]string{"name", "id"},
		},
		{
			"line comment ignored",
			QUESTION,
			"SELECT * FROM t -- WHERE :ignored\nWHERE name = :name",
			"SELECT * FROM t -- WHERE :ignored\nWHERE name = ?",
			[]string{"name"},
		},
		{
			"block comment ignored",
			QUESTION,
			"SELECT * FROM t /* :ignored */ WHERE name = :name",
			"SELECT * FROM t /* :ignored */ WHERE name = ?",
			[]string{"name"},
		},
		{
			"string literal ignored",
			QUESTION,
			"SELECT * FROM t WHERE name = ':literal' AND age = :age",
			"SELECT * FROM t WHERE name = ':literal' AND age = ?",
			[]string{"age"},
		},
		{
			"at bind type",
			AT,
			"INSERT INTO t (name) VALUES (:name)",
			"INSERT INTO t (name) VALUES (@p1)",
			[]string{"name"},
		},
		{
			"named bind type passthrough",
			NAMED,
			"INSERT INTO t (name) VALUES (:name)",
			"INSERT INTO t (name) VALUES (:name)",
			[]string{"name"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotQuery, gotNames, err := compileNamedQuery(tt.bindType, tt.query)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if gotQuery != tt.wantQuery {
				t.Errorf("query = %q, want %q", gotQuery, tt.wantQuery)
			}
			if !reflect.DeepEqual(gotNames, tt.wantNames) {
				t.Errorf("names = %v, want %v", gotNames, tt.wantNames)
			}
		})
	}
}

func TestIn(t *testing.T) {
	tests := []struct {
		name     string
		query    string
		args     []interface{}
		wantQ    string
		wantArgs []interface{}
		wantErr  bool
	}{
		{
			"expand int slice",
			"SELECT * FROM t WHERE id IN (?)",
			[]interface{}{[]int{1, 2, 3}},
			"SELECT * FROM t WHERE id IN (?, ?, ?)",
			[]interface{}{1, 2, 3},
			false,
		},
		{
			"mixed scalar and slice",
			"SELECT * FROM t WHERE name = ? AND id IN (?)",
			[]interface{}{"alice", []int{1, 2}},
			"SELECT * FROM t WHERE name = ? AND id IN (?, ?)",
			[]interface{}{"alice", 1, 2},
			false,
		},
		{
			"no slices passthrough",
			"SELECT * FROM t WHERE a = ? AND b = ?",
			[]interface{}{1, 2},
			"SELECT * FROM t WHERE a = ? AND b = ?",
			[]interface{}{1, 2},
			false,
		},
		{
			"empty slice error",
			"SELECT * FROM t WHERE id IN (?)",
			[]interface{}{[]int{}},
			"",
			nil,
			true,
		},
		{
			"too few args",
			"SELECT * FROM t WHERE a = ? AND b = ?",
			[]interface{}{1},
			"",
			nil,
			true,
		},
		{
			"too many args",
			"SELECT * FROM t WHERE a = ?",
			[]interface{}{1, 2},
			"",
			nil,
			true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotQ, gotArgs, err := In(tt.query, tt.args...)
			if (err != nil) != tt.wantErr {
				t.Fatalf("In() error = %v, wantErr %v", err, tt.wantErr)
			}
			if err != nil {
				return
			}
			if gotQ != tt.wantQ {
				t.Errorf("query = %q, want %q", gotQ, tt.wantQ)
			}
			if !reflect.DeepEqual(gotArgs, tt.wantArgs) {
				t.Errorf("args = %v, want %v", gotArgs, tt.wantArgs)
			}
		})
	}
}

func TestBindArgs_Map(t *testing.T) {
	mapper := defaultMapper
	arg := map[string]interface{}{"name": "alice", "age": 30}
	names := []string{"name", "age"}

	args, err := bindArgs(names, arg, mapper)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if args[0] != "alice" {
		t.Errorf("args[0] = %v, want alice", args[0])
	}
	if args[1] != 30 {
		t.Errorf("args[1] = %v, want 30", args[1])
	}
}

func TestBindArgs_Struct(t *testing.T) {
	type User struct {
		Name string `db:"name"`
		Age  int    `db:"age"`
	}
	mapper := defaultMapper
	arg := User{Name: "bob", Age: 25}
	names := []string{"name", "age"}

	args, err := bindArgs(names, arg, mapper)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if args[0] != "bob" {
		t.Errorf("args[0] = %v, want bob", args[0])
	}
	if args[1] != 25 {
		t.Errorf("args[1] = %v, want 25", args[1])
	}
}

func TestBindArgs_MissingKey(t *testing.T) {
	mapper := defaultMapper
	arg := map[string]interface{}{"name": "alice"}
	names := []string{"name", "missing"}

	_, err := bindArgs(names, arg, mapper)
	if err == nil {
		t.Fatal("expected error for missing key")
	}
}
