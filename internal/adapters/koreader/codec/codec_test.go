// internal/adapters/koreader/codec/codec_test.go
//
// Pure-Go unit tests — no CGO required.
// Covers LocatorType, ToEvent, CanonicalToPull.
// These tests supersede the translate_test.go tests that previously lived in
// package koreader (which requires CGO via internal/core → internal/db).

package codec

import (
	"testing"

	"github.com/readsync/readsync/internal/model"
)

// ─── LocatorType ──────────────────────────────────────────────────────────────

func TestLocatorType_EPUBCFI(t *testing.T) {
	got := LocatorType("epubcfi(/6/4[chap03]!/4/2/12:350)")
	if got != model.LocationKOReaderXPtr {
		t.Errorf("want LocationKOReaderXPtr, got %q", got)
	}
}

func TestLocatorType_PercentString(t *testing.T) {
	if got := LocatorType("0.47"); got != model.LocationPercent {
		t.Errorf("want LocationPercent for '0.47', got %q", got)
	}
}

func TestLocatorType_Empty(t *testing.T) {
	if got := LocatorType(""); got != model.LocationPercent {
		t.Errorf("want LocationPercent for '', got %q", got)
	}
}

func TestLocatorType_PlainNumber(t *testing.T) {
	if got := LocatorType("1.0"); got != model.LocationPercent {
		t.Errorf("want LocationPercent for '1.0', got %q", got)
	}
}

// ─── ToEvent ─────────────────────────────────────────────────────────────────

func TestToEvent_Reading(t *testing.T) {
	req := PushRequest{
		Document:   "abcdef1234567890abcdef1234567890abcdef1234567890abcdef1234567890",
		Progress:   "epubcfi(/6/4[chap03]!/4/2/12:350)",
		Percentage: 0.47,
		Device:     "KOReader",
		DeviceID:   "4b6f626f4c696272",
	}
	ev := ToEvent(req)

	if ev.BookEvidence.KOReaderDocHash != req.Document {
		t.Errorf("KOReaderDocHash: want %q, got %q", req.Document, ev.BookEvidence.KOReaderDocHash)
	}
	if ev.PercentComplete == nil || *ev.PercentComplete != 0.47 {
		t.Errorf("PercentComplete: want 0.47, got %v", ev.PercentComplete)
	}
	if ev.LocatorType != model.LocationKOReaderXPtr {
		t.Errorf("LocatorType: want koreader_xpointer, got %q", ev.LocatorType)
	}
	if ev.ReadStatus != model.StatusReading {
		t.Errorf("ReadStatus: want reading, got %q", ev.ReadStatus)
	}
	if ev.RawLocator == nil || *ev.RawLocator != req.Progress {
		t.Errorf("RawLocator: want %q, got %v", req.Progress, ev.RawLocator)
	}
}

func TestToEvent_Finished(t *testing.T) {
	req := PushRequest{
		Document:   "abcdef1234567890abcdef1234567890abcdef1234567890abcdef1234567890",
		Progress:   "1.0",
		Percentage: 1.0,
		Device:     "KOReader",
	}
	ev := ToEvent(req)
	if ev.ReadStatus != model.StatusFinished {
		t.Errorf("ReadStatus at 100%%: want finished, got %q", ev.ReadStatus)
	}
}

func TestToEvent_ZeroPercent(t *testing.T) {
	req := PushRequest{
		Document:   "abcdef1234567890abcdef1234567890abcdef1234567890abcdef1234567890",
		Progress:   "0",
		Percentage: 0.0,
		Device:     "KOReader",
	}
	ev := ToEvent(req)
	if ev.ReadStatus != model.StatusUnknown {
		t.Errorf("ReadStatus at 0%%: want unknown, got %q", ev.ReadStatus)
	}
}

// ─── CanonicalToPull ──────────────────────────────────────────────────────────

func TestCanonicalToPull_RoundTrip(t *testing.T) {
	pct := 0.47
	loc := "epubcfi(/6/4[chap03]!/4/2/12:350)"
	resp := CanonicalToPull("docHash123", &pct, &loc, "KOReader", "deviceID1", 1714000000)

	if resp.Document != "docHash123" {
		t.Errorf("Document: %q", resp.Document)
	}
	if resp.Percentage != 0.47 {
		t.Errorf("Percentage: %f", resp.Percentage)
	}
	if resp.Progress != loc {
		t.Errorf("Progress: %q", resp.Progress)
	}
	if resp.Device != "KOReader" {
		t.Errorf("Device: %q", resp.Device)
	}
	if resp.Timestamp != 1714000000 {
		t.Errorf("Timestamp: %d", resp.Timestamp)
	}
}

func TestCanonicalToPull_NilPercentAndLocator(t *testing.T) {
	resp := CanonicalToPull("docHash123", nil, nil, "KOReader", "dev1", 0)
	if resp.Percentage != 0.0 {
		t.Errorf("nil pct should produce 0.0, got %f", resp.Percentage)
	}
	if resp.Progress != "" {
		t.Errorf("nil locator should produce empty string, got %q", resp.Progress)
	}
}
