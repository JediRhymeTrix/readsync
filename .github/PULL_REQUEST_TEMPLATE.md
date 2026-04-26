<!-- ReadSync Pull Request Template
     See CONTRIBUTING.md for the full contribution guide.
     Delete sections that are not applicable to this PR. -->

## Summary

<!-- One or two sentences explaining WHAT this PR does and WHY. -->

Closes #<!-- issue number, or remove this line -->

---

## Type of Change

<!-- Check all that apply -->

- [ ] 🐛 Bug fix (non-breaking change that fixes an issue)
- [ ] 🚀 New feature (non-breaking change that adds functionality)
- [ ] ⚠️ Breaking change (fix or feature that changes existing behaviour)
- [ ] ♻️ Refactor (no functional change, no public API change)
- [ ] 📖 Documentation only
- [ ] 🔧 CI / build / tooling
- [ ] ⬆️ Dependency update

---

## Affected Component(s)

<!-- Check all that apply -->

- [ ] KOReader adapter (`internal/adapters/koreader/`)
- [ ] Moon+ WebDAV adapter (`internal/adapters/moon/`)
- [ ] Calibre adapter (`internal/adapters/calibre/`)
- [ ] Goodreads bridge (`internal/adapters/goodreads/`)
- [ ] Admin UI / Setup Wizard (`internal/api/`, `internal/setup/`)
- [ ] Conflict engine (`internal/conflicts/`)
- [ ] Outbox worker (`internal/outbox/`)
- [ ] Identity resolver (`internal/resolver/`)
- [ ] Secrets / credentials (`internal/secrets/`)
- [ ] Logging / diagnostics (`internal/logging/`, `internal/diagnostics/`)
- [ ] Windows Service / tray (`cmd/readsync-service/`, `cmd/readsync-tray/`)
- [ ] CLI (`cmd/readsyncctl/`)
- [ ] Database / migrations (`internal/db/`)
- [ ] Installer (`installer/`)
- [ ] CI / GitHub Actions (`.github/workflows/`)
- [ ] Tests only

---

## Changes Made

<!-- Bullet points describing the key changes. -->

- 
- 

---

## Testing

<!-- Describe how you tested these changes. -->

- [ ] Unit tests pass locally (`go test ./...`)
- [ ] Integration tests pass locally (`CGO_ENABLED=1 go test ./...`)
- [ ] New tests added for new behaviour
- [ ] Manually tested on Windows 10/11 (if applicable)
- [ ] E2E wizard tests pass (`make test-e2e`, if UI changed)

### Test commands run

```bash
# Paste the exact test commands you ran
```

---

## Release Drafter Label

<!-- Apply exactly ONE of these labels to control the SemVer bump:
     breaking / semver-major   →  X.0.0
     enhancement / semver-minor  →  x.Y.0
     bug / fix / semver-patch   →  x.y.Z
     skip-changelog             →  excluded from release notes -->

<!-- Label applied: [paste label here] -->

---

## Checklist

- [ ] My code follows the conventions in [CONTRIBUTING.md](../CONTRIBUTING.md).
- [ ] I have updated documentation (README, docs/, inline comments) as needed.
- [ ] I have NOT included credentials, secrets, or personal data.
- [ ] `go vet ./...` passes with no new warnings.
- [ ] This PR targets the `main` branch (GitHub Flow — see CONTRIBUTING.md).
- [ ] I have applied an appropriate [Release Drafter label](#release-drafter-label).
