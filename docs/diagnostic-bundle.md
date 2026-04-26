# ReadSync Diagnostic Bundle

## Overview

ReadSync can export a diagnostic bundle — a ZIP archive containing logs,
health snapshots, and a redacted configuration summary — for troubleshooting
and support purposes. The bundle **never contains secrets** (passwords,
tokens, credentials). Redaction is enforced by `internal/logging/redact.go`.

---

## Exporting a Bundle

### Via the Admin UI (recommended)

1. Open the admin UI: `http://127.0.0.1:7201/`
2. Navigate to **Repair → Export Diagnostics**.
3. Click **Export**. The bundle is saved to `%APPDATA%\ReadSync\diagnostics\`
   with a timestamp filename, e.g.:
   `ReadSync-diagnostics-20260425-143022.zip`
4. The UI confirms the path after export.

### Via the CLI

```powershell
readsyncctl diagnostics export
# Output: C:\Users\<user>\AppData\Roaming\ReadSync\diagnostics\ReadSync-diagnostics-<ts>.zip
```

### Via the API (admin only, loopback)

```powershell
# Fetch CSRF token first.
$csrf = (Invoke-RestMethod http://127.0.0.1:7201/csrf).csrf

# Trigger export.
Invoke-RestMethod -Method POST http://127.0.0.1:7201/api/repair/export_diagnostics `
  -Headers @{ "X-ReadSync-CSRF" = $csrf }
```

---

## Bundle Contents

| File | Description |
|------|-------------|
| `activity.log` | Human-readable activity stream (last 10 000 lines) |
| `service.jsonl` | Engineering JSONL log (last 10 000 lines) |
| `health.json` | Current health state of all adapters |
| `outbox.json` | Pending and dead-letter outbox jobs (no payloads) |
| `conflicts.json` | Open conflict records |
| `schema_version.txt` | Applied database migrations |
| `system_info.txt` | OS, Go version, service version, uptime |
| `config_redacted.json` | Configuration summary — **all secret fields replaced with `[REDACTED]`** |

---

## Security Guarantees

- All secret fields (`password`, `token`, `api_key`, `credential`, etc.) are
  replaced with `[REDACTED]` before writing to any log or bundle file.
- The bundle itself contains no database content (no book titles, progress
  values, or personal data beyond what appears in logs).
- The bundle is saved to the local user's `AppData` folder and is never
  uploaded automatically.

---

## Verifying Redaction

The log corpus sweep test in `internal/logging/redact_corpus_test.go` and
`tests/security/admin_ui_test.go` verifies that known secret values do not
appear in any log output. Run:

```powershell
go test -v -run TestLogs ./internal/logging/... ./tests/security/...
```

---

## Bundle Size

Typical bundle size: **< 2 MB** (compressed). The activity and JSONL logs are
capped at 10 MB each before rotation, so worst case is ~20 MB uncompressed,
~4 MB compressed.

---

## Sharing the Bundle

Share the bundle directly with ReadSync support by attaching it to a GitHub
issue (redact if public) or via a private channel. Do **not** share the raw
`readsync.db` database file, as it contains your reading progress data.
