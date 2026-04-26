// internal/adapters/moon/parser_test.go
//
// Fixture-driven tests for the Moon+ Layer-3 parser.  Iterates over all
// committed fixtures (real device captures + synthetic) and verifies the
// expected percent / format classification.

package moon

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

const (
	captureDir   = "../../../fixtures/moonplus/captures"
	syntheticDir = "../../../fixtures/moonplus/synthetic"
)

// TestParse_SyntheticFixtures iterates fixtures/moonplus/synthetic/*.po and
// verifies the parser extracts the percent encoded in the filename label.
func TestParse_SyntheticFixtures(t *testing.T) {
	cases := map[string]float64{
		"010pct.po": 0.10,
		"025pct.po": 0.25,
		"050pct.po": 0.50,
		"075pct.po": 0.75,
		"100pct.po": 1.00,
	}
	for name, want := range cases {
		t.Run(name, func(t *testing.T) {
			path := filepath.Join(syntheticDir, name)
			body, err := os.ReadFile(path)
			if err != nil {
				t.Skipf("fixture not present: %v (run `make fixtures`)", err)
			}
			res, err := Parse(name, body, time.Now())
			if err != nil {
				t.Fatalf("Parse(%s): %v", name, err)
			}
			if res.Format != FormatV1Plain {
				t.Errorf("format: want %q got %q", FormatV1Plain, res.Format)
			}
			if diff := res.Percent - want; diff < -0.001 || diff > 0.001 {
				t.Errorf("percent: want %.3f got %.3f", want, res.Percent)
			}
			if res.BookKey == "" {
				t.Error("BookKey is empty")
			}
			if res.Position == "" {
				t.Error("Position is empty")
			}
			if res.ParserVersion != ParserVersion {
				t.Errorf("ParserVersion: want %q got %q", ParserVersion, res.ParserVersion)
			}
		})
	}
}

// TestParse_RealDeviceCaptures runs the parser over every real-device .po
// fixture under fixtures/moonplus/captures/ and verifies that:
//   - the format is classified as FormatV1Plain
//   - the percent is in [0.0, 1.0]
//   - the BookKey is non-empty
func TestParse_RealDeviceCaptures(t *testing.T) {
	entries, err := os.ReadDir(captureDir)
	if err != nil {
		t.Skipf("captures dir not present: %v", err)
	}
	count := 0
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(strings.ToLower(e.Name()), ".po") {
			continue
		}
		count++
		name := e.Name()
		t.Run(name, func(t *testing.T) {
			body, err := os.ReadFile(filepath.Join(captureDir, name))
			if err != nil {
				t.Fatalf("read: %v", err)
			}
			res, err := Parse(name, body, time.Now())
			if err != nil {
				t.Fatalf("Parse: %v\nBody: %q", err, string(body))
			}
			if res.Format != FormatV1Plain {
				t.Errorf("format: want %q got %q", FormatV1Plain, res.Format)
			}
			if res.Percent < 0 || res.Percent > 1.0 {
				t.Errorf("percent out of range: %.3f", res.Percent)
			}
			if res.BookKey == "" {
				t.Error("BookKey empty")
			}
		})
	}
	if count == 0 {
		t.Skip("no real-device captures found (see docs/research/moonplus.md §5)")
	}
}

// TestParse_UnknownFormat ensures unsupported payloads return ErrUnknownFormat.
func TestParse_UnknownFormat(t *testing.T) {
	cases := []struct {
		name string
		body []byte
	}{
		{"empty.po", []byte("")},
		{"binary.po", []byte{0x00, 0x01, 0x02, 0xff}},
		{"no_pct.po", []byte("123*45@0#0:notapct")},
		{"out_of_range.po", []byte("123*45@0#0:200%")},
		{"unknown_suffix.bin", []byte("abc")},
		{"no_colon.po", []byte("12345*0@0#0")},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			res, err := Parse(tc.name, tc.body, time.Now())
			if err == nil {
				t.Fatalf("want ErrUnknownFormat, got nil (res=%+v)", res)
			}
			if res.Format != FormatUnknown {
				t.Errorf("format: want %q got %q", FormatUnknown, res.Format)
			}
		})
	}
}

// TestParse_AnnotationsIgnored: .an files are silently classified unknown
// (see moonplus.md §6 quirk #4).  Adapter must not flag degraded health
// for them — handled at higher layer; here we just confirm the parser
// returns ErrUnknownFormat without panic.
func TestParse_AnnotationsIgnored(t *testing.T) {
	res, err := Parse("notes.an", []byte("anything"), time.Now())
	if err == nil {
		t.Fatalf("want unknown for .an, got %+v", res)
	}
	if res.Format != FormatUnknown {
		t.Errorf("format: want %q got %q", FormatUnknown, res.Format)
	}
}

// TestParse_ToAdapterEvent: verifies the resolver Evidence + locator type.
func TestParse_ToAdapterEvent(t *testing.T) {
	body := []byte("1703471974608*35@2#20432:73.2%")
	res, err := Parse("MyBook.epub.po", body, time.Now())
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	ev := res.ToAdapterEvent()
	if string(ev.Source) != "moon" {
		t.Errorf("source: %q", ev.Source)
	}
	if ev.BookEvidence.MoonKey == "" {
		t.Error("MoonKey empty in evidence")
	}
	if ev.PercentComplete == nil || *ev.PercentComplete < 0.731 || *ev.PercentComplete > 0.733 {
		t.Errorf("percent: %v", ev.PercentComplete)
	}
	if string(ev.LocatorType) != "moon_position" {
		t.Errorf("locator type: %q", ev.LocatorType)
	}
}
