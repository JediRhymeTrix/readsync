# ReadSync — GitHub Publishing Prompt

> **Purpose**: Self-contained, copy-paste prompt for a separate agent (or human)
> that needs to push the ReadSync repository to GitHub and fully configure it.
> Every `gh` CLI and GitHub REST API command needed is included, in order, with
> all placeholders clearly marked.
>
> **Prerequisites**:
> - `git` (repo already committed, ready to push)
> - `gh` CLI ≥ 2.40 — authenticated via `gh auth login`
> - `curl` and `jq`
> - GitHub account with permissions to create repositories
>
> **Placeholders** — replace every `<PLACEHOLDER>` before running:
> - `<OWNER>` — GitHub username or org (e.g. `readsync`)
> - `<REPO>` — repository name (e.g. `readsync`)
> - `<GH_TOKEN>` — Personal Access Token with scopes:
>   `repo`, `workflow`, `write:packages`, `admin:repo_hook`, `security_events`
> - `<LOCAL_REPO_PATH>` — absolute path to the local ReadSync checkout

---

## AGENT INSTRUCTIONS

Run every numbered block **in order**. Do not skip steps.
All commands assume bash/sh (Linux, macOS, or WSL on Windows).

---

## 0. Environment Setup

```bash
export OWNER="<OWNER>"
export REPO="<REPO>"
export GH_TOKEN="<GH_TOKEN>"
export REPO_FULL="${OWNER}/${REPO}"
export LOCAL_REPO="<LOCAL_REPO_PATH>"

# Verify gh auth
gh auth status
# If not logged in: gh auth login --with-token <<< "${GH_TOKEN}"
```

---

## 1. Create the GitHub Repository and Push

```bash
cd "${LOCAL_REPO}"

# Create public repo with description, push local main
gh repo create "${REPO_FULL}" \
  --public \
  --description "Self-hosted Windows reading-progress sync: KOReader ↔ Moon+ ↔ Calibre ↔ Goodreads. No cloud, no API key." \
  --homepage "https://github.com/${REPO_FULL}" \
  --source . \
  --remote origin \
  --push

# If repo already exists, just add remote and push:
# git remote add origin "https://github.com/${REPO_FULL}.git" 2>/dev/null || true
# git push -u origin main

# Push any existing tags
git push origin --tags
```

---

## 2. Set Repository Metadata and About Section

```bash
curl -s -X PATCH \
  -H "Authorization: Bearer ${GH_TOKEN}" \
  -H "Accept: application/vnd.github+json" \
  -H "X-GitHub-Api-Version: 2022-11-28" \
  "https://api.github.com/repos/${REPO_FULL}" \
  -d '{
    "description": "Self-hosted Windows reading-progress sync: KOReader ↔ Moon+ ↔ Calibre ↔ Goodreads. No cloud, no API key.",
    "homepage": "https://github.com/'"${REPO_FULL}"'",
    "has_issues": true,
    "has_projects": false,
    "has_wiki": false,
    "has_discussions": true,
    "allow_squash_merge": true,
    "allow_merge_commit": false,
    "allow_rebase_merge": false,
    "delete_branch_on_merge": true,
    "squash_merge_commit_title": "PR_TITLE",
    "squash_merge_commit_message": "PR_BODY"
  }' | jq '{name:.name, description:.description, homepage:.homepage}'
```

---

## 3. Apply SEO-Optimised Topics (20 tags)

```bash
curl -s -X PUT \
  -H "Authorization: Bearer ${GH_TOKEN}" \
  -H "Accept: application/vnd.github+json" \
  -H "X-GitHub-Api-Version: 2022-11-28" \
  "https://api.github.com/repos/${REPO_FULL}/topics" \
  -d '{
    "names": [
      "go", "golang", "windows", "ebook", "reading-progress",
      "koreader", "calibre", "goodreads", "sqlite", "webdav",
      "sync", "windows-service", "kosync", "moon-plus", "self-hosted",
      "e-reader", "open-source", "reading-tracker", "book-sync", "local-first"
    ]
  }' | jq '.names'
```

---

## 4. Set Default Branch

```bash
curl -s -X PATCH \
  -H "Authorization: Bearer ${GH_TOKEN}" \
  -H "Accept: application/vnd.github+json" \
  -H "X-GitHub-Api-Version: 2022-11-28" \
  "https://api.github.com/repos/${REPO_FULL}" \
  -d '{"default_branch": "main"}' | jq '.default_branch'
```

