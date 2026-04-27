## ReadSync v0.1.0

First public release of ReadSync — a self-hosted Windows background service that synchronises reading progress across KOReader, Moon+ Reader Pro, Calibre, and Goodreads without a cloud dependency or Goodreads API key.

## Highlights

- Local Windows service with SQLite WAL-mode persistence.
- KOReader-compatible KOSync HTTP server on port 7200.
- Moon+ Reader Pro WebDAV `.po` sync support on port 8765.
- Calibre adapter using `calibredb` and `#readsync_progress` custom-column bridging.
- Goodreads progress bridge through the Calibre Goodreads Sync plugin; no Goodreads API calls.
- Browser admin UI, setup wizard, repair actions, diagnostics, tray app, and installer support.
- Conflict detection, identity resolution, retry outbox, CSRF protection, and secret redaction.
- CI, CodeQL, Dependabot, Release Drafter, and Windows binary release automation.

## What changed

### Dependencies
- deps(go)(deps): bump the golang-x group across 1 directory with 4 updates (#16) @dependabot
- deps(go)(deps): bump github.com/kardianos/service from 1.2.2 to 1.2.4 (#5) @dependabot
- deps(go)(deps): bump golang.org/x/crypto from 0.23.0 to 0.45.0 (#10) @dependabot
- deps(tools/winsvc-spike): bump github.com/kardianos/service from 1.2.2 to 1.2.4 in /tools/winsvc-spike (#1) @dependabot
- deps(go): bump gin to 1.12.0

### CI / maintenance
- chore(ci): migrate Go toolchain to 1.25 (#17) @JediRhymeTrix
- docs: align Go toolchain and release notes
- fix(ci): run CodeQL with Go 1.24 (#15) @JediRhymeTrix
- fix(ci): align workspace with Go 1.24 (#14) @JediRhymeTrix
- fix(ci): run workflows with Go 1.24 (#13) @JediRhymeTrix
- fix(ci): run workflows with Go 1.23 (#12) @JediRhymeTrix
- fix(ci): align go workspace version with dependency updates (#11) @JediRhymeTrix
- deps(actions)(deps): bump the third-party-actions group across 1 directory with 2 updates (#8) @dependabot
- deps(actions)(deps): bump github/codeql-action from 3 to 4 in the github-actions group across 1 directory (#7) @dependabot
- deps(actions)(deps): bump the actions-core group across 1 directory with 6 updates (#6) @dependabot

## Downloads

- `ReadSync-v0.1.0-windows-amd64.zip`
- `SHA256SUMS.txt`

Extract the Windows zip and run the installer/service setup as Administrator. Review checksums before installing.
