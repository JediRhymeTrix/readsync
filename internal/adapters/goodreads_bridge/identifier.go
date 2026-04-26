// internal/adapters/goodreads_bridge/identifier.go
//
// Goodreads identifier resolution and missing-ID reporting.
//
// IMPORTANT: ReadSync never *fetches* Goodreads IDs over the network. We
// only read identifiers that the user (or the Goodreads Sync plugin) has
// already attached to a Calibre book under the "goodreads" scheme.

package goodreads_bridge

import (
	"sort"
	"strings"
)

// CalibreBookView is the minimum subset of book metadata the bridge needs
// to compute the missing-ID report. It deliberately depends only on
// strings/ints so it can be populated from anything that reads Calibre
// (the calibre adapter, a fixture, a unit test).
type CalibreBookView struct {
	CalibreID  string
	Title      string
	AuthorSort string

	// GoodreadsID is the value of the "goodreads" identifier scheme, or
	// empty if the book has no such identifier set.
	GoodreadsID string
}

// MissingIDReport summarises which books need a Goodreads identifier
// before the bridge can correlate them.
type MissingIDReport struct {
	// Total is the total number of books inspected.
	Total int

	// WithID is the number of books that have a Goodreads identifier.
	WithID int

	// Missing lists the books that lack a Goodreads identifier, sorted
	// by title for stable output.
	Missing []CalibreBookView
}

// Coverage returns the fraction of books with a Goodreads ID, in [0,1].
// Returns 0 when Total == 0 to avoid a division-by-zero in callers.
func (r MissingIDReport) Coverage() float64 {
	if r.Total == 0 {
		return 0
	}
	return float64(r.WithID) / float64(r.Total)
}

// HasGaps reports whether at least one book is missing a Goodreads ID.
func (r MissingIDReport) HasGaps() bool { return len(r.Missing) > 0 }

// BuildMissingIDReport scans the given books and builds the report. The
// input slice is not mutated.
func BuildMissingIDReport(books []CalibreBookView) MissingIDReport {
	r := MissingIDReport{Total: len(books)}
	for _, b := range books {
		if strings.TrimSpace(b.GoodreadsID) != "" {
			r.WithID++
			continue
		}
		r.Missing = append(r.Missing, b)
	}
	sort.Slice(r.Missing, func(i, j int) bool {
		ti := strings.ToLower(r.Missing[i].Title)
		tj := strings.ToLower(r.Missing[j].Title)
		if ti != tj {
			return ti < tj
		}
		return r.Missing[i].CalibreID < r.Missing[j].CalibreID
	})
	return r
}
