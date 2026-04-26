// internal/adapters/calibre/opf/opf_test.go
//
// Pure-Go unit tests — no CGO required.
// Covers ParseOPFEvent, ExtractValueHash, QuoteEnumValues, RequiredColumns.
//
// XML fixtures use &quot; entities for double-quotes inside attribute values so
// that json.Unmarshal receives valid JSON after XML parsing (matches the
// pattern used in the parent calibre package's opf_test.go).

package opf

import (
	"strings"
	"testing"
)

// ─── ParseOPFEvent ────────────────────────────────────────────────────────────

// opfWithProgress is a fully-populated OPF fixture with all ReadSync columns.
// Single-quoted XML attributes are used for the JSON content values so that
// regular double-quotes inside the JSON are legal without any escaping.
var opfWithProgress = []byte(`<?xml version="1.0" encoding="utf-8"?>
<package version="2.0" xmlns="http://www.idpf.org/2007/opf">
  <metadata xmlns:dc="http://purl.org/dc/elements/1.1/" xmlns:opf="http://www.idpf.org/2007/opf">
    <dc:title>Test Book</dc:title>
    <dc:creator>Doe, John</dc:creator>
    <dc:identifier opf:scheme="ISBN">9781234567890</dc:identifier>
    <dc:identifier opf:scheme="goodreads">12345678</dc:identifier>
    <meta name="calibre:user_metadata:#readsync_progress" content='{"#value#": 47, "datatype": "int"}'/>
    <meta name="calibre:user_metadata:#readsync_progress_mode" content='{"#value#": "percent", "datatype": "enumeration"}'/>
    <meta name="calibre:user_metadata:#readsync_status" content='{"#value#": "reading", "datatype": "enumeration"}'/>
    <meta name="calibre:user_metadata:#readsync_last_synced" content='{"#value#": "2026-04-25T10:00:00Z", "datatype": "datetime"}'/>
  </metadata>
</package>`)

func TestParseOPFEvent_WithProgress(t *testing.T) {
	ev, err := ParseOPFEvent("42", opfWithProgress)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !ev.HasProgress {
		t.Fatal("expected HasProgress=true")
	}
	if ev.BookEvidence.CalibreID != "42" {
		t.Errorf("CalibreID: want 42, got %q", ev.BookEvidence.CalibreID)
	}
	if ev.BookEvidence.ISBN13 != "9781234567890" {
		t.Errorf("ISBN13: want 9781234567890, got %q", ev.BookEvidence.ISBN13)
	}
	if ev.BookEvidence.GoodreadsID != "12345678" {
		t.Errorf("GoodreadsID: want 12345678, got %q", ev.BookEvidence.GoodreadsID)
	}
	if ev.PercentComplete == nil {
		t.Fatal("PercentComplete must not be nil")
	}
	if diff := *ev.PercentComplete - 0.47; diff > 0.001 || diff < -0.001 {
		t.Errorf("PercentComplete: want ~0.47, got %f", *ev.PercentComplete)
	}
	if string(ev.ReadStatus) != "reading" {
		t.Errorf("ReadStatus: want reading, got %q", ev.ReadStatus)
	}
	if ev.DeviceTS == nil {
		t.Error("DeviceTS must not be nil")
	}
}

func TestParseOPFEvent_NoProgress(t *testing.T) {
	opf := []byte(`<?xml version="1.0"?><package version="2.0"><metadata><title>U</title></metadata></package>`)
	ev, err := ParseOPFEvent("99", opf)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ev.HasProgress {
		t.Error("expected HasProgress=false for book with no progress")
	}
}

