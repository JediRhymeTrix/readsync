# GitHub Copilot Instructions for ReadSync

## Project overview

**ReadSync** (`github.com/readsync/readsync`) is a self-hosted Windows
background service (Go 1.22+, SQLite, CGO) that synchronises reading progress
between KOReader, Moon+ Reader Pro, Calibre, and Goodreads.

Full developer reference: [AGENTS.md](../AGENTS.md)

---

## Repository conventions

### Go
- Module: `github.com/readsync/readsync`
- Format: `gofmt` / `goimports` (CI-enforced)
- Error strings: lowercase, no trailing punctuation
- No `panic()` in production paths
- Test files: `*_test.go`; black-box tests use `package <pkg>_test`

### Architecture
- All domain types live in `internal/model/model.go` — do not scatter types.
- The event pipeline (`internal/core/pipeline.go`) is the **only** writer to `canonical_progress`, `progress_events`, `books`, and `conflicts`. Always route events through `core.Pipeline.Submit()`.
- Outbox (`internal/outbox/worker.go`): retry schedule 5s × 2^n ± 20% jitter, capped at 2 hours, 10 attempts max.

### Security (always enforced)
- Admin UI binds to `127.0.0.1:7201` only.
- Every write endpoint requires `X-ReadSync-CSRF` header.
- Credentials go through `internal/secrets/` — never hardcode.
- New secret-like field names: add to `internal/logging/redact.go`.
- No HTTP calls to `goodreads.com`.

### Database migrations
- File: `internal/db/migrations.go`
- **NEVER** modify existing entries — append only.
- All DDL: `CREATE TABLE IF NOT EXISTS`, `CREATE INDEX IF NOT EXISTS`.

---

## CGO constraint

| Package (no CGO) | CGO required |
|-----------------|-------------|
| `internal/adapters/calibre/opf/` | `internal/adapters/calibre/` (parent) |
| `internal/adapters/koreader/codec/` | `internal/adapters/koreader/` (parent) |
| `internal/adapters/moon/` | `internal/core/`, `internal/db/` |
| `internal/resolver/`, `internal/conflicts/`, `internal/logging/` | `tests/integration/` |
| `internal/setup/`, `internal/repair/`, `internal/secrets/`, `internal/api/` | |

Do not add CGO-requiring imports to the no-CGO packages above.

---

## Suggested completions

When completing code in:
- `internal/adapters/`: implement both `Adapter` and `EventEmitter` or `WriteTarget` interfaces from `internal/adapters/adapter.go`.
- `internal/api/routes.go`: new write routes need `requireCSRF(s.csrfToken, ...)`.
- `internal/db/migrations.go`: append a new `{version: N+1, sql: ...}` entry.
- Any new log calls: use `internal/logging/Logger`, never `log.Printf`.
- Any credential read/write: use `internal/secrets/Store` interface.

---

## Test targets

```powershell
make test-unit          # no CGO — fast
make test-security      # CSRF + redaction
make test-integration   # CGO required
make test               # all
```
