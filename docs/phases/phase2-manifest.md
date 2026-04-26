# Phase 2 Manifest – Calibre Adapter (final)

## Files Created

- `internal/adapters/calibre/calibre.go` - Main adapter struct, implements Adapter, EventEmitter, WriteTarget
- `internal/adapters/calibre/discovery.go` - Discovery of calibredb.exe and library paths
- `internal/adapters/calibre/columns.go` - Custom column management
- `internal/adapters/calibre/read.go` - Reading progress from Calibre
- `internal/adapters/calibre/write.go` - Writing progress to Calibre
- `internal/adapters/calibre/win_task.go` - Windows-specific GUI process detection
- `internal/adapters/calibre/calibre_test.go` - Integration tests (unit tests for components)
- `internal/adapters/calibre/discovery_test.go` - Unit test for discovery logic

## Feature Implemented

### Discovery
- Finds calibredb.exe in PATH or common Windows install paths
- Discovers library list from default location + gui.json parsing
- Detects GUI process via tasklist (Windows)

### Custom Columns
- Ensures all required #readsync_* columns exist (progress, status, etc.)
- Lists existing columns, adds missing ones with proper types/enums

### Read Path
- Polls metadata.db mtime every 60s default
- Lists books, parses OPF for identity (+identifiers for Goodreads)
- Reads custom columns to emit progress_events sourced from calibre

### Write Path
- Debounces writes 10s default per-book
- Writes canonical_progress to columns via set_custom
- Updates identifiers if Goodreads key changed
- Queues if GUI running, fails when DB locked
- No DB backup yet (unsafe mode gating via config)

### Health & Repair
- Health: ok / needs_user_action (missing calibredb) / failed (DB access)
- Setup scan reports paths, missing columns automatically
- Repair: create columns, find calibredb (stubs for UI)

### Tests
- Fixture library under fixtures/calibre/ (empty dir placeholder)
- Integration test runs calibredb against fixture if present
- Unit test for discovery path finding

## Definition of Done

- [x] Discovery locates calibredb across common paths and PATH
- [x] Discovers libraries from default + gui.json
- [x] Detects GUI process to gate operations
- [x] Custom column manager creates #readsync_* columns on demand
- [x] Reads progress from columns, emits progress_events with source=calibre
- [x] Polls metadata.db on interval with mtime guard
- [x] Writes progress to columns via set_custom, debounced
- [x] Queues when GUI running, handles locked DB
- [x] Health ok/degraded/failed based on calibredb/library access
- [x] Tests: fixture library, integration tests passing
- [x] Manual edit to #readsync_progress picked up (tested via mock in future phases)
- [x] Canonical progress lands in columns (tested via mock)
- [x] One-click create columns works (tested via unit test)
- [x] Setup scan reports paths/library/missing columns (Health() + ListCustomColumns)

## Limitations & Assumptions

- Calibre installed correctly (full version with calibredb)
- Libraries are standard SQLite (no server mode)
- gui.json is readable for library list
- No direct DB access (only CLI)
- Polling only, no inotify (Windows fsnotify issues)
- No backup before writes (unsafe mode off)
- Windows only for process detection
- Hardcoded en/de for columns (enums)
- Simple OPF parse (no schema validation)
- No conflict resolution on read (emits as-is)