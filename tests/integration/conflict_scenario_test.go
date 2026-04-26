// tests/integration/conflict_scenario_test.go
//
// Integration test for spec §6 conflict scenario:
//   KOReader 72% / Calibre 70% / Goodreads 38% (claims finished).
//
// Uses the real pipeline + in-memory SQLite DB.
// Requires CGO for go-sqlite3.

package integration_test

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/readsync/readsync/internal/core"
	"github.com/readsync/readsync/internal/logging"
	"github.com/readsync/readsync/internal/model"
	"github.com/readsync/readsync/internal/resolver"
)

// TestConflict_SpecSection6 exercises the spec §6 scenario using a single
// book identity (ISBN-13) shared across all three adapters so the pipeline
// correctly correlates events from all sources.
//
// Scenario:
//   Calibre  reports 70%  (submitted first, creates canonical)
//   KOReader reports 72%  (small forward advance – not suspicious)
//   Goodreads reports 100% finished from a 70% canonical – suspicious jump
func TestConflict_SpecSection6(t *testing.T) {
	database := openIntegrationDB(t)
	logger := logging.New(os.Stdout, nil, logging.LevelError)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	pipeline := core.NewPipeline(database.SQL(), logger)
	go pipeline.Run(ctx)

	// All three adapters share the same ISBN-13 so they map to the same book.
	sharedISBN := "9780062316097"
	bookEv := resolver.Evidence{
		ISBN13: sharedISBN,
		Title:  "Spec §6 Test Book",
	}

	t1 := time.Now().Add(-3 * time.Hour)
	t2 := time.Now().Add(-2 * time.Hour)

	// 1. Calibre reports 70% — creates canonical at 70%.
	if err := pipeline.Submit(ctx, core.AdapterEvent{
		BookEvidence:    bookEv,
		Source:          model.SourceCalibre,
		PercentComplete: pf(0.70),
		LocatorType:     model.LocationPercent,
		ReadStatus:      model.StatusReading,
		DeviceTS:        &t1,
	}); err != nil {
		t.Fatalf("Submit calibre 70%%: %v", err)
	}

	// 2. KOReader reports 72% — forward advance, not suspicious, no conflict.
	if err := pipeline.Submit(ctx, core.AdapterEvent{
		BookEvidence:    bookEv,
		Source:          model.SourceKOReader,
		PercentComplete: pf(0.72),
		LocatorType:     model.LocationPercent,
		ReadStatus:      model.StatusReading,
		DeviceTS:        &t2,
	}); err != nil {
		t.Fatalf("Submit koreader 72%%: %v", err)
	}

	// 3. Goodreads claims "finished" — suspicious: canonical is ~72%, below 85%.
	if err := pipeline.Submit(ctx, core.AdapterEvent{
		BookEvidence:    bookEv,
		Source:          model.SourceGoodreadsBridge,
		PercentComplete: pf(1.0),
		LocatorType:     model.LocationPercent,
		ReadStatus:      model.StatusFinished,
	}); err != nil {
		t.Fatalf("Submit goodreads finished: %v", err)
	}

	// Allow pipeline to settle.
	time.Sleep(200 * time.Millisecond)

	// A conflict must have been recorded for the Goodreads suspicious event.
	var conflictCount int
	_ = database.SQL().QueryRowContext(ctx,
		`SELECT COUNT(*) FROM conflicts WHERE status='open'`).Scan(&conflictCount)
	if conflictCount == 0 {
		t.Error("expected at least 1 open conflict: Goodreads finished from <85% canonical")
	}
	t.Logf("open conflicts after spec §6 scenario: %d", conflictCount)

	// Canonical must not have jumped to 100% (blocked by the conflict).
	var canonPct float64
	var canonSource string
	err := database.SQL().QueryRowContext(ctx, `
		SELECT cp.percent_complete, cp.updated_by
		FROM canonical_progress cp
		JOIN books b ON b.id = cp.book_id
		WHERE b.isbn13 = ?
	`, sharedISBN).Scan(&canonPct, &canonSource)
	if err != nil {
		t.Fatalf("canonical_progress not found for isbn13=%s: %v", sharedISBN, err)
	}
	t.Logf("canonical progress: %.2f%% from %s", canonPct*100, canonSource)
	if canonPct >= 0.99 {
		t.Errorf("canonical must not be at 100%% after suspicious Goodreads event; got %.2f%%",
			canonPct*100)
	}
}

// TestConflict_BackwardJump_OpenConflict verifies backward jump creates a conflict.
func TestConflict_BackwardJump_OpenConflict(t *testing.T) {
	database := openIntegrationDB(t)
	logger := logging.New(os.Stdout, nil, logging.LevelError)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	pipeline := core.NewPipeline(database.SQL(), logger)
	go pipeline.Run(ctx)

	book := resolver.Evidence{ISBN13: "9780735224292", Title: "Clean Code"}

	// 80% first.
	if err := pipeline.Submit(ctx, core.AdapterEvent{
		BookEvidence:    book,
		Source:          model.SourceKOReader,
		PercentComplete: pf(0.80),
		LocatorType:     model.LocationPercent,
		ReadStatus:      model.StatusReading,
	}); err != nil {
		t.Fatalf("Submit 80%%: %v", err)
	}

	// Drop to 50% — backward jump > 10%, must create conflict.
	if err := pipeline.Submit(ctx, core.AdapterEvent{
		BookEvidence:    book,
		Source:          model.SourceKOReader,
		PercentComplete: pf(0.50),
		LocatorType:     model.LocationPercent,
		ReadStatus:      model.StatusReading,
	}); err != nil {
		t.Fatalf("Submit 50%%: %v", err)
	}

	var n int
	_ = database.SQL().QueryRowContext(ctx,
		`SELECT COUNT(*) FROM conflicts WHERE status='open'`).Scan(&n)
	if n == 0 {
		t.Error("expected open conflict for 80%→50% backward jump")
	}
}
