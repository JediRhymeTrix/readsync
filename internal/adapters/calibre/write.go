// internal/adapters/calibre/write.go
//
// Write path: apply canonical progress to Calibre custom columns.
// Uses `calibredb set_custom` and `calibredb set_metadata`.

package calibre

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"os/exec"
	"time"

	"github.com/readsync/readsync/internal/logging"
	"github.com/readsync/readsync/internal/model"
)

// writePayload is the JSON-encoded payload stored in OutboxJob.Payload.
type writePayload struct {
	CalibreID       string  `json:"calibre_id"`
	PercentComplete float64 `json:"percent_complete"` // 0.0-1.0
	PageNumber      *int32  `json:"page_number,omitempty"`
	TotalPages      *int32  `json:"total_pages,omitempty"`
	ReadStatus      string  `json:"read_status"`
	RawLocator      string  `json:"raw_locator,omitempty"`
	LocatorType     string  `json:"locator_type,omitempty"`
	GoodreadsID     string  `json:"goodreads_id,omitempty"`
}

// applyWrite executes the write of canonical progress to Calibre.
func (a *Adapter) applyWrite(ctx context.Context, calibredbPath, libraryPath string, job *model.OutboxJob) error {
	var p writePayload
	if err := json.Unmarshal([]byte(job.Payload), &p); err != nil {
		return fmt.Errorf("calibre write: bad payload: %w", err)
	}
	if p.CalibreID == "" {
		return fmt.Errorf("calibre write: missing calibre_id in payload")
	}

	// Write #readsync_progress (store as integer 0-100 for percent mode).
	progress := int(math.Round(p.PercentComplete * 100))
	if err := setCustomField(ctx, calibredbPath, libraryPath, p.CalibreID,
		"readsync_progress", fmt.Sprintf("%d", progress)); err != nil {
		return err
	}

	// Write #readsync_progress_mode.
	mode := string(p.LocatorType)
	if mode == "" {
		mode = "percent"
	}
	if err := setCustomField(ctx, calibredbPath, libraryPath, p.CalibreID,
		"readsync_progress_mode", mode); err != nil {
		return err
	}

	// Write #readsync_status.
	if err := setCustomField(ctx, calibredbPath, libraryPath, p.CalibreID,
		"readsync_status", p.ReadStatus); err != nil {
		return err
	}

	// Write #readsync_last_position.
	if p.RawLocator != "" {
		if err := setCustomField(ctx, calibredbPath, libraryPath, p.CalibreID,
			"readsync_last_position", p.RawLocator); err != nil {
			return err
		}
	}

	// Write #readsync_last_source.
	if err := setCustomField(ctx, calibredbPath, libraryPath, p.CalibreID,
		"readsync_last_source", string(model.SourceCalibre)); err != nil {
		return err
	}

	// Write #readsync_last_synced (ISO8601 UTC).
	now := time.Now().UTC().Format(time.RFC3339)
	if err := setCustomField(ctx, calibredbPath, libraryPath, p.CalibreID,
		"readsync_last_synced", now); err != nil {
		return err
	}

	// If Goodreads ID is provided, update the identifiers field.
	if p.GoodreadsID != "" {
		if err := setMetadataField(ctx, calibredbPath, libraryPath, p.CalibreID,
			"goodreads", p.GoodreadsID); err != nil {
			// Non-fatal: log and continue.
			a.log.Warn("calibre: could not write goodreads identifier",
				logging.F("book_id", p.CalibreID), logging.F("err", err))
		}
	}

	return nil
}

// setCustomField runs `calibredb set_custom LOOKUP_NAME BOOK_ID VALUE`.
func setCustomField(ctx context.Context, calibredbPath, libraryPath, bookID, lookupName, value string) error {
	cmd := exec.CommandContext(ctx, calibredbPath, "set_custom",
		"--library-path", libraryPath,
		lookupName,
		bookID,
		value,
	)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("set_custom %s book=%s: %w\n%s", lookupName, bookID, err, string(out))
	}
	return nil
}

// setMetadataField runs `calibredb set_metadata --field identifiers:TYPE:VALUE`.
func setMetadataField(ctx context.Context, calibredbPath, libraryPath, bookID, idType, idValue string) error {
	field := fmt.Sprintf("identifiers:%s:%s", idType, idValue)
	cmd := exec.CommandContext(ctx, calibredbPath, "set_metadata",
		"--library-path", libraryPath,
		"--field", field,
		bookID,
	)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("set_metadata identifiers book=%s: %w\n%s", bookID, err, string(out))
	}
	return nil
}
