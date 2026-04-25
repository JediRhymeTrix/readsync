// internal/core/pipeline_test.go
//
// End-to-end: fake adapter → pipeline → canonical_progress → outbox.

package core

import (
	"context"
	"database/sql"
	"os"
	"testing"
	"time"

	"github.com/readsync/readsync/internal/db"
	"github.com/readsync/readsync/internal/logging"
	"github.com/readsync/readsync/internal/model"
	"github.com/readsync/readsync/internal/resolver"
)

func openTestDB(t *testing.T) *db.DB {
	t.Helper()
	f, err := os.CreateTemp("", "readsync-test-*.db")
	if err != nil {
		t.Fatalf("temp file: %v", err)
	}
	t.Cleanup(func() { os.Remove(f.Name()) })
	f.Close()
	database, err := db.Open(f.Name())
	if err != nil {
		t.Fatalf("db.Open: %v", err)
	}
	t.Cleanup(func() { database.Close() })
	if err := database.Migrate(); err != nil {
		t.Fatalf("Migrate: %v", err)
	}
	return database
}

func pf64(f float64) *float64 { return &f }

func TestMigrateCreatesAllTables(t *testing.T) {
	database := openTestDB(t)
	for _, table := range []string{
		"books", "book_aliases", "progress_events",
		"canonical_progress", "sync_outbox", "conflicts", "adapter_health",
	} {
		var name string
		err := database.SQL().QueryRow(
			`SELECT name FROM sqlite_master WHERE type='table' AND name=?`, table,
		).Scan(&name)
		if err == sql.ErrNoRows {
			t.Errorf("table %q missing after migrate", table)
		} else if err != nil {
			t.Errorf("query table %q: %v", table, err)
		}
	}
}

func TestPipeline_ProgressFlowEndToEnd(t *testing.T) {
	database := openTestDB(t)
	logger := logging.New(os.Stdout, nil, logging.LevelInfo)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	pipeline := NewPipeline(database.SQL(), logger)
	go pipeline.Run(ctx)

	ev := AdapterEvent{
		BookEvidence: resolver.Evidence{
			ISBN13: "9780735224292",
			Title:  "A Philosophy of Software Design",
		},
		Source:          model.SourceKOReader,
		PercentComplete: pf64(0.47),
		LocatorType:     model.LocationPercent,
		ReadStatus:      model.StatusReading,
	}
	if err := pipeline.Submit(ctx, ev); err != nil {
		t.Fatalf("Submit: %v", err)
	}

	var pct float64
	var source string
	err := database.SQL().QueryRowContext(ctx, `
		SELECT cp.percent_complete, cp.updated_by
		FROM canonical_progress cp JOIN books b ON b.id=cp.book_id
		WHERE b.isbn13=?`, "9780735224292").Scan(&pct, &source)
	if err != nil {
		t.Fatalf("canonical_progress: %v", err)
	}
	if pct < 0.46 || pct > 0.48 {
		t.Errorf("percent_complete=%f want ~0.47", pct)
	}
	if source != string(model.SourceKOReader) {
		t.Errorf("updated_by=%s want koreader", source)
	}

	var outboxCount int
	_ = database.SQL().QueryRowContext(ctx,
		`SELECT COUNT(*) FROM sync_outbox WHERE status='queued'`).Scan(&outboxCount)
	if outboxCount == 0 {
		t.Error("expected outbox jobs after progress event")
	}
	t.Logf("outbox jobs: %d", outboxCount)
}

func TestPipeline_BackwardJumpCreatesConflict(t *testing.T) {
	database := openTestDB(t)
	logger := logging.New(os.Stdout, nil, logging.LevelWarn)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	pipeline := NewPipeline(database.SQL(), logger)
	go pipeline.Run(ctx)

	ev := resolver.Evidence{ISBN13: "9780743273565", Title: "The Great Gatsby"}

	// 80% first.
	if err := pipeline.Submit(ctx, AdapterEvent{
		BookEvidence: ev, Source: model.SourceKOReader,
		PercentComplete: pf64(0.80), LocatorType: model.LocationPercent,
		ReadStatus: model.StatusReading,
	}); err != nil {
		t.Fatalf("Submit 80%%: %v", err)
	}

	// Drop to 50% — backward jump > 10%.
	if err := pipeline.Submit(ctx, AdapterEvent{
		BookEvidence: ev, Source: model.SourceKOReader,
		PercentComplete: pf64(0.50), LocatorType: model.LocationPercent,
		ReadStatus: model.StatusReading,
	}); err != nil {
		t.Fatalf("Submit 50%%: %v", err)
	}

	var n int
	_ = database.SQL().QueryRowContext(ctx,
		`SELECT COUNT(*) FROM conflicts WHERE status='open'`).Scan(&n)
	if n == 0 {
		t.Error("expected conflict for 80%->50% backward jump")
	}
}

func TestPipeline_ValidationRejectsEmptySource(t *testing.T) {
	database := openTestDB(t)
	logger := logging.New(os.Stdout, nil, logging.LevelInfo)
	ctx := context.Background()

	pipeline := NewPipeline(database.SQL(), logger)
	go pipeline.Run(ctx)

	err := pipeline.Submit(ctx, AdapterEvent{PercentComplete: pf64(0.5)})
	if err == nil {
		t.Error("expected validation error for empty source")
	}
}