---

## 5. Configure Branch Protection on `main`

```bash
# Require PRs (1 approval + CODEOWNERS), all 7 CI status checks,
# up-to-date branches, dismiss stale reviews, block force-push and delete.
curl -s -X PUT \
  -H "Authorization: Bearer ${GH_TOKEN}" \
  -H "Accept: application/vnd.github+json" \
  -H "X-GitHub-Api-Version: 2022-11-28" \
  "https://api.github.com/repos/${REPO_FULL}/branches/main/protection" \
  -d '{
    "required_status_checks": {
      "strict": true,
      "contexts": [
        "Phase 0 tools",
        "Phase 1 unit tests (no CGO)",
        "Phase 1 integration tests (CGO + SQLite)",
        "Build Windows binaries (cross-compile)",
        "Phase 7 unit tests (no CGO)",
        "Phase 7 security tests (no CGO)",
        "Phase 7 integration tests (CGO)"
      ]
    },
    "enforce_admins": false,
    "required_pull_request_reviews": {
      "dismissal_restrictions": {},
      "dismiss_stale_reviews": true,
      "require_code_owner_reviews": true,
      "required_approving_review_count": 1,
      "require_last_push_approval": false
    },
    "restrictions": null,
    "allow_force_pushes": false,
    "allow_deletions": false,
    "block_creations": false,
    "required_conversation_resolution": true
  }' | jq '{
    strict_status: .required_status_checks.strict,
    checks: .required_status_checks.contexts,
    dismiss_stale: .required_pull_request_reviews.dismiss_stale_reviews,
    codeowners: .required_pull_request_reviews.require_code_owner_reviews,
    force_push_blocked: (.allow_force_pushes | not)
  }'
```

---

## 6. Enable Dependabot Security Updates

```bash
# Enable vulnerability alerts
curl -s -X PUT \
  -H "Authorization: Bearer ${GH_TOKEN}" \
  -H "Accept: application/vnd.github+json" \
  -H "X-GitHub-Api-Version: 2022-11-28" \
  "https://api.github.com/repos/${REPO_FULL}/vulnerability-alerts"
echo "Vulnerability alerts enabled (HTTP $?)"

# Enable automated security fixes (Dependabot PRs for vulnerable deps)
curl -s -X PUT \
  -H "Authorization: Bearer ${GH_TOKEN}" \
  -H "Accept: application/vnd.github+json" \
  -H "X-GitHub-Api-Version: 2022-11-28" \
  "https://api.github.com/repos/${REPO_FULL}/automated-security-fixes"
echo "Automated security fixes enabled (HTTP $?)"

# Note: .github/dependabot.yml is already committed and covers:
#   gomod (main + 4 tool sub-modules), npm (tests/), github-actions
#   Weekly, monday 06:00 UTC; groups: golang-x, gin-stack, playwright, actions-core
```

---

## 7. Enable CodeQL and Advanced Security

```bash
# CodeQL is configured in .github/workflows/codeql.yml
# Runs on: push to main, PRs to main, weekly Wednesday 02:17 UTC
# Language: go | Queries: security-extended + security-and-quality
curl -s -X PATCH \
  -H "Authorization: Bearer ${GH_TOKEN}" \
  -H "Accept: application/vnd.github+json" \
  -H "X-GitHub-Api-Version: 2022-11-28" \
  "https://api.github.com/repos/${REPO_FULL}" \
  -d '{
    "security_and_analysis": {
      "advanced_security": {"status": "enabled"},
      "secret_scanning": {"status": "enabled"},
      "secret_scanning_push_protection": {"status": "enabled"}
    }
  }' | jq '.security_and_analysis'

# Note: Advanced Security is free for all public repositories.
```

---

## 8. Apply All Labels from `.github/labels.yml`

