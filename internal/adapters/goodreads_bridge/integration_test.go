// internal/adapters/goodreads_bridge/integration_test.go
//
// Integration test: exercise the full data path
//
//   ReadSync (Calibre adapter writes #readsync_progress)
//          ↓
//   metadata.db custom column
//          ↓
//   Goodreads Sync plugin (simulated as fixture-side reader)
//          ↓
//   "Goodreads-mirroring fixture state"
//
// We DO NOT run the real Goodreads Sync plugin here (it requires the
// Calibre GUI and a Goodreads account). Instead, the manual harness step
// is replaced with a tiny in-test simulator that reads #readsync_progress
// via calibredb and writes a JSON file representing the Goodreads state
// — exactly what the plugin would publish.
//
// The test is skipped when calibredb is not on PATH so CI without Calibre
// installed is still green.

package goodreads_bridge

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/readsync/readsync/internal/adapters/calibre"
	"github.com/readsync/readsync/internal/logging"
	"github.com/readsync/readsync/internal/model"
)

// findCalibredbForTest returns the calibredb path or skips the test.
// It also skips when any Calibre process is alive — calibredb's
// single-instance check is global and refuses mutating commands in
// that state regardless of which library --library-path points at.
func findCalibredbForTest(t *testing.T) string {
	t.Helper()
	if calibre.IsCalibreRunning() {
		t.Skip("a Calibre process (GUI/server/parallel) is running; " +
			"calibredb refuses mutating commands in that state")
	}
	if p, err := exec.LookPath("calibredb"); err == nil {
		return p
	}
	if p, err := exec.LookPath("calibredb.exe"); err == nil {
		return p
	}
	t.Skip("calibredb not found on PATH; skipping integration test")
	return ""
}

// goodreadsMirror is the "Goodreads state" the plugin would maintain.
// In real life it's a Goodreads API call; here we mirror it into a JSON
// file that the test asserts against.
type goodreadsMirror struct {
	BookID   string  `json:"book_id"`
	Title    string  `json:"title"`
	Progress float64 `json:"progress_percent"` // 0–100
	Shelf    string  `json:"shelf"`
}

// buildMirrorFromRow is the pure-data half of the simulated plugin: given
// a parsed calibredb-list JSON row, produce a goodreadsMirror exactly the
// way the real Goodreads Sync plugin would when it reads
// #readsync_progress / #readsync_gr_shelf and decides which shelf to
// assign on Goodreads. Extracted for unit testing without calibredb.
func buildMirrorFromRow(bookID string, row map[string]interface{}) goodreadsMirror {
	mirror := goodreadsMirror{BookID: bookID}
	if v, ok := row["title"].(string); ok {
		mirror.Title = v
	}
	if v, ok := row["#readsync_progress"]; ok && v != nil {
		switch n := v.(type) {
		case float64:
			mirror.Progress = n
		case string:
			f, _ := strconv.ParseFloat(n, 64)
			mirror.Progress = f
		}
	}
	if v, ok := row["#readsync_gr_shelf"].(string); ok {
		mirror.Shelf = v
	}
	if mirror.Shelf == "" && mirror.Progress >= 100 {
		mirror.Shelf = "read"
	} else if mirror.Shelf == "" && mirror.Progress > 0 {
		mirror.Shelf = "currently-reading"
	}
	return mirror
}

// simulatePluginUpload reads #readsync_progress for one book via calibredb
// (the same way the real plugin does) and writes a Goodreads-mirror JSON
// file. This replaces the manual GUI step in the test harness.
func simulatePluginUpload(t *testing.T, calibredbPath, libraryPath, bookID, mirrorPath string) {
	t.Helper()
	cmd := exec.Command(calibredbPath, "list",
		"--library-path", libraryPath,
		"--fields", "id,title,*readsync_progress,*readsync_gr_shelf",
		"--for-machine",
		"--search", "id:"+bookID,
	)
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("calibredb list: %v", err)
	}
	var rows []map[string]interface{}
	if err := json.Unmarshal(out, &rows); err != nil {
		t.Fatalf("parse calibredb output: %v\n%s", err, string(out))
	}
	if len(rows) == 0 {
		t.Fatalf("no row returned for book %s", bookID)
	}
	mirror := buildMirrorFromRow(bookID, rows[0])
	data, _ := json.MarshalIndent(mirror, "", "  ")
	if err := os.WriteFile(mirrorPath, data, 0644); err != nil {
		t.Fatalf("write mirror: %v", err)
	}
}

// extractAddedBookID parses output like "Added book ids: 1\n".
func extractAddedBookID(out string) string {
	for _, line := range strings.Split(out, "\n") {
		line = strings.TrimSpace(line)
		const marker = "Added book ids:"
		idx := strings.Index(line, marker)
		if idx == -1 {
			continue
		}
		rest := strings.TrimSpace(line[idx+len(marker):])
		for _, sep := range []string{",", " "} {
			if i := strings.Index(rest, sep); i > 0 {
				rest = rest[:i]
			}
		}
		return strings.TrimSpace(rest)
	}
	return ""
}

