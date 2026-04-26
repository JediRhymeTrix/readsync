// internal/resolver/resolver_bands_test.go
//
// Extended unit tests for all five confidence bands and EvidenceQuality
// scoring from spec §5.

package resolver

import "testing"

// TestBand_AllBoundaries verifies every band boundary value exactly.
func TestBand_AllBoundaries(t *testing.T) {
	cases := []struct {
		score int
		want  ConfidenceBand
		label string
	}{
		// Quarantine band: 0-39
		{0, BandQuarantine, "quarantine-bottom"},
		{39, BandQuarantine, "quarantine-top"},
		// UserReview band: 40-59
		{40, BandUserReview, "userreview-bottom"},
		{55, BandUserReview, "userreview-fuzzy"},
		{59, BandUserReview, "userreview-top"},
		// WritebackWary band: 60-79
		{60, BandWritebackWary, "writebackwary-bottom"},
		{65, BandWritebackWary, "writebackwary-moon"},
		{70, BandWritebackWary, "writebackwary-koreader"},
		{79, BandWritebackWary, "writebackwary-top"},
		// WritebackSafe band: 80-94
		{80, BandWritebackSafe, "writebacksafe-bottom"},
		{90, BandWritebackSafe, "writebacksafe-goodreads"},
		{94, BandWritebackSafe, "writebacksafe-top"},
		// AutoResolve band: 95-100
		{95, BandAutoResolve, "autoresolve-bottom"},
		{100, BandAutoResolve, "autoresolve-filehash"},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.label, func(t *testing.T) {
			got := Band(tc.score)
			if got != tc.want {
				t.Errorf("Band(%d)=%d want %d", tc.score, got, tc.want)
			}
		})
	}
}

// TestWritebackEnabled_AllBands verifies writeback permission per band.
func TestWritebackEnabled_AllBands(t *testing.T) {
	cases := []struct {
		score   int
		enabled bool
		label   string
	}{
		{0, false, "quarantine"},
		{59, false, "userreview-top"},
		{60, true, "writebackwary-bottom"},
		{70, true, "koreader-hash"},
		{80, true, "writebacksafe"},
		{95, true, "autoresolve"},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.label, func(t *testing.T) {
			if WritebackEnabled(tc.score) != tc.enabled {
				t.Errorf("WritebackEnabled(%d)=%v want %v", tc.score, !tc.enabled, tc.enabled)
			}
		})
	}
}

// TestScore_AllSignals verifies each signal score in the ladder.
func TestScore_AllSignals(t *testing.T) {
	cases := []struct {
		name       string
		ev         Evidence
		stored     Evidence
		wantScore  int
		wantReason string
	}{
		{"file_hash=100", Evidence{FileHash: "sha256:aabb"}, Evidence{FileHash: "sha256:aabb"}, 100, "file_hash"},
		{"epub_id=97", Evidence{EpubID: "urn:uuid:test"}, Evidence{EpubID: "urn:uuid:test"}, 97, "epub_id"},
		{"calibre_id=95", Evidence{CalibreID: "7"}, Evidence{CalibreID: "7"}, 95, "calibre_id"},
		{"goodreads_id=90", Evidence{GoodreadsID: "9876"}, Evidence{GoodreadsID: "9876"}, 90, "goodreads_id"},
		{"isbn13=85", Evidence{ISBN13: "9780062316097"}, Evidence{ISBN13: "9780062316097"}, 85, "isbn13"},
		{"isbn10=80", Evidence{ISBN10: "0062316095"}, Evidence{ISBN10: "0062316095"}, 80, "isbn10"},
		{"asin=75", Evidence{ASIN: "B09XYZ"}, Evidence{ASIN: "B09XYZ"}, 75, "asin"},
		{"koreader=70", Evidence{KOReaderDocHash: "deadbeef"}, Evidence{KOReaderDocHash: "deadbeef"}, 70, "koreader_doc_hash"},
		{"moon=65", Evidence{MoonKey: "book#key"}, Evidence{MoonKey: "book#key"}, 65, "moon_key"},
		{"no_match=0", Evidence{FileHash: "sha256:aaaa"}, Evidence{FileHash: "sha256:bbbb"}, 0, "no_match"},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			got := Score(tc.ev, tc.stored)
			if got.Confidence != tc.wantScore {
				t.Errorf("Score()=%d want %d", got.Confidence, tc.wantScore)
			}
			if tc.wantReason != "" && got.Reason != tc.wantReason {
				t.Errorf("Reason=%q want %q", got.Reason, tc.wantReason)
			}
		})
	}
}

// TestScore_HigherSignalWins verifies priority ordering.
func TestScore_HigherSignalWins(t *testing.T) {
	ev := Evidence{FileHash: "sha256:winner", ISBN13: "9780062316097", MoonKey: "mybook#key"}
	stored := Evidence{FileHash: "sha256:winner", ISBN13: "9780062316097", MoonKey: "mybook#key"}
	got := Score(ev, stored)
	if got.Confidence != 100 {
		t.Errorf("expected file_hash to win at 100, got %d (%s)", got.Confidence, got.Reason)
	}
}

// TestScore_CaseInsensitive verifies normalize handles mixed case.
func TestScore_CaseInsensitive(t *testing.T) {
	ev := Evidence{FileHash: "SHA256:ABCDEF1234"}
	stored := Evidence{FileHash: "sha256:abcdef1234"}
	got := Score(ev, stored)
	if got.Confidence != 100 {
		t.Errorf("case-insensitive file_hash: want 100, got %d", got.Confidence)
	}
}

// TestScore_FuzzyTitleAuthor tests Jaro-Winkler fuzzy matching.
func TestScore_FuzzyTitleAuthor(t *testing.T) {
	ev := Evidence{Title: "Dune", AuthorSort: "Herbert, Frank"}
	stored := Evidence{Title: "Dune", AuthorSort: "Herbert, Frank"}
	got := Score(ev, stored)
	if got.Confidence != 55 {
		t.Errorf("exact title+author: want 55, got %d (%s)", got.Confidence, got.Reason)
	}
	if got.Reason != "fuzzy_title_author" {
		t.Errorf("reason: want fuzzy_title_author, got %q", got.Reason)
	}
}
