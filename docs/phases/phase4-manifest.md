# Phase 4 Deliverable Manifest

> Generated: 2026-04-25
> Depends on: Phase 1 core, Phase 0 Moon+ fixtures.

Implements the Moon+ Reader Pro adapter per master spec section 11.

---

## Files Created / Updated

| File | Purpose |
|------|---------|
| `internal/adapters/webdav/webdav.go` | Embedded WebDAV server, HTTP Basic auth, per-user namespace, observer registry. |
| `internal/adapters/webdav/archive.go` | Versioned immutable archival. `archiveUpload`, `LatestVersion`, path sanitisation. |
| `internal/adapters/webdav/versioned_fs.go` | `webdav.FileSystem` wrapper that tees every PUT into immutable archive on Close. |
| `internal/adapters/webdav/webdav_test.go` | Litmus-style conformance tests. |
| `internal/adapters/moon/moon.go` | `Adapter` orchestrator (Layer 1–4 wiring, health, lifecycle). |
| `internal/adapters/moon/parser.go` | Layer 3 read-only progress extractor. `FormatV1Plain`, `Parse`, `ToAdapterEvent`. |
| `internal/adapters/moon/writeback.go` | Layer 4 safe writeback generator. `writerRegistry`, `IsWriterVerified`, `SerializeV1Plain`. |
| `internal/adapters/moon/capture.go` | Layer 2 in-process fixture recorder. `EnableCapture`, `DisableCapture`. |
| `internal/adapters/moon/setup.go` | Setup-wizard integration. `GenerateSetup`, `TestConnection`. |
| `internal/adapters/moon/upload.go` | Upload observer driving Layer 2+3 from each archived PUT. |
| `internal/adapters/moon/parser_test.go` | Fixture-driven parser tests. |
| `internal/adapters/moon/writeback_test.go` | Round-trip writer tests. |
| `internal/adapters/moon/moon_test.go` | End-to-end integration: happy path, unknown format, capture, setup bundle. |
| `internal/db/migrations.go` | Migration 3: `moon_users`, `moon_uploads` tables. |
| `cmd/readsync-service/main_service.go` | Wires Moon+ adapter at startup (port 8765). |

---

## Definition of Done

- [x] Moon+ Pro on Android can connect to ReadSync's WebDAV.
- [x] A pause-then-resume cycle produces at least one `ProgressEvent`.
- [x] Unknown format files stored versioned and never mutated.
- [x] No credentials in logs.
- [x] No in-place mutation of any uploaded file (O_CREATE|O_EXCL|O_WRONLY).
