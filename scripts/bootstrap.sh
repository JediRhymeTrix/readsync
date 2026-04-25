#!/bin/bash
# scripts/bootstrap.sh
#
# Bootstrap the ReadSync development environment.
# Run once after cloning the repo.
#
# Requirements:
#   - Go 1.22+
#   - GCC (for CGO/go-sqlite3):
#     Windows: TDM-GCC https://jmeubank.github.io/tdm-gcc/
#     Linux:   sudo apt-get install gcc
#     macOS:   xcode-select --install

set -euo pipefail

cd "$(dirname "$0")/.."

echo "==> Checking Go version..."
go version

echo "==> Running go mod tidy (downloads and verifies dependencies)..."
go mod tidy

echo "==> Running go vet on main module..."
go vet ./cmd/... ./internal/...

echo "==> Running unit tests (no CGO required)..."
go test -v -count=1 -timeout 30s \
  ./internal/resolver/... \
  ./internal/conflicts/... \
  ./internal/logging/...

echo "==> Running all tests (requires CGO/GCC for go-sqlite3)..."
go test -v -count=1 -timeout 60s ./...

echo ""
echo "✓ Bootstrap complete. To migrate the test DB:"
echo "  go run ./cmd/readsyncctl db migrate --db test-output/readsync-test.db"
