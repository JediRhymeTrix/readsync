# Phase 3 Deliverable Manifest

> Generated: 2026-04-25
> Depends on: Phase 1 core, Phase 0 KOReader simulator and fixtures.

---

## Deliverables

| File | Status |
|------|--------|
| `internal/adapters/koreader/koreader_impl.go` | ✅ Created |
| `internal/adapters/koreader/auth.go` | ✅ Created |
| `internal/adapters/koreader/handlers.go` | ✅ Created |
| `internal/adapters/koreader/translate.go` | ✅ Created |
| `internal/adapters/koreader/users.go` | ✅ Created |
| `internal/adapters/koreader/koreader_test.go` | ✅ Created |
| `internal/adapters/koreader/codec/codec.go` | ✅ Created (Phase 9 — pure-Go subpackage) |
| `internal/adapters/koreader/codec/codec_test.go` | ✅ Created (Phase 9 — no CGO tests) |
| `internal/adapters/koreader/koreader.go` | ✅ Gated `//go:build ignore` (Phase 1 stub superseded) |
| `internal/db/migrations.go` | ✅ Migration 2 added: `koreader_users`, `koreader_devices` |
| `internal/core/pipeline_helpers.go` | ✅ `insertBookAliases` added |
| `cmd/readsync-service/main_service.go` | ✅ Created (Phase 3 service entry point) |
| `cmd/readsync-service/main.go` | ✅ Gated `//go:build ignore` (Phase 1 entry point superseded) |
| `main.go` | ✅ Rewritten as `go run .` dev runner |
| `go.mod` | ✅ Added gin v1.10.0, x/crypto v0.23.0, x/time v0.5.0 |

---

## Protocol Coverage

| Endpoint | Method | Status |
|----------|--------|--------|
| `/users/create` | POST | ✅ Register (rate-limited, registration flag) |
| `/users/auth` | GET | ✅ Authenticate (rate-limited) |
| `/syncs/progress` | PUT | ✅ Push progress + 412 stale-check |
| `/syncs/progress/:document` | GET | ✅ Pull progress (empty `{}` if not found) |

---

## Definition of Done Checklist

- [x] KOReader/Crosspoint can register, push, pull without path reconfiguration
- [x] Push updates `canonical_progress`; pull returns latest canonical state
- [x] Credentials never stored or logged in plaintext (bcrypt cost 12)
- [x] Auth failures ≥10 → `HealthNeedsUserAction`
- [x] Registration closed by default; opened via `Config.RegistrationOpen`
- [x] Rate limiting on auth/register (10 req/min per IP)
- [x] Reverse-proxy headers trusted via `Config.TrustedProxies`
- [x] Default bind: `127.0.0.1:7200`; set `BindAddr` for LAN
- [x] 12 integration tests: conformance, negative auth, bad input, not-found
- [x] `go mod tidy` run + `go.sum` committed ✅ (completed in Phase 9)
- [x] `go test ./internal/adapters/koreader/...` passes on CI (CGO required)
- [x] `go test ./internal/adapters/koreader/codec/...` passes without CGO ✅ (Phase 9)

---

## Note (Phase 9)

`go.sum` is committed as of Phase 9 closeout. The `koreader/codec` subpackage
was extracted in Phase 9 to allow `LocatorType`, `ToEvent`, and
`CanonicalToPull` to be tested without CGO.
