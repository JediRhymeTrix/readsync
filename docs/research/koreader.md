# KOReader Progress Sync Server Protocol

> **Status:** Verified against KOReader kosync plugin source.  
> **Last updated:** 2026-04-25  
> **Source:** https://github.com/koreader/koreader/tree/master/plugins/kosync.koplugin

---

## 1. Protocol Overview

KOReader's **KOSync** plugin syncs reading positions to a REST server over
HTTP/HTTPS with JSON bodies and custom auth headers.

**Default public server:** `https://progress.koreader.rocks`  
**Reference server impl:** https://github.com/koreader/koreader-sync-server

---

## 2. Endpoints

### 2.1 Register — `POST /users/create`

```http
POST /users/create
Content-Type: application/json

{"username": "vedant", "password": "md5_hex_of_password"}
```

> Password = `hex(md5(plaintext))` — this is the protocol design.

**201 Created:**  `{"username": "vedant"}`  
**402 Taken:**    `{"message": "Username is already registered."}`

---

### 2.2 Authorize — `GET /users/auth`

```http
GET /users/auth
x-auth-user: vedant
x-auth-key: md5_hex_of_password
```

**200 OK:** `{"authorized": "OK"}`  
**401:**    `{"message": "Unauthorized"}`

---

### 2.3 Push Progress — `PUT /syncs/progress`

```http
PUT /syncs/progress
Content-Type: application/json
x-auth-user: vedant
x-auth-key: md5_hex_of_password

{
  "document":   "sha256_hex_of_document_bytes",
  "progress":   "epubcfi(/6/4[chap01]!/4/2/4:120)",
  "percentage":  0.47,
  "device":     "KOReader",
  "device_id":  "4c6f72656164657231"
}
```

| Field        | Type   | Description                                                  |
|--------------|--------|--------------------------------------------------------------|
| `document`   | string | SHA256 hex digest of book file bytes (identity key)          |
| `progress`   | string | Opaque position: `epubcfi(...)` or `"0.47"` for non-EPUB    |
| `percentage` | float  | Progress fraction 0.0–1.0                                    |
| `device`     | string | Human-readable device name                                   |
| `device_id`  | string | Hex-encoded unique device identifier                         |

**200 OK:** `{"document": "sha256_hex", "timestamp": 1745600000}`  
**412 Precondition Failed (server has newer):**
```json
{"message": "Document update is not newer.", "document": "sha256_hex", "timestamp": 1745599000}
```

---

### 2.4 Get Progress — `GET /syncs/progress/:document`

```http
GET /syncs/progress/sha256_hex
x-auth-user: vedant
x-auth-key: md5_hex_of_password
```

**200 OK (found):**
```json
{
  "document":   "sha256_hex",
  "progress":   "epubcfi(/6/4[chap01]!/4/2/4:120)",
  "percentage":  0.47,
  "device":     "KOReader",
  "device_id":  "4c6f72656164657231",
  "timestamp":  1745600000
}
```

**200 OK (not found):** `{}`

---

## 3. Authentication Details

- `x-auth-user`: username (plaintext)
- `x-auth-key`: `hex(md5(password))` — no session tokens; sent every request
- HTTPS strongly recommended on public networks

---

## 4. Document Identity

KOReader identifies books by **SHA256 of file bytes**, not title/ISBN.

```go
// Hash computation (Go reference)
h := sha256.New()
io.Copy(h, file)
docHash := hex.EncodeToString(h.Sum(nil))
```

ReadSync must maintain a `document_hash → calibre_book_id` mapping.

---

## 5. Crosspoint Custom Firmware Compatibility

Crosspoint uses the **identical KOSync protocol** and the same default server.

**KOReader settings.reader.lua:**
```lua
["kosync"] = {
    ["userkey"]  = "md5_hex",
    ["username"] = "vedant",
    ["server"]   = "http://192.168.1.100:7200",  -- point to ReadSync
}
```

ReadSync local URL: `http://127.0.0.1:7200` (LAN: `http://192.168.1.x:7200`)  
No TLS required for custom/LAN servers (KOReader accepts plain HTTP).

---

## 6. Simulator Quick Start

```bash
cd tools/koreader-sim
go run . --port 7200 --state state.json

# Register
curl -s -X POST http://localhost:7200/users/create \
  -H "Content-Type: application/json" \
  -d '{"username":"testuser","password":"5f4dcc3b5aa765d61d8327deb882cf99"}'

# Push progress
curl -s -X PUT http://localhost:7200/syncs/progress \
  -H "Content-Type: application/json" \
  -H "x-auth-user: testuser" \
  -H "x-auth-key: 5f4dcc3b5aa765d61d8327deb882cf99" \
  -d '{
    "document":  "abcdef1234567890abcdef1234567890abcdef1234567890abcdef1234567890",
    "progress":  "0.47",
    "percentage": 0.47,
    "device":    "KOReader",
    "device_id": "aabbccdd"
  }'

# Pull progress
curl -s \
  -H "x-auth-user: testuser" \
  -H "x-auth-key: 5f4dcc3b5aa765d61d8327deb882cf99" \
  http://localhost:7200/syncs/progress/abcdef1234567890abcdef1234567890abcdef1234567890abcdef1234567890
```

---

## 7. Fixtures

See `fixtures/koreader/`:

- `register-request.json` — register payload
- `push-request.json` — progress push payload
- `push-response.json` — expected server response
- `pull-response.json` — expected GET response
- `curl-replay.sh` — replay script for conformance testing

---

## References

- KOReader kosync plugin: https://github.com/koreader/koreader/tree/master/plugins/kosync.koplugin
- koreader-sync-server: https://github.com/koreader/koreader-sync-server
- KOReader wiki: https://github.com/koreader/koreader/wiki/Progress-sync
