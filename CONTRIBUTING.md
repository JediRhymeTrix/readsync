# Contributing to ReadSync

Thank you for your interest in contributing! This guide covers branching,
commits, pull requests, and the release process.

---

## Table of Contents

1. [Code of Conduct](#code-of-conduct)
2. [Ways to Contribute](#ways-to-contribute)
3. [Development Setup](#development-setup)
4. [Branching Model — GitHub Flow](#branching-model--github-flow)
5. [Commit Messages](#commit-messages)
6. [Pull Requests](#pull-requests)
7. [Release Process & SemVer](#release-process--semver)
8. [Labels Reference](#labels-reference)
9. [Tests](#tests)
10. [Code Style](#code-style)
11. [Security](#security)

---

## Code of Conduct

All contributors must follow the [Code of Conduct](CODE_OF_CONDUCT.md).

---

## Ways to Contribute

| What | How |
|------|-----|
| Report a bug | [Bug Report template](.github/ISSUE_TEMPLATE/bug_report.yml) |
| Request a feature | [Feature Request template](.github/ISSUE_TEMPLATE/feature_request.yml) |
| Ask a question | [Question template](.github/ISSUE_TEMPLATE/question.yml) or [Discussions](https://github.com/readsync/readsync/discussions) |
| Fix a bug / add feature | Fork → branch → PR |
| Improve documentation | Fork → branch → PR |
| Triage issues | Comment, add labels, link duplicates |

---

## Development Setup

**Prerequisites:** Go 1.22+, GCC (Windows: [TDM-GCC](https://jmeubank.github.io/tdm-gcc/)), Node.js 18+ (E2E only).

```bash
git clone https://github.com/<your-fork>/readsync.git && cd readsync
go mod tidy && go mod download

# Pure unit tests (no CGO)
go test -v ./internal/resolver/... ./internal/conflicts/... ./internal/logging/...

# All tests (CGO required)
CGO_ENABLED=1 go test -v ./...
```

See `make help` for all available targets.

---

## Branching Model — GitHub Flow

ReadSync uses **GitHub Flow**. There is **one** long-lived branch: `main`.

```
main  (always deployable — direct pushes blocked)
  ├── feat/koreader-rate-limit    ← merge and delete
  ├── fix/moon-webdav-auth        ← merge and delete
  └── chore/update-actions        ← merge and delete
```

**Rules:**
- Always branch from `main`.
- One concern per branch; keep them short-lived.
- No `develop`, `release/*`, or `hotfix/*` branches (not GitFlow).
- Push a `vX.Y.Z` tag to trigger a release (maintainers only).

**Branch naming:** `<type>/<short-description>`
e.g. `feat/koreader-push`, `fix/outbox-cap`, `deps/bump-gin-v1.11`

---

## Commit Messages

Follow Conventional Commits (loosely):

```
<type>(<scope>): <imperative summary>

[body — why, not what]

[footer: Closes #123 | BREAKING CHANGE: ...]
```

Types: `feat`, `fix`, `docs`, `refactor`, `test`, `chore`, `ci`, `security`

---

## Pull Requests

1. Fork, create a branch, make changes with tests.
2. Ensure `go vet ./...` and `go test ./...` pass locally.
3. Open a PR against `main` using the [PR template](.github/PULL_REQUEST_TEMPLATE.md).
4. Apply **exactly one** [SemVer label](#release-process--semver).
5. CODEOWNERS will request reviewers automatically.
6. Push commits to address feedback; avoid force-pushing after review starts.
7. A maintainer will **squash-merge** when approved.

---

## Release Process & SemVer

ReadSync follows [Semantic Versioning 2.0.0](https://semver.org/): `MAJOR.MINOR.PATCH`.

### SemVer bump via Release Drafter labels

| Label | Bump | When |
|-------|------|------|
| `breaking` or `semver-major` | **MAJOR** (X.0.0) | Incompatible API change |
| `enhancement`, `feature`, or `semver-minor` | **MINOR** (x.Y.0) | New backwards-compatible feature |
| `bug`, `fix`, `security`, `dependencies`, or `semver-patch` | **PATCH** (x.y.Z) | Bug fix / dependency update |
| `skip-changelog` | (omitted) | CI-only or internal tweak |

### How a release is cut

1. Every PR merged to `main` auto-updates a **draft release** via
   [Release Drafter](.github/release-drafter.yml) — no manual changelog editing.
2. A maintainer reviews the draft, edits if needed, then clicks **Publish**.
3. Publishing creates a `vX.Y.Z` tag → triggers
   [`.github/workflows/release.yml`](.github/workflows/release.yml).
4. The release workflow cross-compiles Windows binaries and attaches
   `ReadSync-vX.Y.Z-windows-amd64.zip` + `SHA256SUMS.txt` to the release.
5. Tags containing `-` (e.g. `v1.0.0-rc.1`) are marked pre-release automatically.

---

## Labels Reference

Managed in [`.github/labels.yml`](.github/labels.yml), synced by CI.

| Label | Purpose |
|-------|---------|
| `bug` / `fix` | Broken behaviour |
| `enhancement` / `feature` | New capability |
| `documentation` | Docs only |
| `security` | Security fix |
| `dependencies` | Dependency update |
| `ci` / `build` | Automation / build system |
| `breaking` / `semver-major/minor/patch` | SemVer version bump control |
| `skip-changelog` | Omit from release notes |
| `good-first-issue` | Beginner-friendly |
| `help-wanted` | Community contributions welcome |
| `work-in-progress` | PR not ready for review |
| `stale` | No activity for 30–60 days (auto-closed after 14 more) |

---

## Tests

| Make target | Scope | CGO? |
|-------------|-------|------|
| `make test-unit` | Resolver, conflicts, logging, outbox, Moon+ | No |
| `make test-security` | CSRF, secret redaction | No |
| `make test-integration` | Pipeline + fake adapters | Yes |
| `make test-phase7` | All P7 unit + security | No |
| `make test` | Everything | Yes |
| `make test-e2e` | Playwright wizard suite | No (Node) |

All new features and bug fixes **must** include tests.

---

## Code Style

- `gofmt` / `goimports` formatting (enforced by CI).
- `go vet ./...` must pass with no new warnings.
- Error strings: lowercase, no trailing punctuation.
- No `panic()` in production code paths.
- Secrets must flow through `internal/secrets/` — never hardcoded or logged.

---

## Security

Do **not** open public issues for security vulnerabilities.
Read [SECURITY.md](SECURITY.md) for the responsible disclosure process.
