// internal/adapters/calibre/opf_test.go
//
// Unit tests for OPF parsing, identifier extraction, and command building.

package calibre

import (
	"testing"
)

func TestParseOPFEvent_WithProgress(t *testing.T) {
	opf := []byte(`<?xml version="1.0" encoding="utf-8"?>
<package version="2.0" xmlns="http://www.idpf.org/2007/opf">
  <metadata xmlns:dc="http://purl.org/dc/elements/1.1/" xmlns:opf="http://www.idpf.org/2007/opf">
    <dc:title>Test Book</dc:title>
    <dc:creator>Doe, John</dc:creator>
    <dc:identifier scheme="ISBN">9781234567890</dc:identifier>
    <dc:identifier scheme="goodreads">12345678</dc:identifier>
    <meta name="calibre:user_metadata:#readsync_progress" content="{&quot;#value#&quot;: 47, &quot;datatype&quot;: &quot;int&quot;}"/>
    <meta name="calibre:user_metadata:#readsync_progress_mode" content="{&quot;#value#&quot;: &quot;percent&quot;, &quot;datatype&quot;: &quot;enumeration&quot;}"/>
    <meta name="calibre:user_metadata:#readsync_status" content="{&quot;#value#&quot;: &quot;reading&quot;, &quot;datatype&quot;: &quot;enumeration&quot;}"/>
    <meta name="calibre:user_metadata:#readsync_last_position" content="{&quot;#value#&quot;: &quot;chapter-5&quot;, &quot;datatype&quot;: &quot;text&quot;}"/>
    <meta name="calibre:user_metadata:#readsync_last_synced" content="{&quot;#value#&quot;: &quot;2026-04-25T10:00:00Z&quot;, &quot;datatype&quot;: &quot;datetime&quot;}"/>
  </metadata>
</package>`)

	ev, ok, err := parseOPFEvent("42", opf)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !ok {
		t.Fatal("expected ok=true, got false")
	}
	if ev.BookEvidence.CalibreID != "42" {
		t.Errorf("calibre_id: want 42, got %s", ev.BookEvidence.CalibreID)
	}
	if ev.BookEvidence.Title != "Test Book" {
		t.Errorf("title: want 'Test Book', got %q", ev.BookEvidence.Title)
	}
	if ev.BookEvidence.ISBN13 != "9781234567890" {
		t.Errorf("isbn13: want 9781234567890, got %q", ev.BookEvidence.ISBN13)
	}
	if ev.BookEvidence.GoodreadsID != "12345678" {
		t.Errorf("goodreads_id: want 12345678, got %q", ev.BookEvidence.GoodreadsID)
	}
	if ev.PercentComplete == nil {
		t.Fatal("percent_complete should not be nil")
	}
	wantPct := 0.47
	if diff := *ev.PercentComplete - wantPct; diff > 0.001 || diff < -0.001 {
		t.Errorf("percent_complete: want ~0.47, got %f", *ev.PercentComplete)
	}
	if string(ev.ReadStatus) != "reading" {
		t.Errorf("read_status: want reading, got %q", ev.ReadStatus)
	}
	if ev.DeviceTS == nil {
		t.Error("device_ts should not be nil")
	}
}

func TestParseOPFEvent_NoProgress(t *testing.T) {
	opf := []byte(`<?xml version="1.0" encoding="utf-8"?>
<package version="2.0">
  <metadata>
    <title>Unread Book</title>
  </metadata>
</package>`)

	_, ok, err := parseOPFEvent("99", opf)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ok {
		t.Error("expected ok=false for book with no progress data")
	}
}

func TestParseOPFEvent_PageMode(t *testing.T) {
	opf := []byte(`<?xml version="1.0" encoding="utf-8"?>
<package version="2.0">
  <metadata>
    <title>Paged Book</title>
    <meta name="calibre:user_metadata:#readsync_progress" content="{&quot;#value#&quot;: 123}"/>
    <meta name="calibre:user_metadata:#readsync_progress_mode" content="{&quot;#value#&quot;: &quot;page&quot;}"/>
    <meta name="calibre:user_metadata:#readsync_status" content="{&quot;#value#&quot;: &quot;reading&quot;}"/>
  </metadata>
</package>`)

	ev, ok, err := parseOPFEvent("7", opf)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !ok {
		t.Fatal("expected ok=true")
	}
	if ev.PageNumber == nil {
		t.Fatal("page_number should not be nil")
	}
	if *ev.PageNumber != 123 {
		t.Errorf("page_number: want 123, got %d", *ev.PageNumber)
	}
}

func TestParseOPFEvent_InvalidXML(t *testing.T) {
	_, _, err := parseOPFEvent("1", []byte("not xml"))
	if err == nil {
		t.Error("expected error for invalid XML")
	}
}

func TestQuoteEnumValues(t *testing.T) {
	got := quoteEnumValues("percent,page,raw")
	want := `"percent","page","raw"`
	if got != want {
		t.Errorf("quoteEnumValues: want %q, got %q", want, got)
	}
}

func TestFindCalibredb_PathNotFound(t *testing.T) {
	// On a system without calibredb, findCalibredb should return an error.
	// This test validates the function runs without panicking.
	// It may or may not find calibredb; we just check it doesn't panic.
	_, _ = findCalibredb()
}

func TestExtractValueHash(t *testing.T) {
	cases := []struct {
		name    string
		input   string
		want    string
	}{
		{"int", `{"#value#": 47, "datatype": "int"}`, "47"},
		{"string", `{"#value#": "reading", "datatype": "enumeration"}`, "reading"},
		{"empty_string", `{"#value#": "", "datatype": "text"}`, ""},
		{"null_value", `{"#value#": null}`, ""},
		{"no_value_hash", `{"datatype": "int"}`, ""},
		{"invalid_json", `not json`, ""},
		{"empty", "", ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := extractValueHash(tc.input)
			if got != tc.want {
				t.Errorf("extractValueHash(%q) = %q, want %q", tc.input, got, tc.want)
			}
		})
	}
}

func TestMissingColumnsLogic(t *testing.T) {
	// Validate that the required column list contains all 8 required columns.
	if len(requiredColumns) != 8 {
		t.Errorf("expected 8 required columns, got %d", len(requiredColumns))
	}
	names := map[string]bool{}
	for _, c := range requiredColumns {
		names[c.Name] = true
	}
	mustHave := []string{
		"#readsync_progress",
		"#readsync_progress_mode",
		"#readsync_status",
		"#readsync_last_position",
		"#readsync_last_source",
		"#readsync_last_synced",
		"#readsync_conflict",
		"#readsync_confidence",
	}
	for _, n := range mustHave {
		if !names[n] {
			t.Errorf("required column %q is missing from requiredColumns", n)
		}
	}
}
