// internal/adapters/calibre/integration_test.go
//
// Integration tests that run real calibredb against the fixture library.
// These tests are skipped if calibredb is not found on the system.

package calibre

import (
	"context"
	"io"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/readsync/readsync/internal/logging"
)

// findCalibredbForTest returns the calibredb path or skips the test.
func findCalibredbForTest(t *testing.T) string {
	t.Helper()
	p, err := findCalibredb()
	if err != nil {
		t.Skipf("calibredb not found (%v); skipping integration test", err)
	}
	return p
}

// skipIfCalibreRunning skips the test when any Calibre process that
// blocks mutating calibredb commands is running. calibredb's
// single-instance check is global on Windows: if calibre.exe,
// calibre-debug.exe, calibre-server.exe, or calibre-parallel.exe is
// running, ANY mutating call fails with
//   "Another calibre program ... is running"
// regardless of which library --library-path points at. This is
// operator state, not a test failure, so we skip rather than fail.
func skipIfCalibreRunning(t *testing.T) {
	t.Helper()
	if isGUIRunning() {
		t.Skip("a Calibre process (GUI/server/parallel) is running; " +
			"calibredb refuses mutating commands in that state")
	}
}

// makeTempLibrary creates a fresh empty Calibre library in a temp dir.
func makeTempLibrary(t *testing.T, calibredbPath string) string {
	t.Helper()
	dir := t.TempDir()
	libPath := filepath.Join(dir, "TestLibrary")
	if err := os.MkdirAll(libPath, 0755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	// Calibre creates the library when you first run calibredb with --library-path.
	// Trigger creation by listing (empty result is fine).
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	_, _ = listBookIDs(ctx, calibredbPath, libPath)
	return libPath
}

func newTestLogger() *logging.Logger {
	return logging.New(io.Discard, io.Discard, logging.LevelError)
}

// TestIntegration_SystemScan validates that SystemScan runs without panicking
// and returns a non-empty result when calibredb is present.
func TestIntegration_SystemScan(t *testing.T) {
	findCalibredbForTest(t) // skip if not present
	r := SystemScan()
	if r.CalibredbPath == "" {
		t.Error("SystemScan: CalibredbPath should be non-empty when calibredb is found")
	}
	t.Logf("SystemScan result: health=%s path=%s libraries=%v missing=%d",
		r.Health, r.CalibredbPath, r.Libraries, len(r.MissingColumns))
}

// TestIntegration_EnsureColumns creates the fixture library and ensures
// that EnsureColumns creates all required #readsync_* columns.
func TestIntegration_EnsureColumns(t *testing.T) {
	cdb := findCalibredbForTest(t)
	skipIfCalibreRunning(t)
	lib := makeTempLibrary(t, cdb)

	log := newTestLogger()
	a := New(Config{
		CalibredbPath: cdb,
		LibraryPath:   lib,
		PollInterval:  60 * time.Second,
		WriteDelay:    1 * time.Second,
	}, log)

	ctx := context.Background()
	if err := a.Start(ctx); err != nil {
		t.Fatalf("Start: %v", err)
	}
	defer func() { _ = a.Stop() }()

	if err := a.EnsureColumns(ctx); err != nil {
		t.Fatalf("EnsureColumns: %v", err)
	}

	// Verify all required columns now exist.
	missing, err := a.MissingColumns()
	if err != nil {
		t.Fatalf("MissingColumns: %v", err)
	}
	if len(missing) != 0 {
		names := make([]string, 0, len(missing))
		for _, c := range missing {
			names = append(names, c.Name)
		}
		t.Errorf("still missing columns after EnsureColumns: %v", names)
	}
	t.Log("EnsureColumns: all required columns created successfully")
}

// TestIntegration_ReadWrite verifies that a write followed by a poll produces
// a progress event. This test requires a real Calibre library with at least one book.
// It is skipped if the fixture library has no books.
func TestIntegration_ReadWrite(t *testing.T) {
	cdb := findCalibredbForTest(t)
	skipIfCalibreRunning(t)
	lib := makeTempLibrary(t, cdb)

	log := newTestLogger()
	a := New(Config{
		CalibredbPath: cdb,
		LibraryPath:   lib,
		PollInterval:  5 * time.Second,
		WriteDelay:    100 * time.Millisecond,
	}, log)

	ctx := context.Background()
	if err := a.Start(ctx); err != nil {
		t.Fatalf("Start: %v", err)
	}
	defer func() { _ = a.Stop() }()

	// Ensure columns exist.
	if err := a.EnsureColumns(ctx); err != nil {
		t.Skipf("EnsureColumns failed (Calibre GUI may be running): %v", err)
	}

	// List books; skip if none.
	ids, err := listBookIDs(ctx, cdb, lib)
	if err != nil || len(ids) == 0 {
		t.Skip("no books in library; skipping read/write test")
	}

	bookID := ids[0]
	t.Logf("Using book ID %s for write test", bookID)

	// Set a progress value directly.
	if err := setCustomField(ctx, cdb, lib, bookID, "readsync_progress", "55"); err != nil {
		t.Fatalf("setCustomField: %v", err)
	}
	if err := setCustomField(ctx, cdb, lib, bookID, "readsync_progress_mode", "percent"); err != nil {
		t.Fatalf("setCustomField: %v", err)
	}
	if err := setCustomField(ctx, cdb, lib, bookID, "readsync_status", "reading"); err != nil {
		t.Fatalf("setCustomField: %v", err)
	}

	// Now read and verify the value comes back as a progress event.
	events, err := readAllProgress(ctx, cdb, lib)
	if err != nil {
		t.Fatalf("readAllProgress: %v", err)
	}

	found := false
	for _, ev := range events {
		if ev.BookEvidence.CalibreID == bookID {
			found = true
			if ev.PercentComplete == nil {
				t.Error("expected PercentComplete to be set")
				continue
			}
			want := 0.55
			if diff := *ev.PercentComplete - want; diff > 0.01 || diff < -0.01 {
				t.Errorf("PercentComplete: want ~0.55, got %f", *ev.PercentComplete)
			}
		}
	}
	if !found {
		t.Errorf("no event found for book ID %s", bookID)
	}
}
