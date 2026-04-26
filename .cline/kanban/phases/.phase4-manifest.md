# Phase 4 Deliverable Manifest

> Generated: 2026-04-25
> Depends on: Phase 1 core, Phase 0 Moon+ fixtures.

Implements the Moon+ Reader Pro adapter per master spec section 11, with
the strict layered safety design called out in the original task brief.

---

## Files Created / Updated

### New packages

| File | Purpose |
|------|---------|
| `internal/adapters/webdav/webdav.go` | Embedded WebDAV server (`Server` struct), HTTP Basic auth, per-user namespace, observer registry. |
| `internal/adapters/webdav/archive.go` | Versioned, immutable archival. `archiveUpload`, `LatestVersion`, path sanitisation helpers. |
| `internal/adapters/webdav/versioned_fs.go` | `webdav.FileSystem` wrapper that scopes paths per-user and tees every PUT into the immutable archive on Close. |
| `internal/adapters/webdav/webdav_test.go` | Litmus-style conformance tests (auth, OPTIONS, PROPFIND, MKCOL, PUT/GET, DELETE, MOVE, LOCK, version-immutability, per-user isolation, no-credentials-in-archive). |
| `internal/adapters/moon/moon.go` | `Adapter` orchestrator (Layer 1–4 wiring, health, lifecycle). |
| `internal/adapters/moon/parser.go` | Layer 3 read-only progress extractor. `FormatV1Plain`, `Parse`, `ToAdapterEvent`. |
| `internal/adapters/moon/writeback.go` | Layer 4 safe writeback generator. `writerRegistry`, `IsWriterVerified`, `SerializeV1Plain`. |
| `internal/adapters/moon/capture.go` | Layer 2 in-process fixture recorder. `EnableCapture`, `DisableCapture`, hard-link / copy fallback. |
| `internal/adapters/moon/setup.go` | Setup-wizard integration. `GenerateSetup`, `TestConnection`, LAN IP picking, QR payload, instructions. |
| `internal/adapters/moon/upload.go` | Upload observer that drives Layer 2 + Layer 3 from each archived PUT. |
| `internal/adapters/moon/parser_test.go` | Fixture-driven parser tests (5 synthetic + 9 real captures + 6 negative cases). |
| `internal/adapters/moon/writeback_test.go` | Round-trip writer tests + verification of writer-gating default. |
| `internal/adapters/moon/moon_test.go` | End-to-end integration: happy path, unknown-format negative test, capture mode, setup bundle, writeback gate. |

### Updated files

| File | Change |
|------|--------|
| `internal/db/migrations.go` | Migration 3 added: `moon_users`, `moon_uploads` tables (versioned upload registry). |
| `cmd/readsync-service/main_service.go` | Wires the Moon+ adapter at startup, listening on `0.0.0.0:8765/moon-webdav/`. |

### Synthetic fixtures generated

`fixtures/moonplus/synthetic/{010,025,050,075,100}pct.po` — exercise the
`{file_id}*{position}@{chapter}#{scroll}:{percentage}%` format for the
parser & round-trip tests. Real-device captures already present in
`fixtures/moonplus/captures/` (Phase 0).

---

## Layer-by-Layer Coverage

### Layer 1 — WebDAV storage compatibility ✅

- Embedded server bound to LAN at the configured prefix (`/moon-webdav/`).
- Methods: PROPFIND / GET / PUT / MKCOL / DELETE / MOVE / LOCK / UNLOCK /
  PROPPATCH (full `golang.org/x/net/webdav` Handler).
- Per-user credentials in `moon_users`, bcrypt cost 12, secrets handed to
  `secrets.Store` (Windows Credential Manager / DPAPI in production,
  `EnvStore` / `MemStore` in dev / tests).
- Every PUT versioned under
  `data/moon/raw/<user>/<path>/<N>.bin` + `<N>.json` manifest. `O_EXCL`
  guarantees no in-place mutation. `moon_uploads` table records sha256,
  size, archive_path, version index (UNIQUE on (user_id, rel_path,
  version)).

