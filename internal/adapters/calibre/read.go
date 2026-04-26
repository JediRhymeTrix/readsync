// internal/adapters/calibre/read.go
//
// Read path: ingest book identity and progress from Calibre.

package calibre

import (
	"context"
	"encoding/json"
	"encoding/xml"
	calibreopf "github.com/readsync/readsync/internal/adapters/calibre/opf"
	"fmt"
	"os/exec"
	"strconv"
	"strings"
	"time"

	"github.com/readsync/readsync/internal/core"
	"github.com/readsync/readsync/internal/model"
	"github.com/readsync/readsync/internal/resolver"
)

// readAllProgress lists all books and reads their #readsync_* columns.
func readAllProgress(ctx context.Context, calibredbPath, libraryPath string) ([]core.AdapterEvent, error) {
	ids, err := listBookIDs(ctx, calibredbPath, libraryPath)
	if err != nil {
		return nil, fmt.Errorf("list books: %w", err)
	}
	var events []core.AdapterEvent
	for _, id := range ids {
		ev, ok, err := readBookProgress(ctx, calibredbPath, libraryPath, id)
		if err != nil {
			continue
		}
		if ok {
			events = append(events, ev)
		}
	}
	return events, nil
}

// listBookIDs runs calibredb list and returns all book IDs as strings.
func listBookIDs(ctx context.Context, calibredbPath, libraryPath string) ([]string, error) {
	cmd := exec.CommandContext(ctx, calibredbPath, "list",
		"--library-path", libraryPath,
		"--fields", "id",
		"--separator", "\t",
		"--for-machine",
	)
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("calibredb list: %w", err)
	}
	// --for-machine outputs a JSON array of objects: [{"id": 1, ...}, ...]
	var books []map[string]interface{}
	if err := json.Unmarshal(out, &books); err != nil {
		// Fallback: try plain text (one id per line).
		var ids []string
		for _, line := range strings.Split(string(out), "\n") {
			line = strings.TrimSpace(line)
			if line == "" || line == "id" {
				continue
			}
			parts := strings.Fields(line)
			if len(parts) > 0 {
				id := strings.Trim(parts[0], `"[],`)
				if id != "" && id != "id" {
					ids = append(ids, id)
				}
			}
		}
		return ids, nil
	}
	ids := make([]string, 0, len(books))
	for _, b := range books {
		if idVal, ok := b["id"]; ok {
			// JSON numbers decode as float64; format as integer string.
			switch v := idVal.(type) {
			case float64:
				ids = append(ids, strconv.FormatInt(int64(v), 10))
			default:
				ids = append(ids, fmt.Sprintf("%v", v))
			}
		}
	}
	return ids, nil
}

// readBookProgress reads one book's OPF and returns an event if progress exists.
func readBookProgress(ctx context.Context, calibredbPath, libraryPath, bookID string) (core.AdapterEvent, bool, error) {
	cmd := exec.CommandContext(ctx, calibredbPath, "show_metadata",
		"--library-path", libraryPath,
		"--as-opf",
		bookID,
	)
	out, err := cmd.Output()
	if err != nil {
		return core.AdapterEvent{}, false, fmt.Errorf("show_metadata %s: %w", bookID, err)
	}
	return parseOPFEvent(bookID, out)
}

// parseOPFEventWrapper delegates to the opf subpackage and converts the result.
// The original parseOPFEvent below is still the implementation; this comment
// marks where we could later swap to calibreopf.ParseOPFEvent entirely.
var _ = calibreopf.ParseOPFEvent // ensure import is used

// OPF XML types.
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

// parseOPFEvent parses OPF XML and returns an AdapterEvent if progress data exists.
func parseOPFEvent(bookID string, opfData []byte) (core.AdapterEvent, bool, error) {
	var pkg opfPackage
	if err := xml.Unmarshal(opfData, &pkg); err != nil {
		return core.AdapterEvent{}, false, fmt.Errorf("OPF parse: %w", err)
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
	// Custom columns in OPF are stored as:
	//   <meta name="calibre:user_metadata:#readsync_progress" content='{"#value#": 47, ...}'/>
	// We extract the "#value#" from each JSON blob.
	colMap := make(map[string]string)
	for _, meta := range m.Metas {
		name := strings.ToLower(strings.TrimSpace(meta.Name))
		const prefix = "calibre:user_metadata:"
		if !strings.HasPrefix(name, prefix) {
			continue
		}
		colName := name[len(prefix):] // e.g. "#readsync_progress"
		val := extractValueHash(meta.Content)
		if val != "" {
			colMap[colName] = val
		}
	}
	progressStr := colMap["#readsync_progress"]
	var pct *float64
	var page *int32
	if progressStr != "" {
		switch colMap["#readsync_progress_mode"] {
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
	readStatus := model.ReadStatus(colMap["#readsync_status"])
	if readStatus == "" {
		readStatus = model.StatusUnknown
	}
	// Only emit events for books that have been actively touched by ReadSync
	// (have a numeric progress value OR have a meaningful read status set).
	hasProgress := pct != nil || page != nil ||
		(readStatus != model.StatusUnknown && readStatus != model.StatusNotStarted && readStatus != "")
	if !hasProgress {
		return core.AdapterEvent{}, false, nil
	}
	rawLoc := colMap["#readsync_last_position"]
	var rawLocPtr *string
	if rawLoc != "" {
		rawLocPtr = &rawLoc
	}
	var deviceTS *time.Time
	if ts := colMap["#readsync_last_synced"]; ts != "" {
		for _, layout := range []string{time.RFC3339, "2006-01-02T15:04:05", "2006-01-02 15:04:05"} {
			if t, err := time.Parse(layout, ts); err == nil {
				deviceTS = &t
				break
			}
		}
	}
	return core.AdapterEvent{
		Source:          model.SourceCalibre,
		BookEvidence:    ev,
		PercentComplete: pct,
		PageNumber:      page,
		ReadStatus:      readStatus,
		RawLocator:      rawLocPtr,
		LocatorType:     model.LocationType(colMap["#readsync_progress_mode"]),
		DeviceTS:        deviceTS,
	}, true, nil
}

// extractValueHash extracts the "#value#" field from a Calibre custom column
// JSON blob (the content attribute of <meta name="calibre:user_metadata:...">).
// Calibre stores: {"#value#": <the_actual_value>, "datatype": "...", ...}
// Returns the value as a string, or "" if not found/parseable.
func extractValueHash(jsonBlob string) string {
	if jsonBlob == "" {
		return ""
	}
	// Fast path: check if it's a simple non-JSON string (shouldn't happen but guard).
	if !strings.Contains(jsonBlob, "#value#") {
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
		// Integer stored as float in JSON.
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
