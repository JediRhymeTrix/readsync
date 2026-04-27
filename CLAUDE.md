# CLAUDE.md — Instructions for Claude AI

This file is automatically read by Claude when working in this repository.
See [AGENTS.md](AGENTS.md) for the full developer reference.

---

## Project identity

**ReadSync** — `github.com/readsync/readsync`
Self-hosted Windows reading-progress sync service.
Go 1.25+, SQLite WAL (CGO), KOSync + WebDAV adapters.

---

## Behaviour rules

### Always do
- Read `AGENTS.md` before making architectural decisions.
- Run `go vet ./...` mentally before suggesting code changes.
- Use `internal/secrets/` for any credential storage.
- Use `internal/logging/` for any log output.
- Append-only to `internal/db/migrations.go` — never modify existing entries.
- Wrap new write endpoints with `requireCSRF(s.csrfToken, handler)`.
- Keep CGO-free packages free of CGO: `internal/adapters/calibre/opf/`, `internal/adapters/koreader/codec/`, `internal/adapters/moon/`, `internal/resolver/`, `internal/conflicts/`, `internal/logging/`, `internal/setup/`, `internal/repair/`, `internal/secrets/`, `internal/api/`.

### Never do
- Import `internal/db` or `internal/core` from a package that must remain CGO-free.
- Use `panic()` in production code paths.
- Hardcode any credentials, passwords, or API tokens.
- Bind the admin UI to `0.0.0.0` — always `127.0.0.1`.
- Add HTTP calls to `goodreads.com` — the bridge is Calibre-plugin-only.
- Open firewall ports 7200/8765 automatically.
- Suggest `rand.Seed()` — Go 1.20+ auto-seeds.

---

## Build commands

```powershell
go mod tidy && go mod download   # first-time setup
pre-commit install               # install local commit hooks
pre-commit run --all-files       # run formatting/config/unit-test hooks
make build                       # all binaries → bin/
make test-unit                   # fast, no CGO
make test-phase7-unit            # all no-CGO packages
make test-security               # CSRF + redaction
make test-integration            # CGO required
make test                        # everything
```

---

## Code style

- `gofmt` / `goimports` — CI-enforced.
- Error strings: lowercase, no trailing period.
- Package names: singular.
- Test files: `<pkg>_test.go`; black-box tests use `package foo_test`.

---

## Key files

| Purpose | File |
|---------|------|
| All domain types | `internal/model/model.go` |
| Adapter interfaces | `internal/adapters/adapter.go` |
| Identity resolution | `internal/resolver/resolver.go` |
| Conflict detection | `internal/conflicts/engine.go` |
| Outbox worker | `internal/outbox/worker.go` |
| Admin API server | `internal/api/server.go` |
| CSRF constant | `internal/api/server.go` → `CSRFHeader = "X-ReadSync-CSRF"` |
| Log redaction | `internal/logging/redact.go` |
| Migrations | `internal/db/migrations.go` |
| Wizard state machine | `internal/setup/wizard.go` |
| Secrets store | `internal/secrets/secrets.go` |
| Pre-commit hooks | `.pre-commit-config.yaml` |
| v0.1.0 release notes backup | `docs/release-notes-v0.1.0.md` |

---

## Testing a security change

```powershell
go test -v -count=1 ./tests/security/...
go test -v -count=1 ./internal/logging/...
```

---

## When in doubt

Consult `AGENTS.md` → section 9 "What to Avoid".
Consult `docs/qa/acceptance-checklist.md` for the 14 acceptance criteria.
Consult `docs/research/` for protocol details.
