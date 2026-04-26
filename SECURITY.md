# Security Policy

## Supported Versions

| Version | Supported |
|---------|-----------|
| Latest release | Yes |
| Previous minor | Security fixes only |
| Older | No |

## Reporting a Vulnerability

**Do NOT open a public GitHub issue for security vulnerabilities.**

Please report security issues by emailing the maintainers directly or
using [GitHub Private Vulnerability Reporting](https://docs.github.com/en/code-security/security-advisories/guidance-on-reporting-and-writing/privately-reporting-a-security-vulnerability)
(Security tab → Report a vulnerability).

### What to include

- Description of the vulnerability and affected component
- Steps to reproduce or proof-of-concept
- Potential impact assessment
- Your suggested fix (optional)

### Response timeline

| Stage | Target |
|-------|--------|
| Acknowledgement | Within 72 hours |
| Initial assessment | Within 7 days |
| Fix or mitigation | Within 90 days (critical: 30 days) |
| Public disclosure | After fix is released |

### Scope

In scope:
- `internal/api/` — Admin UI CSRF, authentication bypass
- `internal/secrets/` — Credential storage and retrieval
- `internal/adapters/` — Injection via protocol payloads
- Windows service privilege escalation
- Log output containing plaintext secrets

Out of scope:
- Issues requiring physical access to the machine
- Denial of service against the local loopback interface
- Vulnerabilities in third-party dependencies not yet patched upstream

## Security Design Notes

- The Admin UI binds to `127.0.0.1:7201` only (loopback, not LAN-accessible by default).
- All write endpoints require a CSRF token (11 endpoints protected).
- Credentials are stored via Windows DPAPI / Credential Manager.
- Log output is scrubbed by `internal/logging/redact.go` before writing.
- No Goodreads API key is stored — the bridge operates via the Calibre plugin only.
