# KOReader Sync Simulator

A lightweight Go HTTP server that implements the **KOSync protocol** used by
KOReader's `kosync.koplugin`. Use it for:

- Conformance testing the ReadSync KOSync endpoint
- Capturing real KOReader payloads for fixture generation
- Local development without needing `progress.koreader.rocks`

## Quick Start

```bash
# Run the simulator on port 7200
go run . --port 7200 --verbose

# In another terminal, replay a full session:
bash ../../fixtures/koreader/curl-replay.sh http://localhost:7200
```

## Flags

| Flag        | Default    | Description                          |
|-------------|------------|--------------------------------------|
| `--port`    | `7200`     | HTTP port to listen on               |
| `--state`   | (none)     | JSON file for state persistence      |
| `--verbose` | `false`    | Log every request/response           |

## Endpoints

| Method | Path                       | Description            |
|--------|----------------------------|------------------------|
| POST   | `/users/create`            | Register user          |
| GET    | `/users/auth`              | Authenticate           |
| PUT    | `/syncs/progress`          | Push progress          |
| GET    | `/syncs/progress/:docHash` | Pull progress          |

## Protocol Notes

- Auth: `x-auth-user` + `x-auth-key` headers (MD5 hex of password)
- Document key: 64-char SHA256 hex of book file bytes
- Percentage: float 0.0–1.0
- Stale push (server has equal/newer timestamp) → 412

## Pointing KOReader to This Simulator

In KOReader settings: Plugins → KOSync → Custom server URL:
```
http://<YOUR_PC_LAN_IP>:7200
```

Or in `settings.reader.lua`:
```lua
["kosync"] = {
    ["server"]   = "http://192.168.1.100:7200",
    ["username"] = "youruser",
    ["userkey"]  = "md5hexofpassword",
}
```

## Build (Windows binary)

```bash
GOOS=windows GOARCH=amd64 go build -o koreader-sim.exe .
```
