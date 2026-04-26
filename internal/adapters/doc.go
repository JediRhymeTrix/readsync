// Package adapters defines the interfaces all ReadSync sync adapters must implement.
//
// # Interfaces
//
// Adapter is the base interface: Source, Start, Stop, Health.
//
// EventEmitter extends Adapter for adapters that push events into the pipeline
// (KOReader, Moon+, Calibre watcher, Fake). Call SetPipeline before Start.
//
// WriteTarget extends Adapter for adapters that receive canonical progress
// and write it back to the underlying system (Calibre, Goodreads bridge).
//
// # Adapter implementations
//
//   - internal/adapters/calibre/   — calibredb subprocess + OPF parser
//   - internal/adapters/koreader/  — KOSync-compatible HTTP server (Gin)
//   - internal/adapters/moon/      — Moon+ WebDAV server + .po parser
//   - internal/adapters/goodreads/ — Goodreads bridge (Calibre plugin column)
//   - internal/adapters/fake/      — Scripted fake adapter for tests
//
// # CGO note
//
// This package (adapter.go) is pure Go. Calibre and KOReader sub-packages
// require CGO (via internal/core → internal/db → go-sqlite3). Moon+,
// calibre/opf, and koreader/codec are CGO-free subpackages.
package adapters
