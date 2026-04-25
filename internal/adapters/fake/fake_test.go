// internal/adapters/fake/fake_test.go
//
// Integration test: fake adapter emits scripted events into the pipeline.
// Verifies the full flow: fake → pipeline → canonical_progress → outbox.

package fake_test

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/readsync/readsync/internal/adapters/fake"
	"github.com/readsync/readsync/internal/core"
	"github.com/readsync/readsync/internal/db"
	"github.com/readsync/readsync/internal/logging"
	"github.com/readsync/readsync/internal/model"
	"github.com/readsync/readsync/internal/resolver"
)

func openTestDB(t *testing.T) *db.DB {
	t.Helper()
	f, err := os.CreateTemp("", "readsync-fake-test-*.db")
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

func TestFakeAdapter_EmitsScriptedEvents(t *testing.T) {
	database := openTestDB(t)
	logger := logging.New(os.Stdout, nil, logging.LevelInfo)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	pipeline := core.NewPipeline(database.SQL(), logger)
	go pipeline.Run(ctx)

	book := resolver.Evidence{
		ISBN13: "9780062316097",
		Title:  "The Martian",
	}
	ts1 := time.Now().Add(-2 * time.Hour)
	ts2 := time.Now().Add(-1 * time.Hour)
	ts3 := time.Now()

	script := []fake.ScriptedEvent{
		{
			Delay:           0,
			BookEvidence:    book,
			PercentComplete: pf64(0.10),
			LocatorType:     model.LocationPercent,
			ReadStatus:      model.StatusReading,
			DeviceTS:        &ts1,
		},
		{
			Delay:           10 * time.Millisecond,
			BookEvidence:    book,
			PercentComplete: pf64(0.45),
			LocatorType:     model.LocationPercent,
			ReadStatus:      model.StatusReading,
			DeviceTS:        &ts2,
		},
		{
			Delay:           10 * time.Millisecond,
			BookEvidence:    book,
			PercentComplete: pf64(1.0),
			LocatorType:     model.LocationPercent,
			ReadStatus:      model.StatusFinished,
			DeviceTS:        &ts3,
		},
	}

	adapter := fake.New(model.SourceKOReader, script)
	adapter.SetPipeline(pipeline)
	if err := adapter.Start(ctx); err != nil {
		t.Fatalf("adapter.Start: %v", err)
	}

	// Wait for all events to be processed.
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if len(adapter.Emitted()) >= len(script) {
			break
		}
		time.Sleep(20 * time.Millisecond)
	}

	emitted := adapter.Emitted()
	if len(emitted) != len(script) {
		t.Fatalf("emitted %d events, expected %d", len(emitted), len(script))
	}

	errs := adapter.Errors()
	if len(errs) != 0 {
		t.Errorf("adapter errors: %v", errs)
	}

	// Verify canonical_progress shows 100% (finished).
	var pct float64
	var status string
	err := database.SQL().QueryRowContext(ctx, `
		SELECT cp.percent_complete, cp.read_status
		FROM canonical_progress cp
		JOIN books b ON b.id = cp.book_id
		WHERE b.isbn13 = ?
	`, "9780062316097").Scan(&pct, &status)
	if err != nil {
		t.Fatalf("canonical_progress query: %v", err)
	}
	if pct < 0.99 {
		t.Errorf("expected canonical pct ~1.0, got %f", pct)
	}
	if status != string(model.StatusFinished) {
		t.Errorf("expected status=finished, got %s", status)
	}

	// Verify progress_events table has 3 entries.
	var count int
	_ = database.SQL().QueryRowContext(ctx,
		`SELECT COUNT(*) FROM progress_events pe
		 JOIN books b ON b.id = pe.book_id WHERE b.isbn13 = ?`,
		"9780062316097").Scan(&count)
	if count != 3 {
		t.Errorf("expected 3 progress_events, got %d", count)
	}
}
