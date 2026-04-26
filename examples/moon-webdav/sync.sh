#!/usr/bin/env bash
# examples/moon-webdav/sync.sh
#
# Simulates a Moon+ Reader Pro WebDAV sync sequence against ReadSync.
#
# Protocol: WebDAV with .po position files.
# Docs: docs/research/moonplus.md
#
# Prerequisites:
#   - ReadSync (or moon-fixture-recorder) listening on port 8765.
#   - curl
#
# Usage: ./sync.sh [HOST:PORT]
#
# Moon+ always writes to: {WebDAV_root}/Apps/Books/.Moon+/Cache/{filename}.po
# ReadSync watches this full path tree automatically.

set -euo pipefail

HOST="${1:-127.0.0.1:8765}"
BASE="http://${HOST}/moon-webdav"

USERNAME="moonuser"
PASSWORD="moonpass"
AUTH="${USERNAME}:${PASSWORD}"

# Book filename (Moon+ uses filename, not hash, for identity)
BOOK="My Great Novel.epub"
PO_PATH="/Apps/Books/.Moon+/Cache/${BOOK}.po"

# Moon+ .po format: {file_id}*{position}@{chapter}#{scroll}:{percentage}%
# file_id = millisecond epoch of the book file's mtime
FILE_ID="1703471974608"

echo "==> Moon+ WebDAV sync example against ${BASE}"
echo ""

# 1. MKCOL — create the directory tree (Moon+ does this on first sync)
echo "--- Step 1: Create directory tree ---"
for DIR in \
    "/Apps" \
    "/Apps/Books" \
    "/Apps/Books/.Moon+" \
    "/Apps/Books/.Moon+/Cache"; do
  curl -s -w "MKCOL ${DIR}: HTTP %{http_code}\n" \
    -X MKCOL "${BASE}${DIR}" \
    -u "${AUTH}" || true
done
echo ""

# 2. PROPFIND — check if .po file exists (Moon+ always checks before writing)
echo "--- Step 2: PROPFIND (check existence) ---"
curl -s -w "\nHTTP %{http_code}\n" \
  -X PROPFIND "${BASE}${PO_PATH}" \
  -u "${AUTH}" \
  -H "Depth: 0"
echo ""

# 3. PUT — upload reading position at 25.8%
echo "--- Step 3: PUT position (25.8%) ---"
PO_CONTENT="${FILE_ID}*12@0#33241:25.8%"
curl -s -w "\nHTTP %{http_code}\n" \
  -X PUT "${BASE}${PO_PATH}" \
  -u "${AUTH}" \
  -H "Content-Type: application/octet-stream" \
  -d "${PO_CONTENT}"
echo ""

# 4. GET — read back the position
echo "--- Step 4: GET position ---"
curl -s -w "\nHTTP %{http_code}\n" \
  -X GET "${BASE}${PO_PATH}" \
  -u "${AUTH}"
echo ""

# 5. PUT — update position to 73.2% (Moon+ may PUT multiple times per session)
echo "--- Step 5: PUT updated position (73.2%) ---"
PO_CONTENT="${FILE_ID}*35@2#20432:73.2%"
curl -s -w "\nHTTP %{http_code}\n" \
  -X PUT "${BASE}${PO_PATH}" \
  -u "${AUTH}" \
  -H "Content-Type: application/octet-stream" \
  -d "${PO_CONTENT}"
echo ""

echo "==> Done. The .po format: {file_id}*{position}@{chapter}#{scroll}:{percentage}%"
echo "    ReadSync parses the ':{percentage}%' suffix as the reading progress."
echo "    Check http://127.0.0.1:7201/ to see the sync result."