### Layer 2 — Fixture recorder ✅

- `EnableCapture(dir)` flips capture mode on at runtime (or via
  `Config.CaptureDir` at startup).
- Each archived upload is hard-linked into `dir`; `os.Link` failure falls
  back to a `copyFile` clone so the canonical archive stays untouched.

### Layer 3 — Read-only progress extractor ✅

- Format registry: `FormatV1Plain` is the only verified format
  (`po-v1-plain`).  `ParserVersion = "moon-parser/1.0.0"` is recorded on
  each `Result` for diagnostic traceability.
- Unknown payloads return `ErrUnknownFormat`, set adapter health to
  `degraded`, persist `parse_error` on `moon_uploads`, and never trigger
  pipeline submission.
- Verified path emits `ProgressEvent` with `source=moon`,
  `LocatorType=moon_position`, `MoonKey = basename + "#" + file_id`.

### Layer 4 — Safe writeback generator ✅

- `writerRegistry` is the static gate. `FormatV1Plain.Verified=false` by
  default — the master spec requires a committed writer fixture set
  before flipping the bit.
- `Adapter.WriteProgress` returns an explicit error so the outbox marks
  the job `blocked_by_adapter_health`. The setup wizard's `WritebackOK`
  flag is `false` and the wizard surfaces the fallback hint.
- `SerializeV1Plain` exists and is round-trip verified by tests over all
  14 fixtures, ready for the day a writer fixture set lands.

### Identity ✅

- `MoonKey` flows through `resolver.Evidence`. Phase 1's
  `findBookByEvidence` already routes `MoonKey` through `book_aliases`
  with `source=moon`. Adapter inserts an alias on first observation via
  the existing `insertBookAliases`.

### Setup integration ✅

- `Adapter.GenerateSetup(username)` returns a `SetupBundle` with:
  - `ServerURL`  : `http://<lan-ip>:8765/moon-webdav/`
  - `Username` / `Password` (random 24-char URL-safe)
  - `Instructions` : 9-step Moon+ Pro guide ending with the
    *Miscellaneous → Sync reading positions via Dropbox/WebDAV → WebDAV*
    flow.
  - `QRPayload` : `moon-webdav://user:pass@host:port/prefix`
  - `WritebackOK` / `Hint` : the Layer-4 fallback warning.
- `Adapter.TestConnection(ctx, user, pass)` issues exactly the same
  `PROPFIND Depth: 0` Moon+ Pro sends.

---

## Tests

| Test | Coverage |
|------|----------|
| `TestParse_SyntheticFixtures` (5 cases) | Synthetic .po fixtures parse with correct percent. |
| `TestParse_RealDeviceCaptures` (9 cases) | All real-device captures from Phase 0 parse without error and yield in-range percent. |
| `TestParse_UnknownFormat` (6 cases) | Empty, binary, malformed, out-of-range, unknown-suffix, missing-colon all classified `FormatUnknown`. |
| `TestParse_AnnotationsIgnored` | `.an` suffix returns unknown without panic. |
| `TestParse_ToAdapterEvent` | Resulting `core.AdapterEvent` carries `source=moon`, `MoonKey`, `LocatorType=moon_position`. |
| `TestWriteback_NotVerifiedByDefault` | Layer-4 invariant: writer is not verified out of the box. |
| `TestWriteback_RoundTrip` (14 cases) | parse → mutate → serialize → reparse equals expected for every fixture. |
| `TestSerialize_Edge` | Bad-input guards (FormatUnknown, pct ranges). |
| `TestWebDAV_AuthRequired` | Anonymous PROPFIND → 401 + WWW-Authenticate. |
| `TestWebDAV_OptionsAdvertisesMethods` | `Allow:` includes OPTIONS, PROPFIND, GET, PUT, DELETE, MKCOL. |
| `TestWebDAV_PropfindDepthZero` | Returns 207 Multi-Status. |
| `TestWebDAV_MkcolPutGet` | Round-trip + duplicate MKCOL → 405. |
| `TestWebDAV_VersionedImmutable` | Three sequential PUTs produce three immutable bins; a fourth PUT does not mutate any earlier file. |
| `TestWebDAV_DeleteAndMove` | DELETE removes, MOVE renames. |
| `TestWebDAV_LockUnlock` | LOCK request accepted (no-op acceptable). |
| `TestWebDAV_NoCredentialsInArchive` | Walks `data/` and asserts the password byte string is not present anywhere. |
| `TestWebDAV_BadPassword` | Wrong password → 401. |
| `TestWebDAV_PerUserIsolation` | Alice cannot read Bob's path → 404. |
| `TestMoon_HappyPath` | PUT a fixture-supported `.po` → `canonical_progress` row materialises, `updated_by=moon`, percent within tolerance, `moon_uploads.parsed=1`. |
| `TestMoon_UnknownFormat` | Never-seen file → bytes preserved on disk byte-for-byte, `Health=Degraded`, hint contains repair guidance, no canonical row, `parse_error` persisted. |
| `TestMoon_CaptureMode` | Capture directory receives a fixture file after a PUT. |
| `TestMoon_SetupBundle` | URL contains `/moon-webdav/`, password not leaked into instructions, writeback flagged off with hint. |
| `TestMoon_WriteProgressBlocked` | Writeback target refuses with a clear "writeback disabled" error. |

