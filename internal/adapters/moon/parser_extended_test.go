// internal/adapters/moon/parser_extended_test.go
//
// Additional Moon+ parser tests covering boundary values, numeric
// normalization (spec §8), and round-trip codec verification.

package moon

import (
	"strings"
	"testing"
	"time"
)

// TestParse_BoundaryPercentages tests 0% and 100% exactly.
func TestParse_BoundaryPercentages(t *testing.T) {
	cases := []struct {
		name    string
		body    string
		wantPct float64
	}{
		{"0pct", "filekey*pos@ch#scroll:0%", 0.0},
		{"0.0pct", "filekey*pos@ch#scroll:0.0%", 0.0},
		{"100pct", "filekey*pos@ch#scroll:100%", 1.0},
		{"100.0pct", "filekey*pos@ch#scroll:100.0%", 1.0},
		{"50pct", "filekey*pos@ch#scroll:50%", 0.5},
		{"73.2pct", "filekey*pos@ch#scroll:73.2%", 0.732},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			res, err := Parse("book.po", []byte(tc.body), time.Now())
			if err != nil {
				t.Fatalf("Parse: %v", err)
			}
			if diff := res.Percent - tc.wantPct; diff > 0.001 || diff < -0.001 {
				t.Errorf("Percent: want %.4f, got %.4f", tc.wantPct, res.Percent)
			}
		})
	}
}

// TestParse_PercentOutOfRange verifies rejection of values > 100.
func TestParse_PercentOutOfRange(t *testing.T) {
	cases := []struct {
		name string
		body string
	}{
		{"200pct", "filekey*pos@ch#scroll:200%"},
		{"101pct", "filekey*pos@ch#scroll:101%"},
		{"999pct", "filekey*pos@ch#scroll:999%"},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			res, err := Parse("book.po", []byte(tc.body), time.Now())
			if err == nil {
				t.Fatalf("expected error for %q, got nil (res=%+v)", tc.body, res)
			}
			if res.Format != FormatUnknown {
				t.Errorf("format must be unknown, got %q", res.Format)
			}
		})
	}
}

// TestParse_EmptyAndMalformed tests rejection of malformed payloads.
func TestParse_EmptyAndMalformed(t *testing.T) {
	cases := []string{
		"",
		"noasterisk",
		"*",
		"filekey*pos@ch#scroll", // missing :pct%
		"filekey*pos@ch#scroll:",
		"filekey*pos@ch#scroll:notanumber%",
	}
	for _, body := range cases {
		_, err := Parse("book.po", []byte(body), time.Now())
		if err == nil {
			t.Errorf("expected error for body=%q, got nil", body)
		}
	}
}

// TestParse_FormatV1PlainFields verifies all fields are populated.
func TestParse_FormatV1PlainFields(t *testing.T) {
	body := "1703471974608*35@2#20432:73.2%"
	res, err := Parse("MyBook.epub.po", []byte(body), time.Now())
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if res.Format != FormatV1Plain {
		t.Errorf("Format: want %q, got %q", FormatV1Plain, res.Format)
	}
	if res.BookKey == "" {
		t.Error("BookKey must not be empty")
	}
	if !strings.Contains(res.BookKey, "MyBook.epub") {
		t.Errorf("BookKey should contain filename base: %q", res.BookKey)
	}
	if res.Position == "" {
		t.Error("Position must not be empty")
	}
	if res.ParserVersion != ParserVersion {
		t.Errorf("ParserVersion: want %q, got %q", ParserVersion, res.ParserVersion)
	}
	if res.Device != "moon+" {
		t.Errorf("Device: want 'moon+', got %q", res.Device)
	}
}

// TestParse_ToAdapterEvent_StatusMapping verifies read status inference.
func TestParse_ToAdapterEvent_StatusMapping(t *testing.T) {
	cases := []struct {
		body       string
		wantStatus string
	}{
		{"key*pos@ch#scroll:0%", "unknown"},
		{"key*pos@ch#scroll:50%", "reading"},
		{"key*pos@ch#scroll:100%", "finished"},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.wantStatus, func(t *testing.T) {
			res, err := Parse("book.po", []byte(tc.body), time.Now())
			if err != nil {
				t.Fatalf("Parse: %v", err)
			}
			ev := res.ToAdapterEvent()
			if string(ev.ReadStatus) != tc.wantStatus {
				t.Errorf("ReadStatus: want %q, got %q", tc.wantStatus, ev.ReadStatus)
			}
		})
	}
}

// TestParse_FilenameVariants tests case-insensitive .po suffix handling.
func TestParse_FilenameVariants(t *testing.T) {
	body := []byte("key*pos@ch#scroll:50%")
	variants := []string{"Book.po", "Book.PO", "Book.Po"}
	for _, name := range variants {
		res, err := Parse(name, body, time.Now())
		if err != nil {
			t.Errorf("Parse(%q): %v", name, err)
			continue
		}
		if res.Format != FormatV1Plain {
			t.Errorf("Parse(%q): format want %q got %q", name, FormatV1Plain, res.Format)
		}
	}
}

// TestParse_AnnotationsFile verifies .an is silently classified unknown.
func TestParse_AnnotationsFile(t *testing.T) {
	res, err := Parse("notes.an", []byte("some annotation data"), time.Now())
	if err == nil {
		t.Fatalf("expected error for .an file, got nil (res=%+v)", res)
	}
	if res.Format != FormatUnknown {
		t.Errorf("format must be FormatUnknown, got %q", res.Format)
	}
}
