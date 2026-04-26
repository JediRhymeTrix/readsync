# Phase 5 Manifest - Goodreads Bridge (final)

> Generated: 2026-04-25
> Depends on: Phase 2 Calibre adapter.

## Summary

Phase 5 implements the Goodreads bridge as an OPTIONAL companion path
that goes through the user's existing Calibre Goodreads Sync plugin.
ReadSync NEVER calls the Goodreads API, NEVER scrapes Goodreads, and
NEVER vendors any plugin code (GPL-3.0).

## Files Created (Phase 5)

| File | Purpose |
|------|---------|
| `internal/adapters/goodreads_bridge/goodreads.go` | Adapter struct: implements Adapter, EventEmitter, WriteTarget. |
| `internal/adapters/goodreads_bridge/modes.go` | BridgeMode enum: disabled, manual-plugin, guided-plugin, companion-plugin, experimental-direct. |
| `internal/adapters/goodreads_bridge/detect.go` | Plugin detection by scanning `%APPDATA%\calibre\plugins\` and parsing `pluginsCustomization.json`. |
| `internal/adapters/goodreads_bridge/identifier.go` | `BuildMissingIDReport` -- list books that lack a Goodreads identifier (no auto-fetch). |
| `internal/adapters/goodreads_bridge/stale.go` | Stale-state detection (spec section 6) + writeback safety gate (spec section 8). |
| `internal/adapters/goodreads_bridge/bridge_test.go` | 37 unit tests covering all of the above plus pure-function tests of the integration helpers. |
| `internal/adapters/goodreads_bridge/integration_test.go` | End-to-end test: ReadSync writes `#readsync_progress`; simulated plugin reads it back into a Goodreads-mirror JSON file. Skipped when calibredb is missing or any Calibre process is running. |
| `.phase5-manifest.md` | This file. |

## Pre-existing issues fixed in this PR

| File | Change |
|------|--------|
| `internal/adapters/calibre/calibre.go` | Removed a stray closing `}` left over from Phase 2 that prevented the Calibre package from compiling. |
| `internal/adapters/calibre/win_process.go` | Extended `isGUIRunning()` to also detect `calibre-server.exe` and `calibre-parallel.exe` (these block calibredb mutations just like `calibre.exe`). Added exported `IsCalibreRunning()` for cross-package test reuse. |
| `internal/adapters/calibre/integration_test.go` | Added `skipIfCalibreRunning()` guard to `TestIntegration_EnsureColumns` and `TestIntegration_ReadWrite`. Tests now skip cleanly instead of failing when any Calibre process is alive. |
| `internal/repair/repair.go` | Fixed `go vet` error: `shifted operand 1 (type float64) must be integer`. New code is numerically equivalent: `int64(50ms) * (int64(1) << i)`. |
| `internal/logging/redact.go` | Fixed `IsSecretKey("author") == true` (substring match on `auth`). Replaced with token-boundary match honoring `_`, `-`, and string-end. All 22 existing test cases now pass. |


## Bridge Modes

| Mode | v1 status | Behaviour |
|------|-----------|-----------|
| `disabled` | shipping | Detection still runs; no events/writes. |
| `manual-plugin` | default, shipping | ReadSync writes `#readsync_progress`; user manually triggers Goodreads Sync. Logs `Goodreads bridge skipped: manual mode`. |
| `guided-plugin` | shipping | Same data path; UI layer surfaces a checklist (Phase 4). Logs `Goodreads bridge skipped: guided mode`. |
| `companion-plugin` | hooks only | Configuration accepted; `WriteProgress` returns "not yet implemented (v2)". Health: disabled. |
| `experimental-direct` | gated stub | Refuses to start unless `ExperimentalDirectAck=true`. With ack, reports `HealthDegraded`. Performs no network I/O. |

## Plugin Detection

`DetectPlugin(pluginsDir)` performs the following:

1. Walk `*.zip` files in the plugins directory; filename match
   (case-insensitive) on `goodreads` -> plugin installed.
2. Parse `pluginsCustomization.json`; read `progress_column`,
   `reading_list_column`, and `sync_reading_progress` from the
   `Goodreads Sync` key.
3. Surface results via `Detection.Installed`,
   `Detection.ProgressColumnConfigured()` (true iff column is
   `#readsync_progress`), and `Detection.ShelfColumnConfigured()`
   (true iff column is `#readsync_gr_shelf`).