CGO / sqlite3 tests run on CI only (TDM-GCC required on Windows;
`apt-get install gcc` on Linux). Pure-Go tests (parser, writeback
round-trip) run anywhere.

---

## Definition of Done

- [x] Moon+ Pro on Android can connect to ReadSync's WebDAV using the
      wizard-generated URL+creds. (`SetupBundle.ServerURL/Username/Password`,
      `TestConnection` PROPFIND.)
- [x] A pause-then-resume cycle in Moon+ produces at least one
      `ProgressEvent` for a fixture-supported format.
      (`TestMoon_HappyPath` asserts `canonical_progress.updated_by='moon'`
      with the expected percent.)
- [x] Unknown format files are stored versioned and never mutated; user
      sees an actionable warning. (`TestMoon_UnknownFormat`,
      `TestWebDAV_VersionedImmutable`.)
- [x] No credentials in logs (logger field set to `io.Discard`/redaction
      pipeline; `TestWebDAV_NoCredentialsInArchive` walks the archive
      and asserts the password is not present).
- [x] No in-place mutation of any uploaded file. Each archive bin is
      created with `O_CREATE|O_EXCL|O_WRONLY`, mode 0o444; sequential
      PUTs allocate new version numbers.

---

## Build & Run

```powershell
# Prereq: Go 1.22+, TDM-GCC for go-sqlite3
go mod tidy
$env:CGO_ENABLED='1'

# Build everything
go build ./internal/... ./cmd/...

# Pure-Go tests (no CGO required)
go test ./internal/adapters/moon/ -run 'TestParse|TestWriteback|TestSerialize'

# Full suite (requires CGO)
go test -count=1 -timeout 120s \
    ./internal/adapters/moon/... ./internal/adapters/webdav/...

# Run service in foreground for manual Moon+ Pro testing
go run ./cmd/readsync-service run
# WebDAV will be available at http://<lan-ip>:8765/moon-webdav/
```

---

## Outstanding Items / Notes for Reviewer

1. **Writer fixture matrix not yet committed.** `writerRegistry` keeps
   `FormatV1Plain.Verified=false` deliberately. The Layer-4 spec requires
   committed `fixtures/moonplus/writers/po-v1-plain/{input,expected,mutate}`
   files before the flag flips. Round-trip is otherwise green.
2. **`go.sum` will need refreshing.** No new top-level imports were added —
   we re-use `golang.org/x/net` (already in `go.sum` as an indirect dep) for
   the `webdav` subpackage. `make deps` should remain happy.
3. **Real-device .an / .po pairing.** Per `docs/research/moonplus.md` §6,
   Moon+ also writes `.an` annotation files. These are deliberately
   classified as `FormatUnknown` *without* triggering degraded health
   (Layer-3 contract).  See `parser.go` switch.
