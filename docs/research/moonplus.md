# Moon+ Reader Pro WebDAV Sync Research

> **Status:** Verified against real Moon+ Pro v9 device captures (2026-04-25).  
> **Last updated:** 2026-04-25

---

## 1. Moon+ Pro Sync Overview



Moon+ Reader Pro supports **reading position sync** via WebDAV (self-hosted).

**Sync trigger behavior (verified from real captures):**
- **On pause/close:** Moon+ uploads current reading position via HTTP PUT
- **On open/resume:** Moon+ fetches reading position via HTTP GET/PROPFIND
- **Sync is per-book**, keyed by book filename (not hash)
- **May PUT multiple times** per session — always use the latest value

---

## 2. WebDAV Configuration

Settings → Sync → Sync reading positions → WebDAV

| Setting      | Example Value                    |
|--------------|----------------------------------|
| Server URL   | `http://192.168.1.100:8765/dav/` |
| Username     | `readsync`                       |
| Sync folder  | `/moonreader/` (ignored — see §3.5) |

> **Important:** Despite the "Sync folder" setting, Moon+ Pro v9 always writes
> to `{WebDAV_root}/Apps/Books/.Moon+/Cache/{epub_filename}.po` regardless of
> what sync folder is configured. See §3.5 for the confirmed path.

---

## 3. Position File Format (`.po`)

> **Updated from real device captures 2026-04-25.**  
> The `.po` file is **plain UTF-8 text**, not binary. Earlier community
> documentation of a binary `@MRP` format was incorrect for Moon+ Pro v9+.

### 3.1 Format

Single line, no newline, structured as:

```
{file_id}*{position}@{chapter}#{scroll}:{percentage}%
```

### 3.2 Fields

| Field       | Delimiter | Example       | Meaning                                      |
|-------------|-----------|---------------|----------------------------------------------|
| `file_id`   | before `*`| `1703471974608` | Book identity — millisecond epoch timestamp of the EPUB file's mtime |
| `position`  | `*` … `@` | `35`          | Opaque reading position index                |
| `chapter`   | `@` … `#` | `2`           | Chapter index (0-based)                      |
| `scroll`    | `#` … `:` | `20432`       | Scroll offset in pixels within the chapter   |
| `percentage`| `:` … `%` | `73.2`        | **Reading progress percentage (0–100)** — use this field directly |

### 3.3 Real capture examples

```
1703471974608*0@0#0:0.0%          ← book just opened, 0%
1703471974608*12@0#33241:25.8%    ← reading, 25.8%
1703471974608*35@2#20432:73.2%    ← reading, 73.2%
1703471974608*52@0#9486:100%      ← finished, 100%
```

### 3.4 Parsing (Go)

```go
// Parse a Moon+ .po file content into a progress percentage.
// Returns percentage as float64 (0.0–100.0) and the raw position string.
func ParsePO(content string) (percentage float64, position string, err error) {
    // Find the colon that precedes the percentage.
    ci := strings.LastIndex(content, ":")
    if ci < 0 {
        return 0, "", fmt.Errorf("invalid .po format: no colon")
    }
    position = content[:ci]
    pctStr := strings.TrimSuffix(content[ci+1:], "%")
    percentage, err = strconv.ParseFloat(strings.TrimSpace(pctStr), 64)
    return
}
```

### 3.5 WebDAV path

Moon+ does **not** use the configured sync folder. It always writes to:
```
{WebDAV_root}/Apps/Books/.Moon+/Cache/{epub_filename}.po
```
Example: `/dav/Apps/Books/.Moon+/Cache/-30- Press Quarterly.epub.po`

ReadSync must watch this full path tree, not just `/moonreader/`.

---

## 4. WebDAV Operations Moon+ Uses

| Operation  | Method   | Purpose                                   |
|------------|----------|-------------------------------------------|
| Check file | PROPFIND | Check if `.po` exists and get ETag        |
| Upload     | PUT      | Write updated position file               |
| Download   | GET      | Read position on resume                   |
| Create dir | MKCOL    | Create sync folder if not present         |

Moon+ does NOT use: DELETE, MOVE, COPY, LOCK.

---

## 5. Capture Script (Step-by-Step)

Prerequisites: Moon+ Pro on Android, PC running `moon-fixture-recorder` on same WiFi.

```bash
cd tools/moon-fixture-recorder
go run . --port 8765 --capture-dir ../../fixtures/moonplus/captures
```

**Sessions to capture:**
1. Navigate to ~10% → close book → recorder saves `<bookname>_<timestamp>.po`
2. Reopen → navigate to ~25% → close → another timestamped capture
3. Reopen → navigate to ~50% → close
4. Reopen → navigate to ~75% → close
5. Read to end → close

Each close/pause triggers a PUT. The recorder logs the filename and first 16 bytes
of every capture. The `captures/` directory will contain multiple timestamped `.po`
files per session.

**Diff analysis (PowerShell):**

```powershell
# List captures in order
Get-ChildItem fixtures\moonplus\captures\*.po | Sort-Object LastWriteTime

# Compare two captures as text (format is plain UTF-8)
Get-Content "fixtures\moonplus\captures\<bookname>_<ts1>.po"
Get-Content "fixtures\moonplus\captures\<bookname>_<ts2>.po"
# The :XX.X% field at the end changes between sessions — that is the progress value.
```

---

## 6. Known Quirks (Updated from Real Captures)

1. **Sync folder ignored**: Moon+ ignores the configured sync folder entirely. It
   always writes to `{root}/Apps/Books/.Moon+/Cache/{filename}.po`. ReadSync must
   watch the full `Apps/Books/.Moon+/Cache/` subtree.
2. **Plain text format**: The `.po` file is plain UTF-8 text, not binary. The
   `@MRP` binary format documented in older community posts does not apply to
   Moon+ Pro v9+.
3. **Percentage is explicit**: The `:73.2%` suffix is the direct progress value —
   no need to derive from numerator/denominator.
4. **Also syncs `.an` files**: Moon+ GETs a `.an` (annotations?) file alongside
   every `.po`. ReadSync can ignore `.an` files for progress tracking.
5. **GET before PUT**: Moon+ always GETs the existing `.po` before writing, then
   PUTs the update. A 404 GET response is fine — Moon+ proceeds to PUT anyway.
6. **Multiple PUTs per session**: Moon+ may PUT several times while reading (not
   just on close). Each PUT has the latest progress value — always take the last one.
7. **Filename special characters**: Book filenames can contain `-`, spaces, `+`,
   parentheses. The recorder and ReadSync must handle these without URL-encoding issues.

---

## References

- Moon+ Reader Pro: https://play.google.com/store/apps/details?id=com.flyersoft.moonreaderp
- WebDAV RFC 4918: https://tools.ietf.org/html/rfc4918
- Community `.po` analysis: https://www.mobileread.com/forums/showthread.php?t=304402
- golang.org/x/net/webdav (Phase 3): https://pkg.go.dev/golang.org/x/net/webdav
