# ReadSync Phase 0 - Makefile
# Targets for building, running, and testing Phase 0 tools.

GOOS    ?= windows
GOARCH  ?= amd64
GOFLAGS := CGO_ENABLED=0 GOOS=$(GOOS) GOARCH=$(GOARCH)

.PHONY: all fixtures koreader-sim moon-recorder winsvc-spike vet clean help

## help: Show this help message
help:
	@echo "ReadSync Phase 0 Makefile"
	@echo ""
	@echo "Targets:"
	@grep -E '^## ' Makefile | sed 's/## /  /'

## all: Build all tools
all: koreader-sim moon-recorder winsvc-spike

## koreader-sim: Build KOReader sync simulator
koreader-sim:
	cd tools/koreader-sim && go build -o koreader-sim.exe .

## moon-recorder: Build Moon+ fixture recorder
moon-recorder:
	cd tools/moon-fixture-recorder && go build -o moon-fixture-recorder.exe .

## winsvc-spike: Build Windows service spike (Windows only)
winsvc-spike:
	cd tools/winsvc-spike && $(GOFLAGS) go build -o readsync-spike.exe .

## fixtures: Generate synthetic fixture files
fixtures:
	cd tools/generate-fixtures && go run . --root ../..
	@echo "Synthetic fixtures generated in fixtures/moonplus/synthetic/"

## run-koreader-sim: Run KOReader simulator (port 7200)
run-koreader-sim:
	cd tools/koreader-sim && go run . --port 7200 --verbose

## run-moon-recorder: Run Moon+ fixture recorder (port 8765)
run-moon-recorder:
	cd tools/moon-fixture-recorder && \
	  go run . --port 8765 --verbose \
	  --capture-dir ../../fixtures/moonplus/captures

## replay-koreader: Run the KOReader curl replay script
replay-koreader:
	bash fixtures/koreader/curl-replay.sh http://localhost:7200

## vet: Run go vet on all tools
vet:
	go vet ./tools/koreader-sim/...
	go vet ./tools/moon-fixture-recorder/...
	go vet ./tools/winsvc-spike/...
	go vet ./tools/generate-fixtures/...

## clean: Remove built binaries
clean:
	rm -f tools/koreader-sim/koreader-sim.exe
	rm -f tools/moon-fixture-recorder/moon-fixture-recorder.exe
	rm -f tools/winsvc-spike/readsync-spike.exe
