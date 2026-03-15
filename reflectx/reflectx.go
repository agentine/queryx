// Package reflectx provides struct field mapping with caching for queryx.
//
// It maps database column names to struct fields using struct tags, with
// support for embedded structs and aggressive caching to minimize reflection
// in hot paths.
package reflectx

import (
	"reflect"
	"strings"
	"sync"
)

// Mapper maps struct field names to database column names using struct tags.
// It caches type information to minimize reflection overhead.
type Mapper struct {
	tagName string
	cache   sync.Map // map[reflect.Type]*StructMap
}

// NewMapper returns a new Mapper that reads the given struct tag.
func NewMapper(tagName string) *Mapper {
	return &Mapper{tagName: tagName}
}

// FieldMap returns a map of column names to field indices for the given type.
// Results are cached for subsequent calls with the same type.
func (m *Mapper) FieldMap(t reflect.Type) map[string][]int {
	sm := m.TypeMap(t)
	result := make(map[string][]int, len(sm.Names))
	for name, fi := range sm.Names {
		result[name] = fi.Index
	}
	return result
}

// TypeMap returns the full struct mapping for the given type.
func (m *Mapper) TypeMap(t reflect.Type) *StructMap {
	if t.Kind() == reflect.Ptr {
		t = t.Elem()
	}
	if v, ok := m.cache.Load(t); ok {
		return v.(*StructMap)
	}
	sm := m.mapType(t)
	m.cache.Store(t, sm)
	return sm
}

// FieldByIndexes returns the nested field value for the given index chain,
// allocating intermediate pointers as needed.
func FieldByIndexes(v reflect.Value, indexes []int) reflect.Value {
	for _, i := range indexes {
		v = reflect.Indirect(v)
		v = v.Field(i)
	}
	return v
}

// FieldByIndexesReadOnly returns the nested field value for the given index
// chain without allocating intermediate pointers. It returns the zero Value
// if a nil pointer is encountered.
func FieldByIndexesReadOnly(v reflect.Value, indexes []int) reflect.Value {
	for _, i := range indexes {
		v = reflect.Indirect(v)
		if !v.IsValid() {
			return v
		}
		v = v.Field(i)
	}
	return v
}

// StructMap contains the mapping information for a struct type.
type StructMap struct {
	// Tree is the root of the field tree.
	Tree *FieldInfo
	// Names maps column names to field info.
	Names map[string]*FieldInfo
	// Index maps field index paths to field info.
	Index []*FieldInfo
}

// FieldInfo represents a mapped struct field.
type FieldInfo struct {
	// Index is the reflect index chain to reach this field.
	Index []int
	// Path is the dot-separated path of field names.
	Path string
	// Name is the mapped column name (from tag or field name).
	Name string
	// Field is the reflect.StructField.
	Field reflect.StructField
	// Zero is the zero value for this field's type.
	Zero reflect.Value
	// Options are the parsed tag options (e.g., "omitempty").
	Options map[string]string
	// Embedded indicates this field is an embedded struct.
	Embedded bool
	// Children are the fields of an embedded struct.
	Children []*FieldInfo
	// Parent is the parent field info, if this is nested.
	Parent *FieldInfo
}

func (m *Mapper) mapType(t reflect.Type) *StructMap {
	sm := &StructMap{
		Tree:  &FieldInfo{},
		Names: map[string]*FieldInfo{},
	}
	m.mapFields(t, sm, sm.Tree, nil, "")
	return sm
}

func (m *Mapper) mapFields(t reflect.Type, sm *StructMap, parent *FieldInfo, index []int, prefix string) {
	if t.Kind() == reflect.Ptr {
		t = t.Elem()
	}
	for i := 0; i < t.NumField(); i++ {
		f := t.Field(i)
		if !f.IsExported() {
			continue
		}

		fi := &FieldInfo{
			Field:  f,
			Parent: parent,
			Zero:   reflect.New(f.Type).Elem(),
		}

		// Build index chain.
		fi.Index = make([]int, len(index)+1)
		copy(fi.Index, index)
		fi.Index[len(index)] = i

		// Parse tag.
		name, opts := m.parseTag(f)
		if name == "-" {
			continue
		}
		fi.Options = opts

		// Handle embedded structs.
		ft := f.Type
		if ft.Kind() == reflect.Ptr {
			ft = ft.Elem()
		}
		if f.Anonymous && ft.Kind() == reflect.Struct && name == "" {
			fi.Embedded = true
			fi.Name = f.Name
			fi.Path = joinPath(prefix, f.Name)
			parent.Children = append(parent.Children, fi)
			sm.Index = append(sm.Index, fi)
			m.mapFields(ft, sm, fi, fi.Index, fi.Path)
			continue
		}

		// Determine column name.
		if name != "" {
			fi.Name = name
		} else {
			fi.Name = strings.ToLower(f.Name)
		}
		fi.Path = joinPath(prefix, fi.Name)

		parent.Children = append(parent.Children, fi)
		sm.Index = append(sm.Index, fi)

		// Only the first field with a given name wins.
		if _, exists := sm.Names[fi.Name]; !exists {
			sm.Names[fi.Name] = fi
		}
	}
}

func (m *Mapper) parseTag(f reflect.StructField) (string, map[string]string) {
	tag := f.Tag.Get(m.tagName)
	if tag == "" {
		return "", nil
	}
	parts := strings.Split(tag, ",")
	name := parts[0]
	opts := make(map[string]string, len(parts)-1)
	for _, p := range parts[1:] {
		kv := strings.SplitN(p, "=", 2)
		k := strings.TrimSpace(kv[0])
		if len(kv) == 2 {
			opts[k] = strings.TrimSpace(kv[1])
		} else {
			opts[k] = ""
		}
	}
	return name, opts
}

func joinPath(prefix, name string) string {
	if prefix == "" {
		return name
	}
	return prefix + "." + name
}

// TraversalsByName returns a slice of index-chains for each name in the given
// column list, using the mapper's type information. If a name is not found,
// the corresponding entry is nil.
func (m *Mapper) TraversalsByName(t reflect.Type, names []string) [][]int {
	sm := m.TypeMap(t)
	result := make([][]int, len(names))
	for i, name := range names {
		if fi, ok := sm.Names[name]; ok {
			result[i] = fi.Index
		}
	}
	return result
}

// TraversalsByNameFunc is like TraversalsByName but applies fn to normalize
// column names before lookup.
func (m *Mapper) TraversalsByNameFunc(t reflect.Type, names []string, fn func(string) string) [][]int {
	// Build a lookup with normalized keys.
	sm := m.TypeMap(t)
	norm := make(map[string]*FieldInfo, len(sm.Names))
	for k, v := range sm.Names {
		norm[fn(k)] = v
	}
	result := make([][]int, len(names))
	for i, name := range names {
		if fi, ok := norm[fn(name)]; ok {
			result[i] = fi.Index
		}
	}
	return result
}
