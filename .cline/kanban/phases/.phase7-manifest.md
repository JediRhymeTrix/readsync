# Phase 7 Manifest — QA & Hardening

> Generated: 2026-04-25
> Depends on: Phases 0–6.

## Summary

Phase 7 delivers the complete QA and hardening pass against master spec
section 20 acceptance criteria, including unit tests, integration tests,
security tests, release prep artifacts, and the acceptance-criteria checklist.

---

## Files Created (Phase 7)

### Unit Tests

| File | Tests Added |
|------|-------------|
| `internal/resolver/resolver_bands_test.go` | All 5 confidence bands, all 10 signal scores, case-insensitive normalization, fuzzy title/author, higher-signal-wins priority |
| `internal/conflicts/conflict_scenario_test.go` | Spec §6 three-way scenario (KOReader 72% / Calibre 70% / Goodreads 38%), all 5 suspicious-jump detectors, abandoned bypass |
| `internal/outbox/statemachine_test.go` | queued→succeeded, queued→retrying, retrying→deadletter, backoff growth, max delay cap, wrong-target ignored |
| `internal/logging/redact_corpus_test.go` | Log corpus sweep (6 real secret values), boundary word matching (author≠auth), bearer/basic pattern redaction, long-token field redaction |
| `internal/adapters/calibre/calibre_unit_test.go` | OPF parser (percent mode, page mode, ISBN-13/10 routing, no-progress not emitted), `extractValueHash` (int/string/null/missing/empty), required columns count/names, `quoteEnumValues` |
| `internal/adapters/moon/parser_extended_test.go` | Boundary percentages (0%, 100%), out-of-range rejection (200%), malformed payloads, filename case variants, ToAdapterEvent status mapping, `.an` annotations ignored |
| `internal/adapters/koreader/translate_test.go` | `locatorType` (EPUB CFI, percent string, empty), `toAdapterEvent` (reading/finished/0%), `canonicalToPull` round-trip and nil handling |

### Integration Tests (fake adapters)

| File | Tests |
|------|-------|
| `tests/integration/e2e_test.go` | Fake KOReader push → canonical; outbox jobs queued after write; DB WAL mode confirmed |
| `tests/integration/conflict_scenario_test.go` | Spec §6 pipeline scenario; backward jump creates open conflict |

### Security Tests

| File | Tests |
|------|-------|
| `tests/security/admin_ui_test.go` | Admin UI construction (loopback default); CSRF required on 11 write endpoints; GET endpoints bypass CSRF; valid token accepted; secrets not in JSONL output |

### Release Prep

| File | Description |
|------|-------------|
| `CHANGELOG.md` | Versioned changelog from Phase 0–7 |
| `docs/diagnostic-bundle.md` | Diagnostic bundle export procedure and security guarantees |
| `docs/qa/acceptance-checklist.md` | Spec §20 criterion-to-test mapping for all 14 ACs |
| `README.md` | Updated with setup walkthrough (5 steps) |
| `Makefile` | Added `test-phase7-unit`, `test-security`, `test-integration`, `test-phase7` targets |
| `.github/workflows/ci.yml` | Added `phase7-unit`, `phase7-security`, `phase7-integration` CI jobs |

---

## Test Count (Phase 7 additions)

| Package | New Tests |
|---------|-----------|
| `internal/resolver/` | +20 (bands_test.go) |
| `internal/conflicts/` | +12 (conflict_scenario_test.go) |
| `internal/outbox/` | +6 (statemachine_test.go) |
| `internal/logging/` | +8 (redact_corpus_test.go) |
| `internal/adapters/calibre/` | +13 (calibre_unit_test.go) |
| `internal/adapters/moon/` | +11 (parser_extended_test.go) |
| `internal/adapters/koreader/` | +7 (translate_test.go) |
| `tests/integration/` | +4 (e2e_test.go + conflict_scenario_test.go) |
| `tests/security/` | +5 (admin_ui_test.go) |
| **Total** | **+86 tests** |

---

## Acceptance Criteria Coverage

All 14 acceptance criteria from spec §20 have linked tests or documented
manual verification. See `docs/qa/acceptance-checklist.md` for the full
mapping.

| AC | Description | Evidence |
|----|-------------|----------|
| AC-1 | Windows service install | `installer/smoke.ps1` ✅ |
| AC-2 | Wizard configuration | `tests/wizard.spec.js` + `setup_test.go` ✅ |
| AC-3 | KOReader push → canonical | `koreader_test.go` + `e2e_test.go` ✅ |
| AC-4 | Canonical → Calibre columns | `calibre_unit_test.go` + `integration_test.go` ✅ |
| AC-5 | Goodreads bridge | `bridge_test.go` ✅ |
| AC-6 | Moon+ WebDAV | `moon_test.go` + `webdav_test.go` ✅ |
| AC-7 | Moon+ safe storage | `moon_test.go` + `parser_extended_test.go` ✅ |
| AC-8 | Conflict detection | `conflict_scenario_test.go` + pipeline_test.go ✅ |
| AC-9 | Outbox retries | `statemachine_test.go` ✅ |
| AC-10 | Log redaction | `redact_corpus_test.go` + `admin_ui_test.go` ✅ |
| AC-11 | Crash recovery | WAL mode tested + documented 📋 |
| AC-12 | Idle resources | Design proof documented 📋 |
| AC-13 | No GR API key | Architecture + grep-verified ✅ |
| AC-14 | No Electron | Architecture + go.mod verified ✅ |

---

## How to Run Phase 7 Tests

```powershell
# Unit tests (no CGO required):
go test -v -count=1 ./internal/resolver/... ./internal/conflicts/... `
         ./internal/logging/... ./internal/outbox/... `
         ./internal/adapters/moon/...

# Calibre + KOReader unit tests (CGO required):
$env:CGO_ENABLED = "1"
go test -v -count=1 ./internal/adapters/calibre/...
go test -v -count=1 ./internal/adapters/koreader/...

# Security tests (no CGO required):
go test -v -count=1 ./tests/security/...

# Integration tests (CGO required):
$env:CGO_ENABLED = "1"
go test -v -count=1 ./tests/integration/...

# All Phase 7 (shortcut):
make test-phase7
```
