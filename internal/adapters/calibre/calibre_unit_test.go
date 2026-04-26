// internal/adapters/calibre/calibre_unit_test.go
//
// Additional Calibre unit tests supplementing opf_test.go.
// Tests here cover cases NOT already covered in opf_test.go.

package calibre

import (
	"strings"
	"testing"
)

// TestParseOPFEvent_ISBN13vs10 verifies that a 13-digit identifier is stored
// as ISBN-13 and a 10-digit one is stored as ISBN-10.
func TestParseOPFEvent_ISBN13vs10(t *testing.T) {
	opf13 := []byte(`<?xml version='1.0' encoding='utf-8'?>
<package xmlns="http://www.idpf.org/2007/opf" version="2.0">
  <metadata xmlns:dc="http://purl.org/dc/elements/1.1/"
            xmlns:opf="http://www.idpf.org/2007/opf">
    <dc:title>ISBN-13 Book</dc:title>
    <dc:identifier opf:scheme="isbn">9780735224292</dc:identifier>
    <meta name="calibre:user_metadata:#readsync_progress"
          content='{"#value#": 50, "datatype": "int"}'/>
    <meta name="calibre:user_metadata:#readsync_status"
          content='{"#value#": "reading", "datatype": "enumeration"}'/>
  </metadata>
</package>`)
	ev, ok, err := parseOPFEvent("1", opf13)
	if err != nil || !ok {
		t.Fatalf("parse: err=%v ok=%v", err, ok)
	}
	if ev.BookEvidence.ISBN13 != "9780735224292" {
		t.Errorf("ISBN13: want '9780735224292', got %q", ev.BookEvidence.ISBN13)
	}
	if ev.BookEvidence.ISBN10 != "" {
		t.Errorf("ISBN10 should be empty for 13-digit ISBN, got %q", ev.BookEvidence.ISBN10)
	}

	opf10 := []byte(`<?xml version='1.0' encoding='utf-8'?>
<package xmlns="http://www.idpf.org/2007/opf" version="2.0">
  <metadata xmlns:dc="http://purl.org/dc/elements/1.1/"
            xmlns:opf="http://www.idpf.org/2007/opf">
    <dc:title>ISBN-10 Book</dc:title>
    <dc:identifier opf:scheme="isbn">0735224293</dc:identifier>
    <meta name="calibre:user_metadata:#readsync_progress"
          content='{"#value#": 30, "datatype": "int"}'/>
    <meta name="calibre:user_metadata:#readsync_status"
          content='{"#value#": "reading", "datatype": "enumeration"}'/>
  </metadata>
</package>`)
	ev10, ok10, err10 := parseOPFEvent("2", opf10)
	if err10 != nil || !ok10 {
		t.Fatalf("parse10: err=%v ok=%v", err10, ok10)
	}
	if ev10.BookEvidence.ISBN10 != "0735224293" {
		t.Errorf("ISBN10: want '0735224293', got %q", ev10.BookEvidence.ISBN10)
	}
	if ev10.BookEvidence.ISBN13 != "" {
		t.Errorf("ISBN13 should be empty for 10-digit ISBN, got %q", ev10.BookEvidence.ISBN13)
	}
}

// TestParseOPFEvent_AmazonASIN tests that Amazon/mobi-asin identifiers are parsed.
func TestParseOPFEvent_AmazonASIN(t *testing.T) {
	opf := []byte(`<?xml version='1.0' encoding='utf-8'?>
<package xmlns="http://www.idpf.org/2007/opf" version="2.0">
  <metadata xmlns:dc="http://purl.org/dc/elements/1.1/"
            xmlns:opf="http://www.idpf.org/2007/opf">
    <dc:title>Amazon Book</dc:title>
    <dc:identifier opf:scheme="amazon">B08N5LNQCX</dc:identifier>
    <meta name="calibre:user_metadata:#readsync_progress"
          content='{"#value#": 60, "datatype": "int"}'/>
    <meta name="calibre:user_metadata:#readsync_status"
          content='{"#value#": "reading", "datatype": "enumeration"}'/>
  </metadata>
</package>`)
	ev, ok, err := parseOPFEvent("3", opf)
	if err != nil || !ok {
		t.Fatalf("parse: err=%v ok=%v", err, ok)
	}
	if ev.BookEvidence.ASIN != "B08N5LNQCX" {
		t.Errorf("ASIN: want 'B08N5LNQCX', got %q", ev.BookEvidence.ASIN)
	}
}

// TestParseOPFEvent_RawPositionStored verifies raw locator from
// #readsync_last_position is stored in AdapterEvent.RawLocator.
func TestParseOPFEvent_RawPositionStored(t *testing.T) {
	opf := []byte(`<?xml version='1.0' encoding='utf-8'?>
<package xmlns="http://www.idpf.org/2007/opf" version="2.0">
  <metadata xmlns:dc="http://purl.org/dc/elements/1.1/">
    <dc:title>Located Book</dc:title>
    <meta name="calibre:user_metadata:#readsync_progress"
          content='{"#value#": 73, "datatype": "int"}'/>
    <meta name="calibre:user_metadata:#readsync_status"
          content='{"#value#": "reading", "datatype": "enumeration"}'/>
    <meta name="calibre:user_metadata:#readsync_last_position"
          content='{"#value#": "epubcfi(/6/4[chap01]!/4/2:100)", "datatype": "text"}'/>
  </metadata>
</package>`)
	ev, ok, err := parseOPFEvent("5", opf)
	if err != nil || !ok {
		t.Fatalf("parse: err=%v ok=%v", err, ok)
	}
	if ev.RawLocator == nil {
		t.Fatal("RawLocator should not be nil")
	}
	if !strings.Contains(*ev.RawLocator, "epubcfi") {
		t.Errorf("RawLocator: want epubcfi, got %q", *ev.RawLocator)
	}
}

// TestRequiredColumns_ContainsProgress verifies the spec-mandated
// #readsync_progress column is in the required list.
func TestRequiredColumns_ContainsProgress(t *testing.T) {
	found := false
	for _, col := range requiredColumns {
		if col.Name == "#readsync_progress" {
			found = true
			if col.DataType != "int" {
				t.Errorf("#readsync_progress DataType: want 'int', got %q", col.DataType)
			}
		}
	}
	if !found {
		t.Error("#readsync_progress not in requiredColumns")
	}
}

// TestQuoteEnumValues_WithSpaces verifies values with leading/trailing spaces are trimmed.
func TestQuoteEnumValues_WithSpaces(t *testing.T) {
	got := quoteEnumValues(" not_started , reading , finished ")
	if !strings.Contains(got, `"not_started"`) {
		t.Errorf("trimming failed: %q", got)
	}
	if strings.Contains(got, " ") {
		t.Errorf("result still has spaces: %q", got)
	}
}
