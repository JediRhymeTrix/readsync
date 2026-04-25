# Windows Service Framework Decision

> **Status:** Decision recorded — Go selected.  
> **Last updated:** 2026-04-25  
> See also: `docs/adr/0001-language-and-service-framework.md`

---

## 1. Options Evaluated

### Option A: Go + `kardianos/service`

**Library:** https://github.com/kardianos/service  
**License:** Apache-2.0

| Aspect                  | Assessment                                                          |
|-------------------------|---------------------------------------------------------------------|
| Windows service support | Full: Install/Start/Stop/Uninstall via `service.Control()`         |
| Cross-platform         | Yes: also launchd (macOS) and systemd (Linux)                       |
| Build complexity        | `GOOS=windows go build` → single static binary                      |
| Admin for install       | Required for SCM registration                                       |
| Logging                 | Windows Event Log via `service.Logger`                             |
| Binary size             | ~5–10 MB statically linked                                          |
| CGO required            | No (pure Go)                                                        |
| Hello-world complexity  | ~50 lines                                                           |

### Option B: Rust + `windows-service` crate

**Crate:** https://crates.io/crates/windows-service  
**License:** MIT

| Aspect                  | Assessment                                                          |
|-------------------------|---------------------------------------------------------------------|
| Windows service support | Full: `define_windows_service!` macro + dispatcher                  |
| Cross-platform         | Windows only                                                        |
| Build complexity        | Requires MSVC or mingw; harder to cross-compile                     |
| Binary size             | ~1–3 MB (smaller)                                                   |
| Hello-world complexity  | ~80 lines + boilerplate macros                                      |

---

## 2. Decision Matrix

| Criterion                      | Go                            | Rust                           |
|--------------------------------|-------------------------------|--------------------------------|
| WebDAV server (Moon+)          | ✅ `golang.org/x/net/webdav`  | ❌ No stdlib, needs ext crate   |
| HTTP/JSON (KOReader sim)       | ✅ `net/http` stdlib          | ✅ `axum`                       |
| `calibredb` subprocess         | ✅ `os/exec`                  | ✅ `std::process::Command`      |
| Filesystem watcher             | ✅ `fsnotify`                 | ✅ `notify` crate               |
| Windows registry               | ✅ `x/sys/windows/registry`   | ✅ `winreg`                     |
| Cross-compile (Linux CI)       | ✅ Trivial                    | ⚠️ Requires MSVC on builder     |
| Developer familiarity          | ✅ Primary language           | ⚠️ Secondary                    |
| Build speed                    | ✅ Fast                       | ⚠️ Slow first build             |

---

## 3. Decision: **Go with `kardianos/service`**

**Rationale:**

1. **WebDAV stdlib**: `golang.org/x/net/webdav` is essential for Moon+ sync
   and fixture recorder; no Rust equivalent in stdlib.
2. **Single static binary**: `CGO_ENABLED=0 go build` produces a zero-dependency
   `.exe` — ideal for a Windows background service.
3. **Cross-compilation**: Go trivially targets Windows from Linux CI runners
   without MSVC toolchain.
4. **`kardianos/service` maturity**: Battle-tested in HashiCorp Vault, Consul.
5. **Developer familiarity**: Go is the team's primary language.

---

## 4. `readsyncctl` Commands

```
readsyncctl install    # Register Windows service (requires admin)
readsyncctl uninstall  # Remove service (requires admin)
readsyncctl start      # Start service (requires admin)
readsyncctl stop       # Stop service (requires admin)
readsyncctl status     # Show status (no admin needed)
readsyncctl run        # Foreground mode for debugging
```

---

## 5. Spike Verification

See `tools/winsvc-spike/` for hello-world implementation.

```powershell
cd tools\winsvc-spike
go build -o readsync-spike.exe .
.\readsync-spike.exe install
.\readsync-spike.exe start
.\readsync-spike.exe status
.\readsync-spike.exe stop
.\readsync-spike.exe uninstall
```

Service installs as "ReadSyncSpike"; logs heartbeat to Windows Event Log.

---

## References

- kardianos/service: https://github.com/kardianos/service
- golang.org/x/net/webdav: https://pkg.go.dev/golang.org/x/net/webdav
- windows-service crate: https://docs.rs/windows-service
- Windows Services docs: https://docs.microsoft.com/en-us/windows/win32/services/services
