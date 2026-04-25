# Goodreads Bridge Research

> **Status:** Verified via Calibre plugin source review and community docs.  
> **Last updated:** 2026-04-25  
> **IMPORTANT:** ReadSync does NOT scrape Goodreads or touch Amazon/Kindle cloud APIs.

---

## 1. Goodreads Sync Plugin Overview

The **Goodreads Sync** plugin (by Grant Drake) is a Calibre plugin that:

- Links Calibre books to Goodreads editions via the Goodreads API.
- Syncs shelf status (`currently-reading`, `to-read`, `read`) ↔ a Calibre custom column.
- Syncs reading progress percentage ↔ a Calibre custom column.
- All writes go back to Calibre metadata; ReadSync only touches that metadata.

**License:** GPL-3.0 — ReadSync must NOT vendor or link any plugin code.

---

## 2. Plugin Installation Detection

### 2.1 Plugin folder (Windows)

```
%APPDATA%\calibre\plugins\
```

Plugin appears as one of:
```
%APPDATA%\calibre\plugins\Goodreads Sync.zip
%APPDATA%\calibre\plugins\goodreads_sync.zip
```

### 2.2 Detection algorithm

```
1. Enumerate %APPDATA%\calibre\plugins\*.zip
2. Check ZIP for "plugin-import-name-goodreads_sync.txt"
   OR filename contains "goodreads" (case-insensitive)
3. Found → plugin installed
4. Read pluginsCustomization.json for "progress_column" key
```

### 2.3 Configuration JSON path

```
%APPDATA%\calibre\plugins\pluginsCustomization.json
```

```json
{
  "Goodreads Sync": {
    "progress_column": "#readsync_progress",
    "reading_list_column": "#readsync_gr_shelf",
    "rating_column": "rating"
  }
}
```

---

## 3. Reading Progress Setting

Point the plugin's **"Reading Progress column"** to `#readsync_progress`.

### 3.1 Manual configuration flow

1. Calibre → Preferences → Plugins → "Goodreads Sync" → Customize
2. "Custom Columns" tab → set **"Reading Progress column"** to `#readsync_progress`
3. Set **"Reading List column"** to `#readsync_gr_shelf`
4. Click OK and restart Calibre

### 3.2 Guided flow (ReadSync setup wizard)

```
Step 1: Verify plugin is installed
Step 2: Detect current "Reading Progress column" setting
Step 3: If wrong column → display instructions to user
Step 4: Poll pluginsCustomization.json until setting is updated
Step 5: Confirm and proceed
```

---

## 4. Sync Flow

```
KOReader / Moon+ Reader
      │ push progress (47%)
      ▼
ReadSync service
      │ calibredb set_custom readsync_progress BOOK_ID 47
      ▼
Calibre metadata.db (#readsync_progress = 47)
      │ [User triggers Goodreads Sync in Calibre GUI]
      ▼
Goodreads Sync Plugin → Goodreads API → currently-reading 47%
```

**Reverse flow:**
```
Goodreads progress updated to 60%
      │ [User triggers Goodreads Sync]
      ▼
Goodreads Sync Plugin writes #readsync_progress = 60 to Calibre
      │ ReadSync detects change via filesystem watcher
      ▼
ReadSync pushes 60% to KOReader / Moon+
```

---

## 5. Shelf Mapping

| Goodreads Shelf     | `#readsync_gr_shelf` | Notes                        |
|---------------------|----------------------|------------------------------|
| `currently-reading` | `currently-reading`  | Progress tracking active     |
| `to-read`           | `to-read`            | No progress expected         |
| `read`              | `read`               | Progress at 100%             |
| (none)              | `""`                 | Book not linked to Goodreads |

ReadSync auto-updates `#readsync_gr_shelf` to `read` when progress = 100.

---

## 6. Manual Sync Trigger Note

Calibre has no CLI to run a specific plugin. ReadSync therefore:

1. Updates `#readsync_progress` via `calibredb set_custom` immediately.
2. Leaves Goodreads sync to the user's Calibre workflow.
3. Shows a Windows toast: "Progress updated for 'Title'. Open Calibre to sync to Goodreads."

---

## 7. GPL-3.0 Compliance

- ReadSync does NOT copy, link, import, or vendor any plugin code.
- ReadSync reads plugin config from JSON files (data, not code — OK).
- ReadSync writes to Calibre DB via `calibredb` CLI (plugin reads that independently).

---

## References

- Plugin thread: https://www.mobileread.com/forums/showthread.php?t=123281
- Plugin source (GPL-3.0): https://github.com/kiwidude68/calibre_plugins
- Goodreads API status: https://help.goodreads.com/s/article/Does-Goodreads-support-the-use-of-APIs
