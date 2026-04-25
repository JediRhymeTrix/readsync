#!/usr/bin/env bash
# tools/setup-calibre-columns.sh
#
# Creates all ReadSync custom columns in a Calibre library.
# Idempotent: silently skips columns that already exist.
#
# Usage:
#   bash setup-calibre-columns.sh [LIBRARY_PATH]
#
# Environment:
#   CALIBRE_LIBRARY_PATH  Override library path (or pass as $1)
#
# Requirements: calibredb in PATH (part of Calibre installation)
#   Windows: C:\Program Files\Calibre2\calibredb.exe
#   macOS:   /Applications/calibre.app/Contents/MacOS/calibredb
#   Linux:   /usr/bin/calibredb

set -euo pipefail

LIBRARY="${1:-${CALIBRE_LIBRARY_PATH:-}}"
if [ -n "$LIBRARY" ]; then
  LIB_ARG="--library-path"
  LIB_VAL="$LIBRARY"
else
  LIB_ARG=""
  LIB_VAL=""
fi

CALIBREDB="${CALIBREDB_PATH:-calibredb}"

create_column() {
  local label="$1" name="$2" dtype="$3" display="$4"
  local output
  if [ -n "$LIB_ARG" ]; then
    output=$("$CALIBREDB" add_custom_column \
      "$LIB_ARG" "$LIB_VAL" \
      --label "$label" --name "$name" \
      --datatype "$dtype" \
      --display "$display" \
      --is-multiple false 2>&1) || true
  else
    output=$("$CALIBREDB" add_custom_column \
      --label "$label" --name "$name" \
      --datatype "$dtype" \
      --display "$display" \
      --is-multiple false 2>&1) || true
  fi

  if echo "$output" | grep -qi "already exists"; then
    echo "  [skip] #$label already exists"
  elif echo "$output" | grep -qi "error"; then
    echo "  [WARN] #$label: $output"
  else
    echo "  [ok]   #$label created"
  fi
}

echo "=== ReadSync Calibre Column Setup ==="
[ -n "$LIBRARY" ] && echo "Library: $LIBRARY" || echo "Library: (default)"
echo ""
echo "Creating custom columns..."

create_column "readsync_progress" \
  "Reading Progress" \
  "int" \
  '{"description":"ReadSync: reading progress 0-100","number_format":null}'

create_column "readsync_position" \
  "Sync Position" \
  "text" \
  '{"description":"ReadSync: opaque position locator (CFI or Moon+ format)"}'

create_column "readsync_device" \
  "Sync Device" \
  "text" \
  '{"description":"ReadSync: name of last device to update progress"}'

create_column "readsync_synced" \
  "Sync Timestamp" \
  "datetime" \
  '{"description":"ReadSync: UTC timestamp of last successful sync"}'

create_column "readsync_gr_shelf" \
  "Goodreads Shelf" \
  "text" \
  '{"description":"ReadSync: Goodreads shelf (currently-reading/to-read/read)"}'

echo ""
echo "Verifying..."
if [ -n "$LIB_ARG" ]; then
  "$CALIBREDB" custom_columns "$LIB_ARG" "$LIB_VAL" | grep "readsync" || true
else
  "$CALIBREDB" custom_columns | grep "readsync" || true
fi

echo ""
echo "=== Done ==="
