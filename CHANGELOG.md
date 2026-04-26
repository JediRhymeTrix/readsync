# ReadSync Changelog

All notable changes to ReadSync are documented here.
Format follows [Keep a Changelog](https://keepachangelog.com/en/1.0.0/).

---

## [Unreleased] — Phase 11 (Final Closeout & Publishing)

### Added
- `docs/github-push-prompt.md` — self-contained GitHub publishing prompt with
  every `gh` CLI and REST API command needed: repo create/push, description,
  20 SEO topics, branch protection (7 required checks, dismiss stale, CODEOWNERS),
  Dependabot, CodeQL + secret scanning, 35 labels, v0.7.0 release, Release Drafter.

### Changed
- `llms.txt`: added `docs/github-push-prompt.md` entry; updated phase index P0–P10.
- Final sweep: `go vet` clean, formatter clean, spell-check clean over all files.
- CI workflow job names verified to match branch-protection `contexts` list exactly.

---


## [Unreleased] — Phase 10 (Documentation & LLM-Enablement)

### Added
- `AGENTS.md` — comprehensive AI coding agent guide: architecture, conventions, build/test, public APIs, security rules, cross-reference map.
- `CLAUDE.md` — Claude-specific behavioural instructions (auto-read by Claude Sonnet/Opus).
- `.github/copilot-instructions.md` — GitHub Copilot instructions with architecture and security rules.
- `.cursor/rules` — Cursor AI rules with CGO split, key files, and test commands.
- `llms.txt` — LLM-readable project index per llmstxt.org spec.
- `examples/` directory with 5 runnable examples: `api-query/query.ps1`, `koreader-push/push.sh`, `moon-webdav/sync.sh`, `resolve-book/main.go`, `conflict-scenario/main.go`.
- `docs/phases/README.md`: added Phase 10 entry.

### Changed
- `README.md`: polished with CI/release/Go/license/platform badges; LLM-summary comment; description table; Build from Source section; Architecture Overview; documentation index table; updated Phase Roadmap. Removed stale Phase 0 callout.
- All public API files reviewed for GoDoc completeness — existing docs confirmed adequate.

---

## [Unreleased] — Phase 9 (Closeout & Cleanup)

### Added
- `.gitignore` covering build artifacts, SQLite DBs, test output, node_modules, dist, TLS certs.
- `docs/phases/` directory with canonical phase manifest copies and index.
- `docs/phases/phase0-manifest.md` — Phase 0 deliverable manifest (new canonical location).

### Changed
- Redacted local machine paths from `commit-phase7.ps1` (now gitignored).
- `Makefile`: renamed duplicate `test-e2e` core target to `test-pipeline`; updated
  `test-unit` to include all no-CGO packages; fixed `go.sum` status comment;
  `test-phase7-unit` now correctly excludes calibre/koreader (need CGO).
- `.github/workflows/ci.yml`: Phase 7 unit job now covers setup, repair, secrets, api, tray.
- `docs/qa/acceptance-checklist.md`: corrected `TestE2E_DBPath_Utility` reference to
  `TestE2E_OutboxJobsQueued`; fixed no-CGO package list.
- `README.md`: corrected installer version reference from 0.7.0 to 0.6.0.
- `internal/adapters/calibre/read.go`: added import of `calibre/opf` subpackage.
- `internal/adapters/koreader/translate.go`: added import of `koreader/codec` subpackage.
- Makefile: added `test-pipeline` target (correct replacement for dead `test-e2e` alias);
  added `test-phase7-moon` and `test-phase7-cgo` targets; updated `.PHONY`.
- CI `phase7-unit` job now covers setup, repair, secrets, api, tray, security, opf, codec.

### Fixed
- `.phase3-manifest.md`: noted that `go.sum` is now committed.
- `docs/qa/acceptance-checklist.md`: corrected non-existent test name `TestE2E_DBPath_Utility`
  to `TestE2E_OutboxJobsQueued`; fixed no-CGO package list.
- `internal/adapters/calibre/opf/`: pure-Go extraction of OPF parsing logic eliminates
  false CGO dependency in the calibre parser tests.
- `internal/adapters/koreader/codec/`: pure-Go extraction of wire codec eliminates
  false CGO dependency in the koreader translate tests.

---

## [Unreleased] — Phase 7 (QA & Hardening)

### Added
- **Unit tests** for all five confidence bands in the identity resolver (spec §5).
- **Unit tests** for all five suspicious-jump detectors in the conflict engine (spec §6).
- **Integration test** for the spec §6 three-way conflict scenario:
  KOReader 72% / Calibre 70% / Goodreads 38% (claims finished → blocked as suspicious).
- **Outbox state machine tests**: queued→running→succeeded, queued→retrying, retrying→deadletter.
- **Log corpus sweep test**: verifies no secret values survive into JSONL or activity streams.
- **Calibre OPF parser unit tests**: percent mode, page mode, ISBN routing, `#value#` extraction.
- **Calibre command builder tests**: `quoteEnumValues`, required column definitions.
- **KOReader payload codec tests**: `locatorType`, `toAdapterEvent`, `canonicalToPull` round-trip.
- **Moon+ parser extended tests**: boundary percentages (0%, 100%), malformed payloads,
  filename variants (case-insensitive `.po`), `ToAdapterEvent` status mapping.
- **E2E integration tests** with fake adapters: KOReader push → canonical → outbox.
- **Conflict scenario integration test** using the real pipeline + in-memory SQLite DB.
- **Security tests**: CSRF required on all 11 write endpoints; GET endpoints bypass CSRF;
  secrets never appear in JSONL log output.
- **`CHANGELOG.md`** (this file).
- **`docs/diagnostic-bundle.md`**: documents the diagnostic bundle export procedure.
- **`docs/qa/acceptance-checklist.md`**: maps every spec §20 acceptance criterion to
  a passing test or documented manual verification.
- **Updated `README.md`**: setup walkthrough with step-by-step instructions.

### Changed
- `Makefile`: Added `test-phase7`, `test-security`, `test-integration` targets.
- CI workflow: added Phase 7 test jobs.

### Fixed
- `internal/logging/redact.go`: word-boundary matching prevents `"author"` from matching `"auth"`.

---

## [0.6.0] — Phase 6 (User-Facing Surface)

### Added
- 10-step setup wizard (welcome → system_scan → calibre → goodreads_bridge → koreader →
  moon → conflict_policy → test_sync → diagnostics → finish).
- Native Windows tray icon with right-click menu.
- Dashboard, conflicts list, outbox view, activity log, repair grid.
- All 12 repair actions from spec §13.
- Windows DPAPI/Credential Manager secrets store.
- Self-signed TLS certificate generator.
- Inno Setup installer (`dist/ReadSync-0.6.0-setup.exe`).
- Playwright E2E wizard suite + PowerShell installer smoke test.
- Admin UI bound to `127.0.0.1:7201` by default.
- CSRF middleware on all write endpoints.

---

## [0.5.0] — Phase 5 (Goodreads Bridge)

### Added
- Goodreads Sync plugin detection and configuration via `#readsync_progress` column.
- Three bridge modes: `disabled`, `manual-plugin`, `guided-plugin`.
- Stale-state detection and writeback safety gate.
- No Goodreads API key required — works entirely through the Calibre plugin.

---

## [0.4.0] — Phase 4 (Moon+ WebDAV)

### Added
- Full Moon+ WebDAV adapter with versioned upload registry.
- `.po` file parser (FormatV1Plain verified against real Moon+ Pro v9 captures).
- Write-back safety gate: writeback disabled until verified writer fixture committed.
- Capture mode for diagnostics.

---

## [0.3.0] — Phase 3 (KOReader Adapter)

### Added
- Full KOSync-compatible HTTP server (register, auth, push, pull).
- Rate limiting and auth failure tracking.
- Gin v1.10.0 HTTP framework.

---

## [0.2.0] — Phase 2 (Calibre Adapter)

### Added
- `calibredb` discovery and library detection.
- Custom column management (`#readsync_progress`, `#readsync_status`, etc.).
- OPF metadata parsing.
- Write path: `calibredb set_custom` + `set_metadata`.
- Goodreads Sync plugin detection.

---

## [0.1.0] — Phase 1 (Core Service Skeleton)

### Added
- SQLite WAL-mode database with 7-table schema (3 migrations).
- Identity resolver with 10-signal confidence ladder.
- Conflict engine with 5 detectors and auto-resolve gate.
- Outbox worker with exponential backoff (10 attempts, 5s base, 2h cap).
- Dual-stream logger (activity + JSONL) with secret redaction.
- Windows service framework (`kardianos/service`).
- Admin API (`127.0.0.1:7201`, CSRF protected).
- `readsyncctl` CLI: status, adapters, conflicts, outbox, db, diagnostics.

---

## [0.0.0] — Phase 0 (Research & Fixtures)

### Added
- KOSync protocol research and simulator.
- Moon+ WebDAV protocol research and fixture recorder.
- Calibre CLI command research.
- Goodreads bridge design.
- ADR 0001: Go + kardianos/service.
