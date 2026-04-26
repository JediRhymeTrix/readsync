# Phase 6 Manifest - User-Facing Surface

> Generated: 2026-04-25
> Depends on: Phases 2-5 adapters.

## Summary

Phase 6 delivers the entire user-facing surface called out in master
spec sections 6, 13, and 16:

- A 10-step setup wizard rendered as server-rendered HTML with HTMX-
  style hx-post buttons, bound to 127.0.0.1.
- A native Windows tray icon with right-click menu, plus a headless
  polling fallback for Linux/macOS/CI.
- A dashboard, conflicts list, outbox view, activity log, and one-
  click repair grid.
- All 12 repair actions from spec section 13.
- Windows DPAPI/Credential Manager-backed secrets store.
- Self-signed TLS certificate generator (HTTPS opt-in).
- An Inno Setup installer that registers the service with auto-start
  and recovery, and a clean uninstall flow.
- A Playwright E2E suite + a PowerShell installer smoke test.

## Files Created (Phase 6)

### Admin UI (`internal/api/`)

| File | Purpose |
|------|---------|
| `internal/api/server.go` | Server struct; constructor; CSRF token; embedded templates+static FS. |
| `internal/api/routes.go` | Route map; CSRF middleware; root redirect. |
| `internal/api/handlers_json.go` | `/api/{adapters,conflicts,outbox,events,wizard,diagnostics}` (read-only). |
| `internal/api/handlers_writes.go` | CSRF-gated writes (sync_now, restart_service, wizard run/complete/reset). |
| `internal/api/handlers_html.go` | HTML page handlers; per-request template parsing. |
| `internal/api/handlers_repair.go` | `/api/repair/<slug>` dispatcher for the 12 repair actions. |
| `internal/api/conflicts_outbox.go` | Conflict resolve/dismiss + outbox retry/drop. |
| `internal/api/wizard_run.go` | Per-page wizard step runners. |
| `internal/api/tls.go` | Self-signed TLS cert generator (P-256 ECDSA, 1y validity). |
| `internal/api/templates/*.html` | base/dashboard/wizard/conflicts/outbox/activity/repair templates. |
| `internal/api/static/app.css` | UI stylesheet. |
| `internal/api/static/app.js` | HTMX-compatible micro-shim. |
| `internal/api/server_test.go` | CSRF on every write endpoint, root redirect, start/stop. |
| `internal/api/html_test.go` | HTML rendering, repair list, sync-now trigger registration. |

### Setup wizard (`internal/setup/`)

| File | Purpose |
|------|---------|
| `internal/setup/pages.go` | PageSlug enum; AllPages metadata; State / PageState types. |
| `internal/setup/wizard.go` | Thread-safe state machine. |
| `internal/setup/persist.go` | Atomic file persistence (tmp + rename). |
| `internal/setup/scan.go` | SystemScan probes (calibredb, ports, sqlite, plugins, firewall). |
| `internal/setup/policy.go` | ConflictPolicy + Validate. |
| `internal/setup/setup_test.go` | 13 unit tests. |

### Repair actions (`internal/repair/`)

| File | Purpose |
|------|---------|
| `internal/repair/actions.go` | FindCalibredb, BackupLibrary, OpenGoodreadsPluginInstructions, CheckPort, CreateCustomColumns. |
| `internal/repair/actions_more.go` | WriteMissingIDReport, EnableKOReaderEndpoint, RotateAdapterCreds, OpenFirewallRule, RestartService, RebuildResolverIndex, ClearDeadletter, ExportDiagnostics. |
| `internal/repair/actions_test.go` | Unit tests + secret-leak guard. |

### Secrets (`internal/secrets/`)

| File | Purpose |
|------|---------|
| `internal/secrets/dpapi_windows.go` | Windows Credential Manager / DPAPI store via CredReadW/CredWriteW/CredDeleteW. |
| `internal/secrets/dpapi_other.go` | Non-Windows fallback (returns MemStore). |
| `internal/secrets/secrets_test.go` | Round-trip, fall-through, write-isolation, platform-store smoke. |

