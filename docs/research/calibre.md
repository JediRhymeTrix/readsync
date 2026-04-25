# Calibre Integration Research

> **Status:** Verified against Calibre 7.x CLI documentation and source.  
> **Last updated:** 2026-04-25  
> **Author:** ReadSync Phase 0 research

---

## 1. `calibredb` Command Reference

Calibre ships a command-line database tool `calibredb` that operates on a local
library (or a running Calibre content server). All commands below are confirmed
against Calibre 7.x.

### 1.1 Global flags

```
calibredb [command] [options] -- [arguments]

--library-path   Path to a Calibre library folder (contains metadata.db).
                 Defaults to the user's default library if omitted.
--username       Username for content-server authentication.
--password       Password for content-server authentication.
--with-library   URL of a Calibre content server (http://host:port#Library).
```

### 1.2 `calibredb list`

```bash
# List specific fields as JSON (parseable by ReadSync)
calibredb list \
  --fields id,title,authors,identifiers,#readsync_progress,#readsync_position \
  --for-machine

# With explicit library path
calibredb list --library-path "C:\Users\Vedant\Calibre Library"
```

**Output (`--for-machine`):** JSON array of objects, keys are field names.

### 1.3 `calibredb search`

Returns comma-separated book IDs matching a search expression.

```bash
calibredb search "#readsync_progress:>0"
calibredb search "last_modified:>2026-01-01"
calibredb search "identifier:goodreads:123456"
```

### 1.4 `calibredb show_metadata --as-opf`

Dump full metadata in OPF/XML. Custom columns appear as:
```xml
<meta name="calibre:user_metadata:#readsync_progress" content="{JSON_BLOB}"/>
```
Access value via `content["#value#"]` after JSON-parsing the `content` attr.

### 1.5 `calibredb custom_columns`

```bash
calibredb custom_columns
# Output (one per line):
# #readsync_progress (int)      - Reading progress 0-100
# #readsync_position (text)     - KOReader/Moon+ opaque position string
# #readsync_device   (text)     - Last syncing device name
# #readsync_synced   (datetime) - Timestamp of last successful sync
# #readsync_gr_shelf (text)     - Goodreads shelf name
```

### 1.6 `calibredb add_custom_column`

```bash
calibredb add_custom_column \
  --label readsync_progress \
  --name "Reading Progress" \
  --datatype int \
  --display '{"description": "Reading progress 0-100 (ReadSync)"}' \
  --is-multiple false

calibredb add_custom_column \
  --label readsync_position \
  --name "Sync Position" \
  --datatype text \
  --display '{"description": "Opaque position string from KOReader/Moon+"}' \
  --is-multiple false

calibredb add_custom_column \
  --label readsync_synced \
  --name "Sync Timestamp" \
  --datatype datetime \
  --display '{"description": "Timestamp of last ReadSync update (UTC)"}' \
  --is-multiple false

calibredb add_custom_column \
  --label readsync_gr_shelf \
  --name "Goodreads Shelf" \
  --datatype text \
  --display '{"description": "Goodreads shelf (currently-reading/to-read/read)"}' \
  --is-multiple false
```

**Flags:**
- `--label` → internal lookup name (becomes `#label` in search syntax)
- `--datatype` → `text`, `int`, `float`, `bool`, `rating`, `datetime`, `comments`, `series`, `enumeration`, `composite`

### 1.7 `calibredb set_custom`

```bash
# Set progress to 47 for book ID 42
calibredb set_custom readsync_progress 42 47

# Set opaque position string
calibredb set_custom readsync_position 42 "epubcfi(/6/4[chap01]!/4/2/4:120)"

# Set UTC sync timestamp
calibredb set_custom readsync_synced 42 "2026-04-25T14:30:00+00:00"

# Multiple books at once
calibredb set_custom readsync_progress 42,43,44 75
```

### 1.8 `calibredb set_metadata --field identifiers:...`

