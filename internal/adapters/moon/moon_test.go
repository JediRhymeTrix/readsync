//go:build cgo

// internal/adapters/moon/moon_test.go
//
// End-to-end integration test for the Moon+ adapter.  Requires CGO/GCC
// for go-sqlite3 (same as the KOReader integration suite).

package moon_test

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/readsync/readsync/internal/adapters/moon"
	"github.com/readsync/readsync/internal/core"
	"github.com/readsync/readsync/internal/db"
	"github.com/readsync/readsync/internal/logging"
	"github.com/readsync/readsync/internal/model"
	"github.com/readsync/readsync/internal/secrets"
)

const (
	moonUser = "phone-1"
	moonPass = "supersecret-1234"
)

type moonEnv struct {
	adapter  *moon.Adapter
	pipeline *core.Pipeline
	db       *db.DB
	httpsrv  *httptest.Server
	dataDir  string
	captures string
}

func setupMoon(t *testing.T) *moonEnv {
	t.Helper()
	tmp := t.TempDir()
	dbFile := filepath.Join(tmp, "test.db")
	database, err := db.Open(dbFile)
	if err != nil {
		t.Fatalf("db.Open: %v", err)
	}
	t.Cleanup(func() { database.Close() })
	if err := database.Migrate(); err != nil {
		t.Fatalf("Migrate: %v", err)
	}
	logger := logging.New(io.Discard, io.Discard, logging.LevelError)

	pipeline := core.NewPipeline(database.SQL(), logger)
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	go pipeline.Run(ctx)

	captures := filepath.Join(tmp, "captures")
	cfg := moon.Defaults()
	cfg.WebDAV.DataDir = filepath.Join(tmp, "data")
	cfg.WebDAV.URLPrefix = "/moon-webdav/"
	cfg.CaptureDir = captures

	adapter, err := moon.New(cfg, database.SQL(), logger, secrets.NewMemStore())
	if err != nil {
		t.Fatalf("moon.New: %v", err)
	}
	adapter.SetPipeline(pipeline)

	if err := adapter.WebDAVServer().CreateUser(moonUser, moonPass); err != nil {
		t.Fatalf("CreateUser: %v", err)
	}
	hs := httptest.NewServer(adapter.WebDAVServer())
	t.Cleanup(hs.Close)

	if err := adapter.Start(ctx); err != nil {
		t.Fatalf("adapter.Start: %v", err)
	}

	return &moonEnv{
		adapter: adapter, pipeline: pipeline, db: database,
		httpsrv: hs, dataDir: cfg.WebDAV.DataDir, captures: captures,
	}
}

func putBody(t *testing.T, e *moonEnv, path string, body []byte) *http.Response {
	t.Helper()
	req, _ := http.NewRequest("PUT", e.httpsrv.URL+path, bytes.NewReader(body))
	req.SetBasicAuth(moonUser, moonPass)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("PUT %s: %v", path, err)
	}
	return resp
}

// TestMoon_HappyPath: PUT a fixture-supported .po → canonical_progress holds
// the correct percent and updated_by=moon.
func TestMoon_HappyPath(t *testing.T) {
	e := setupMoon(t)
	body := []byte("1703471974608*35@2#20432:73.2%")
	resp := putBody(t, e,
		"/moon-webdav/Apps/Books/.Moon+/Cache/MyBook.epub.po", body)
	resp.Body.Close()
	if resp.StatusCode != http.StatusCreated && resp.StatusCode != http.StatusOK {
		t.Fatalf("PUT status: %d", resp.StatusCode)
	}
	deadline := time.Now().Add(3 * time.Second)
	var pct float64
	var source string
	for time.Now().Before(deadline) {
		err := e.db.SQL().QueryRow(`
			SELECT cp.percent_complete, cp.updated_by
			FROM canonical_progress cp
			JOIN book_aliases ba ON ba.book_id = cp.book_id
			WHERE ba.source = 'moon'
			LIMIT 1`).Scan(&pct, &source)
		if err == nil {
			break
		}
		time.Sleep(50 * time.Millisecond)
	}
	if source != string(model.SourceMoon) {
		t.Fatalf("canonical updated_by: want moon, got %q (pct=%v)", source, pct)
	}
	if pct < 0.731 || pct > 0.733 {
		t.Errorf("pct: want ~0.732, got %v", pct)
	}

	var parsed int
	if err := e.db.SQL().QueryRow(
		`SELECT parsed FROM moon_uploads ORDER BY id DESC LIMIT 1`,
	).Scan(&parsed); err != nil {
		t.Fatalf("moon_uploads: %v", err)
	}
	if parsed != 1 {
		t.Errorf("moon_uploads.parsed: want 1, got %d", parsed)
	}
}

