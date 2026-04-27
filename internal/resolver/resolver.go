// internal/resolver/resolver.go
//
// Identity resolver: maps adapter-supplied evidence to a canonical Book ID.
//
// Scoring ladder (spec §5):
//   file_hash         → 100
//   epub_id           → 97
//   calibre_id        → 95
//   goodreads_id      → 90
//   isbn13            → 85
//   isbn10            → 80
//   asin              → 75
//   koreader_doc_hash → 70
//   moon_key          → 65
//   fuzzy title+author→ 55  (Jaro-Winkler ≥ 0.9)
//   title only        → 30
//   no match          →  0
//
// Confidence bands:
//   95–100 : auto-resolve + writeback
//   80–94  : writeback enabled; conflict auto-resolved
//   60–79  : writeback enabled; suspicious events surface conflict
//   40–59  : writeback disabled; flag for user
//   0–39   : quarantine; no writeback

package resolver

import (
	"strings"
	"unicode"
)

// Evidence is the set of identifiers supplied by an adapter for a book.
// All fields are optional; empty string means "not provided".
type Evidence struct {
	FileHash        string
	EpubID          string
	CalibreID       string
	GoodreadsID     string
	ISBN13          string
	ISBN10          string
	ASIN            string
	KOReaderDocHash string
	MoonKey         string
	Title           string
	AuthorSort      string
}

// Match is the result of identity resolution.
type Match struct {
	// Confidence is 0–100.
	Confidence int
	// Reason is a short human-readable description of which signal matched.
	Reason string
}

// ConfidenceBand categorises a raw confidence score.
type ConfidenceBand int

const (
	BandQuarantine    ConfidenceBand = iota // 0-39
	BandUserReview                          // 40-59
	BandWritebackWary                       // 60-79
	BandWritebackSafe                       // 80-94
	BandAutoResolve                         // 95-100
)

// Band returns the ConfidenceBand for a score.
func Band(confidence int) ConfidenceBand {
	switch {
	case confidence >= 95:
		return BandAutoResolve
	case confidence >= 80:
		return BandWritebackSafe
	case confidence >= 60:
		return BandWritebackWary
	case confidence >= 40:
		return BandUserReview
	default:
		return BandQuarantine
	}
}

// WritebackEnabled returns true if the confidence band permits adapter
// writebacks (outbox jobs may proceed).
func WritebackEnabled(confidence int) bool {
	return Band(confidence) >= BandWritebackWary
}

// AutoResolveEnabled returns true if conflicts can be auto-resolved.
func AutoResolveEnabled(confidence int) bool {
	return Band(confidence) >= BandWritebackSafe
}

// EvidenceQuality returns the confidence score reflecting the quality of
// the evidence itself, independent of any stored record match.
// Used when creating a new book record from adapter evidence.
func EvidenceQuality(ev Evidence) int {
	switch {
	case ev.FileHash != "":
		return 100
	case ev.EpubID != "":
		return 97
	case ev.CalibreID != "":
		return 95
	case ev.GoodreadsID != "":
		return 90
	case ev.ISBN13 != "":
		return 85
	case ev.ISBN10 != "":
		return 80
	case ev.ASIN != "":
		return 75
	case ev.KOReaderDocHash != "":
		return 70
	case ev.MoonKey != "":
		return 65
	case ev.Title != "" && ev.AuthorSort != "":
		return 40
	case ev.Title != "":
		return 20
	default:
		return 0
	}
}

