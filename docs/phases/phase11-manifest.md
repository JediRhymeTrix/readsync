# Phase 11 Manifest - Final Closeout & Publishing

## Summary

Phase 11 prepared repository for GitHub publishing and release automation. It captured exact publishing steps, branch-protection expectations, labels, release flow, and final verification context.

## Source Evidence

- `CHANGELOG.md` Phase 11 section.
- `docs/github-push-prompt.md` publishing prompt.
- Commit history: `6e5a962 docs(publish): add GitHub publishing prompt and update llms.txt`, `7560189 chore: update codeowners for publish`.

## Deliverables

- `docs/github-push-prompt.md` self-contained GitHub publishing prompt covering:
  - repository creation and push
  - repository description and topics
  - branch protection and CODEOWNERS
  - Dependabot
  - CodeQL and secret scanning
  - labels
  - release creation
  - Release Drafter
- `llms.txt` updated with publishing prompt and phase index.
- CI workflow job names verified against branch-protection contexts.
- Final verification sweep documented: vet, formatting, spell-check, CI context review.
- Release workflow prepared to produce Windows binaries and attach checksums.

## Definition of Done

- [x] Publishing procedure documented in one reproducible prompt.
- [x] Required branch protection contexts documented.
- [x] Release automation documented.
- [x] Final closeout state reflected in docs and changelog.