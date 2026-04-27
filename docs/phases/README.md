# Phase Delivery Manifests

This directory mirrors `.cline/kanban/phases/`, which is the source of truth for phase records. Phase manifests record what shipped, what tests cover it, and what was verified.

| Phase | Manifest | Status | Description |
|-------|----------|--------|-------------|
| 0 | [phase0-manifest.md](phase0-manifest.md) | Complete | Research, simulators, fixtures, ADR |
| 1 | [phase1-manifest.md](phase1-manifest.md) | Complete | Core service skeleton, SQLite, resolver, pipeline, outbox, conflicts, logging, CLI/API |
| 2 | [phase2-manifest.md](phase2-manifest.md) | Complete | Calibre adapter |
| 3 | [phase3-manifest.md](phase3-manifest.md) | Complete | KOReader KOSync adapter |
| 4 | [phase4-manifest.md](phase4-manifest.md) | Complete | Moon+ WebDAV adapter |
| 5 | [phase5-manifest.md](phase5-manifest.md) | Complete | Goodreads bridge |
| 6 | [phase6-manifest.md](phase6-manifest.md) | Complete | Admin UI, setup wizard, repair actions, secrets, tray, installer |
| 7 | [phase7-manifest.md](phase7-manifest.md) | Complete | QA and hardening against acceptance criteria |
| 8 | [phase8-manifest.md](phase8-manifest.md) | Recovered | Integration stabilization and release preparation |
| 9 | [phase9-manifest.md](phase9-manifest.md) | Complete | Closeout, cleanup, CI/Makefile fixes, manifest migration |
| 10 | [phase10-manifest.md](phase10-manifest.md) | Recovered | Documentation and LLM enablement |
| 11 | [phase11-manifest.md](phase11-manifest.md) | Recovered | Final closeout and publishing |

## Notes

- Original source manifests existed for phases 0-7 and 9.
- No original phase 8, 10, or 11 manifest files were found in worktree, git history, or stash.
- Recovered phase 8, 10, and 11 manifests were reconstructed from commit history, `CHANGELOG.md`, and existing documentation.
- Hidden legacy filenames in `.cline/kanban/phases/.phaseN-manifest.md` are retained for traceability, but non-hidden `phaseN-manifest.md` files are canonical now.