func TestParseOPFEvent_PageMode(t *testing.T) {
	opf := []byte(`<?xml version="1.0"?><package version="2.0"><metadata>
  <meta name="calibre:user_metadata:#readsync_progress" content='{"#value#": 123}'/>
  <meta name="calibre:user_metadata:#readsync_progress_mode" content='{"#value#": "page"}'/>
  <meta name="calibre:user_metadata:#readsync_status" content='{"#value#": "reading"}'/>
</metadata></package>`)
	ev, err := ParseOPFEvent("7", opf)
	if err != nil || !ev.HasProgress {
		t.Fatalf("parse: err=%v hasProgress=%v", err, ev.HasProgress)
	}
	if ev.PageNumber == nil || *ev.PageNumber != 123 {
		t.Errorf("PageNumber: want 123, got %v", ev.PageNumber)
	}
}

func TestParseOPFEvent_InvalidXML(t *testing.T) {
	if _, err := ParseOPFEvent("1", []byte("not xml")); err == nil {
		t.Error("expected error for invalid XML")
	}
}

func TestParseOPFEvent_ISBN13vs10(t *testing.T) {
	opf13 := []byte(`<?xml version='1.0'?>
<package xmlns="http://www.idpf.org/2007/opf" version="2.0">
  <metadata xmlns:dc="http://purl.org/dc/elements/1.1/" xmlns:opf="http://www.idpf.org/2007/opf">
    <dc:identifier opf:scheme="isbn">9780735224292</dc:identifier>
    <meta name="calibre:user_metadata:#readsync_progress" content='{"#value#": 50}'/>
    <meta name="calibre:user_metadata:#readsync_status" content='{"#value#": "reading"}'/>
  </metadata></package>`)
	ev13, err := ParseOPFEvent("1", opf13)
	if err != nil || !ev13.HasProgress {
		t.Fatalf("isbn13 parse: err=%v hasProgress=%v", err, ev13.HasProgress)
	}
	if ev13.BookEvidence.ISBN13 != "9780735224292" {
		t.Errorf("ISBN13: want 9780735224292, got %q", ev13.BookEvidence.ISBN13)
	}
	if ev13.BookEvidence.ISBN10 != "" {
		t.Errorf("ISBN10 must be empty for 13-digit ISBN, got %q", ev13.BookEvidence.ISBN10)
	}

	opf10 := []byte(`<?xml version='1.0'?>
<package xmlns="http://www.idpf.org/2007/opf" version="2.0">
  <metadata xmlns:dc="http://purl.org/dc/elements/1.1/" xmlns:opf="http://www.idpf.org/2007/opf">
    <dc:identifier opf:scheme="isbn">0735224293</dc:identifier>
    <meta name="calibre:user_metadata:#readsync_progress" content='{"#value#": 30}'/>
    <meta name="calibre:user_metadata:#readsync_status" content='{"#value#": "reading"}'/>
  </metadata></package>`)
	ev10, err := ParseOPFEvent("2", opf10)
	if err != nil || !ev10.HasProgress {
		t.Fatalf("isbn10 parse: err=%v hasProgress=%v", err, ev10.HasProgress)
	}
	if ev10.BookEvidence.ISBN10 != "0735224293" {
		t.Errorf("ISBN10: want 0735224293, got %q", ev10.BookEvidence.ISBN10)
	}
	if ev10.BookEvidence.ISBN13 != "" {
		t.Errorf("ISBN13 must be empty for 10-digit ISBN, got %q", ev10.BookEvidence.ISBN13)
	}
}

func TestParseOPFEvent_ASIN(t *testing.T) {
	opf := []byte(`<?xml version='1.0'?>
<package xmlns="http://www.idpf.org/2007/opf" version="2.0">
  <metadata xmlns:dc="http://purl.org/dc/elements/1.1/" xmlns:opf="http://www.idpf.org/2007/opf">
    <dc:identifier opf:scheme="amazon">B08N5LNQCX</dc:identifier>
    <meta name="calibre:user_metadata:#readsync_progress" content='{"#value#": 60}'/>
    <meta name="calibre:user_metadata:#readsync_status" content='{"#value#": "reading"}'/>
  </metadata></package>`)
	ev, err := ParseOPFEvent("3", opf)
	if err != nil || !ev.HasProgress {
		t.Fatalf("parse: err=%v hasProgress=%v", err, ev.HasProgress)
	}
	if ev.BookEvidence.ASIN != "B08N5LNQCX" {
		t.Errorf("ASIN: want B08N5LNQCX, got %q", ev.BookEvidence.ASIN)
	}
}

