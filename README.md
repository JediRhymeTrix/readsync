# ReadSync

<!-- LLM-SUMMARY: ReadSync is a self-hosted Windows reading-progress sync service. Go 1.25+, SQLite, KOSync + WebDAV adapters for KOReader and Moon+ Pro, Calibre integration, Goodreads bridge via plugin. No API keys needed. -->

[![CI](https://github.com/JediRhymeTrix/readsync/actions/workflows/ci.yml/badge.svg)](https://github.com/JediRhymeTrix/readsync/actions/workflows/ci.yml)
[![Release](https://img.shields.io/github/v/release/JediRhymeTrix/readsync?include_prereleases&sort=semver)](https://github.com/JediRhymeTrix/readsync/releases/latest)
[![Go 1.25+](https://img.shields.io/badge/go-1.25%2B-blue)](https://golang.org/)
[![Platform: Windows 10/11](https://img.shields.io/badge/platform-Windows%2010%2F11-blue)](https://github.com/JediRhymeTrix/readsync/releases)

ReadSync is a self-hosted Windows background service that syncs reading progress across KOReader, Moon+ Reader Pro, Calibre, and Goodreads without cloud dependency or Goodreads API key.

## Supported integrations

| App | Protocol | ReadSync role |
|-----|----------|---------------|
| KOReader | KOSync HTTP API | Drop-in KOSync server on port 7200 |
| Moon+ Reader Pro | WebDAV `.po` sync | Embedded WebDAV server on port 8765 |
| Calibre | `calibredb` + OPF | Reads/writes `#readsync_progress` custom column |
| Goodreads | Calibre Goodreads Sync plugin | Bridges plugin progress through Calibre custom column |

## Current status

All 12 documented phases are complete or recovered in `docs/phases/` and `.cline/kanban/phases/`:

| Phase | Status | Summary |
|-------|--------|---------|
| 0 | Complete | Research, simulators, fixtures, ADR |
| 1 | Complete | Core service skeleton, SQLite, resolver, pipeline, outbox, conflicts, logging, CLI/API |
| 2 | Complete | Calibre adapter |
| 3 | Complete | KOReader KOSync adapter |
| 4 | Complete | Moon+ WebDAV adapter |
| 5 | Complete | Goodreads bridge |
| 6 | Complete | Admin UI, setup wizard, repair actions, secrets, tray, installer |
| 7 | Complete | QA and hardening |
| 8 | Recovered | Integration stabilization and release preparation |
| 9 | Complete | Closeout, cleanup, CI/Makefile fixes, manifest migration |
| 10 | Recovered | Documentation and LLM enablement |
| 11 | Recovered | Final closeout and publishing |

See `docs/phases/README.md` for phase manifests. `.cline/kanban/phases/` is source of truth.

## Install

Download Windows release from GitHub Releases, run installer as Administrator, then open admin UI at:

```
http://127.0.0.1:7201/
```

Wizard configures Calibre, Goodreads bridge mode, KOReader credentials, Moon+ WebDAV credentials, ports, firewall hints, and initial sync.

## Build from source

```powershell
git clone https://github.com/JediRhymeTrix/readsync.git
cd readsync
go mod tidy
go mod download
pip install pre-commit
pre-commit install
pre-commit run --all-files
make test-unit
make build
```

Full tests require CGO + GCC because `github.com/mattn/go-sqlite3` is used:

```powershell
make test
```

Windows CGO needs TDM-GCC or equivalent in PATH. Linux CI installs `gcc`.

Pre-commit runs whitespace/config checks, `gofmt`, `go mod tidy`, and fast no-CGO unit tests before commits. Install it once per clone with `pre-commit install`.

## Device setup

### KOReader

Set KOReader KOSync custom server:

```
Protocol: http
Host:     <your-pc-ip>
Port:     7200
Username: <wizard value>
Password: <wizard value>
```

### Moon+ Reader Pro

Set WebDAV sync:

```
Server:   http://<your-pc-ip>:8765/moon-webdav/
Username: <wizard value>
Password: <wizard value>
```

### Goodreads

Install Calibre Goodreads Sync plugin and configure progress column `#readsync_progress`. ReadSync uses Calibre/plugin bridge; no Goodreads API key.

## Architecture

```
cmd/readsync-service/   Windows service entry point
cmd/readsyncctl/        CLI: status, adapters, conflicts, outbox, db, diagnostics
cmd/readsync-tray/      Native tray icon
internal/model/         Domain types and enums
internal/db/            SQLite WAL-mode + migrations
internal/core/          Event pipeline
internal/resolver/      10-signal identity ladder + fuzzy match
internal/conflicts/     Conflict detectors and auto-resolve gate
internal/outbox/        Retry queue with exponential backoff
internal/logging/       Activity + JSONL logging, secret redaction
internal/secrets/       Windows DPAPI / Credential Manager store
internal/api/           Local admin UI/API, CSRF protected
internal/setup/         10-step wizard
internal/repair/        Repair actions
internal/diagnostics/   Health snapshot + bundle exporter
internal/adapters/      Calibre, KOReader, Moon+, Goodreads, fake adapters
tests/integration/      E2E tests with fake adapters
tests/security/         CSRF and redaction tests
```

## Documentation

| File | Purpose |
|------|---------|
| `AGENTS.md` | Coding-agent architecture and rules |
| `CLAUDE.md` | Claude-specific project instructions |
| `llms.txt` | LLM-readable project index |
| `docs/phases/` | Phase manifests 0-11 |
| `docs/research/` | Protocol and platform research |
| `docs/qa/` | Acceptance matrix, checklist, fixture plan |
| `docs/github-push-prompt.md` | GitHub publishing procedure |
| `docs/release-notes-v0.1.0.md` | Backed-up canonical v0.1.0 draft release notes |

## Security defaults

- Admin UI binds to `127.0.0.1:7201`.
- KOReader defaults to `127.0.0.1:7200` unless LAN bind is configured.
- Moon+ WebDAV defaults to authenticated access.
- Credentials are hashed or stored via Windows secret storage.
- CSRF protection covers mutating admin endpoints.
- Logs redact secrets.

## License

MIT.
