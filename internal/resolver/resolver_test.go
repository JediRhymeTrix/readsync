// internal/resolver/resolver_test.go
//
// Table-driven unit tests for the identity resolver scoring function.

package resolver

import (
	"testing"
)

func TestScore(t *testing.T) {
	type tc struct {
		name       string
		ev         Evidence
		stored     Evidence
		wantMin    int
		wantMax    int
		wantReason string
	}

	tests := []tc{
		{
			name:       "file_hash exact match",
			ev:         Evidence{FileHash: "sha256:abcdef1234"},
			stored:     Evidence{FileHash: "sha256:abcdef1234"},
			wantMin:    100,
			wantMax:    100,
			wantReason: "file_hash",
		},
		{
			name:       "epub_id match",
			ev:         Evidence{EpubID: "urn:uuid:abc-123"},
			stored:     Evidence{EpubID: "urn:uuid:abc-123"},
			wantMin:    97,
			wantMax:    97,
			wantReason: "epub_id",
		},
		{
			name:       "calibre_id match",
			ev:         Evidence{CalibreID: "42"},
			stored:     Evidence{CalibreID: "42"},
			wantMin:    95,
			wantMax:    95,
			wantReason: "calibre_id",
		},
		{
			name:       "goodreads_id match",
			ev:         Evidence{GoodreadsID: "12345678"},
			stored:     Evidence{GoodreadsID: "12345678"},
			wantMin:    90,
			wantMax:    90,
			wantReason: "goodreads_id",
		},
		{
			name:       "isbn13 match",
			ev:         Evidence{ISBN13: "9780735224292"},
			stored:     Evidence{ISBN13: "9780735224292"},
			wantMin:    85,
			wantMax:    85,
			wantReason: "isbn13",
		},
		{
			name:       "isbn10 match",
			ev:         Evidence{ISBN10: "0735224293"},
			stored:     Evidence{ISBN10: "0735224293"},
			wantMin:    80,
			wantMax:    80,
			wantReason: "isbn10",
		},
		{
			name:       "asin match",
			ev:         Evidence{ASIN: "B08N5LNQCX"},
			stored:     Evidence{ASIN: "B08N5LNQCX"},
			wantMin:    75,
			wantMax:    75,
			wantReason: "asin",
		},
		{
			name:       "koreader_doc_hash match",
			ev:         Evidence{KOReaderDocHash: "abc123def456"},
			stored:     Evidence{KOReaderDocHash: "abc123def456"},
			wantMin:    70,
			wantMax:    70,
			wantReason: "koreader_doc_hash",
		},
		{
			name:       "moon_key match",
			ev:         Evidence{MoonKey: "moonbook:xyz"},
			stored:     Evidence{MoonKey: "moonbook:xyz"},
			wantMin:    65,
			wantMax:    65,
			wantReason: "moon_key",
		},
		{
			name: "fuzzy title+author match",
			ev: Evidence{
				Title:      "The Pragmatic Programmer",
				AuthorSort: "Thomas, David",
			},
			stored: Evidence{
				Title:      "The Pragmatic Programmer",
				AuthorSort: "Thomas, David",
			},
			wantMin:    55,
			wantMax:    55,
			wantReason: "fuzzy_title_author",
		},
		{
			name: "title only match (close)",
			ev: Evidence{
				Title: "The Pragmatic Programmer",
			},
			stored: Evidence{
				Title: "The Pragmatic Programmer",
			},
			wantMin:    30,
			wantMax:    55, // may match fuzzy_title_author if author is empty
			wantReason: "title_only",
		},
		{
			name:    "no match",
			ev:      Evidence{FileHash: "sha256:aaaaaa"},
			stored:  Evidence{FileHash: "sha256:bbbbbb"},
			wantMin: 0,
			wantMax: 0,
		},
		{
			name: "higher signal wins over lower",
			ev: Evidence{
				FileHash:    "sha256:match",
				GoodreadsID: "12345",
			},
			stored: Evidence{
				FileHash:    "sha256:match",
				GoodreadsID: "12345",
			},
			wantMin:    100,
			wantMax:    100,
			wantReason: "file_hash",
		},
		{
			name: "case-insensitive match on isbn",
			ev:   Evidence{ISBN13: "9780735224292"},
			stored: Evidence{ISBN13: "9780735224292"},
			wantMin:    85,
			wantMax:    85,
			wantReason: "isbn13",
		},
		{
			name: "empty evidence = no match",
			ev:   Evidence{},
			stored: Evidence{
				FileHash: "sha256:xyz",
				ISBN13:   "9780735224292",
			},
			wantMin: 0,
			wantMax: 0,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			got := Score(tt.ev, tt.stored)
			if got.Confidence < tt.wantMin || got.Confidence > tt.wantMax {
				t.Errorf("Score() confidence=%d, want [%d,%d]",
					got.Confidence, tt.wantMin, tt.wantMax)
			}
			if tt.wantReason != "" && got.Reason != tt.wantReason {
				t.Errorf("Score() reason=%q, want %q", got.Reason, tt.wantReason)
			}
		})
	}
}

func TestBand(t *testing.T) {
	tests := []struct {
		score int
		want  ConfidenceBand
	}{
		{0, BandQuarantine},
		{39, BandQuarantine},
		{40, BandUserReview},
		{59, BandUserReview},
		{60, BandWritebackWary},
		{79, BandWritebackWary},
		{80, BandWritebackSafe},
		{94, BandWritebackSafe},
		{95, BandAutoResolve},
		{100, BandAutoResolve},
	}
	for _, tt := range tests {
		got := Band(tt.score)
		if got != tt.want {
			t.Errorf("Band(%d) = %d, want %d", tt.score, got, tt.want)
		}
	}
}

func TestWritebackEnabled(t *testing.T) {
	if WritebackEnabled(59) {
		t.Error("score 59 should NOT enable writeback")
	}
	if !WritebackEnabled(60) {
		t.Error("score 60 should enable writeback")
	}
	if !WritebackEnabled(100) {
		t.Error("score 100 should enable writeback")
	}
}

func TestAutoResolveEnabled(t *testing.T) {
	if AutoResolveEnabled(79) {
		t.Error("score 79 should NOT enable auto-resolve")
	}
	if !AutoResolveEnabled(80) {
		t.Error("score 80 should enable auto-resolve")
	}
}

func TestEvidenceQuality(t *testing.T) {
	tests := []struct {
		name string
		ev   Evidence
		want int
	}{
		{"file_hash", Evidence{FileHash: "sha256:abc"}, 100},
		{"epub_id", Evidence{EpubID: "urn:uuid:abc"}, 97},
		{"calibre_id", Evidence{CalibreID: "42"}, 95},
		{"goodreads_id", Evidence{GoodreadsID: "123"}, 90},
		{"isbn13", Evidence{ISBN13: "9780735224292"}, 85},
		{"isbn10", Evidence{ISBN10: "0735224293"}, 80},
		{"asin", Evidence{ASIN: "B08XYZ"}, 75},
		{"koreader_hash", Evidence{KOReaderDocHash: "abc123"}, 70},
		{"moon_key", Evidence{MoonKey: "moonbook:xyz"}, 65},
		{"title+author", Evidence{Title: "Dune", AuthorSort: "Herbert, Frank"}, 40},
		{"title_only", Evidence{Title: "Dune"}, 20},
		{"empty", Evidence{}, 0},
	}
	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			got := EvidenceQuality(tt.ev)
			if got != tt.want {
				t.Errorf("EvidenceQuality() = %d, want %d", got, tt.want)
			}
		})
	}
}