```bash
# 35 labels across: Type, Status, SemVer bump, Area, Platform
# --force updates existing labels without error

create_label() {
  local name="$1" color="$2" desc="$3"
  gh label create "${name}" \
    --color "${color}" \
    --description "${desc}" \
    --repo "${REPO_FULL}" \
    --force 2>&1
}

# ── Type ──────────────────────────────────────────────────────────────────────
create_label "bug"           "d73a4a" "Something isn't working"
create_label "feature"       "0075ca" "New feature request"
create_label "enhancement"   "a2eeef" "Improvement to an existing feature"
create_label "documentation" "0075ca" "Improvements or additions to documentation"
create_label "question"      "d876e3" "Further information is requested"
create_label "chore"         "e4e669" "Routine maintenance, cleanup, or housekeeping"
create_label "refactor"      "e4e669" "Code refactor without functional change"

# ── Status ────────────────────────────────────────────────────────────────────
create_label "help-wanted"      "008672" "Extra attention is needed; community contributions welcome"
create_label "good-first-issue" "7057ff" "Good for newcomers"
create_label "work-in-progress" "fbca04" "This PR is not ready for review yet"
create_label "needs-triage"     "e11d48" "Needs maintainer triage before work can begin"
create_label "needs-repro"      "f9c513" "Bug needs a reproduction case"
create_label "stale"            "b0b0b0" "No activity for 30-60 days; will be closed automatically"
create_label "closed-stale"     "b0b0b0" "Closed automatically due to inactivity"
create_label "pinned"           "0052cc" "Should not be marked stale"
create_label "duplicate"        "cfd3d7" "This issue or pull request already exists"
create_label "wontfix"          "ffffff" "This will not be worked on"
create_label "invalid"          "e4e669" "This doesn't seem right"

# ── SemVer bump ───────────────────────────────────────────────────────────────
create_label "semver-major"    "d93f0b" "Release Drafter: bumps MAJOR version (X.0.0)"
create_label "semver-minor"    "e99695" "Release Drafter: bumps MINOR version (x.Y.0)"
create_label "semver-patch"    "f9d0c4" "Release Drafter: bumps PATCH version (x.y.Z)"
create_label "breaking"        "d93f0b" "Introduces a breaking change (bumps MAJOR)"
create_label "skip-changelog"  "eeeeee" "Exclude this PR from the Release Drafter changelog"
create_label "release"         "0052cc" "Release management - exclude from changelog"

# ── Area ──────────────────────────────────────────────────────────────────────
create_label "area/ci"           "bfd4f2" "CI/CD workflows and automation"
create_label "ci"                "bfd4f2" "CI/CD workflows and automation (alias)"
create_label "build"             "bfd4f2" "Build system (Makefile, installer, scripts)"
create_label "security"          "ff0000" "Security vulnerability or hardening"
create_label "dependencies"      "0366d6" "Dependency updates (go.mod, npm)"
create_label "database"          "5319e7" "SQLite schema, migrations, WAL mode"
create_label "installer"         "c2e0c6" "Inno Setup installer or distribution"
create_label "adapter/koreader"  "b60205" "KOSync adapter"
create_label "adapter/moon"      "0e8a16" "Moon+ WebDAV adapter"
create_label "adapter/calibre"   "1d76db" "Calibre adapter"
create_label "adapter/goodreads" "e4b429" "Goodreads bridge adapter"

# ── Platform ──────────────────────────────────────────────────────────────────
create_label "platform/windows" "006b75" "Windows-specific issue or code"

# ── Configuration ─────────────────────────────────────────────────────────────
create_label "configuration" "fbca04" "Configuration question or issue"

# ── Fix ───────────────────────────────────────────────────────────────────────
create_label "fix" "d73a4a" "A fix/patch (alias for bug, contributes to semver-patch)"

echo "All 35 labels applied."
```

---

## 9. Create the First Release (v0.7.0)

