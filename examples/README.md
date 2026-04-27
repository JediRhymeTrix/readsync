# ReadSync Examples

This directory contains runnable examples demonstrating how to use ReadSync's
API, integrate with it programmatically, and test against it.

## Contents

| Example | Language | Description |
|---------|----------|-------------|
| [`api-query/`](api-query/) | PowerShell | Query the admin API: status, adapters, conflicts |
| [`koreader-push/`](koreader-push/) | Shell (curl) | Simulate a KOReader progress push |
| [`moon-webdav/`](moon-webdav/) | Shell (curl) | Simulate a Moon+ WebDAV sync |
| [`resolve-book/`](resolve-book/) | Go | Use the identity resolver directly |
| [`conflict-scenario/`](conflict-scenario/) | Go | Trigger and inspect a conflict |

## Prerequisites

- ReadSync service running (`http://127.0.0.1:7201/`)
- For shell examples: `curl` (any modern version)
- For Go examples: Go 1.25+ and `go mod tidy` from the repo root

## Running the admin API examples

```powershell
# Start the service first, or use the fake server for testing:
go run ./tests/fakeserver -port 7201

# Then in another shell:
cd examples/api-query
./query.ps1
```

## Running the KOReader example

```bash
# The service must be listening on port 7200.
# For testing, you can use the KOReader simulator:
cd tools/koreader-sim
go run . --port 7200 --verbose &

# Then run the example:
cd ../../examples/koreader-push
bash push.sh
```