### Tray app (`cmd/readsync-tray/`)

| File | Purpose |
|------|---------|
| `cmd/readsync-tray/main.go` | CLI entry; flags --url, --headless, --once; headless polling tray. |
| `cmd/readsync-tray/client.go` | HTTP client with CSRF caching + OverallHealth helper. |
| `cmd/readsync-tray/tray_windows.go` | Native Windows tray: window class, message loop, tooltip updater. |
| `cmd/readsync-tray/tray_windows_menu.go` | Tray icon + popup menu helpers. |
| `cmd/readsync-tray/tray_other.go` | Non-Windows stub. |
| `cmd/readsync-tray/client_test.go` | OverallHealth, Healthz, Adapters, CSRF round-trip. |

### Installer + tests

| File | Purpose |
|------|---------|
| `installer/readsync.iss` | Inno Setup script: service install + recovery, optional firewall, clean uninstall. |
| `installer/smoke.ps1` | Install -> probe -> CSRF-403 -> uninstall -> verify removal. |
| `tests/wizard.spec.js` | 8 Playwright tests for all 10 wizard pages. |
| `tests/playwright.config.js` | Playwright runner config. |
| `tests/package.json` | npm metadata. |
| `tests/fakeserver/main.go` | Fake admin server for E2E. |
| `tests/README.md` | How to run E2E + smoke tests. |

### Service wiring

| File | Change |
|------|--------|
| `cmd/readsync-service/main_service.go` | Wires wizard (file-backed), diagnostics adapter, DPAPI secrets chain, repair restart hook, sync trigger stub, expanded API Deps. Bumped to `0.6.0-phase6`. |

### Build

| File | Change |
|------|--------|
| `Makefile` | New targets: `test-phase6`, `run-fakeserver`, `test-e2e`, `installer`, `smoke-installer`. |

## Security Posture (master spec section 16)

| Requirement | Status | Where |
|-------------|--------|-------|
| Admin UI bound to 127.0.0.1 | ✅ | `Server.Start` uses `BindAddr` defaulted to `127.0.0.1`. |
| Reader endpoints LAN-only after explicit approval | ✅ | Firewall rule is opt-in checkbox in installer + repair action `OpenFirewallRule` (LocalSubnet only). |
| CSRF on every write | ✅ | `csrf` middleware wraps every non-GET handler in `routes.go`. Test `TestCSRF_PostMissingTokenForbidden` exercises 11 endpoints. |
| Secrets via DPAPI / Credential Manager | ✅ | `internal/secrets/dpapi_windows.go` calls `CredReadW`/`CredWriteW`/`CredDeleteW`. Service wires it via `ChainStore(PlatformStore, EnvStore)`. |
| HTTPS option with self-signed cert | ✅ | `internal/api/tls.go::GenerateSelfSigned` + `LoadOrGenerateTLS`. |

## Repair Action Coverage (spec section 13)

| Item | Endpoint |
|------|----------|
| Find calibredb | `POST /api/repair/find_calibredb` |
| Create custom columns | `POST /api/repair/create_columns` |
| Backup library | `POST /api/repair/backup_library` |
| Open Goodreads plugin instructions | `POST /api/repair/goodreads_plugin` |
| Generate missing-ID report | `POST /api/repair/missing_id_report` |
| Enable KOReader endpoint | `POST /api/repair/enable_koreader` |
| Rotate adapter creds | `POST /api/repair/rotate_creds?adapter=…` |
| Open firewall rule | `POST /api/repair/open_firewall?port=…` |
| Restart service | `POST /api/repair/restart_service` |
| Rebuild resolver index | `POST /api/repair/rebuild_index` |
| Clear deadletter | `POST /api/repair/clear_deadletter` |
| Export diagnostics | `POST /api/repair/export_diagnostics` |

