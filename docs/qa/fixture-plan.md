# ReadSync QA Fixture Plan

> **Status:** Draft — Last updated: 2026-04-25

---

## 1. Fixture Taxonomy

### 1.1 Book Identity Fixtures

**Location:** `fixtures/books/`

| File                   | Description                                            |
|------------------------|--------------------------------------------------------|
| `minimal.epub`         | Minimal valid EPUB 3 (10 pages, ~100KB)                |
| `minimal-v2.epub`      | Same content, different binary (different hash)        |
| `short-story.epub`     | 5-chapter EPUB for CFI position tests                  |
| `identifiers.json`     | Pre-computed SHA256 + Calibre IDs + ISBNs              |
| `hash-map.json`        | `{sha256_hex: calibre_book_id}` mapping                |

**`identifiers.json` schema:**
```json
[{
  "calibre_id": 1, "title": "Minimal Test Book",
  "isbn": "9780000000001", "goodreads_id": "12345",
  "sha256": "abcdef...64hex...", "file": "books/minimal.epub"
}]
```

---

### 1.2 KOReader Push/Pull Fixtures

**Location:** `fixtures/koreader/`

| File                      | Description                               |
|---------------------------|-------------------------------------------|
| `register-request.json`   | POST /users/create payload                |
| `register-response.json`  | Expected 201 response                     |
| `push-0pct.json`          | PUT progress at 0%                        |
| `push-47pct.json`         | PUT progress at 47% (mid-chapter CFI)     |
| `push-100pct.json`        | PUT progress at 100% (finished)           |
| `push-response.json`      | Expected 200 response to push             |
| `push-stale-412.json`     | Expected 412 (server has newer)           |
| `pull-found.json`         | GET /syncs/progress/:doc → found          |
| `pull-notfound.json`      | GET /syncs/progress/:doc → empty `{}`     |
| `curl-replay.sh`          | Replay full register+push+pull session    |

**`push-47pct.json`:**
```json
{
  "document":   "abcdef1234...64hex...",
  "progress":   "epubcfi(/6/4[chap03]!/4/2/12:350)",
  "percentage":  0.47,
  "device":     "KOReader",
  "device_id":  "4b6f626f4c696272"
}
```

---

### 1.3 Moon+ WebDAV Captures

**Location:** `fixtures/moonplus/`

> **Format:** Plain UTF-8 text, one line: `{file_id}*{position}@{chapter}#{scroll}:{percentage}%`  
> **Path:** Moon+ writes to `Apps/Books/.Moon+/Cache/{epub_filename}.po` (sync folder ignored)

| File                                          | Description                                  |
|-----------------------------------------------|----------------------------------------------|
| `captures/*_20260425T220909Z.po`              | Real capture — `-30- Press Quarterly` at 0.0%  |
| `captures/*_20260425T220927Z.po`              | Real capture — 25.8%                         |
| `captures/*_20260425T221004Z.po`              | Real capture — 73.2%                         |
| `captures/*_20260425T221013Z.po`              | Real capture — 100%                          |
| `synthetic/010pct.po`                         | Generated .po at 10.0% (CI use)              |
| `synthetic/025pct.po`                         | Generated .po at 25.0% (CI use)              |
| `synthetic/050pct.po`                         | Generated .po at 50.0% (CI use)              |
| `synthetic/075pct.po`                         | Generated .po at 75.0% (CI use)              |
| `synthetic/100pct.po`                         | Generated .po at 100% (CI use)               |
| `webdav-propfind-request.xml`                 | Moon+ PROPFIND request                       |
| `webdav-propfind-response.xml`                | Minimal PROPFIND response                    |
| `webdav-put-headers.json`                     | Headers Moon+ sends with PUT                 |

---

### 1.4 Calibre Library Fixtures

**Location:** `fixtures/calibre/`

| File                           | Description                                  |
|--------------------------------|----------------------------------------------|
| `library/metadata.db`          | Minimal Calibre SQLite DB (3 books)          |
| `opf/book1.opf`                | OPF with goodreads + ISBN identifiers        |
| `opf/book2.opf`                | OPF with koreader identifier                 |
| `opf/book3.opf`                | OPF with no identifiers (bare book)          |
| `calibredb-list-output.json`   | Expected `calibredb list --for-machine` out  |
| `calibredb-search-results.txt` | Expected search result IDs                   |
| `custom-columns.txt`           | Expected `calibredb custom_columns` output   |

---

### 1.5 Goodreads-Derived State Fixtures

**Location:** `fixtures/goodreads/`

| File                          | Description                                   |
|-------------------------------|-----------------------------------------------|
| `plugin-config-enabled.json`  | pluginsCustomization.json, plugin configured  |
| `plugin-config-disabled.json` | pluginsCustomization.json, wrong column       |
| `plugin-config-missing.json`  | pluginsCustomization.json, no GR key          |
| `shelf-currently-reading.json`| Calibre state when on GR currently-reading    |
| `shelf-read.json`             | Calibre state when book marked as read        |
| `metadata-after-sync.json`    | calibredb list output after plugin sync       |

---

## 2. Fixture Versioning Policy

- All fixtures committed to git (`.po` files are plain-text, very small)
- Each fixture has a corresponding `.meta.json`:
  ```json
  {
    "created": "2026-04-25T14:00:00Z",
    "source": "real-device | synthetic | calibre-cli | manual",
    "device_version": "KOReader 2024.04",
    "notes": "Captured on Kobo Libra 2, chapter 3 mid-paragraph"
  }
  ```
- `captures/` files (`source: "real-device"`) required for Phase 3 acceptance
- CI uses `synthetic/` equivalents

---

## 3. Generation Commands

```bash
# Generate synthetic Moon+ .po files
cd tools/moon-fixture-recorder
go run ./cmd/generate-synthetic --out ../../fixtures/moonplus/synthetic

# Validate JSON fixtures
find fixtures -name "*.json" | xargs -I{} python3 -c \
  "import json,sys; json.load(open('{}'))" && echo "All JSON valid"
```

---

## References

- Master spec §20: Acceptance criteria
- `docs/qa/acceptance-matrix.md`: test cases mapped to fixtures
