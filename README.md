# ReadSync

<!-- LLM-SUMMARY: ReadSync is a self-hosted Windows reading-progress sync service. Go 1.22+, SQLite, KOSync + WebDAV adapters for KOReader and Moon+ Pro, Calibre integration, Goodreads bridge via plugin. No API keys needed. -->

[![CI](https://github.com/readsync/readsync/actions/workflows/ci.yml/badge.svg)](https://github.com/readsync/readsync/actions/workflows/ci.yml)
[![Release](https://img.shields.io/github/v/release/readsync/readsync)](https://github.com/readsync/readsync/releases/latest)
[![Go 1.22+](https://img.shields.io/badge/go-1.22%2B-blue)](https://golang.org/)
[![License: MIT](https://img.shields.io/badge/license-MIT-green)](LICENSE)
[![Platform: Windows 10/11](https://img.shields.io/badge/platform-Windows%2010%2F11-blue)](https://github.com/readsync/readsync/releases)

**ReadSync** is a self-hosted Windows background service that keeps reading progress in sync across [KOReader](https://koreader.rocks/), [Moon+ Reader Pro](https://play.google.com/store/apps/details?id=com.flyersoft.moonreaderp), [Calibre](https://calibre-ebook.com/), and [Goodreads](https://www.goodreads.com/) — automatically, with no cloud subscription or API key.

| App | Protocol | ReadSync role |
|-----|----------|---------------|
| **KOReader** (e-ink, Android) | KOSync HTTP API | Drop-in KOSync server — port 7200 |
| **Moon+ Reader Pro** (Android) | WebDAV sync (`.po` files) | Embedded WebDAV server — port 8765 |
| **Calibre** (Windows library) | `calibredb` + OPF | Reads/writes `#readsync_progress` column |
| **Goodreads** | Calibre Goodreads Sync plugin | Writes `#readsync_progress` for the plugin |

**No Goodreads API key required.** **No cloud dependency.** Everything runs on your local Windows PC.

---


## Build from Source

```powershell
git clone https://github.com/readsync/readsync.git && cd readsync
go mod tidy && go mod download
make build              # → bin/readsync-service.exe, bin/readsyncctl.exe, bin/readsync-tray.exe
make test-unit          # fast pure-Go tests (no CGO)
make test               # full suite (CGO required — install TDM-GCC first)
```

Or download the pre-built installer from [Releases](https://github.com/readsync/readsync/releases/latest) and run as Administrator.

---





**Windows reading-progress sync service** — synchronizes reading positions
between KOReader, Moon+ Reader Pro, Calibre, and Goodreads.



> **Phase 0 Research & Fixtures** — This repository contains research notes,
> protocol simulators, and fixture collections. No production code yet.

---

## Setup Walkthrough

### Prerequisites

| Requirement | Version | Notes |
|-------------|---------|-------|
| Windows 10/11 | 64-bit | Service runs as LocalSystem |
| Calibre | 6+ | Optional but recommended |

### Step 1 — Install

Run `ReadSync-0.6.0-setup.exe` (or latest release) as Administrator. The installer:
1. Copies binaries to `%ProgramFiles%\ReadSync\`.
2. Registers `ReadSync` Windows service with auto-start + crash recovery.
3. Starts the service and launches the system tray icon.

> **Firewall**: An optional task opens ports 7200 (KOReader) and 8765 (Moon+)
> on the private-network profile only. Leave unchecked unless you need LAN sync.

### Step 2 — Open the Admin UI

The service binds to `http://127.0.0.1:7201/`. Open in any browser;
you will be redirected to the 10-step setup wizard automatically.

### Step 3 — Configure KOReader

In KOReader: **Settings → KOSync → Custom sync server**

```
Protocol: http
Host:     <your-pc-ip>
Port:     7200
Username: <from wizard step 5>
Password: <from wizard step 5>
```

### Step 4 — Configure Moon+ Pro

In Moon+ Pro: **Settings → Sync Reading → WebDAV Sync**

```
Server:   http://<your-pc-ip>:8765/moon-webdav/
Username: <from wizard step 6>
Password: <from wizard step 6>
```

### Step 5 — Goodreads (optional, no API key required)

1. Install the Calibre Goodreads Sync plugin.
2. Set its progress column to `#readsync_progress`.
3. Select **manual-plugin** or **guided-plugin** mode in the wizard step 4.



---

## Repository Structure

```
ReadSync/
├── docs/
│   ├── research/
│   │   ├── calibre.md          Calibre CLI commands + custom columns
│   │   ├── goodreads-bridge.md Goodreads Sync plugin integration
│   │   ├── koreader.md         KOSync server protocol documentation
│   │   ├── moonplus.md         Moon+ WebDAV sync protocol
│   │   └── windows-service.md  Windows service framework decision
│   ├── adr/
│   │   └── 0001-language-and-service-framework.md  ADR: Go + kardianos/service
│   └── qa/
│       ├── fixture-plan.md     Fixture taxonomy and generation
│       └── acceptance-matrix.md  Test matrix mapped to spec §20
├── fixtures/
│   ├── books/          Book identity fixtures (hash maps, identifiers)
│   ├── calibre/        Calibre library snapshots + OPF files
│   ├── goodreads/      Goodreads plugin config fixtures
│   ├── koreader/       KOSync push/pull JSON payloads
│   └── moonplus/       Moon+ .po binary captures + WebDAV fixtures
└── tools/
    ├── koreader-sim/       KOSync-compatible HTTP simulator (Go)
    ├── moon-fixture-recorder/  WebDAV fixture recorder (Go)
    └── winsvc-spike/       Windows service hello-world (Go)
```

---

## Quick Start

### KOReader Simulator

```bash
cd tools/koreader-sim
go run . --port 7200 --verbose

# Replay curl script:
bash ../../fixtures/koreader/curl-replay.sh
```

### Moon+ Fixture Recorder

```bash
cd tools/moon-fixture-recorder
go run . --port 8765 --verbose

# Generate synthetic .po files for CI:
go run ./cmd/generate-synthetic
```

### Windows Service Spike

```powershell
cd tools\winsvc-spike
go build -o readsync-spike.exe .
.\readsync-spike.exe install    # requires admin
.\readsync-spike.exe start      # requires admin
.\readsync-spike.exe status
.\readsync-spike.exe stop       # requires admin
.\readsync-spike.exe uninstall  # requires admin
```

---

## Language Decision

**Go 1.22+ with `kardianos/service`**. See `docs/adr/0001-language-and-service-framework.md`.

---

## Architecture Overview

```
cmd/readsync-service/   Windows Service entry point (kardianos/service)
cmd/readsyncctl/        CLI: status, adapters, conflicts, outbox, db, diagnostics
cmd/readsync-tray/      System tray icon (native Win32 syscall)
internal/model/         ALL domain types and enums (no logic)
internal/db/            SQLite WAL-mode + 3 migrations (APPEND ONLY)
internal/core/          Event pipeline (single-writer goroutine)
internal/resolver/      10-signal identity ladder + Jaro-Winkler fuzzy match
internal/conflicts/     5-detector conflict engine + auto-resolve gate
internal/outbox/        Exponential backoff outbox (10 attempts, 5s–2h)
internal/logging/       Dual-stream logger (activity + JSONL) + secret redaction
internal/secrets/       Windows DPAPI / Credential Manager store
internal/api/           Admin HTTP server (127.0.0.1:7201, CSRF protected)
internal/setup/         10-step wizard state machine
internal/repair/        12 repair actions (port picker, DB repair, …)
internal/diagnostics/   Health snapshot + bundle exporter
internal/adapters/      Calibre + KOReader + Moon+ + Goodreads + Fake adapters
tests/integration/      E2E tests with fake adapters + in-memory SQLite
tests/security/         CSRF and secret-redaction black-box tests
```

See [AGENTS.md](AGENTS.md) for the full architecture guide and developer reference.



## Phase Roadmap

| Phase | Description                                    |
|-------|------------------------------------------------|
| **P0**| Research, simulators, fixtures (this phase)    |
| P1    | Project scaffolding, CI/CD, module structure   |
| P2    | Core sync engine: KOReader ↔ Calibre           |
| P3    | Moon+ WebDAV ↔ Calibre                         |
| P4    | Windows service production hardening           |
| P5    | Goodreads bridge integration                   |
| P6    | Installer + UI (readsyncctl)                   |
| P7    | Production release                             |


---

## Phase 1: Core Service Skeleton

Phase 1 implements the load-bearing foundation:

```
go.mod                          Module: github.com/readsync/readsync
internal/model/                 Domain types (Book, ProgressEvent, OutboxJob, ...)
internal/db/                    SQLite WAL-mode + migration runner (7 tables)
internal/resolver/              Identity resolver (10-signal ladder, fuzzy match)
internal/conflicts/             Conflict engine (5 detectors, auto-resolve gate)
internal/outbox/                Outbox worker (exp. backoff, fair scheduler)
internal/core/                  Event pipeline (single-writer goroutine)
internal/logging/               Dual-stream logging + secret redaction
internal/secrets/               Credential management
internal/diagnostics/           System health reporting
internal/repair/                Self-healing (busy retry, port picker)
internal/api/                   Local admin API (127.0.0.1:7201, CSRF)
internal/adapters/fake/         Scripted fake adapter for E2E tests
cmd/readsync-service/           Windows Service binary
cmd/readsyncctl/                CLI: status/adapters/conflicts/outbox/db/diagnostics
```

### Build & Test (Phase 1)

```powershell
# Prerequisites: Go 1.22+, TDM-GCC (https://jmeubank.github.io/tdm-gcc/)

# Download dependencies (generates go.sum)
go mod tidy

# Unit tests — no CGO needed
go test -v ./internal/resolver/... ./internal/conflicts/... ./internal/logging/...

# All tests — requires CGO/GCC
go test -v ./...

# Migrate a test database
go run ./cmd/readsyncctl db migrate --db test.db

# Build service binary
go build -o readsync-service.exe ./cmd/readsync-service/
```