// TestMoon_UnknownFormat: a never-seen file is stored, never parsed, and
// the adapter goes degraded with a repair hint.
func TestMoon_UnknownFormat(t *testing.T) {
	e := setupMoon(t)
	weird := []byte{0xde, 0xad, 0xbe, 0xef, 0xfe, 0xed, 0xfa, 0xce}
	resp := putBody(t, e,
		"/moon-webdav/Apps/Books/.Moon+/Cache/MyBook.epub.weird", weird)
	resp.Body.Close()
	if resp.StatusCode != http.StatusCreated && resp.StatusCode != http.StatusOK {
		t.Fatalf("PUT status: %d", resp.StatusCode)
	}

	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if e.adapter.Health() == model.HealthDegraded {
			break
		}
		time.Sleep(30 * time.Millisecond)
	}
	if e.adapter.Health() != model.HealthDegraded {
		t.Fatalf("health: want degraded, got %q", e.adapter.Health())
	}
	hint := e.adapter.HealthHint()
	if hint == "" {
		t.Error("HealthHint empty")
	}
	if !strings.Contains(hint, "New Moon+ format observed") ||
		!strings.Contains(hint, "fixtures/moonplus/") {
		t.Errorf("hint missing repair guidance: %q", hint)
	}

	// Bytes still on disk, exact match.
	archive := filepath.Join(e.dataDir, "raw", moonUser, "Apps", "Books",
		".Moon+", "Cache", "MyBook.epub.weird", "1.bin")
	got, err := os.ReadFile(archive)
	if err != nil {
		t.Fatalf("archive read %s: %v", archive, err)
	}
	if !bytes.Equal(got, weird) {
		t.Errorf("archive mismatch: got %x, want %x", got, weird)
	}

	// No canonical_progress row was created.
	var n int
	if err := e.db.SQL().QueryRow(
		`SELECT COUNT(*) FROM canonical_progress`).Scan(&n); err != nil {
		t.Fatalf("count canonical: %v", err)
	}
	if n != 0 {
		t.Errorf("canonical_progress: want 0 rows, got %d", n)
	}

	// parse_error was recorded on moon_uploads.
	var parseErr string
	if err := e.db.SQL().QueryRow(
		`SELECT COALESCE(parse_error,'') FROM moon_uploads ORDER BY id DESC LIMIT 1`,
	).Scan(&parseErr); err != nil {
		t.Fatalf("parse_error: %v", err)
	}
	if parseErr == "" {
		t.Error("parse_error: want non-empty")
	}
}

// TestMoon_CaptureMode: every upload is linked into the capture directory.
func TestMoon_CaptureMode(t *testing.T) {
	e := setupMoon(t)
	body := []byte("1703471974608*5@0#0:10.0%")
	resp := putBody(t, e, "/moon-webdav/c.po", body)
	resp.Body.Close()

	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		entries, _ := os.ReadDir(e.captures)
		if len(entries) > 0 {
			return
		}
		time.Sleep(30 * time.Millisecond)
	}
	t.Errorf("capture dir is empty after PUT")
}

// TestMoon_SetupBundle: GenerateSetup returns a usable URL+credentials and
// flags Moon+ writeback as disabled (Layer 4 default).
func TestMoon_SetupBundle(t *testing.T) {
	e := setupMoon(t)
	bundle, err := e.adapter.GenerateSetup("alice")
	if err != nil {
		t.Fatalf("GenerateSetup: %v", err)
	}
	if bundle.Username != "alice" || bundle.Password == "" {
		t.Errorf("bundle creds: %+v", bundle)
	}
	if !strings.Contains(bundle.ServerURL, "/moon-webdav/") {
		t.Errorf("server URL: %q", bundle.ServerURL)
	}
	if bundle.WritebackOK {
		t.Error("writeback should be disabled by default (Layer 4 invariant)")
	}
	if !strings.Contains(bundle.Hint, "Moon+ writeback is disabled") {
		t.Errorf("setup warning missing: %q", bundle.Hint)
	}
	if len(bundle.Instructions) < 5 {
		t.Errorf("instructions too short")
	}
	for _, ins := range bundle.Instructions {
		if strings.Contains(ins, bundle.Password) {
			t.Errorf("password leaked into instructions")
		}
	}
}

// TestMoon_WriteProgressBlocked: WriteProgress refuses while no verified
// writer fixture is committed (Layer 4 invariant).
func TestMoon_WriteProgressBlocked(t *testing.T) {
	e := setupMoon(t)
	err := e.adapter.WriteProgress(context.Background(), &model.OutboxJob{})
	if err == nil {
		t.Fatal("WriteProgress must error when writeback is gated")
	}
	if !strings.Contains(err.Error(), "writeback") {
		t.Errorf("error should mention writeback gate: %v", err)
	}
}