```bash
cd "${LOCAL_REPO}"

# Tag the current HEAD of main as v0.7.0
# (This is the first public release, combining P7/P9/P10 work;
#  last internal milestone was 0.6.0)
git tag -a v0.7.0 \
  -m "Release v0.7.0 — QA hardening, closeout, documentation, and LLM-enablement sweep"
git push origin v0.7.0

# Create the GitHub Release with full release notes
gh release create v0.7.0 \
  --repo "${REPO_FULL}" \
  --title "v0.7.0 — QA Hardening, Closeout & Documentation" \
  --latest \
  --verify-tag \
  --notes "$(cat <<'RELEASE_NOTES'
## ReadSync v0.7.0

First public release of ReadSync — a self-hosted Windows background service that
synchronises reading progress across KOReader, Moon+ Reader Pro, Calibre, and
Goodreads. No cloud dependency. No Goodreads API key required.

### What's included

**🔒 QA & Security Hardening (Phase 7)**
- Unit tests for all 5 confidence bands in the identity resolver
- Unit tests for all 5 suspicious-jump detectors in the conflict engine
- Integration test for the spec §6 three-way conflict scenario
- Outbox state-machine tests: queued→succeeded, retrying, deadletter
- Log corpus sweep: verifies no secrets survive into JSONL output
- Security tests: CSRF required on all 11 write endpoints
- All 14 acceptance criteria verified (automated + documented)

**🧹 Closeout & Cleanup (Phase 9)**
- `.gitignore` covering build artefacts, SQLite DBs, TLS certs
- Pure-Go `calibre/opf` and `koreader/codec` subpackages (no CGO)
- CI Phase 7 unit job covers all no-CGO packages

**📖 Documentation & LLM-Enablement (Phase 10)**
- `AGENTS.md` — comprehensive AI coding-agent guide
- `CLAUDE.md` — Claude-specific instructions
- `.github/copilot-instructions.md` — GitHub Copilot context
- `llms.txt` — LLM-readable project index (llmstxt.org spec)
- `examples/` — 5 runnable usage examples

### Installing

Download `ReadSync-v0.7.0-windows-amd64.zip` from the assets below.
Extract and run the installer as Administrator.

### Building from source

```powershell
git clone https://github.com/readsync/readsync.git && cd readsync
go mod tidy && go mod download
make build        # requires TDM-GCC for CGO
make test-unit    # fast, no CGO
make test         # full suite
```

**Full Changelog**: https://github.com/readsync/readsync/blob/main/CHANGELOG.md
RELEASE_NOTES
)"

echo "Release v0.7.0 created."
```

---

## 10. Trigger Release Drafter for Next Draft

```bash
# Release Drafter (.github/workflows/release-drafter.yml) auto-updates
# a draft release on every push to main. Trigger it manually now to
# pre-create the v0.8.0 draft.
gh workflow run release-drafter.yml \
  --repo "${REPO_FULL}" \
  --ref main
echo "Release Drafter triggered."
```

---

## 11. Trigger Label Sync Workflow

```bash
# Sync labels from .github/labels.yml via the CI workflow
# (Step 8 already applied them via gh CLI; this ensures the automated sync is wired)
gh workflow run labels.yml \
  --repo "${REPO_FULL}" \
  --ref main
echo "Label sync workflow triggered."
```

---

## 12. Enable GitHub Discussions

```bash
# Enable Discussions for community Q&A (already set in Step 2, belt-and-suspenders)
curl -s -X PATCH \
  -H "Authorization: Bearer ${GH_TOKEN}" \
  -H "Accept: application/vnd.github+json" \
  -H "X-GitHub-Api-Version: 2022-11-28" \
  "https://api.github.com/repos/${REPO_FULL}" \
  -d '{"has_discussions": true}' | jq '.has_discussions'
```

---

## 13. Trigger Initial CI Run and Verify

```bash
# Trigger CI on main to confirm all jobs pass
gh workflow run ci.yml \
  --repo "${REPO_FULL}" \
  --ref main

# Watch for completion (Ctrl+C to exit early)
gh run list --repo "${REPO_FULL}" --limit 5
```

---

## 14. Full Verification Sweep

