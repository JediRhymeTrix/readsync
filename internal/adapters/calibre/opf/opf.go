// internal/adapters/calibre/opf/opf.go
//
// Pure-Go OPF parsing — no CGO, no internal/core, no internal/db.
// Import chain: internal/model, internal/resolver, stdlib only.
//
// The parent calibre package wraps the returned Event into core.AdapterEvent.
// Tests in this package compile and run without any C toolchain.

package opf

import (
	"encoding/json"
	"encoding/xml"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/readsync/readsync/internal/model"
	"github.com/readsync/readsync/internal/resolver"
)

// ColumnDef describes one Calibre custom column ReadSync manages.
type ColumnDef struct {
	Name     string // lookup name e.g. "#readsync_progress"
	Label    string // UI label
	DataType string // "int", "text", "datetime", "enumeration"
	Values   string // comma-separated allowed values (enumeration only)
}

// RequiredColumns is the authoritative list of columns ReadSync requires.
var RequiredColumns = []ColumnDef{
	{Name: "#readsync_progress", Label: "ReadSync Progress", DataType: "int"},
	{Name: "#readsync_progress_mode", Label: "ReadSync Progress Mode", DataType: "enumeration", Values: "percent,page,raw"},
	{Name: "#readsync_status", Label: "ReadSync Status", DataType: "enumeration", Values: "not_started,reading,finished,abandoned"},
	{Name: "#readsync_last_position", Label: "ReadSync Last Position", DataType: "text"},
	{Name: "#readsync_last_source", Label: "ReadSync Last Source", DataType: "text"},
	{Name: "#readsync_last_synced", Label: "ReadSync Last Synced", DataType: "datetime"},
	{Name: "#readsync_conflict", Label: "ReadSync Conflict", DataType: "text"},
	{Name: "#readsync_confidence", Label: "ReadSync Confidence", DataType: "int"},
}

// Event is the parsed result of a Calibre OPF record.
type Event struct {
	BookEvidence    resolver.Evidence
	PercentComplete *float64
	PageNumber      *int32
	ReadStatus      model.ReadStatus
	LocatorType     model.LocationType
	RawLocator      *string
	DeviceTS        *time.Time
	// HasProgress is false for books that have no ReadSync data yet.
	HasProgress bool
}

type opfPackage struct {
	XMLName  xml.Name    `xml:"package"`
	Metadata opfMetadata `xml:"metadata"`
}
type opfMetadata struct {
	Title       string     `xml:"title"`
	Creator     string     `xml:"creator"`
	Identifiers []opfIdent `xml:"identifier"`
	Metas       []opfMeta  `xml:"meta"`
}
type opfIdent struct {
	Scheme string `xml:"scheme,attr"`
	Value  string `xml:",chardata"`
}
type opfMeta struct {
	Name    string `xml:"name,attr"`
	Content string `xml:"content,attr"`
}

// ParseOPFEvent parses OPF XML from `calibredb show_metadata --as-opf`.
// Returns an error only on malformed XML.
// A zero-HasProgress Event (not an error) means the book has no ReadSync data.
func ParseOPFEvent(bookID string, opfData []byte) (Event, error) {
	var pkg opfPackage
	if err := xml.Unmarshal(opfData, &pkg); err != nil {
		return Event{}, fmt.Errorf("opf.Parse: %w", err)
	}
	m := pkg.Metadata
	ev := resolver.Evidence{
		CalibreID:  bookID,
		Title:      strings.TrimSpace(m.Title),
		AuthorSort: strings.TrimSpace(m.Creator),
	}
	for _, id := range m.Identifiers {
		scheme := strings.ToLower(strings.TrimSpace(id.Scheme))
		val := strings.TrimSpace(id.Value)
		switch scheme {
		case "isbn":
			if len(val) == 13 {
				ev.ISBN13 = val
			} else if len(val) == 10 {
				ev.ISBN10 = val
			}
		case "goodreads":
			ev.GoodreadsID = val
		case "amazon", "asin", "mobi-asin":
			ev.ASIN = val
		}
	}
	colMap := buildColMap(m.Metas)
	return buildEvent(ev, colMap), nil
}

func buildColMap(metas []opfMeta) map[string]string {
	out := make(map[string]string)
	for _, meta := range metas {
		name := strings.ToLower(strings.TrimSpace(meta.Name))
		const prefix = "calibre:user_metadata:"
		if !strings.HasPrefix(name, prefix) {
			continue
		}
		if val := ExtractValueHash(meta.Content); val != "" {
			out[name[len(prefix):]] = val
		}
	}
	return out
}

func buildEvent(ev resolver.Evidence, col map[string]string) Event {
	progressStr := col["#readsync_progress"]
	var pct *float64
	var page *int32
	if progressStr != "" {
		switch col["#readsync_progress_mode"] {
		case "percent":
			if v, err := strconv.ParseFloat(progressStr, 64); err == nil {
				p := v / 100.0
				pct = &p
			}
		case "page":
			if v, err := strconv.ParseInt(progressStr, 10, 32); err == nil {
				p := int32(v)
				page = &p
			}
		default:
			if v, err := strconv.ParseFloat(progressStr, 64); err == nil && v > 0 {
				p := v / 100.0
				pct = &p
			}
		}
	}
	readStatus := model.ReadStatus(col["#readsync_status"])
	if readStatus == "" {
		readStatus = model.StatusUnknown
	}
	hasProgress := pct != nil || page != nil ||
		(readStatus != model.StatusUnknown && readStatus != model.StatusNotStarted && readStatus != "")

	rawLoc := col["#readsync_last_position"]
	var rawLocPtr *string
	if rawLoc != "" {
		rawLocPtr = &rawLoc
	}
	var deviceTS *time.Time
	if ts := col["#readsync_last_synced"]; ts != "" {
		for _, layout := range []string{time.RFC3339, "2006-01-02T15:04:05", "2006-01-02 15:04:05"} {
			if t, err := time.Parse(layout, ts); err == nil {
				deviceTS = &t
				break
			}
		}
	}
	locType := model.LocationType(col["#readsync_progress_mode"])
	if locType == "" {
		locType = model.LocationPercent
	}
	return Event{
		BookEvidence:    ev,
		PercentComplete: pct,
		PageNumber:      page,
		ReadStatus:      readStatus,
		LocatorType:     locType,
		RawLocator:      rawLocPtr,
		DeviceTS:        deviceTS,
		HasProgress:     hasProgress,
	}
}

// ExtractValueHash extracts the "#value#" field from a Calibre custom-column
// JSON blob.  Returns "" when not found or unparseable.
func ExtractValueHash(jsonBlob string) string {
	if jsonBlob == "" || !strings.Contains(jsonBlob, "#value#") {
		return ""
	}
	var obj map[string]interface{}
	if err := json.Unmarshal([]byte(jsonBlob), &obj); err != nil {
		return ""
	}
	val, ok := obj["#value#"]
	if !ok || val == nil {
		return ""
	}
	switch v := val.(type) {
	case string:
		return v
	case float64:
		return strconv.FormatFloat(v, 'f', -1, 64)
	case bool:
		if v {
			return "true"
		}
		return "false"
	default:
		return fmt.Sprintf("%v", v)
	}
}

// QuoteEnumValues converts "a,b,c" to `"a","b","c"` for calibredb --display.
func QuoteEnumValues(vals string) string {
	parts := strings.Split(vals, ",")
	quoted := make([]string, 0, len(parts))
	for _, p := range parts {
		if p = strings.TrimSpace(p); p != "" {
			quoted = append(quoted, `"`+p+`"`)
		}
	}
	return strings.Join(quoted, ",")
}
