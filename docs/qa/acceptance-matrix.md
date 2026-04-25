# ReadSync Acceptance Test Matrix

> Mapped to master spec §20. Last updated: 2026-04-25  
> ✅=P0 | 🔧=Phase 2+ | 📱=Real device | 🔑=Admin

---

## AC-01: KOReader Progress Push

| ID       | Description                   | Fixture                           | Status |
|----------|-------------------------------|-----------------------------------|--------|
| KO-01-01 | Register → 201                | `koreader/register-request.json`  | ✅ P0  |
| KO-01-02 | Auth valid → 200              | `koreader/auth-request-headers.json` | ✅ P0 |
| KO-01-03 | Auth bad → 401                | —                                 | ✅ P0  |
| KO-01-04 | Push 0%                       | `koreader/push-0pct.json`         | ✅ P0  |
| KO-01-05 | Push 47% CFI                  | `koreader/push-47pct.json`        | ✅ P0  |
| KO-01-06 | Push 100%                     | `koreader/push-100pct.json`       | ✅ P0  |
| KO-01-07 | Pull → exact payload back     | `koreader/pull-found.json`        | ✅ P0  |
| KO-01-08 | Pull unknown → `{}`           | `koreader/pull-notfound.json`     | ✅ P0  |
| KO-01-09 | Stale push → 412              | `koreader/push-stale-412.json`    | ✅ P0  |
| KO-01-10 | Real KOReader (LAN)           | —                                 | 📱 P3  |

## AC-02: KOReader → Calibre

| ID       | Description                   | Fixture                           | Status |
|----------|-------------------------------|-----------------------------------|--------|
| KO-02-01 | 47% → calibredb = 47          | `calibre/calibredb-list-output.json` | 🔧 P2 |
| KO-02-02 | Hash → Calibre ID             | `books/hash-map.json`             | 🔧 P2  |
| KO-02-03 | Unknown hash → warn           | —                                 | 🔧 P2  |
| KO-02-04 | `#readsync_position` updated  | `calibre/opf/book1.opf`           | 🔧 P2  |
| KO-02-05 | `#readsync_device` updated    | —                                 | 🔧 P2  |
| KO-02-06 | `#readsync_synced` updated    | —                                 | 🔧 P2  |

## AC-03: Moon+ WebDAV Upload

| ID       | Description                   | Fixture                           | Status |
|----------|-------------------------------|-----------------------------------|--------|
| MN-03-01 | MKCOL creates folder          | —                                 | ✅ P0  |
| MN-03-02 | PUT uploads .po               | `moonplus/webdav-put-headers.json`| ✅ P0  |
| MN-03-03 | PROPFIND returns ETag         | `moonplus/webdav-propfind-*.xml`  | ✅ P0  |
| MN-03-04 | GET retrieves last PUT        | —                                 | ✅ P0  |
| MN-03-05 | Spaces in filename            | —                                 | ✅ P0  |
| MN-03-06 | Real Moon+ upload captured    | `moonplus/captures/session*.po`   | 📱 P0  |
| MN-03-07 | 5-session diff → bytes found  | `moonplus/captures/*`             | 📱 P0  |

## AC-04: Moon+ → Calibre

| ID       | Description                   | Fixture                           | Status |
|----------|-------------------------------|-----------------------------------|--------|
| MN-04-01 | Synthetic .po → %             | `moonplus/synthetic/050pct.po`    | 🔧 P2  |
| MN-04-02 | Real .po → %                  | `moonplus/captures/session3*.po`  | 📱🔧 P3 |
| MN-04-03 | % → calibredb set             | —                                 | 🔧 P2  |
| MN-04-04 | Filename → book identity      | `books/identifiers.json`          | 🔧 P2  |

## AC-05: Calibre Custom Columns

| ID       | Description                   | Fixture                      | Status |
|----------|-------------------------------|------------------------------|--------|
| CB-05-01 | Creates 5 columns             | `calibre/custom-columns.txt` | 🔧 P2  |
| CB-05-02 | Idempotent 2nd run            | —                            | 🔧 P2  |
| CB-05-03 | Verify via CLI output         | `calibre/custom-columns.txt` | 🔧 P2  |

## AC-06: Windows Service Lifecycle

| ID       | Description                   | Status     |
|----------|-------------------------------|------------|
| SV-06-01 | `readsyncctl install`         | ✅🔑 P0   |
| SV-06-02 | `readsyncctl start`           | ✅🔑 P0   |
| SV-06-03 | `readsyncctl status` Running  | ✅ P0      |
| SV-06-04 | `readsyncctl stop`            | ✅🔑 P0   |
| SV-06-05 | `readsyncctl uninstall`       | ✅🔑 P0   |
| SV-06-06 | Auto-restart on crash         | 🔧🔑 P4   |

## AC-07: Goodreads Plugin Detection

| ID       | Description                   | Fixture                                 | Status |
|----------|-------------------------------|-----------------------------------------|--------|
| GR-07-01 | Detect plugin ZIP             | —                                       | 🔧 P2  |
| GR-07-02 | Read progress_column          | `goodreads/plugin-config-enabled.json`  | 🔧 P2  |
| GR-07-03 | Wrong column → alert          | `goodreads/plugin-config-disabled.json` | 🔧 P2  |
| GR-07-04 | Missing config → no-op        | `goodreads/plugin-config-missing.json`  | 🔧 P2  |

## AC-08: Progress Normalization

| ID       | Description                   | Status  |
|----------|-------------------------------|---------|
| NR-08-01 | KOReader float → int          | 🔧 P2   |
| NR-08-02 | Moon+ num/denom → int         | 🔧 P2   |
| NR-08-03 | Calibre int → KOReader float  | 🔧 P2   |
| NR-08-04 | Boundaries and rounding       | 🔧 P2   |

---

## Coverage by Phase

| Phase | ACs           | Tests |
|-------|---------------|-------|
| P0    | 01, 03, 06    | 20    |
| P2    | 02,04,05,07,08 | 22   |
| P3    | 01-10, 04-02  | 2     |
| P4    | 06-06         | 1     |
