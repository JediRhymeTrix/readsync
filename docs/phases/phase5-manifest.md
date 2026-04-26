# Phase 5 Manifest — Goodreads Bridge

> Generated: 2026-04-25
> Depends on: Phase 2 Calibre adapter.

Phase 5 implements the Goodreads bridge as an OPTIONAL companion path that
goes through the user's existing Calibre Goodreads Sync plugin. ReadSync
NEVER calls the Goodreads API, NEVER scrapes Goodreads, and NEVER vendors
any plugin code (GPL-3.0).

---

## Files Created (Phase 5)

| File | Purpose |
|------|---------|
| `internal/adapters/goodreads_bridge/goodreads.go` | Adapter struct: Adapter, EventEmitter, WriteTarget. |
| `internal/adapters/goodreads_bridge/modes.go` | BridgeMode enum: disabled, manual-plugin, guided-plugin, companion-plugin, experimental-direct. |
| `internal/adapters/goodreads_bridge/detect.go` | Plugin detection by scanning `%APPDATA%\calibre\plugins\`. |
| `internal/adapters/goodreads_bridge/identifier.go` | `BuildMissingIDReport` — list books lacking a Goodreads identifier. |
| `internal/adapters/goodreads_bridge/stale.go` | Stale-state detection (spec §6) + writeback safety gate (spec §8). |
| `internal/adapters/goodreads_bridge/bridge_test.go` | 37 unit tests. |
| `internal/adapters/goodreads_bridge/integration_test.go` | End-to-end test. Skipped when calibredb is missing or Calibre is running. |

---

## Definition of Done

- [x] ReadSync canonical progress consistently lands in `#readsync_progress`.
- [x] Goodreads-derived events never overwrite a fresher local reader event.
- [x] Clear user-facing log lines and `goodreads_bridge_stale` conflict reason.
- [x] No Goodreads API calls, no scraping, no plugin code loaded.
- [x] All 37 unit tests pass; integration test runs or skips cleanly.
