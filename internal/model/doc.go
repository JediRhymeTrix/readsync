// Package model defines all ReadSync domain types.
//
// All core structs (Book, ProgressEvent, CanonicalProgress, OutboxJob,
// Conflict, AdapterHealth, BookAlias) and their enumerations (Source,
// ReadStatus, LocationType, OutboxStatus, ConflictStatus, AdapterHealthState)
// live here.
//
// This package contains no business logic. All struct fields use `db:` tags
// for direct SQLite row scanning. Pointer fields represent nullable columns.
//
// Design note: no type in this package may import any other internal package.
// It is the root of the dependency graph.
package model
