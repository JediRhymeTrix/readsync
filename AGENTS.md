# AGENTS.md — AI Coding Agent Guide for ReadSync

> Primary reference for AI coding agents (Claude, Copilot, Cursor, Cline, etc.)
> working on this repository. Human contributors: see [CONTRIBUTING.md](CONTRIBUTING.md).

---

## 1. Project Summary

**ReadSync** (`github.com/readsync/readsync`) is a Windows background service
(Go 1.25+) that synchronises reading progress between:

| App | Protocol | Port |
|-----|----------|------|
| KOReader (e-ink/Android) | KOSync HTTP API | 7200 |
| Moon+ Reader Pro (Android) | WebDAV (`.po` files) | 8765 |
| Calibre (Windows) | `calibredb` subprocess + OPF XML | — |
| Goodreads | Calibre plugin column (`#readsync_progress`) | — |

**No Goodreads API key. No cloud.** Everything runs locally on Windows 10/11.

Module: `github.com/readsync/readsync` | Go: 1.25+ | SQLite via `go-sqlite3` (CGO)

---

## 2. Repository Layout

```
ReadSync/
├── cmd/
│   ├── readsync-service/     Windows Service binary (kardianos/service)
│   ├── readsyncctl/          CLI: status, adapters, conflicts, outbox, db, diagnostics
│   └── readsync-tray/        System tray icon (native Win32 syscall)
├── internal/
│   ├── model/model.go        ALL domain types and enums (no logic)
│   ├── db/                   SQLite open/close + migrations (APPEND ONLY)
│   ├── core/pipeline.go      Single-writer event goroutine (spec §4)
│   ├── resolver/resolver.go  Score() + Band() + EvidenceQuality() + Jaro-Winkler
│   ├── conflicts/engine.go   DetectSuspiciousJump() + CanAutoResolve() + ChooseWinner()
│   ├── outbox/worker.go      Worker + FairScheduler + SQLStore + NextRetry()
│   ├── logging/              Logger (activity + JSONL) + redact.go
│   ├── secrets/              Store interface + DPAPI (Windows) + MemStore fallback
│   ├── api/                  Admin HTTP server (127.0.0.1:7201) + CSRF middleware
│   ├── setup/                10-step wizard state machine + SystemScan
│   ├── repair/               12 repair actions
│   ├── diagnostics/          Health snapshot + bundle exporter
│   └── adapters/
│       ├── adapter.go        Adapter / EventEmitter / WriteTarget interfaces
│       ├── calibre/          calibredb subprocess adapter
│       │   └── opf/          Pure-Go OPF parser (NO CGO)
│       ├── koreader/         KOSync HTTP server (Gin v1.12.0)
│       │   └── codec/        Pure-Go wire codec (NO CGO)
│       ├── moon/             Moon+ WebDAV server + .po parser (NO CGO)
│       ├── goodreads/        Goodreads bridge (Calibre plugin column only)
│       └── fake/             Scripted fake adapter for tests
├── tests/
│   ├── integration/          E2E: fake adapters + in-memory SQLite (CGO)
│   ├── security/             CSRF + secret-redaction black-box
│   └── fakeserver/           In-memory admin server for Playwright
├── docs/
│   ├── adr/                  Architecture Decision Records
│   ├── research/             koreader.md, moonplus.md, calibre.md, goodreads-bridge.md
│   ├── phases/               Delivery manifests P0–P9
│   └── qa/                   acceptance-checklist.md, acceptance-matrix.md
├── fixtures/                 Protocol captures (KOReader JSON, Moon+ .po, Calibre OPF)
├── tools/                    Phase 0 standalone simulators (own go.mod)
├── examples/                 Runnable usage examples
├── installer/                Inno Setup + smoke.ps1
├── scripts/bootstrap.sh
├── .pre-commit-config.yaml
├── Makefile / go.mod / go.sum
├── AGENTS.md  ← you are here
├── CLAUDE.md / llms.txt
└── .github/
    ├── workflows/ci.yml / release.yml
    └── copilot-instructions.md
```

---

## 3. Key Conventions

### Go style
- `gofmt` / `goimports` — enforced by CI.
- Error strings: lowercase, no trailing punctuation.
- No `panic()` in production paths.
- Package names: singular (`model`, `outbox`, `resolver`).
- Build tags: `//go:build windows` for platform-specific; `//go:build ignore` to exclude superseded files.

### Branching / commits
- Branch from `main` only. GitHub Flow. No `develop` / `release/*`.
- Branch names: `<type>/<short-description>` (e.g. `feat/koreader-rate-limit`).
- Conventional Commits: `feat(koreader): add rate limit`.

### Migrations — CRITICAL
- **NEVER modify** an existing `migration{version: N}` entry.
- **ALWAYS append** with the next sequential version number.
- All DDL must use `IF NOT EXISTS` (idempotent).

### CGO split
| No CGO | CGO required |
|--------|-------------|
| `internal/adapters/calibre/opf/` | `internal/adapters/calibre/` (parent) |
| `internal/adapters/koreader/codec/` | `internal/adapters/koreader/` (parent) |
| `internal/adapters/moon/` | `internal/core/`, `internal/db/` |
| `internal/resolver/`, `internal/conflicts/`, `internal/logging/` | `tests/integration/` |
| `internal/setup/`, `internal/repair/`, `internal/secrets/`, `internal/api/`, `tests/security/` | |

---

## 4. Build & Test