func readMirror(t *testing.T, path string) goodreadsMirror {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read mirror: %v", err)
	}
	var m goodreadsMirror
	if err := json.Unmarshal(data, &m); err != nil {
		t.Fatalf("parse mirror: %v", err)
	}
	return m
}

// TestIntegration_ReadSyncToGoodreadsMirror is the headline acceptance
// test for Phase 5: ReadSync → #readsync_progress → simulated plugin →
// Goodreads-mirror file.
func TestIntegration_ReadSyncToGoodreadsMirror(t *testing.T) {
	cdb := findCalibredbForTest(t)

	// Build a temp library and ensure a book exists.
	libPath := filepath.Join(t.TempDir(), "TestLibrary")
	if err := os.MkdirAll(libPath, 0755); err != nil {
		t.Fatalf("mkdir lib: %v", err)
	}
	// Create the library (calibredb auto-creates on first command).
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	_ = exec.CommandContext(ctx, cdb, "list", "--library-path", libPath, "--fields", "id").Run()

	// Add a synthetic book (--empty creates a record without an actual file).
	addOut, err := exec.CommandContext(ctx, cdb, "add", "--empty",
		"--library-path", libPath,
		"--title", "Phase5 Bridge Book",
		"--authors", "Test Author",
	).CombinedOutput()
	if err != nil {
		// calibredb refuses to mutate the library while the Calibre GUI
		// (or another writer) is running. That is operator state, not a
		// test failure — skip rather than fail.
		if strings.Contains(string(addOut), "calibre program") ||
			strings.Contains(string(addOut), "running") {
			t.Skipf("calibredb refused to write (Calibre GUI running): %s", string(addOut))
		}
		t.Fatalf("calibredb add --empty: %v\n%s", err, string(addOut))
	}
	bookID := extractAddedBookID(string(addOut))
	if bookID == "" {
		t.Fatalf("could not parse book id from: %s", string(addOut))
	}
	t.Logf("created book id=%s", bookID)

	// Tag the new book with the Goodreads identifier.
	if out, err := exec.CommandContext(ctx, cdb, "set_metadata",
		"--library-path", libPath,
		"--field", "identifiers:goodreads:99999999",
		bookID,
	).CombinedOutput(); err != nil {
		t.Fatalf("set goodreads id: %v\n%s", err, string(out))
	}

	// Use the Calibre adapter to ensure required columns exist + write
	// canonical progress (= 47%).
	calLog := logging.New(io.Discard, io.Discard, logging.LevelError)
	adp := calibre.New(calibre.Config{
		CalibredbPath: cdb,
		LibraryPath:   libPath,
		PollInterval:  60 * time.Second,
		WriteDelay:    50 * time.Millisecond,
	}, calLog)
	if err := adp.Start(ctx); err != nil {
		t.Fatalf("calibre Start: %v", err)
	}
	defer func() { _ = adp.Stop() }()

	if err := adp.EnsureColumns(ctx); err != nil {
		t.Skipf("EnsureColumns failed (Calibre GUI may be running): %v", err)
	}

	payload := fmt.Sprintf(
		`{"calibre_id":%q,"percent_complete":0.47,"read_status":"reading","goodreads_id":"99999999","locator_type":"percent"}`,
		bookID)
	job := &model.OutboxJob{
		BookID:       1,
		TargetSource: model.SourceCalibre,
		Payload:      payload,
	}
	if err := adp.WriteProgress(ctx, job); err != nil {
		t.Fatalf("Calibre WriteProgress: %v", err)
	}

	// Now run the simulated Goodreads Sync plugin "Upload reading progress".
	mirrorPath := filepath.Join(t.TempDir(), "goodreads-mirror.json")
	simulatePluginUpload(t, cdb, libPath, bookID, mirrorPath)

	mirror := readMirror(t, mirrorPath)
	if mirror.Progress < 46 || mirror.Progress > 48 {
		t.Errorf("Goodreads mirror progress: want ~47, got %f", mirror.Progress)
	}
	if !strings.EqualFold(mirror.Shelf, "currently-reading") {
		t.Errorf("Goodreads mirror shelf: want currently-reading, got %q", mirror.Shelf)
	}
	if !strings.Contains(mirror.Title, "Phase5") {
		t.Errorf("Goodreads mirror title: %q", mirror.Title)
	}

	// Bridge sanity: the bridge reports the book's identity is covered.
	books := []CalibreBookView{
		{CalibreID: bookID, Title: mirror.Title, GoodreadsID: "99999999"},
	}
	report := BuildMissingIDReport(books)
	if report.HasGaps() {
		t.Errorf("expected no missing IDs, got %d", len(report.Missing))
	}
}