```bash
# Set Goodreads ID
calibredb set_metadata 42 --field "identifiers:goodreads:123456"

# Set multiple identifiers
calibredb set_metadata 42 \
  --field "identifiers:goodreads:123456" \
  --field "identifiers:isbn:9781234567890"

# Clear an identifier
calibredb set_metadata 42 --field "identifiers:goodreads:"
```

---

## 2. Required `#readsync_*` Custom Columns

Per master spec §7:

| Lookup Name          | Label               | Type       | Default | Description                                    |
|----------------------|---------------------|------------|---------|------------------------------------------------|
| `#readsync_progress` | `readsync_progress` | `int`      | `0`     | Reading progress percentage (0–100)            |
| `#readsync_position` | `readsync_position` | `text`     | `""`    | Opaque position string (CFI / KOReader locator)|
| `#readsync_device`   | `readsync_device`   | `text`     | `""`    | Name of last syncing device                    |
| `#readsync_synced`   | `readsync_synced`   | `datetime` | `null`  | UTC timestamp of last successful sync          |
| `#readsync_gr_shelf` | `readsync_gr_shelf` | `text`     | `""`    | Goodreads shelf name                           |

---

## 3. Calibre Content Server vs Local Library

| Mode           | Flag                    | Notes                                                                 |
|----------------|-------------------------|-----------------------------------------------------------------------|
| Local library  | `--library-path PATH`   | Direct SQLite; fastest; requires Calibre GUI closed or WAL mode       |
| Content server | `--with-library URL`    | HTTP API; Calibre GUI must be running; supports auth                  |

**Detection heuristic:**
1. Try `calibredb list --library-path PATH --fields id --limit 1`
2. Exit 0 → use local mode
3. Non-zero + content-server URL configured → use server mode
4. Else → warn user

---

## 4. Identifier Schemes

| Scheme      | Example value   | Source               |
|-------------|-----------------|----------------------|
| `goodreads` | `123456`        | Goodreads book ID    |
| `isbn`      | `9781234567890` | International std.   |
| `amazon`    | `B08XYZ1234`    | ASIN (read-only)     |
| `google`    | `abc123`        | Google Books ID      |
| `koreader`  | `sha256:abcdef` | KOReader doc hash    |

---

## 5. OPF Metadata Parsing Notes

- Identifiers: `<dc:identifier opf:scheme="SCHEME">VALUE</dc:identifier>`
- Custom columns: `<meta name="calibre:user_metadata:#COLUMN" content="JSON"/>`
- JSON blob keys: `#value#`, `datatype`, `column_metadata`, `#extra#`

### Go struct reference (Phase 2)

```go
type OPFMeta struct {
    Name    string `xml:"name,attr"`
    Content string `xml:"content,attr"`
}
// Access: json.Unmarshal(meta.Content) → map["#value#"]
```

---

## 6. Known Limitations & Edge Cases

1. **Calibre GUI lock**: WAL mode mitigates contention; ReadSync must retry
   with exponential backoff when DB is busy.

2. **`set_custom` atomicity**: Each call is a separate SQLite transaction.
   For multi-field updates, use `set_metadata` with a generated OPF file.

3. **Column label restrictions**: Lowercase alphanumeric + underscore, max 20
   chars. The `#` prefix is added by Calibre automatically.

4. **`calibredb` PATH on Windows**: Default install is
   `C:\Program Files\Calibre2\calibredb.exe`. ReadSync must locate via
   registry `HKLM\SOFTWARE\calibre\` or PATH lookup.

5. **Spaces in library path**: Always quote `--library-path` on Windows.

---

## References

- Calibre CLI: https://manual.calibre-ebook.com/generated/en/cli-index.html
- `calibredb` docs: https://manual.calibre-ebook.com/generated/en/calibredb.html
- Custom columns guide: https://manual.calibre-ebook.com/gui.html#adding-custom-columns
- OPF 2.0 spec: https://idpf.org/epub/20/spec/OPF_2.0.1_draft.htm
