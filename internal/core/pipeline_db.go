// internal/core/pipeline_db.go
//
// DB write helpers for the event pipeline - part 2.

package core

import (
	"context"
	"database/sql"
	"encoding/json"
	"time"

	"github.com/readsync/readsync/internal/model"
)

func insertEvent(ctx context.Context, tx *sql.Tx, bookID int64, confidence int, ev AdapterEvent) (int64, error) {
	now := time.Now().UTC().Format(time.RFC3339Nano)
	var dts any
	if ev.DeviceTS != nil {
		dts = ev.DeviceTS.UTC().Format(time.RFC3339Nano)
	}
	res, err := tx.ExecContext(ctx, `
		INSERT INTO progress_events(book_id,source,received_at,device_ts,
			percent_complete,page_number,total_pages,
			raw_locator,locator_type,read_status,identity_confidence)
		VALUES(?,?,?,?,?,?,?,?,?,?,?)
	`, bookID, string(ev.Source), now, dts,
		ev.PercentComplete, ev.PageNumber, ev.TotalPages,
		ev.RawLocator, string(ev.LocatorType), string(ev.ReadStatus), confidence)
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

func loadCanonical(ctx context.Context, tx *sql.Tx, bookID int64) (*model.CanonicalProgress, error) {
	var c model.CanonicalProgress
	var updatedAtStr, locType, readStatus, updatedBy string
	var userPinned int
	err := tx.QueryRowContext(ctx, `
		SELECT book_id,updated_at,updated_by,event_id,
		       percent_complete,page_number,total_pages,
		       raw_locator,locator_type,read_status,user_pinned
		FROM canonical_progress WHERE book_id=?`, bookID,
	).Scan(&c.BookID, &updatedAtStr, &updatedBy, &c.EventID,
		&c.PercentComplete, &c.PageNumber, &c.TotalPages,
		&c.RawLocator, &locType, &readStatus, &userPinned)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	// Parse the TEXT timestamp stored in ISO-8601 format.
	if t, err := time.Parse(time.RFC3339Nano, updatedAtStr); err == nil {
		c.UpdatedAt = t
	}
	c.LocatorType = model.LocationType(locType)
	c.ReadStatus = model.ReadStatus(readStatus)
	c.UpdatedBy = model.Source(updatedBy)
	c.UserPinned = userPinned != 0
	return &c, nil
}

func buildProgressEvent(bookID, eventID int64, confidence int, ev AdapterEvent) *model.ProgressEvent {
	return &model.ProgressEvent{
		ID: eventID, BookID: bookID, Source: ev.Source,
		ReceivedAt: time.Now().UTC(), DeviceTS: ev.DeviceTS,
		PercentComplete: ev.PercentComplete, PageNumber: ev.PageNumber,
		TotalPages: ev.TotalPages, RawLocator: ev.RawLocator,
		LocatorType: ev.LocatorType, ReadStatus: ev.ReadStatus,
		IdentityConfidence: confidence,
	}
}

func insertConflict(ctx context.Context, tx *sql.Tx, bookID, eventA, eventB int64, reason string) (int64, error) {
	now := time.Now().UTC().Format(time.RFC3339Nano)
	res, err := tx.ExecContext(ctx, `
		INSERT INTO conflicts(book_id,detected_at,status,event_a_id,event_b_id,reason)
		VALUES(?,?,?,?,?,?)
	`, bookID, now, string(model.ConflictOpen), eventA, eventB, reason)
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

func upsertCanonical(ctx context.Context, tx *sql.Tx, bookID, eventID int64, ev AdapterEvent) error {
	now := time.Now().UTC().Format(time.RFC3339Nano)
	_, err := tx.ExecContext(ctx, `
		INSERT INTO canonical_progress(book_id,updated_at,updated_by,event_id,
			percent_complete,page_number,total_pages,
			raw_locator,locator_type,read_status,user_pinned)
		VALUES(?,?,?,?,?,?,?,?,?,?,0)
		ON CONFLICT(book_id) DO UPDATE SET
			updated_at=excluded.updated_at, updated_by=excluded.updated_by,
			event_id=excluded.event_id,
			percent_complete=excluded.percent_complete,
			page_number=excluded.page_number, total_pages=excluded.total_pages,
			raw_locator=excluded.raw_locator, locator_type=excluded.locator_type,
			read_status=excluded.read_status
	`, bookID, now, string(ev.Source), eventID,
		ev.PercentComplete, ev.PageNumber, ev.TotalPages,
		ev.RawLocator, string(ev.LocatorType), string(ev.ReadStatus))
	return err
}

func writebackTargets(originator model.Source) []model.Source {
	all := []model.Source{
		model.SourceCalibre, model.SourceKOReader,
		model.SourceMoon, model.SourceGoodreadsBridge,
	}
	var targets []model.Source
	for _, s := range all {
		if s != originator {
			targets = append(targets, s)
		}
	}
	return targets
}

func enqueueWritebacks(ctx context.Context, tx *sql.Tx, bookID, eventID int64, originator model.Source) error {
	now := time.Now().UTC().Format(time.RFC3339Nano)
	for _, target := range writebackTargets(originator) {
		payload, _ := json.Marshal(map[string]any{"book_id": bookID, "event_id": eventID})
		_, err := tx.ExecContext(ctx, `
			INSERT INTO sync_outbox(book_id,target_source,status,attempts,payload,created_at,updated_at)
			VALUES(?,?,'queued',0,?,?,?)
		`, bookID, string(target), string(payload), now, now)
		if err != nil {
			return err
		}
	}
	return nil
}

func enqueueBlockedJobs(ctx context.Context, tx *sql.Tx, bookID, eventID int64, originator model.Source, conflictID int64) error {
	now := time.Now().UTC().Format(time.RFC3339Nano)
	for _, target := range writebackTargets(originator) {
		payload, _ := json.Marshal(map[string]any{"book_id": bookID, "event_id": eventID})
		_, err := tx.ExecContext(ctx, `
			INSERT INTO sync_outbox(book_id,target_source,status,attempts,payload,
			                        blocking_conflict_id,created_at,updated_at)
			VALUES(?,?,'blocked_by_conflict',0,?,?,?,?)
		`, bookID, string(target), string(payload), conflictID, now, now)
		if err != nil {
			return err
		}
	}
	return nil
}
