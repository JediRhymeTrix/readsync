#!/usr/bin/env bash
# fixtures/koreader/curl-replay.sh
# Replays a full KOReader register + push + pull session against the simulator.
# Usage: bash curl-replay.sh [BASE_URL]
# Default BASE_URL: http://localhost:7200

set -e

BASE="${1:-http://localhost:7200}"
USER="testuser"
PASS_MD5="5f4dcc3b5aa765d61d8327deb882cf99"  # md5("password")
DOC="abcdef1234567890abcdef1234567890abcdef1234567890abcdef1234567890"

echo "=== KOReader Simulator Replay ==="
echo "Server: $BASE"
echo ""

echo "--- 1. Register user ---"
curl -sf -X POST "$BASE/users/create" \
  -H "Content-Type: application/json" \
  -d "{\"username\":\"$USER\",\"password\":\"$PASS_MD5\"}" | python3 -m json.tool
echo ""

echo "--- 2. Authorize ---"
curl -sf "$BASE/users/auth" \
  -H "x-auth-user: $USER" \
  -H "x-auth-key: $PASS_MD5" | python3 -m json.tool
echo ""

echo "--- 3. Push 47% progress ---"
curl -sf -X PUT "$BASE/syncs/progress" \
  -H "Content-Type: application/json" \
  -H "x-auth-user: $USER" \
  -H "x-auth-key: $PASS_MD5" \
  -d "{
    \"document\":  \"$DOC\",
    \"progress\":  \"epubcfi(/6/4[chap03]!/4/2/12:350)\",
    \"percentage\": 0.47,
    \"device\":    \"KOReader\",
    \"device_id\": \"4b6f626f4c6962726132\"
  }" | python3 -m json.tool
echo ""

echo "--- 4. Pull progress ---"
curl -sf "$BASE/syncs/progress/$DOC" \
  -H "x-auth-user: $USER" \
  -H "x-auth-key: $PASS_MD5" | python3 -m json.tool
echo ""

echo "--- 5. Push stale (same timestamp should return 412) ---"
HTTP_CODE=$(curl -s -o /dev/null -w "%{http_code}" -X PUT "$BASE/syncs/progress" \
  -H "Content-Type: application/json" \
  -H "x-auth-user: $USER" \
  -H "x-auth-key: $PASS_MD5" \
  -d "{
    \"document\":  \"$DOC\",
    \"progress\":  \"epubcfi(/6/4[chap01]!/4/2/1:10)\",
    \"percentage\": 0.10,
    \"device\":    \"KOReader\",
    \"device_id\": \"4b6f626f4c6962726132\"
  }")
echo "HTTP status: $HTTP_CODE (expected 412 for stale update)"
echo ""

echo "=== Replay complete ==="