// Score computes a confidence score for the supplied evidence against a
// stored book's identifiers. It returns the best matching signal found.
//
// This is a pure function: it does not touch the database.
func Score(ev Evidence, stored Evidence) Match {
	// Exact-match signals in descending priority order.
	type signal struct {
		got    string
		want   string
		score  int
		reason string
	}
	signals := []signal{
		{ev.FileHash, stored.FileHash, 100, "file_hash"},
		{ev.EpubID, stored.EpubID, 97, "epub_id"},
		{ev.CalibreID, stored.CalibreID, 95, "calibre_id"},
		{ev.GoodreadsID, stored.GoodreadsID, 90, "goodreads_id"},
		{ev.ISBN13, stored.ISBN13, 85, "isbn13"},
		{ev.ISBN10, stored.ISBN10, 80, "isbn10"},
		{ev.ASIN, stored.ASIN, 75, "asin"},
		{ev.KOReaderDocHash, stored.KOReaderDocHash, 70, "koreader_doc_hash"},
		{ev.MoonKey, stored.MoonKey, 65, "moon_key"},
	}
	for _, s := range signals {
		if s.got != "" && s.want != "" && normalize(s.got) == normalize(s.want) {
			return Match{Confidence: s.score, Reason: s.reason}
		}
	}

	// Fuzzy title + author.
	if ev.Title != "" && stored.Title != "" &&
		ev.AuthorSort != "" && stored.AuthorSort != "" {
		ts := jaroWinkler(normalize(ev.Title), normalize(stored.Title))
		as := jaroWinkler(normalize(ev.AuthorSort), normalize(stored.AuthorSort))
		if ts >= 0.92 && as >= 0.88 {
			return Match{Confidence: 55, Reason: "fuzzy_title_author"}
		}
	}

	// Title-only fuzzy.
	if ev.Title != "" && stored.Title != "" {
		ts := jaroWinkler(normalize(ev.Title), normalize(stored.Title))
		if ts >= 0.95 {
			return Match{Confidence: 30, Reason: "title_only"}
		}
	}

	return Match{Confidence: 0, Reason: "no_match"}
}

// normalize lower-cases and strips punctuation for fuzzy comparison.
func normalize(s string) string {
	var b strings.Builder
	for _, r := range strings.ToLower(s) {
		if unicode.IsLetter(r) || unicode.IsDigit(r) || r == ' ' {
			b.WriteRune(r)
		}
	}
	return strings.TrimSpace(b.String())
}

// jaroWinkler computes the Jaro-Winkler similarity between two strings.
// Returns 0.0–1.0.
func jaroWinkler(s, t string) float64 {
	j := jaro(s, t)
	if j < 0.7 {
		return j
	}
	// Prefix length (max 4).
	l := 0
	maxPfx := 4
	if len(s) < maxPfx {
		maxPfx = len(s)
	}
	if len(t) < maxPfx {
		maxPfx = len(t)
	}
	for i := 0; i < maxPfx; i++ {
		if s[i] == t[i] {
			l++
		} else {
			break
		}
	}
	const p = 0.1
	return j + float64(l)*p*(1-j)
}

func jaro(s, t string) float64 {
	if s == t {
		return 1.0
	}
	ls, lt := len(s), len(t)
	if ls == 0 || lt == 0 {
		return 0.0
	}
	matchDist := ls
	if lt > matchDist {
		matchDist = lt
	}
	matchDist = matchDist/2 - 1
	if matchDist < 0 {
		matchDist = 0
	}

	sMatched := make([]bool, ls)
	tMatched := make([]bool, lt)
	matches := 0
	transpositions := 0

	for i := 0; i < ls; i++ {
		start := i - matchDist
		if start < 0 {
			start = 0
		}
		end := i + matchDist + 1
		if end > lt {
			end = lt
		}
		for j := start; j < end; j++ {
			if tMatched[j] || s[i] != t[j] {
				continue
			}
			sMatched[i] = true
			tMatched[j] = true
			matches++
			break
		}
	}
	if matches == 0 {
		return 0.0
	}

	k := 0
	for i := 0; i < ls; i++ {
		if !sMatched[i] {
			continue
		}
		for !tMatched[k] {
			k++
		}
		if s[i] != t[k] {
			transpositions++
		}
		k++
	}
	m := float64(matches)
	return (m/float64(ls) + m/float64(lt) + (m-float64(transpositions)/2)/m) / 3.0
}
