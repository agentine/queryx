package queryx

import (
	"fmt"
	"reflect"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/agentine/queryx/reflectx"
)

// Rebind transforms a query with ? placeholders to the target bind type.
func Rebind(bindType BindType, query string) string {
	switch bindType {
	case QUESTION, UNKNOWN:
		return query
	case DOLLAR:
		return rebindDollar(query)
	case AT:
		return rebindAt(query)
	case NAMED:
		return query // named queries are already in :name format
	}
	return query
}

func rebindDollar(query string) string {
	var b strings.Builder
	b.Grow(len(query) + 16)
	n := 0
	for i := 0; i < len(query); i++ {
		ch := query[i]
		if ch == '?' {
			n++
			b.WriteByte('$')
			b.WriteString(strconv.Itoa(n))
		} else {
			b.WriteByte(ch)
		}
	}
	return b.String()
}

func rebindAt(query string) string {
	var b strings.Builder
	b.Grow(len(query) + 16)
	n := 0
	for i := 0; i < len(query); i++ {
		ch := query[i]
		if ch == '?' {
			n++
			b.WriteString("@p")
			b.WriteString(strconv.Itoa(n))
		} else {
			b.WriteByte(ch)
		}
	}
	return b.String()
}

// In expands slice arguments in a query with ? placeholders.
// For example:
//
//	In("SELECT * FROM t WHERE id IN (?)", []int{1, 2, 3})
//
// returns:
//
//	"SELECT * FROM t WHERE id IN (?, ?, ?)", [1, 2, 3]
func In(query string, args ...interface{}) (string, []interface{}, error) {
	// Find which args are slices.
	newArgs := make([]interface{}, 0, len(args))
	var b strings.Builder
	b.Grow(len(query))

	argIdx := 0
	for i := 0; i < len(query); i++ {
		if query[i] == '?' {
			if argIdx >= len(args) {
				return "", nil, fmt.Errorf("queryx.In: more placeholders than arguments")
			}
			arg := args[argIdx]
			argIdx++

			v := reflect.ValueOf(arg)
			if v.Kind() == reflect.Slice {
				if v.Len() == 0 {
					return "", nil, fmt.Errorf("queryx.In: empty slice passed for IN clause")
				}
				for j := 0; j < v.Len(); j++ {
					if j > 0 {
						b.WriteString(", ")
					}
					b.WriteByte('?')
					newArgs = append(newArgs, v.Index(j).Interface())
				}
			} else {
				b.WriteByte('?')
				newArgs = append(newArgs, arg)
			}
		} else {
			b.WriteByte(query[i])
		}
	}
	if argIdx != len(args) {
		return "", nil, fmt.Errorf("queryx.In: more arguments than placeholders")
	}
	return b.String(), newArgs, nil
}

// Named parameter support

// compileNamedQuery parses a query with :name placeholders and returns
// a query with bind-type-appropriate placeholders and the list of parameter names.
func compileNamedQuery(bindType BindType, query string) (string, []string, error) {
	var b strings.Builder
	b.Grow(len(query))
	var names []string
	n := 0

	i := 0
	for i < len(query) {
		ch, size := utf8.DecodeRuneInString(query[i:])

		// Skip string literals.
		if ch == '\'' {
			j := i + size
			for j < len(query) {
				if query[j] == '\'' {
					if j+1 < len(query) && query[j+1] == '\'' {
						j += 2 // escaped quote
						continue
					}
					j++
					break
				}
				j++
			}
			b.WriteString(query[i:j])
			i = j
			continue
		}

		// Skip line comments.
		if ch == '-' && i+1 < len(query) && query[i+1] == '-' {
			j := i + 2
			for j < len(query) && query[j] != '\n' {
				j++
			}
			b.WriteString(query[i:j])
			i = j
			continue
		}

		// Skip block comments.
		if ch == '/' && i+1 < len(query) && query[i+1] == '*' {
			j := i + 2
			for j+1 < len(query) {
				if query[j] == '*' && query[j+1] == '/' {
					j += 2
					break
				}
				j++
			}
			if j+1 >= len(query) && !(j < len(query) && query[j-1] == '/') {
				j = len(query)
			}
			b.WriteString(query[i:j])
			i = j
			continue
		}

		// Postgres cast (::) — not a named param.
		if ch == ':' && i+1 < len(query) && query[i+1] == ':' {
			b.WriteString("::")
			i += 2
			continue
		}

		// Named parameter.
		if ch == ':' {
			j := i + size
			for j < len(query) {
				c := query[j]
				if (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9') || c == '_' {
					j++
				} else {
					break
				}
			}
			name := query[i+size : j]
			if name == "" {
				return "", nil, fmt.Errorf("queryx: empty named parameter at position %d", i)
			}
			names = append(names, name)
			n++
			switch bindType {
			case DOLLAR:
				b.WriteByte('$')
				b.WriteString(strconv.Itoa(n))
			case AT:
				b.WriteString("@p")
				b.WriteString(strconv.Itoa(n))
			case NAMED:
				b.WriteByte(':')
				b.WriteString(name)
			default:
				b.WriteByte('?')
			}
			i = j
			continue
		}

		b.WriteString(query[i : i+size])
		i += size
	}
	return b.String(), names, nil
}

// bindNamedMapper binds named parameters from a struct or map argument.
func bindNamedMapper(bindType BindType, query string, arg interface{}, mapper *reflectx.Mapper) (string, []interface{}, error) {
	compiled, names, err := compileNamedQuery(bindType, query)
	if err != nil {
		return "", nil, err
	}
	args, err := bindArgs(names, arg, mapper)
	if err != nil {
		return "", nil, err
	}
	return compiled, args, nil
}

// bindArgs extracts argument values from a struct or map for the given names.
func bindArgs(names []string, arg interface{}, mapper *reflectx.Mapper) ([]interface{}, error) {
	v := reflect.ValueOf(arg)
	if v.Kind() == reflect.Ptr {
		v = v.Elem()
	}

	args := make([]interface{}, len(names))
	switch v.Kind() {
	case reflect.Map:
		for i, name := range names {
			val := v.MapIndex(reflect.ValueOf(name))
			if !val.IsValid() {
				return nil, fmt.Errorf("queryx: named parameter %q not found in map", name)
			}
			args[i] = val.Interface()
		}
	case reflect.Struct:
		sm := mapper.TypeMap(v.Type())
		for i, name := range names {
			fi, ok := sm.Names[name]
			if !ok {
				return nil, fmt.Errorf("queryx: named parameter %q not found in struct %s", name, v.Type())
			}
			f := reflectx.FieldByIndexesReadOnly(v, fi.Index)
			if !f.IsValid() {
				args[i] = nil
			} else {
				args[i] = f.Interface()
			}
		}
	default:
		return nil, fmt.Errorf("queryx: unsupported arg type %T for named query; expected struct or map", arg)
	}
	return args, nil
}
