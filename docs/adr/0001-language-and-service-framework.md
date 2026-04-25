# ADR 0001: Core Language and Windows Service Framework

| Field       | Value                          |
|-------------|--------------------------------|
| **ID**      | 0001                           |
| **Status**  | Accepted                       |
| **Date**    | 2026-04-25                     |
| **Deciders**| ReadSync Phase 0 research      |

---

## Context

ReadSync is a Windows background service that:

1. Implements a KOSync-compatible HTTP server (KOReader progress sync)
2. Implements an embedded WebDAV server (Moon+ Reader Pro sync)
3. Invokes `calibredb` subprocess for Calibre metadata updates
4. Watches the filesystem for Calibre library changes
5. Reads Windows registry to locate Calibre installation
6. Installs and operates as a proper Windows Service (SCM-managed)
7. Ships as a single self-contained binary with no runtime dependencies

Two implementation options were evaluated:
- **Option A**: Go 1.22+ with `kardianos/service` library
- **Option B**: Rust 1.78+ with `windows-service` crate

Full analysis: `docs/research/windows-service.md`

---

## Decision

**ReadSync will be implemented in Go 1.22+ using the `kardianos/service`
library for Windows service lifecycle management.**

---

## Consequences

### Positive

1. **WebDAV available**: `golang.org/x/net/webdav` provides a production-quality
   WebDAV server for Moon+ sync. The Phase 0 recorder uses stdlib-only net/http
   for the narrow WebDAV subset Moon+ needs; the full x/net/webdav package is
   available for Phase 3 production use. No Rust equivalent exists in stdlib.

2. **Zero-dependency binary**: `CGO_ENABLED=0 GOOS=windows GOARCH=amd64 go build`
   produces a single `.exe` with no DLL or runtime dependencies. Users only
   need to run the installer — no .NET, no Visual C++ Redistributable.

3. **Cross-compilation**: Go compiles to Windows from Linux CI runners trivially.
   The GitHub Actions pipeline uses `ubuntu-latest` runners and cross-compiles
   without MSVC toolchain installation.

4. **`kardianos/service` maturity**: Used in production by HashiCorp Vault
   (Windows), Consul, and dozens of commercial products. Well-tested against
   Windows Vista through Windows 11 SCM.

5. **HTTP server stdlib**: `net/http` handles both the KOSync endpoint and the
   WebDAV endpoint without external dependencies.

6. **Subprocess management**: `os/exec` with `context.WithTimeout` reliably
   manages `calibredb` invocations including stderr capture and timeout
   cancellation.

7. **Team expertise**: Go is the team's primary language. Rust would add
   ramp-up cost without proportional benefit.

### Negative / Trade-offs

1. **Binary size**: Go binaries are larger (~8 MB) than equivalent Rust
   binaries (~2 MB). Acceptable given modern disk sizes.

2. **Memory safety**: Rust provides stronger compile-time memory safety
   guarantees. Go has a garbage collector which adds ~20ms pause risk on
   very large sync operations. Mitigation: keep hot paths allocation-free.

3. **Startup latency**: Go's runtime init is ~50ms vs Rust's ~5ms. Acceptable
   for a background service.

### Neutral

- Both languages support Windows registry access, SQLite, filesystem watching.
- Both produce Windows-native executables.

---

## Key Dependencies

| Package                            | Purpose                           | License    |
|------------------------------------|-----------------------------------|------------|
| `github.com/kardianos/service`     | Windows service lifecycle         | Apache-2.0 |
| `golang.org/x/net/webdav`          | Embedded WebDAV server (Moon+)    | BSD-3      |
| `golang.org/x/sys/windows`         | Windows API, registry access      | BSD-3      |
| `github.com/fsnotify/fsnotify`     | Filesystem watcher (Calibre DB)   | BSD-3      |
| `github.com/mattn/go-sqlite3`      | SQLite (future: local state DB)   | MIT        |

---

## Alternatives Not Chosen

### Rust + `windows-service`

**Rejected because:**
- No WebDAV implementation in stdlib; `dav-server` crate is unmaintained.
- Cross-compilation from Linux to Windows requires MSVC or complex mingw setup.
- Build times significantly slower (5–10 min full build vs 30 sec in Go).
- No blocking technical advantage over Go for this use case.

### Python + `pywin32`

**Rejected because:**
- Requires Python runtime on user's machine.
- No single-binary distribution.
- Performance concerns with concurrent HTTP + WebDAV + subprocess management.

### Node.js + `node-windows`

**Rejected because:**
- No single-binary distribution without pkg/nexe.
- Large runtime footprint.
- Not suitable for a system service.

---

## Review Checklist

- [x] WebDAV server requirement addressed
- [x] Windows service lifecycle addressed
- [x] Binary distribution model addressed
- [x] CI/CD cross-compilation addressed
- [x] All rejected alternatives documented
- [x] Key dependencies listed with licenses
- [x] Spike implementation verified: `tools/winsvc-spike/`