4. Tolerate missing dir / missing file / missing key -- these are
   reported as "not installed/not configured" rather than as errors.

## Safety Gates (spec sections 6, 8, 9)

`EvaluateWriteback(canon, obs, now)` enforces:

1. `IdentityConfidence >= 90` (`MinConfidenceForWriteback`).
2. `DeviceTS` is present and not more than 1 hour in the future.
3. The current canonical record was NOT updated by a non-Goodreads
   source within the last 24 hours (`LocalChangeRecency`).
4. Not a stale regression -- `DetectStaleFinished` rejects a Goodreads
   "finished" claim while local progress is below 85%
   (`FinishedRegressionThreshold`).

These gates implement the precedence:
local reader > Calibre manual > Goodreads/Kindle-derived.

## Stale-State Example (spec section 6)

| Local canonical | Goodreads claim | Outcome |
|-----------------|-----------------|---------|
| 50% (KOReader)  | shelf=read       | `goodreads_bridge_stale` conflict; canonical NOT updated. |
| 90% (KOReader)  | shelf=read       | Allowed (canonical reaches 100%). |
| 40% (KOReader)  | progress=100%    | Stale conflict (works on percent claim too). |

Conflicts surface with reason `goodreads_bridge_stale`.


## Sandboxing the Calibre integration test

`calibredb` enforces a GLOBAL single-instance lock on Windows: when any
`calibre.exe`, `calibre-debug.exe`, `calibre-server.exe`, or
`calibre-parallel.exe` process is alive, ANY mutating `calibredb`
command (e.g. `add`, `set_custom`, `add_custom_column`) fails with
"Another calibre program ... is running" -- regardless of which library
`--library-path` points at, regardless of `CALIBRE_CONFIG_DIRECTORY`.

Isolated config dirs and brand-new tempdir libraries do not work; the
lock is enforced via a Windows named mutex, not a file lock, so it
cannot be circumvented from the test side without stopping Calibre.

Mitigation strategy:

1. Skip cleanly when Calibre is alive (`calibre.IsCalibreRunning()` guard).
2. Cover the simulator's pure logic with deterministic unit tests
   (`TestBuildMirrorFromRow_*`, `TestExtractAddedBookID/*`) that run
   regardless of Calibre's state. These exercise the same shelf-inference
   logic the integration test relies on, with calibredb-style row maps
   crafted in code.
3. Rely on existing Calibre adapter tests (Phase 2 `opf_test.go`) for
   coverage of the OPF parsing and column writer paths.

When run on a host where no Calibre process is alive, the integration
test DOES execute end-to-end against a real `calibredb`.

## Definition of Done -- Checklist

- [x] Setup wizard scan reports plugin presence + configured column
      (`Adapter.Detection()`).
- [x] ReadSync canonical progress consistently lands in
      `#readsync_progress`.
- [x] Goodreads-derived events never overwrite a fresher local reader
      event (gate `recent_local_change` in `EvaluateWriteback`).
- [x] Clear user-facing log lines and `goodreads_bridge_stale` reason.
- [x] Compile-time interface assertions for `Adapter`, `EventEmitter`,
      `WriteTarget`.
- [x] Plugin detection + missing-ID report tested with
      `fixtures/goodreads/*.json`.
- [x] Stale-state detection unit-tested with a real `CanonicalProgress`.
- [x] Writeback gate unit-tested for every blocker.
- [x] Integration test runs end-to-end OR skips cleanly. Pure-function
      unit tests cover the simulator's logic in the skipped case.
- [x] Pre-existing `go vet` errors and silent test failures fixed.

## Non-Goals (explicit)

- No Goodreads API calls. No scraping. No Kindle-cloud calls.
- No plugin code loaded, executed, copied, or vendored.
- No automatic resolution of missing Goodreads IDs.
- No Phase 4 UI (the guided-plugin checklist surface lives there).
- Companion-plugin RPC is intentionally a hook only.

## How to Run the Tests

```powershell
$env:GOWORK = "off"          # main module is not in go.work
go vet ./...                 # passes
go build ./...               # passes
go test -count=1 -timeout 120s ./internal/...
```

- Without Calibre installed: 36 unit tests pass + 1 integration test skips.
- With Calibre installed but no calibre process running: all 37 tests pass.
- With Calibre running: 36 unit tests pass + 1 integration test skip.
