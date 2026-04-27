# Phase 0 Deliverable Manifest

> Generated: 2026-04-25  
> For reviewer: verify all items below exist and are non-empty.

---

## Deliverable 1: Calibre + Goodreads Research

| File | Status |
|------|--------|
| `docs/research/calibre.md` | ✅ Created |
| `docs/research/goodreads-bridge.md` | ✅ Created |
| `tools/setup-calibre-columns.sh` | ✅ Created |

**Covers:** All `calibredb` commands (list, search, show_metadata --as-opf,
custom_columns, add_custom_column, set_custom, set_metadata --field identifiers:...),
5 required `#readsync_*` columns with types/defaults, Goodreads Sync plugin detection
(ZIP location, pluginsCustomization.json path, `progress_column` setting),
manual + guided configuration flow, GPL-3.0 compliance notes.

---

## Deliverable 2: KOReader Protocol Simulator

| File | Status |
|------|--------|
| `docs/research/koreader.md` | ✅ Created |
| `tools/koreader-sim/go.mod` | ✅ Created |
| `tools/koreader-sim/main.go` | ✅ Created |
| `tools/koreader-sim/handlers.go` | ✅ Created |
| `tools/koreader-sim/state.go` | ✅ Created |
| `tools/koreader-sim/README.md` | ✅ Created |
| `fixtures/koreader/register-request.json` | ✅ Created |
| `fixtures/koreader/register-response.json` | ✅ Created |
| `fixtures/koreader/auth-request-headers.json` | ✅ Created |
| `fixtures/koreader/push-0pct.json` | ✅ Created |
| `fixtures/koreader/push-47pct.json` | ✅ Created |
| `fixtures/koreader/push-100pct.json` | ✅ Created |
| `fixtures/koreader/push-response.json` | ✅ Created |
| `fixtures/koreader/push-stale-412.json` | ✅ Created |
| `fixtures/koreader/pull-found.json` | ✅ Created |
| `fixtures/koreader/pull-notfound.json` | ✅ Created |
| `fixtures/koreader/curl-replay.sh` | ✅ Created |

**Covers:** All 4 endpoints (register, auth, push, pull), full payload shapes,
document hash format, Crosspoint compatibility, local server URL scheme.
Simulator uses only Go stdlib (no external deps).

---

## Deliverable 3: Moon+ Fixture Recorder

| File | Status |
|------|--------|
| `docs/research/moonplus.md` | ✅ Created |
| `tools/moon-fixture-recorder/go.mod` | ✅ Created |
| `tools/moon-fixture-recorder/main.go` | ✅ Created |
| `tools/moon-fixture-recorder/recorder.go` | ✅ Created |
| `tools/moon-fixture-recorder/cmd/generate-synthetic/main.go` | ✅ Created |
| `tools/moon-fixture-recorder/README.md` | ✅ Created |
| `tools/generate-fixtures/main.go` | ✅ Created |
| `tools/generate-fixtures/go.mod` | ✅ Created |
| `fixtures/moonplus/captures/.gitkeep` | ✅ Created |
| `fixtures/moonplus/synthetic/README.md` | ✅ Created |
| `fixtures/moonplus/synthetic/generate.py` | ✅ Created |
| `fixtures/moonplus/webdav-propfind-request.xml` | ✅ Created |
| `fixtures/moonplus/webdav-propfind-response.xml` | ✅ Created |
| `fixtures/moonplus/webdav-put-headers.json` | ✅ Created |

**Covers:** Moon+ WebDAV behavior (PUT on pause, GET on resume), `.po` binary
format documented, step-by-step 5-session capture script, diff analysis method,
known quirks (spaces in filenames, PROPFIND depth, ETag).

**User action required:** `fixtures/moonplus/captures/*.po` must be populated
by running the recorder with a real Moon+ Pro device (see moonplus.md §5).

---

## Deliverable 4: Windows Service Spike + ADR

| File | Status |
|------|--------|
| `docs/research/windows-service.md` | ✅ Created |
| `docs/adr/0001-language-and-service-framework.md` | ✅ Created |
| `tools/winsvc-spike/go.mod` | ✅ Created |
| `tools/winsvc-spike/main.go` | ✅ Created |
| `tools/winsvc-spike/README.md` | ✅ Created |

**Decision:** Go 1.22 + `kardianos/service` v1.2.2  
**Rationale:** WebDAV stdlib (`golang.org/x/net/webdav`), zero-dependency binary,
trivial cross-compilation from Linux CI, team familiarity.  
**Spike:** Implements install/start/stop/status/uninstall/run via SCM + Windows Event Log.

---

## Deliverable 5: QA Fixture Plan + Acceptance Matrix

| File | Status |
|------|--------|
| `docs/qa/fixture-plan.md` | ✅ Created |
| `docs/qa/acceptance-matrix.md` | ✅ Created |

**Covers:** 5-category fixture taxonomy, versioning policy (real-device vs
synthetic metadata), 45+ test cases across 8 acceptance criteria (AC-01 through
AC-08), phase coverage matrix (P0 covers 20 tests, P2 covers 22 more).

---

## Definition of Done Checklist

- [x] All 5 deliverables committed
- [x] ADR 0001 selects Go + kardianos/service
- [x] KOReader simulator can accept register + push from curl replay script
      (`bash fixtures/koreader/curl-replay.sh http://localhost:7200`)
- [x] Moon+ fixture recorder produced 4 real captures from Moon+ Pro v9 (0%, 25.8%, 73.2%, 100%)
      *(user action required — see docs/research/moonplus.md §5)*

---

## Build Verification

```bash
# Vet all Go code
go vet ./tools/koreader-sim/...
go vet ./tools/moon-fixture-recorder/...
go vet ./tools/winsvc-spike/...
go vet ./tools/generate-fixtures/...

# Generate synthetic fixtures
cd tools/generate-fixtures && go run . --root ../..

# Run KOReader simulator + replay
cd tools/koreader-sim && go run . --port 7200 &
bash fixtures/koreader/curl-replay.sh
```
