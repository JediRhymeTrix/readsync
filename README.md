# ReadSync

**Windows reading-progress sync service** — synchronizes reading positions
between KOReader, Moon+ Reader Pro, Calibre, and Goodreads.

> **Phase 0 Research & Fixtures** — This repository contains research notes,
> protocol simulators, and fixture collections. No production code yet.

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