```bash
echo "=== Repository ==="
gh repo view "${REPO_FULL}" \
  --json name,description,homepage,defaultBranchRef,visibility,hasIssuesEnabled,hasDiscussionsEnabled \
  | jq .

echo ""
echo "=== Topics ==="
curl -s \
  -H "Authorization: Bearer ${GH_TOKEN}" \
  -H "Accept: application/vnd.github+json" \
  -H "X-GitHub-Api-Version: 2022-11-28" \
  "https://api.github.com/repos/${REPO_FULL}/topics" \
  | jq '(.names | length), .names'

echo ""
echo "=== Branch Protection ==="
curl -s \
  -H "Authorization: Bearer ${GH_TOKEN}" \
  -H "Accept: application/vnd.github+json" \
  -H "X-GitHub-Api-Version: 2022-11-28" \
  "https://api.github.com/repos/${REPO_FULL}/branches/main/protection" \
  | jq '{
      strict_checks: .required_status_checks.strict,
      num_checks: (.required_status_checks.contexts | length),
      dismiss_stale: .required_pull_request_reviews.dismiss_stale_reviews,
      codeowners: .required_pull_request_reviews.require_code_owner_reviews,
      force_push_blocked: (.allow_force_pushes | not),
      delete_blocked: (.allow_deletions | not)
    }'

echo ""
echo "=== Labels (should be 35+) ==="
gh label list --repo "${REPO_FULL}" | wc -l

echo ""
echo "=== Latest Release ==="
gh release view v0.7.0 \
  --repo "${REPO_FULL}" \
  --json tagName,name,publishedAt,isLatest | jq .

echo ""
echo "=== Security Settings ==="
curl -s \
  -H "Authorization: Bearer ${GH_TOKEN}" \
  -H "Accept: application/vnd.github+json" \
  -H "X-GitHub-Api-Version: 2022-11-28" \
  "https://api.github.com/repos/${REPO_FULL}" \
  | jq '.security_and_analysis'

echo ""
echo "Done. Visit: https://github.com/${REPO_FULL}"
```

---

## Reference: Repository Configuration Summary

| Setting | Value |
|---------|-------|
| Visibility | Public |
| Default branch | `main` |
| Description | Self-hosted Windows reading-progress sync: KOReader ↔ Moon+ ↔ Calibre ↔ Goodreads. No cloud, no API key. |
| Homepage | `https://github.com/<OWNER>/<REPO>` |
| Issues | ✅ Enabled |
| Wiki | ❌ Disabled |
| Projects | ❌ Disabled |
| Discussions | ✅ Enabled |
| Merge strategy | Squash only (merge commit + rebase disabled) |
| Delete branch on merge | ✅ Auto-delete |
| Branch protection: `main` | PR required (1 approval + CODEOWNERS), 7 required status checks, strict (up-to-date), dismiss stale reviews, force-push blocked, delete blocked |
| Required CI checks | 7 jobs from `ci.yml` (see Step 5) |
| Dependabot | ✅ Vulnerability alerts + auto security PRs + `dependabot.yml` |
| CodeQL | ✅ `codeql.yml` — Go, `security-extended`, weekly |
| Secret scanning | ✅ + push protection |
| Labels | 35 labels (type, status, semver, area, platform) |
| Topics | 20 SEO-optimised tags |
| First public release | `v0.7.0` (marked latest) |
| Release Drafter | ✅ Auto-drafts next release on every merge to `main` |
| Stale bot | ✅ Issues: 60 d stale / 14 d close; PRs: 30 d stale / 14 d close |

---

## Reference: CI Status Check Names (for Step 5)

These exact strings come from the `name:` fields in `.github/workflows/ci.yml`.
If CI jobs are renamed, update the `contexts` array in Step 5 to match.

```
Phase 0 tools
Phase 1 unit tests (no CGO)
Phase 1 integration tests (CGO + SQLite)
Build Windows binaries (cross-compile)
Phase 7 unit tests (no CGO)
Phase 7 security tests (no CGO)
Phase 7 integration tests (CGO)
```

---

## Reference: Files That Configure GitHub Automation

All files below are committed in the repo. No manual configuration needed
beyond running the steps above.

| File | Purpose |
|------|---------|
| `.github/workflows/ci.yml` | Main CI: unit, integration, cross-compile |
| `.github/workflows/release.yml` | Release: cross-compile + attach assets on tag push |
| `.github/workflows/release-drafter.yml` | Auto-update draft release on each push to `main` |
| `.github/workflows/codeql.yml` | CodeQL Go analysis (push, PR, weekly) |
| `.github/workflows/labels.yml` | Sync labels from `.github/labels.yml` |
| `.github/workflows/stale.yml` | Auto-mark/close stale issues and PRs |
| `.github/dependabot.yml` | Dependabot: gomod ×5 modules, npm, github-actions |
| `.github/release-drafter.yml` | Categories, semver resolver, changelog template |
| `.github/labels.yml` | Label definitions (35 labels) |
| `.github/CODEOWNERS` | Auto-assign `@<OWNER>/core` for review on all paths |
| `.github/PULL_REQUEST_TEMPLATE.md` | PR checklist |
| `.github/ISSUE_TEMPLATE/` | Bug report, feature request, question, config issue |
| `.github/labeler.yml` | Path-based auto-labeller |
