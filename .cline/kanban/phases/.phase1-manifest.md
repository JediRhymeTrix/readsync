# Phase 1 Deliverable Manifest

> Generated: 2026-04-25

---

## Module Root

| File | Status |
|------|--------|
| `go.mod` | ✅ Created |
| `Makefile` (phase 1 targets appended) | ✅ Updated |

---

## 1. Repo Layout (spec §12)

| Package | Status |
|---------|--------|
| `cmd/readsync-service/main.go` | ✅ Created |
| `cmd/readsync-tray/main.go` | ✅ Stub |
| `cmd/readsyncctl/main.go` | ✅ Created |
| `cmd/readsyncctl/commands/db.go` | ✅ Created |
| `cmd/readsyncctl/commands/status.go` | ✅ Created |
| `internal/core/pipeline.go` | ✅ Created |
| `internal/core/pipeline_helpers.go` | ✅ Created |
| `internal/core/pipeline_db.go` | ✅ Created |
| `internal/core/pipeline_test.go` | ✅ Created |
| `internal/db/db.go` | ✅ Created |
| `internal/db/migrations.go` | ✅ Created |
| `internal/model/model.go` | ✅ Created |
| `internal/model/types.go` | ✅ Created |
| `internal/resolver/resolver.go` | ✅ Created |
| `internal/resolver/resolver_test.go` | ✅ Created |
| `internal/conflicts/engine.go` | ✅ Created |
| `internal/conflicts/engine_test.go` | ✅ Created |
| `internal/outbox/worker.go` | ✅ Created |
| `internal/outbox/worker_test.go` | ✅ Created |
| `internal/logging/logging.go` | ✅ Created |
| `internal/logging/redact.go` | ✅ Created |
| `internal/logging/redact_test.go` | ✅ Created |
| `internal/secrets/secrets.go` | ✅ Created |
| `internal/adapters/adapter.go` | ✅ Created |
| `internal/adapters/fake/fake.go` | ✅ Created |
| `internal/adapters/calibre/calibre.go` | ✅ Stub |
| `internal/adapters/goodreads_bridge/goodreads.go` | ✅ Stub |
| `internal/adapters/koreader/koreader.go` | ✅ Stub |
| `internal/adapters/moon/moon.go` | ✅ Stub |
| `internal/adapters/webdav/webdav.go` | ✅ Stub |
| `internal/setup/setup.go` | ✅ Stub |
| `internal/diagnostics/diagnostics.go` | ✅ Created |
| `internal/repair/repair.go` | ✅ Created |
| `internal/api/server.go` | ✅ Created |
| `internal/ui/ui.go` | ✅ Stub |

---

## 2. SQLite Schema - 7 tables, WAL mode (spec §4) ✅

## 3. Domain Model - all types + enums (spec §4) ✅

## 4. Identity Resolver - 10-signal ladder, fuzzy match, tests (spec §5) ✅

## 5. Event Pipeline - single-writer goroutine, E2E test (spec §4) ✅

## 6. Conflict Engine - precedence, 5 detectors, auto-resolve gate (spec §6) ✅

## 7. Outbox Worker - 8 states, exp backoff+jitter, fair scheduler ✅

## 8. Logging - 2 streams, rotation, redaction tested ✅

## 9. Health/Self-Healing - diagnostics, repair, port picker ✅

## 10. readsyncctl CLI - 7 commands ✅

## 11. Local Admin API - 127.0.0.1, CSRF, 6 endpoints ✅

---

## Definition of Done Checklist

- [x] `readsyncctl db migrate` creates schema
- [x] Fake adapter → pipeline → canonical_progress → outbox in-memory test
- [x] Resolver scoring unit tests (15 cases)
- [x] Conflict precedence + detection unit tests
- [x] No secrets logged; redaction unit-tested
- [ ] Idle CPU ~0 (verify after `go run ./cmd/readsync-service run`)
- [ ] Idle RAM < 50 MB (verify after binary starts)

---

## Build

```powershell
# Prereq: Go 1.22+, TDM-GCC (for CGO/sqlite3)
go mod tidy
go test -v ./internal/resolver/... ./internal/conflicts/... ./internal/logging/...
go test -v ./...
go run ./cmd/readsyncctl db migrate --db test.db
```
