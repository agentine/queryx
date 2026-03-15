package reflectx

import (
	"reflect"
	"strings"
	"testing"
)

type Simple struct {
	ID   int    `db:"id"`
	Name string `db:"name"`
}

type WithIgnored struct {
	ID      int    `db:"id"`
	Ignored string `db:"-"`
	Name    string `db:"name"`
}

type Embedded struct {
	Simple
	Email string `db:"email"`
}

type DeepEmbedded struct {
	Embedded
	Phone string `db:"phone"`
}

type WithPointerEmbed struct {
	*Simple
	Extra string `db:"extra"`
}

type NoTags struct {
	ID   int
	Name string
}

type WithOptions struct {
	ID   int    `db:"id,omitempty"`
	Name string `db:"name,readonly"`
}

type Conflict struct {
	ID int `db:"id"`
	// unexported field, should be skipped
	hidden int //nolint:unused
	Name   string `db:"name"`
}

func TestMapperFieldMap(t *testing.T) {
	m := NewMapper("db")
	fm := m.FieldMap(reflect.TypeOf(Simple{}))

	if _, ok := fm["id"]; !ok {
		t.Error("expected 'id' in field map")
	}
	if _, ok := fm["name"]; !ok {
		t.Error("expected 'name' in field map")
	}
	if len(fm) != 2 {
		t.Errorf("expected 2 fields, got %d", len(fm))
	}
}

func TestMapperIgnored(t *testing.T) {
	m := NewMapper("db")
	fm := m.FieldMap(reflect.TypeOf(WithIgnored{}))

	if _, ok := fm["Ignored"]; ok {
		t.Error("field with db:\"-\" should be ignored")
	}
	if len(fm) != 2 {
		t.Errorf("expected 2 fields, got %d", len(fm))
	}
}

func TestMapperEmbedded(t *testing.T) {
	m := NewMapper("db")
	fm := m.FieldMap(reflect.TypeOf(Embedded{}))

	for _, name := range []string{"id", "name", "email"} {
		if _, ok := fm[name]; !ok {
			t.Errorf("expected %q in field map", name)
		}
	}
}

func TestMapperDeepEmbedded(t *testing.T) {
	m := NewMapper("db")
	fm := m.FieldMap(reflect.TypeOf(DeepEmbedded{}))

	for _, name := range []string{"id", "name", "email", "phone"} {
		if _, ok := fm[name]; !ok {
			t.Errorf("expected %q in field map", name)
		}
	}
}

func TestMapperNoTags(t *testing.T) {
	m := NewMapper("db")
	fm := m.FieldMap(reflect.TypeOf(NoTags{}))

	// Without tags, field names are lowercased.
	if _, ok := fm["id"]; !ok {
		t.Error("expected 'id' in field map")
	}
	if _, ok := fm["name"]; !ok {
		t.Error("expected 'name' in field map")
	}
}

func TestMapperOptions(t *testing.T) {
	m := NewMapper("db")
	sm := m.TypeMap(reflect.TypeOf(WithOptions{}))

	fi := sm.Names["id"]
	if fi == nil {
		t.Fatal("expected 'id' in type map")
	}
	if _, ok := fi.Options["omitempty"]; !ok {
		t.Error("expected 'omitempty' option on 'id'")
	}

	fi = sm.Names["name"]
	if fi == nil {
		t.Fatal("expected 'name' in type map")
	}
	if _, ok := fi.Options["readonly"]; !ok {
		t.Error("expected 'readonly' option on 'name'")
	}
}

func TestMapperCaching(t *testing.T) {
	m := NewMapper("db")
	sm1 := m.TypeMap(reflect.TypeOf(Simple{}))
	sm2 := m.TypeMap(reflect.TypeOf(Simple{}))
	if sm1 != sm2 {
		t.Error("expected same StructMap instance from cache")
	}
}

func TestMapperPointerType(t *testing.T) {
	m := NewMapper("db")
	fm := m.FieldMap(reflect.TypeOf(&Simple{}))

	if _, ok := fm["id"]; !ok {
		t.Error("expected 'id' in field map for pointer type")
	}
}

func TestFieldByIndexes(t *testing.T) {
	e := Embedded{
		Simple: Simple{ID: 42, Name: "alice"},
		Email:  "alice@example.com",
	}
	v := reflect.ValueOf(&e).Elem()

	m := NewMapper("db")
	sm := m.TypeMap(reflect.TypeOf(Embedded{}))

	fi := sm.Names["id"]
	fv := FieldByIndexes(v, fi.Index)
	if fv.Int() != 42 {
		t.Errorf("expected id=42, got %d", fv.Int())
	}

	fi = sm.Names["email"]
	fv = FieldByIndexes(v, fi.Index)
	if fv.String() != "alice@example.com" {
		t.Errorf("expected email=alice@example.com, got %s", fv.String())
	}
}

func TestFieldByIndexesReadOnly_NilPointer(t *testing.T) {
	v := reflect.ValueOf(&WithPointerEmbed{Extra: "x"}).Elem()
	m := NewMapper("db")
	sm := m.TypeMap(reflect.TypeOf(WithPointerEmbed{}))

	fi := sm.Names["id"]
	fv := FieldByIndexesReadOnly(v, fi.Index)
	if fv.IsValid() {
		t.Error("expected invalid Value for nil pointer embed")
	}
}

func TestTraversalsByName(t *testing.T) {
	m := NewMapper("db")
	typ := reflect.TypeOf(Simple{})
	traversals := m.TraversalsByName(typ, []string{"id", "name", "missing"})

	if traversals[0] == nil {
		t.Error("expected non-nil traversal for 'id'")
	}
	if traversals[1] == nil {
		t.Error("expected non-nil traversal for 'name'")
	}
	if traversals[2] != nil {
		t.Error("expected nil traversal for 'missing'")
	}
}

func TestTraversalsByNameFunc(t *testing.T) {
	m := NewMapper("db")
	typ := reflect.TypeOf(Simple{})

	// Normalize to lowercase for case-insensitive matching.
	fn := strings.ToLower

	traversals := m.TraversalsByNameFunc(typ, []string{"ID", "NAME"}, fn)
	if traversals[0] == nil {
		t.Error("expected non-nil traversal for 'ID' (normalized)")
	}
	if traversals[1] == nil {
		t.Error("expected non-nil traversal for 'NAME' (normalized)")
	}
}

func TestUnexportedFieldsSkipped(t *testing.T) {
	m := NewMapper("db")
	fm := m.FieldMap(reflect.TypeOf(Conflict{}))

	if len(fm) != 2 {
		t.Errorf("expected 2 fields (unexported skipped), got %d", len(fm))
	}
}
