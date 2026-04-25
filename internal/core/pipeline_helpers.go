// internal/core/pipeline_helpers.go
//
// Helper functions for the event pipeline - part 1: validation, resolution.

package core

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/readsync/readsync/internal/model"
	"github.com/readsync/readsync/internal/resolver"
)

func validateEvent(ev AdapterEvent) error {
	if ev.Source == "" {
		return fmt.Errorf("source is required")
	}
	if ev.PercentComplete != nil && (*ev.PercentComplete < 0 || *ev.PercentComplete > 1.01) {
		return fmt.Errorf("percent_complete out of range: %f", *ev.PercentComplete)
	}
	return nil
}

func normalizeEvent(ev AdapterEvent) AdapterEvent {
	if ev.PercentComplete != nil {
		v := *ev.PercentComplete
		if v > 1.0 {
			v = 1.0
		}
		if v < 0 {
			v = 0
		}
		ev.PercentComplete = &v
	}
	if ev.ReadStatus == "" {
		ev.ReadStatus = model.StatusUnknown
	}
	if ev.LocatorType == "" {
		ev.LocatorType = model.LocationRaw
	}
	return ev
}

func resolveBook(ctx context.Context, tx *sql.Tx, ev AdapterEvent) (int64, int, error) {
	bookID, err := findBookByEvidence(ctx, tx, ev.BookEvidence)
	if err != nil {
		return 0, 0, err
	}
	if bookID > 0 {
		stored, err := loadBookEvidence(ctx, tx, bookID)
		if err != nil {
			return 0, 0, err
		}
		m := resolver.Score(ev.BookEvidence, stored)
		return bookID, m.Confidence, nil
	}
	// New book: confidence reflects the quality of the incoming evidence.
	newID, err := createBook(ctx, tx, ev.BookEvidence)
	if err != nil {
		return 0, 0, err
	}
	confidence := resolver.EvidenceQuality(ev.BookEvidence)
	return newID, confidence, nil
}

func findBookByEvidence(ctx context.Context, tx *sql.Tx, ev resolver.Evidence) (int64, error) {
	if ev.CalibreID != "" {
		var id int64
		err := tx.QueryRowContext(ctx,
			`SELECT id FROM books WHERE calibre_id=? LIMIT 1`, ev.CalibreID,
		).Scan(&id)
		if err == nil {
			return id, nil
		}
		if err != sql.ErrNoRows {
			return 0, err
		}
	}
	type lookup struct{ col, val string }
	for _, l := range []lookup{
		{"file_hash", ev.FileHash}, {"epub_id", ev.EpubID},
		{"goodreads_id", ev.GoodreadsID}, {"isbn13", ev.ISBN13},
		{"isbn10", ev.ISBN10}, {"asin", ev.ASIN},
	} {
		if l.val == "" {
			continue
		}
		var id int64
		err := tx.QueryRowContext(ctx,
			fmt.Sprintf(`SELECT id FROM books WHERE %s=? LIMIT 1`, l.col), l.val,
		).Scan(&id)
		if err == sql.ErrNoRows {
			continue
		}
		if err != nil {
			return 0, err
		}
		return id, nil
	}
	for _, kv := range []struct{ source, key string }{
		{string(model.SourceKOReader), ev.KOReaderDocHash},
		{string(model.SourceMoon), ev.MoonKey},
	} {
		if kv.key == "" {
			continue
		}
		var id int64
		err := tx.QueryRowContext(ctx,
			`SELECT book_id FROM book_aliases WHERE source=? AND adapter_key=? LIMIT 1`,
			kv.source, kv.key).Scan(&id)
		if err == sql.ErrNoRows {
			continue
		}
		if err != nil {
			return 0, err
		}
		return id, nil
	}
	return 0, nil
}

func loadBookEvidence(ctx context.Context, tx *sql.Tx, bookID int64) (resolver.Evidence, error) {
	var ev resolver.Evidence
	var calibreID sql.NullString
	err := tx.QueryRowContext(ctx, `
		SELECT COALESCE(file_hash,''), COALESCE(epub_id,''),
		       CAST(COALESCE(calibre_id,'') AS TEXT),
		       COALESCE(goodreads_id,''), COALESCE(isbn13,''), COALESCE(isbn10,''),
		       COALESCE(asin,''), COALESCE(title,''), COALESCE(author_sort,'')
		FROM books WHERE id=?`, bookID,
	).Scan(&ev.FileHash, &ev.EpubID, &calibreID, &ev.GoodreadsID,
		&ev.ISBN13, &ev.ISBN10, &ev.ASIN, &ev.Title, &ev.AuthorSort)
	ev.CalibreID = calibreID.String
	return ev, err
}

func createBook(ctx context.Context, tx *sql.Tx, ev resolver.Evidence) (int64, error) {
	now := time.Now().UTC().Format(time.RFC3339Nano)
	var calibreID any
	if ev.CalibreID != "" {
		calibreID = ev.CalibreID
	}
	res, err := tx.ExecContext(ctx, `
		INSERT INTO books(created_at,updated_at,calibre_id,goodreads_id,isbn13,isbn10,
		                  asin,epub_id,file_hash,title,author_sort,identity_confidence)
		VALUES(?,?,?,?,?,?,?,?,?,?,?,0)
	`, now, now, calibreID, nullStr(ev.GoodreadsID), nullStr(ev.ISBN13),
		nullStr(ev.ISBN10), nullStr(ev.ASIN), nullStr(ev.EpubID),
		nullStr(ev.FileHash), ev.Title, ev.AuthorSort)
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

func nullStr(s string) any {
	if s == "" {
		return nil
	}
	return s
}
