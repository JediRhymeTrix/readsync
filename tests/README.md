# ReadSync end-to-end tests

This directory holds the Playwright wizard suite and the installer
smoke-test script. None of these are required for `go test` -- the
unit suite covers the same functionality at the handler level.

## Contents

| Path                            | Purpose                                          |
|---------------------------------|--------------------------------------------------|
| `wizard.spec.js`                | Playwright wizard E2E (10 pages + CSRF assertion). |
| `playwright.config.js`          | Playwright runner configuration.                 |
| `package.json`                  | npm metadata; only dev-dep is `@playwright/test`. |
| `fakeserver/main.go`            | Boots the admin UI with an in-memory wizard.     |
| `../installer/smoke.ps1`        | Installer install -> probe -> uninstall script.  |

## Running the wizard E2E

```powershell
# 1. Build + start the fake server (no DB, no adapters):
go build -o fakeserver.exe ./tests/fakeserver
./fakeserver.exe -port 7201

# 2. In another shell, install Playwright once + run the spec:
cd tests
npm install
npx playwright install chromium
npx playwright test wizard.spec.js
```

A successful run prints:

```
8 passed (XXs)
```

Failure modes:

* If the fake server is not running, every test reports a connection
  refused error against `READSYNC_URL`.
* If you edit any of the wizard `.html` templates, re-run the test --
  Playwright's selectors hit page text and aria roles directly.

## Running the installer smoke test

This script requires:

* a Windows host;
* a built installer in `dist\ReadSync-<ver>-setup.exe`;
* an elevated PowerShell (UAC).

```powershell
.\installer\smoke.ps1 -Installer dist\ReadSync-0.6.0-setup.exe
```

The script prints `SMOKE TEST PASSED` on success and exits non-zero
on any step failure (so it can gate releases in CI).
