# Phase 6 Manifest — User-Facing Surface

> Generated: 2026-04-25
> Depends on: Phases 2–5 adapters.

Phase 6 delivers the entire user-facing surface: 10-step setup wizard,
native Windows tray icon, admin UI dashboard, all 12 repair actions,
DPAPI secrets store, self-signed TLS, Inno Setup installer, Playwright E2E.

---

## Files Created (Phase 6)

### Admin UI (`internal/api/`)

| File | Purpose |
|------|---------|
| `internal/api/server.go` | Server struct; constructor; CSRF token; embedded templates+static FS. |
| `internal/api/routes.go` | Route map; CSRF middleware; root redirect. |
| `internal/api/handlers_json.go` | `/api/{adapters,conflicts,outbox,events,wizard,diagnostics}`. |
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
| `internal/secrets/dpapi_windows.go` | Windows Credential Manager / DPAPI store. |
| `internal/secrets/dpapi_other.go` | Non-Windows fallback (returns MemStore). |
| `internal/secrets/secrets_test.go` | Round-trip, fall-through, write-isolation, platform-store smoke. |

### Installer + E2E tests

| File | Purpose |
|------|---------|
| `installer/readsync.iss` | Inno Setup script: service install + recovery, optional firewall, clean uninstall. |
| `installer/smoke.ps1` | Install → probe → CSRF-403 → uninstall → verify removal. |
| `tests/wizard.spec.js` | 8 Playwright tests for all 10 wizard pages. |
| `tests/playwright.config.js` | Playwright runner config. |
| `tests/package.json` | npm metadata. |
| `tests/fakeserver/main.go` | Fake admin server for E2E. |

---

## Security Posture (spec §16)

| Requirement | Status |
|-------------|--------|
| Admin UI bound to 127.0.0.1 | ✅ |
| CSRF on every write (11 endpoints) | ✅ |
| Secrets via DPAPI / Credential Manager | ✅ |
| HTTPS option with self-signed cert | ✅ |