```powershell
# Setup (one time)
go mod tidy && go mod download
pip install pre-commit
pre-commit install
pre-commit run --all-files

# Build all binaries → bin/
make build

# Lint
go vet ./cmd/... ./internal/...

# Tests (no CGO)
make test-unit          # resolver, conflicts, logging
make test-phase7-unit   # all no-CGO packages + security + tray
make test-phase7-moon   # moon + calibre/opf + koreader/codec
make test-security      # CSRF + secret-redaction

# Tests (CGO required — install TDM-GCC first)
make test-integration
make test               # everything

# Single package
go test -v -count=1 -run TestScore ./internal/resolver/...

# Playwright E2E (Node.js 18+ required)
make run-fakeserver     # keep running in one shell
make test-e2e
```

---

## 5. Pipeline Architecture

```
Adapter → core.Pipeline.Submit()
              │ (single-writer goroutine)
  validate → normalize → resolveBook → insertEvent
  → loadCanonical → DetectSuspiciousJump → upsertCanonical
  → enqueueWritebacks → tx.Commit()
              │
  outbox.Worker.Run() every 2s
              │
  adapter.WriteTarget.WriteProgress()
```

**Only the pipeline goroutine writes** to `books`, `progress_events`,
`conflicts`, `canonical_progress`, and `sync_outbox`. The outbox worker
only updates `sync_outbox` status.

---

## 6. Public Interfaces

### Adapter (`internal/adapters/adapter.go`)
```go
type Adapter interface {
    Source() model.Source
    Start(ctx context.Context) error
    Stop() error
    Health() model.AdapterHealthState
}
type EventEmitter interface { Adapter; SetPipeline(p *core.Pipeline) }
type WriteTarget  interface { Adapter; WriteProgress(ctx context.Context, job *model.OutboxJob) error }
```

### Resolver (`internal/resolver/resolver.go`)
```go
func Score(ev Evidence, stored Evidence) Match       // 0-100 confidence
func Band(confidence int) ConfidenceBand             // Quarantine / UserReview / WritebackWary / WritebackSafe / AutoResolve
func WritebackEnabled(confidence int) bool           // true if >= 60
func EvidenceQuality(ev Evidence) int                // quality without DB lookup
```

### Conflict engine (`internal/conflicts/engine.go`)
```go
// 5 detectors: backward jump >10%, Goodreads finished <85%, page count changed, confidence <60, locator type changed
func DetectSuspiciousJump(canon *model.CanonicalProgress, ev *model.ProgressEvent) SuspiciousJump
func CanAutoResolve(p AutoResolveParams) bool
func ChooseWinner(a, b *model.ProgressEvent) (*model.ProgressEvent, string)
```

### Admin API (`internal/api/server.go`)
```go
func New(deps Deps) (*Server, error)              // Port: 7201, BindAddr: 127.0.0.1
func (s *Server) Start(ctx context.Context) error
func (s *Server) CSRFToken() string               // X-ReadSync-CSRF header value
```

### Outbox (`internal/outbox/worker.go`)
```go
func NextRetry(attempt int) time.Time             // 5s × 2^(n-1) ± 20% jitter, cap 2h
func Enqueue(ctx, db, bookID, target, payload)    // status = queued
```

---

## 7. Database Schema (Summary)

3 migrations, SQLite WAL mode.

**Migration 1**: `books`, `book_aliases`, `progress_events`, `conflicts`,
`canonical_progress`, `sync_outbox`, `adapter_health`

**Migration 2**: `koreader_users`, `koreader_devices`

**Migration 3**: `moon_users`, `moon_uploads`

PRAGMA: `journal_mode=WAL`, `synchronous=NORMAL`, `busy_timeout=5000`, `foreign_keys=ON`

---

## 8. Security Rules — NON-NEGOTIABLE

1. Admin UI binds to `127.0.0.1:7201` only — never `0.0.0.0`.
2. All write endpoints require CSRF — `requireCSRF(s.csrfToken, handler)`.
3. All credentials via `internal/secrets/` — never hardcode or log.
4. New secret-like field names → add to `internal/logging/redact.go` `secretKeys`.
5. No Goodreads API HTTP calls — bridge is Calibre column writes only.
6. Firewall rules opt-in only — never auto-open ports 7200/8765.

---

## 9. What to Avoid

| ❌ Don't | ✅ Do |
|----------|-------|
| Modify existing DB migrations | Append `migration{version: N+1}` |
| Import `internal/db` from no-CGO package | Use `opf/` or `codec/` subpackages |
| `panic()` in production | Return `error` |
| Hardcode credentials | `internal/secrets/ChainStore` |
| Write `canonical_progress` outside pipeline | `core.Pipeline.Submit()` |
| Write endpoint without CSRF | `requireCSRF(...)` |
| Admin on `0.0.0.0` | `deps.BindAddr` (defaults `127.0.0.1`) |
| Log secrets | Secrets never enter logging |
| Phase manifest at repo root | `docs/phases/` |
| Force-push after PR review | Push new commits |

---

## 10. Cross-Reference Map

| Need | File |
|------|------|
| Pre-commit hooks | `.pre-commit-config.yaml` |
| KOReader protocol | `docs/research/koreader.md` |
| Moon+ protocol | `docs/research/moonplus.md` |
| Calibre CLI | `docs/research/calibre.md` |
| Goodreads bridge | `docs/research/goodreads-bridge.md` |
| Language ADR | `docs/adr/0001-language-and-service-framework.md` |
| 14 acceptance criteria | `docs/qa/acceptance-checklist.md` |
| Phase history | `docs/phases/` |
| Diagnostic bundle | `docs/diagnostic-bundle.md` |
| CI/CD | `.github/workflows/ci.yml` |
| Release process | `CONTRIBUTING.md#release-process--semver` |
| v0.1.0 release notes backup | `docs/release-notes-v0.1.0.md` |
| Security policy | `SECURITY.md` |
| Support | `SUPPORT.md` |
| Examples | `examples/` |
| LLM index | `llms.txt` |
