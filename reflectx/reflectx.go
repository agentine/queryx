// Package reflectx provides struct field mapping with caching for queryx.
//
// It maps database column names to struct fields using struct tags, with
// support for embedded structs and aggressive caching to minimize reflection
// in hot paths.
package reflectx