func TestParseOPFEvent_RawLocator(t *testing.T) {
	opf := []byte(`<?xml version='1.0'?>
<package xmlns="http://www.idpf.org/2007/opf" version="2.0">
  <metadata xmlns:dc="http://purl.org/dc/elements/1.1/">
    <meta name="calibre:user_metadata:#readsync_progress" content='{"#value#": 73}'/>
    <meta name="calibre:user_metadata:#readsync_status" content='{"#value#": "reading"}'/>
    <meta name="calibre:user_metadata:#readsync_last_position" content='{"#value#": "epubcfi(/6/4[chap01]!/4/2:100)"}'/>
  </metadata></package>`)
	ev, err := ParseOPFEvent("5", opf)
	if err != nil || !ev.HasProgress {
		t.Fatalf("parse: err=%v hasProgress=%v", err, ev.HasProgress)
	}
	if ev.RawLocator == nil || !strings.Contains(*ev.RawLocator, "epubcfi") {
		t.Errorf("RawLocator: want epubcfi string, got %v", ev.RawLocator)
	}
}

// ─── ExtractValueHash ─────────────────────────────────────────────────────────

func TestExtractValueHash(t *testing.T) {
	cases := []struct{ name, input, want string }{
		{"int", `{"#value#": 47, "datatype": "int"}`, "47"},
		{"string", `{"#value#": "reading"}`, "reading"},
		{"empty_string", `{"#value#": ""}`, ""},
		{"null_value", `{"#value#": null}`, ""},
		{"no_value_hash", `{"datatype": "int"}`, ""},
		{"invalid_json", `not json`, ""},
		{"empty", "", ""},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			got := ExtractValueHash(tc.input)
			if got != tc.want {
				t.Errorf("ExtractValueHash(%q) = %q, want %q", tc.input, got, tc.want)
			}
		})
	}
}

// ─── QuoteEnumValues ──────────────────────────────────────────────────────────

func TestQuoteEnumValues(t *testing.T) {
	got := QuoteEnumValues("percent,page,raw")
	if got != `"percent","page","raw"` {
		t.Errorf("QuoteEnumValues: got %q", got)
	}
}

func TestQuoteEnumValues_Spaces(t *testing.T) {
	got := QuoteEnumValues(" not_started , reading , finished ")
	if !strings.Contains(got, `"not_started"`) {
		t.Errorf("trimming failed: %q", got)
	}
	if strings.Contains(got, " ") {
		t.Errorf("spaces remain: %q", got)
	}
}

// ─── RequiredColumns ──────────────────────────────────────────────────────────

func TestRequiredColumns_Count(t *testing.T) {
	if len(RequiredColumns) != 8 {
		t.Errorf("expected 8 required columns, got %d", len(RequiredColumns))
	}
}

func TestRequiredColumns_ContainsProgress(t *testing.T) {
	for _, col := range RequiredColumns {
		if col.Name == "#readsync_progress" {
			if col.DataType != "int" {
				t.Errorf("#readsync_progress DataType: want int, got %q", col.DataType)
			}
			return
		}
	}
	t.Error("#readsync_progress not in RequiredColumns")
}

func TestRequiredColumns_AllPresent(t *testing.T) {
	names := map[string]bool{}
	for _, c := range RequiredColumns {
		names[c.Name] = true
	}
	for _, n := range []string{
		"#readsync_progress", "#readsync_progress_mode", "#readsync_status",
		"#readsync_last_position", "#readsync_last_source", "#readsync_last_synced",
		"#readsync_conflict", "#readsync_confidence",
	} {
		if !names[n] {
			t.Errorf("required column %q is missing", n)
		}
	}
}
