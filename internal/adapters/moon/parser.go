// internal/adapters/moon/parser.go
//
// Layer 3 of the Moon+ adapter: read-only progress extractor.
//
// Strict policy: this parser will only extract structured progress for a
// (filename-suffix, format-version) pair that has been verified by a
// committed fixture.  Any unrecognised payload is treated as "unknown
// format", left untouched in the versioned archive, and surfaced via the
// adapter health as `degraded` with an actionable repair hint.
//
// Verified format (FormatV1Plain):
//   File suffix : .po
//   Layout      : "{file_id}*{position}@{chapter}#{scroll}:{percentage}%"
//   Verified    : 2026-04-25 from real Moon+ Pro v9 captures
//                 (docs/research/moonplus.md §3) and synthetic fixtures.

package moon

import (
	"errors"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/readsync/readsync/internal/core"
	"github.com/readsync/readsync/internal/model"
	"github.com/readsync/readsync/internal/resolver"
)

// Format identifies a single verified Moon+ on-wire format.
type Format string

const (
	FormatV1Plain Format = "po-v1-plain"
	FormatUnknown Format = "unknown"
)

// ParserVersion is the parser code revision.  Bump on every change to the
// extraction logic so we can identify which parser produced a stored event.
const ParserVersion = "moon-parser/1.0.0"

// Result is the outcome of parsing a single uploaded file.
type Result struct {
	Format Format

	BookKey    string
	Percent    float64
	Position   string
	LastReadTS time.Time
	Device     string

	ParserVersion string
	FormatVersion string
}

// poV1Re matches "{file_id}*{position}@{chapter}#{scroll}:{percent}%".
var poV1Re = regexp.MustCompile(
	`^(?P<file_id>[^*]*)\*(?P<position>[^@]*)@(?P<chapter>[^#]*)#(?P<scroll>[^:]*):(?P<pct>[0-9]+(?:\.[0-9]+)?)%$`)

// ErrUnknownFormat is returned when the parser cannot classify a payload.
var ErrUnknownFormat = errors.New("moon: unknown format")

// Parse classifies and extracts progress from a raw upload.
func Parse(filename string, body []byte, receivedAt time.Time) (Result, error) {
	low := strings.ToLower(filename)
	switch {
	case strings.HasSuffix(low, ".po"):
		return parsePOV1(filename, body, receivedAt)
	case strings.HasSuffix(low, ".an"):
		// Annotations file - ignored for progress (moonplus.md §6 #4).
		return Result{Format: FormatUnknown, ParserVersion: ParserVersion},
			ErrUnknownFormat
	default:
		return Result{Format: FormatUnknown, ParserVersion: ParserVersion},
			fmt.Errorf("%w: unrecognised suffix %q", ErrUnknownFormat, filename)
	}
}

func parsePOV1(filename string, body []byte, receivedAt time.Time) (Result, error) {
	text := strings.TrimSpace(string(body))
	if text == "" {
		return Result{Format: FormatUnknown, ParserVersion: ParserVersion},
			fmt.Errorf("%w: empty .po body", ErrUnknownFormat)
	}
	m := poV1Re.FindStringSubmatch(text)
	if m == nil {
		return Result{Format: FormatUnknown, ParserVersion: ParserVersion},
			fmt.Errorf("%w: malformed .po body", ErrUnknownFormat)
	}
	pctStr := m[poV1Re.SubexpIndex("pct")]
	pctVal, err := strconv.ParseFloat(pctStr, 64)
	if err != nil {
		return Result{Format: FormatUnknown, ParserVersion: ParserVersion},
			fmt.Errorf("%w: bad percentage %q", ErrUnknownFormat, pctStr)
	}
	if pctVal < 0 || pctVal > 100.5 {
		return Result{Format: FormatUnknown, ParserVersion: ParserVersion},
			fmt.Errorf("%w: percentage out of range %.2f", ErrUnknownFormat, pctVal)
	}
	pct := pctVal / 100.0
	if pct > 1.0 {
		pct = 1.0
	}

	// Strip the .po (case-insensitive) suffix to derive the canonical
	// basename used in the book key.  We only reach this branch via the
	// ".po" check in Parse, so the suffix is guaranteed to be present.
	base := filename
	if len(base) >= 3 && strings.EqualFold(base[len(base)-3:], ".po") {
		base = base[:len(base)-3]
	}
	fileID := strings.TrimSpace(m[poV1Re.SubexpIndex("file_id")])
	bookKey := base
	if fileID != "" {
		bookKey = base + "#" + fileID
	}

	return Result{
		Format:        FormatV1Plain,
		BookKey:       bookKey,
		Percent:       pct,
		Position:      text,
		LastReadTS:    receivedAt,
		Device:        "moon+",
		ParserVersion: ParserVersion,
		FormatVersion: string(FormatV1Plain),
	}, nil
}

// ToAdapterEvent converts a Result into a core.AdapterEvent ready for pipeline.
func (r Result) ToAdapterEvent() core.AdapterEvent {
	pct := r.Percent
	loc := r.Position
	ts := r.LastReadTS
	rs := model.StatusUnknown
	switch {
	case pct >= 1.0:
		rs = model.StatusFinished
	case pct > 0:
		rs = model.StatusReading
	}
	return core.AdapterEvent{
		BookEvidence:    resolver.Evidence{MoonKey: r.BookKey},
		Source:          model.SourceMoon,
		DeviceTS:        &ts,
		PercentComplete: &pct,
		RawLocator:      &loc,
		LocatorType:     model.LocationMoonPosition,
		ReadStatus:      rs,
	}
}