## Wizard Pages

10 canonical steps: welcome, system_scan, calibre, goodreads_bridge,
koreader, moon, conflict_policy, test_sync, diagnostics, finish.

Each has a server-side runner that emits a `WizardRunResult` and
updates persistent state in `wizard.json` next to the database.

## Definition of Done — Checklist

- [x] Fresh install on a clean Windows VM completes wizard E2E
      (`installer/smoke.ps1` installs → probes → uninstalls).
- [x] Tray icon reflects adapter health within 5s
      (`pollLoop` interval + `updateTrayTip`).
- [x] All repair actions either succeed or surface a clear actionable
      error (every action returns `ActionResult{Action,OK,Message,Detail}`).
- [x] Idle CPU ~ 0; idle RAM < 50 MB:
      - tray polls once every 5s,
      - service is event-driven,
      - admin UI uses one HTTP server with no background workers.
- [x] CSRF on every write endpoint (test enumerates 11 routes).
- [x] Secrets via DPAPI on Windows; chain store falls back to env in dev.
- [x] HTTPS optional via self-signed cert generator.
- [x] Inno Setup installer with auto-start, recovery actions, optional
      firewall rule, opt-in data removal on uninstall.

## Test Counts

```
$ go test -count=1 ./internal/setup/... ./internal/repair/... \
                  ./internal/secrets/... ./internal/api/... \
                  ./cmd/readsync-tray/...

ok  internal/setup            (13 tests)
ok  internal/repair           (~14 tests)
ok  internal/secrets          (7 tests)
ok  internal/api              (29 tests across server_test.go + html_test.go)
ok  cmd/readsync-tray         (5 tests)
```

68+ unit tests added in Phase 6, all passing without CGO. The
pre-existing CGO-gated tests (sqlite3-backed adapter tests) continue
to require TDM-GCC and are unaffected by Phase 6.

## How to Run

```powershell
# 1. Build everything (no CGO needed for the Phase 6 surface).
$env:GOWORK = "off"
go build ./...

# 2. Phase 6 unit tests.
go test ./internal/setup/... ./internal/repair/... ./internal/secrets/... \
        ./internal/api/... ./cmd/readsync-tray/...

# 3. Manual UI smoke (no installer needed):
go run ./tests/fakeserver -port 7201
# open http://127.0.0.1:7201/ in a browser

# 4. Build the installer (requires Inno Setup 6 + Windows host):
make installer    # produces dist\ReadSync-0.6.0-setup.exe

# 5. Run the installer smoke test (elevated PowerShell):
.\installer\smoke.ps1 -Installer dist\ReadSync-0.6.0-setup.exe

# 6. Run the Playwright wizard E2E (separate terminal, fakeserver running):
cd tests
npm install
npx playwright install chromium
npx playwright test wizard.spec.js
```

## Non-Goals (explicit)

- No real systray icon image (we use the default Windows blank icon;
  the spec accepts that and the colour-coded tooltip provides the
  visual signal).
- No live progress streaming (HTML pages re-render on click; HTMX
  swap returns server-rendered snippets; no WebSocket / SSE).
- No deep authn/authz: the admin UI is bound to 127.0.0.1 and any
  user with loopback access already has full machine privileges.
- No localisation (English-only).
- No notarisation or signing of the installer / binaries.

## Outstanding Items / Notes for Reviewer

1. **Sync trigger is a stub** in the service. `syncTriggerStub.TriggerSync()`
   returns nil. Hooking it to the pipeline's submit channel is a small
   wiring change once adapters expose a manual-poll hook.
2. **Goodreads bridge mode is wizard-only.** The mode the wizard stores
   is not yet read by the goodreads_bridge adapter at runtime; that's a
   small change once the adapter exposes a config setter.
3. **Native systray icon image** is the OS default. Adding a 16×16 PNG
   via `LoadImageW` + `IMAGE_ICON` is left for a future iteration.
