#!/usr/bin/env bash
# examples/koreader-push/push.sh
#
# Simulates a KOReader reading-progress sync sequence against ReadSync.
#
# Protocol: KOSync (compatible with the public koreader-sync-server).
# Docs: docs/research/koreader.md
#
# Prerequisites:
#   - ReadSync (or koreader-sim) listening on port 7200.
#   - curl
#
# Usage: ./push.sh [HOST:PORT]
#
# Example (real service):    ./push.sh 127.0.0.1:7200
# Example (LAN device):      ./push.sh 192.168.1.100:7200
# Example (simulator):       cd ../../tools/koreader-sim && go run . --port 7200 --verbose &
#                             ./push.sh 127.0.0.1:7200

set -euo pipefail

HOST="${1:-127.0.0.1:7200}"
BASE="http://${HOST}"

# Credentials (these must match what you set in the wizard, step 5)
USERNAME="testuser"
# KOSync password = hex(md5(plaintext)) — "password" → md5 below
PASSWORD_HASH="5f4dcc3b5aa765d61d8327deb882cf99"

# Book identity: SHA-256 of the book file bytes (ReadSync maps this to Calibre)
DOC_HASH="abcdef1234567890abcdef1234567890abcdef1234567890abcdef1234567890"

echo "==> KOReader sync example against ${BASE}"
echo ""

# 1. Register user (idempotent — 402 if already exists)
echo "--- Step 1: Register user '${USERNAME}' ---"
curl -s -w "\nHTTP %{http_code}\n" \
  -X POST "${BASE}/users/create" \
  -H "Content-Type: application/json" \
  -d "{\"username\":\"${USERNAME}\",\"password\":\"${PASSWORD_HASH}\"}"
echo ""

# 2. Auth check
echo "--- Step 2: Verify credentials ---"
curl -s -w "\nHTTP %{http_code}\n" \
  "${BASE}/users/auth" \
  -H "x-auth-user: ${USERNAME}" \
  -H "x-auth-key: ${PASSWORD_HASH}"
echo ""

# 3. Push progress (47% through an EPUB)
echo "--- Step 3: Push progress (47%) ---"
curl -s -w "\nHTTP %{http_code}\n" \
  -X PUT "${BASE}/syncs/progress" \
  -H "Content-Type: application/json" \
  -H "x-auth-user: ${USERNAME}" \
  -H "x-auth-key: ${PASSWORD_HASH}" \
  -d "{
    \"document\":  \"${DOC_HASH}\",
    \"progress\":  \"epubcfi(/6/4[ch01]!/4/2/1:47)\",
    \"percentage\": 0.47,
    \"device\":    \"KOReader\",
    \"device_id\": \"deadbeef01234567\"
  }"
echo ""

# 4. Pull progress back
echo "--- Step 4: Pull progress ---"
curl -s -w "\nHTTP %{http_code}\n" \
  "${BASE}/syncs/progress/${DOC_HASH}" \
  -H "x-auth-user: ${USERNAME}" \
  -H "x-auth-key: ${PASSWORD_HASH}"
echo ""

# 5. Push a higher progress value
echo "--- Step 5: Push progress (73%) ---"
curl -s -w "\nHTTP %{http_code}\n" \
  -X PUT "${BASE}/syncs/progress" \
  -H "Content-Type: application/json" \
  -H "x-auth-user: ${USERNAME}" \
  -H "x-auth-key: ${PASSWORD_HASH}" \
  -d "{
    \"document\":  \"${DOC_HASH}\",
    \"progress\":  \"0.73\",
    \"percentage\": 0.73,
    \"device\":    \"KOReader\",
    \"device_id\": \"deadbeef01234567\"
  }"
echo ""

echo "==> Done. Check the ReadSync admin UI at http://127.0.0.1:7201/ to see the sync result."
