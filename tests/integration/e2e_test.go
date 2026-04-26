// tests/integration/e2e_test.go
//
// Integration tests using fake adapters (requires CGO for go-sqlite3).
// End-to-end: Fake KOReader push → pipeline → canonical_progress → outbox.

package integration_test

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

func openIntegrationDB(t *testing.T) *db.DB {
	t.Helper()
	f, err := os.CreateTemp("", "rs-int-*.db")
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

func pf(f float64) *float64 { return &f }

// TestE2E_FakeKOReaderPush_UpdatesCanonical tests fake push → canonical_progress.
func TestE2E_FakeKOReaderPush_UpdatesCanonical(t *testing.T) {
	database := openIntegrationDB(t)
	logger := logging.New(os.Stdout, nil, logging.LevelError)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	pipeline := core.NewPipeline(database.SQL(), logger)
	go pipeline.Run(ctx)

	book := resolver.Evidence{ISBN13: "9780062316097", Title: "The Martian"}
	script := []fake.ScriptedEvent{{
		Delay: 0, BookEvidence: book,
		PercentComplete: pf(0.47),
		LocatorType:     model.LocationPercent,
		ReadStatus:      model.StatusReading,
	}}

	adapter := fake.New(model.SourceKOReader, script)
	adapter.SetPipeline(pipeline)
	if err := adapter.Start(ctx); err != nil {
		t.Fatalf("adapter.Start: %v", err)
	}

	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if len(adapter.Emitted()) >= 1 {
			break
		}
		time.Sleep(20 * time.Millisecond)
	}

	var pct float64
	var source string
	err := database.SQL().QueryRowContext(ctx, `
		SELECT cp.percent_complete, cp.updated_by
		FROM canonical_progress cp
		JOIN books b ON b.id = cp.book_id
		WHERE b.isbn13 = ?
	`, "9780062316097").Scan(&pct, &source)
	if err != nil {
		t.Fatalf("canonical_progress query: %v", err)
	}
	if pct < 0.46 || pct > 0.48 {
		t.Errorf("percent_complete: want ~0.47, got %f", pct)
	}
	if source != string(model.SourceKOReader) {
		t.Errorf("updated_by: want koreader, got %q", source)
	}
	if errs := adapter.Errors(); len(errs) > 0 {
		t.Errorf("adapter errors: %v", errs)
	}
}

// TestE2E_OutboxJobsQueued verifies outbox jobs enqueued after canonical write.
func TestE2E_OutboxJobsQueued(t *testing.T) {
	database := openIntegrationDB(t)
	logger := logging.New(os.Stdout, nil, logging.LevelError)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	pipeline := core.NewPipeline(database.SQL(), logger)
	go pipeline.Run(ctx)

	book := resolver.Evidence{ISBN13: "9780743273565", Title: "The Great Gatsby"}
	script := []fake.ScriptedEvent{{
		Delay: 0, BookEvidence: book,
		PercentComplete: pf(0.60),
		LocatorType:     model.LocationPercent,
		ReadStatus:      model.StatusReading,
	}}

	adapter := fake.New(model.SourceKOReader, script)
	adapter.SetPipeline(pipeline)
	if err := adapter.Start(ctx); err != nil {
		t.Fatalf("Start: %v", err)
	}

	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if len(adapter.Emitted()) >= 1 {
			break
		}
		time.Sleep(20 * time.Millisecond)
	}

	var outboxCount int
	_ = database.SQL().QueryRowContext(ctx,
		`SELECT COUNT(*) FROM sync_outbox WHERE status='queued'`).Scan(&outboxCount)
	if outboxCount == 0 {
		t.Error("expected outbox jobs enqueued after canonical write")
	}
}
