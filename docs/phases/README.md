# Phase Delivery Manifests

This directory contains the deliverable manifests from each phase of ReadSync
development. Each manifest records what was built, what tests cover it, and
what was verified.

| Phase | Manifest | Description |
|-------|----------|-------------|
| 0 | [phase0-manifest.md](phase0-manifest.md) | Research, simulators, fixtures |
| 1 | [phase1-manifest.md](phase1-manifest.md) | Core service skeleton |
| 3 | [phase3-manifest.md](phase3-manifest.md) | KOReader adapter |
| 4 | [phase4-manifest.md](phase4-manifest.md) | Moon+ WebDAV adapter |
| 5 | [phase5-manifest.md](phase5-manifest.md) | Goodreads bridge |
| 6 | [phase6-manifest.md](phase6-manifest.md) | User-facing surface, installer, wizard |
| 7 | [phase7-manifest.md](phase7-manifest.md) | QA & hardening, all 14 ACs |

> Phase 2 had no separate manifest — deliverables are covered by phases 1 and 3.

---

## Phase 9 — Closeout & Cleanup

See `.phase9-manifest.md` in the repo root for the full Phase 9 record.

Key Phase 9 deliverables:
- `.gitignore` (new)
- `internal/adapters/calibre/opf/` — pure-Go OPF parser subpackage (no CGO)
- `internal/adapters/koreader/codec/` — pure-Go wire codec subpackage (no CGO)
- `docs/phases/` — this directory (canonical manifest location)
- Makefile `test-pipeline` target (fixes duplicate `test-e2e`)
- CI updated to cover all no-CGO packages in `phase7-unit` job

## Phase 10 — Documentation & LLM-Enablement Sweep

Key Phase 10 deliverables:
- `README.md` — polished with badges, full usage, config reference, architecture overview
- `AGENTS.md` — comprehensive AI coding agent guide (architecture, conventions, build/test, security)
- `CLAUDE.md` — Claude-specific instructions (automatically read by Claude)
- `.github/copilot-instructions.md` — GitHub Copilot context
- `.cursor/rules` — Cursor AI rules
- `llms.txt` — LLM-readable project index (per llmstxt.org spec)
- `examples/` — runnable usage examples (API query, KOReader, Moon+ WebDAV, resolver, conflicts)
- All public API reviewed for GoDoc completeness

