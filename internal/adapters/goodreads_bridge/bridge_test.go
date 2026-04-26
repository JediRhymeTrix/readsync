// internal/adapters/goodreads_bridge/bridge_test.go
//
// Unit tests for the Goodreads bridge:
//
//   - Plugin detection (uses fixtures/goodreads/*.json)
//   - Bridge mode validation
//   - Missing-Goodreads-IDs report
//   - Stale-state detection (spec §6 example)
//   - Writeback safety gates (spec §8 rules)
//   - Adapter Start/Stop/WriteProgress lifecycle

package goodreads_bridge

import (
	"context"
	"io"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/readsync/readsync/internal/logging"
	"github.com/readsync/readsync/internal/model"
)

// fixtureCopy copies a single fixture JSON file from fixtures/goodreads
// into a temp directory under the canonical filename
// "pluginsCustomization.json".
func fixtureCopy(t *testing.T, srcRel, dst string) {
	t.Helper()
	root := repoRoot(t)
	src := filepath.Join(root, "fixtures", "goodreads", srcRel)
	data, err := os.ReadFile(src)
	if err != nil {
		t.Fatalf("read fixture %s: %v", src, err)
	}
	if err := os.WriteFile(dst, data, 0644); err != nil {
		t.Fatalf("write %s: %v", dst, err)
	}
}

// repoRoot walks upward from the test file until it finds go.mod.
func repoRoot(t *testing.T) string {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	for i := 0; i < 8; i++ {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
	t.Fatalf("could not find repo root from %s", dir)
	return ""
}

// makeFakePluginZip drops a .zip placeholder so detection treats the
// plugin as installed. We never read the contents — the file just needs
// to exist with a recognisable name.
func makeFakePluginZip(t *testing.T, dir, name string) {
	t.Helper()
	p := filepath.Join(dir, name)
	if err := os.WriteFile(p, []byte("PK\x03\x04 not-a-real-zip"), 0644); err != nil {
		t.Fatalf("write %s: %v", p, err)
	}
}

func newTestLogger() *logging.Logger {
	return logging.New(io.Discard, io.Discard, logging.LevelError)
}

// ─── Detection tests ────────────────────────────────────────────────────

func TestDetectPlugin_Enabled(t *testing.T) {
	dir := t.TempDir()
	makeFakePluginZip(t, dir, "Goodreads Sync.zip")
	fixtureCopy(t, "plugin-config-enabled.json", filepath.Join(dir, "pluginsCustomization.json"))

	d, err := DetectPlugin(dir)
	if err != nil {
		t.Fatalf("DetectPlugin: %v", err)
	}
	if !d.Installed {
		t.Error("expected Installed=true")
	}
	if !d.ConfigFound {
		t.Error("expected ConfigFound=true")
	}
	if d.ProgressColumn != "#readsync_progress" {
		t.Errorf("ProgressColumn: want #readsync_progress, got %q", d.ProgressColumn)
	}
	if !d.ProgressColumnConfigured() {
		t.Error("ProgressColumnConfigured() should be true")
	}
	if !d.ShelfColumnConfigured() {
		t.Errorf("ShelfColumnConfigured() should be true (got column %q)", d.ShelfColumn)
	}
	if !d.SyncProgressEnabled {
		t.Error("expected SyncProgressEnabled=true")
	}
}

func TestDetectPlugin_ColumnMismatch(t *testing.T) {
	dir := t.TempDir()
	makeFakePluginZip(t, dir, "goodreads_sync.zip")
	fixtureCopy(t, "plugin-config-disabled.json", filepath.Join(dir, "pluginsCustomization.json"))

	d, err := DetectPlugin(dir)
	if err != nil {
		t.Fatalf("DetectPlugin: %v", err)
	}
	if !d.Installed {
		t.Error("expected Installed=true")
	}
	if d.ProgressColumnConfigured() {
		t.Error("ProgressColumnConfigured(): expected false because progress_column is #reading_progress")
	}
	if d.SyncProgressEnabled {
		t.Error("expected SyncProgressEnabled=false")
	}
}

func TestDetectPlugin_NotInstalled(t *testing.T) {
	dir := t.TempDir()
	// no zip dropped
	fixtureCopy(t, "plugin-config-missing.json", filepath.Join(dir, "pluginsCustomization.json"))

	d, err := DetectPlugin(dir)
	if err != nil {
		t.Fatalf("DetectPlugin: %v", err)
	}
	if d.Installed {
		t.Error("expected Installed=false (no zip in dir)")
	}
	if d.ConfigFound {
		t.Error("ConfigFound: expected false because key 'Goodreads Sync' is absent")
	}
}

func TestDetectPlugin_MissingDirectory(t *testing.T) {
	d, err := DetectPlugin(filepath.Join(t.TempDir(), "definitely-not-here"))
	if err != nil {
		t.Fatalf("DetectPlugin should not error on missing dir: %v", err)
	}
	if d.Installed || d.ConfigFound {
		t.Error("missing dir should yield Installed=false ConfigFound=false")
	}
	if len(d.Notes) == 0 {
		t.Error("missing dir should append a note")
	}
}



// ─── Mode tests ─────────────────────────────────────────────────────────

func TestModes_IsValid(t *testing.T) {
	for _, m := range AllModes() {
		if !m.IsValid() {
			t.Errorf("mode %q reported as invalid", m)
		}
	}
	if BridgeMode("nonsense").IsValid() {
		t.Error("unknown mode reported as valid")
	}
}

func TestModes_IsActive(t *testing.T) {
	cases := []struct {
		m    BridgeMode
		want bool
	}{
		{ModeDisabled, false},
		{ModeManualPlugin, true},
		{ModeGuidedPlugin, true},
		{ModeCompanionPlugin, false},
		{ModeExperimentalDirect, false},
	}
	for _, c := range cases {
		if got := c.m.IsActive(); got != c.want {
			t.Errorf("%s.IsActive() = %v, want %v", c.m, got, c.want)
		}
	}
}

// ─── Missing-ID report tests ────────────────────────────────────────────

func TestBuildMissingIDReport(t *testing.T) {
	books := []CalibreBookView{
		{CalibreID: "1", Title: "Alpha", GoodreadsID: "111"},
		{CalibreID: "2", Title: "Beta", GoodreadsID: ""},
		{CalibreID: "3", Title: "Gamma", GoodreadsID: " "},   // whitespace = missing
		{CalibreID: "4", Title: "Delta", GoodreadsID: "444"}, // present
	}
	r := BuildMissingIDReport(books)
	if r.Total != 4 {
		t.Errorf("Total: want 4, got %d", r.Total)
	}
	if r.WithID != 2 {
		t.Errorf("WithID: want 2, got %d", r.WithID)
	}
	if !r.HasGaps() {
		t.Error("HasGaps: want true")
	}
	if len(r.Missing) != 2 {
		t.Fatalf("Missing: want 2 entries, got %d", len(r.Missing))
	}
	// Sorted alphabetically: Beta, Gamma.
	if r.Missing[0].Title != "Beta" || r.Missing[1].Title != "Gamma" {
		t.Errorf("Missing not sorted by title: %v", r.Missing)
	}
	if cov := r.Coverage(); cov < 0.49 || cov > 0.51 {
		t.Errorf("Coverage: want ~0.5, got %f", cov)
	}
}

func TestBuildMissingIDReport_Empty(t *testing.T) {
	r := BuildMissingIDReport(nil)
	if r.Total != 0 || r.HasGaps() || r.Coverage() != 0 {
		t.Errorf("empty input: want zeroed report, got %+v", r)
	}
}


// ─── Stale-state detection tests (spec §6 example) ─────────────────────

func TestDetectStaleFinished_LocalAt50_GoodreadsRead(t *testing.T) {
	canon := &model.CanonicalProgress{
		PercentComplete: pct(0.50),
		UpdatedBy:       model.SourceKOReader,
	}
	obs := GoodreadsObservation{Shelf: "read"}
	r := DetectStaleFinished(canon, obs)
	if !r.Stale {
		t.Fatal("stale: want true (50% local, GR finished)")
	}
	if r.Reason != "goodreads_bridge_stale" {
		t.Errorf("Reason = %q", r.Reason)
	}
}

func TestDetectStaleFinished_LocalAt90_GoodreadsRead(t *testing.T) {
	canon := &model.CanonicalProgress{PercentComplete: pct(0.90)}
	obs := GoodreadsObservation{Shelf: "read"}
	if r := DetectStaleFinished(canon, obs); r.Stale {
		t.Errorf("stale should be false at 90%%; got %+v", r)
	}
}

func TestDetectStaleFinished_NoCanonical(t *testing.T) {
	if r := DetectStaleFinished(nil, GoodreadsObservation{Shelf: "read"}); r.Stale {
		t.Error("nil canonical should never be stale")
	}
}

func TestDetectStaleFinished_GoodreadsNotFinished(t *testing.T) {
	canon := &model.CanonicalProgress{PercentComplete: pct(0.10)}
	obs := GoodreadsObservation{Shelf: "currently-reading"}
	if r := DetectStaleFinished(canon, obs); r.Stale {
		t.Errorf("non-finished GR shouldn't be stale; got %+v", r)
	}
}

func TestDetectStaleFinished_GoodreadsFinishedViaPercent(t *testing.T) {
	canon := &model.CanonicalProgress{PercentComplete: pct(0.40)}
	obs := GoodreadsObservation{PercentComplete: pct(1.0)}
	r := DetectStaleFinished(canon, obs)
	if !r.Stale {
		t.Error("100%% GR with 40%% local should be stale")
	}
}


// ─── Writeback gate tests (spec §8) ────────────────────────────────────

func TestEvaluateWriteback_HighConfidenceAllows(t *testing.T) {
	now := time.Now()
	ts := now.Add(-2 * time.Hour)
	obs := GoodreadsObservation{
		PercentComplete:    pct(0.6),
		Shelf:              "currently-reading",
		DeviceTS:           &ts,
		IdentityConfidence: 95,
	}
	d := EvaluateWriteback(nil, obs, now)
	if !d.Allow {
		t.Errorf("expected allow, got %+v", d)
	}
}

func TestEvaluateWriteback_LowConfidenceBlocks(t *testing.T) {
	now := time.Now()
	ts := now.Add(-1 * time.Hour)
	obs := GoodreadsObservation{
		DeviceTS:           &ts,
		IdentityConfidence: 80, // < 90 gate
	}
	d := EvaluateWriteback(nil, obs, now)
	if d.Allow {
		t.Error("low-confidence event should be blocked")
	}
	if d.Reason == "" {
		t.Error("Reason should explain the block")
	}
}

func TestEvaluateWriteback_MissingTimestampBlocks(t *testing.T) {
	now := time.Now()
	obs := GoodreadsObservation{IdentityConfidence: 95}
	d := EvaluateWriteback(nil, obs, now)
	if d.Allow {
		t.Error("missing device_ts should be blocked")
	}
}

func TestEvaluateWriteback_FutureTimestampBlocks(t *testing.T) {
	now := time.Now()
	future := now.Add(24 * time.Hour)
	obs := GoodreadsObservation{
		DeviceTS:           &future,
		IdentityConfidence: 95,
	}
	d := EvaluateWriteback(nil, obs, now)
	if d.Allow {
		t.Error("future timestamp should be blocked")
	}
	if d.Reason != "device_ts_in_future" {
		t.Errorf("Reason = %q", d.Reason)
	}
}

func TestEvaluateWriteback_RecentLocalChangeBlocks(t *testing.T) {
	now := time.Now()
	ts := now.Add(-30 * time.Minute)
	canon := &model.CanonicalProgress{
		PercentComplete: pct(0.40),
		UpdatedAt:       now.Add(-1 * time.Hour), // within recency window
		UpdatedBy:       model.SourceKOReader,
	}
	obs := GoodreadsObservation{
		PercentComplete:    pct(0.55),
		DeviceTS:           &ts,
		IdentityConfidence: 95,
	}
	d := EvaluateWriteback(canon, obs, now)
	if d.Allow {
		t.Errorf("recent local KOReader update should block GR writeback; got %+v", d)
	}
}

func TestEvaluateWriteback_StaleRegressionBlocks(t *testing.T) {
	now := time.Now()
	ts := now.Add(-1 * time.Hour)
	canon := &model.CanonicalProgress{
		PercentComplete: pct(0.40),
		UpdatedAt:       now.Add(-72 * time.Hour),
		UpdatedBy:       model.SourceKOReader,
	}
	obs := GoodreadsObservation{
		Shelf:              "read",
		DeviceTS:           &ts,
		IdentityConfidence: 95,
	}
	d := EvaluateWriteback(canon, obs, now)
	if d.Allow {
		t.Errorf("stale regression must be blocked; got %+v", d)
	}
	if d.Reason != "goodreads_bridge_stale" {
		t.Errorf("Reason = %q (want goodreads_bridge_stale)", d.Reason)
	}
}


// ─── Adapter lifecycle tests ───────────────────────────────────────────

func TestAdapter_DisabledByDefault(t *testing.T) {
	a := New(DefaultConfig(), newTestLogger())
	if a.Source() != model.SourceGoodreadsBridge {
		t.Errorf("Source: got %s", a.Source())
	}
	if a.Mode() != ModeDisabled {
		t.Errorf("default mode: got %s", a.Mode())
	}
	if err := a.Start(context.Background()); err != nil {
		t.Fatalf("Start: %v", err)
	}
	if a.Health() != model.HealthDisabled {
		t.Errorf("disabled adapter health: %s", a.Health())
	}
	if err := a.Stop(); err != nil {
		t.Errorf("Stop: %v", err)
	}
}

func TestAdapter_ManualMode_HealthyWithEnabledFixture(t *testing.T) {
	dir := t.TempDir()
	makeFakePluginZip(t, dir, "Goodreads Sync.zip")
	fixtureCopy(t, "plugin-config-enabled.json", filepath.Join(dir, "pluginsCustomization.json"))

	a := New(Config{Mode: ModeManualPlugin, PluginsDir: dir}, newTestLogger())
	if err := a.Start(context.Background()); err != nil {
		t.Fatalf("Start: %v", err)
	}
	if h := a.Health(); h != model.HealthOK {
		t.Errorf("health: want ok, got %s (%s)", h, a.HealthNote())
	}
	det := a.Detection()
	if det == nil || !det.Installed {
		t.Errorf("expected detection.Installed=true, got %+v", det)
	}
}

func TestAdapter_ManualMode_NeedsActionWhenColumnWrong(t *testing.T) {
	dir := t.TempDir()
	makeFakePluginZip(t, dir, "goodreads_sync.zip")
	fixtureCopy(t, "plugin-config-disabled.json", filepath.Join(dir, "pluginsCustomization.json"))

	a := New(Config{Mode: ModeManualPlugin, PluginsDir: dir}, newTestLogger())
	if err := a.Start(context.Background()); err != nil {
		t.Fatalf("Start: %v", err)
	}
	if h := a.Health(); h != model.HealthNeedsUserAction {
		t.Errorf("health: want needs_user_action, got %s", h)
	}
}

func TestAdapter_ExperimentalDirectRequiresAck(t *testing.T) {
	a := New(Config{Mode: ModeExperimentalDirect}, newTestLogger())
	if err := a.Start(context.Background()); err == nil {
		t.Error("experimental-direct without ack should fail to start")
	}
	if a.Health() != model.HealthNeedsUserAction {
		t.Errorf("health: want needs_user_action, got %s", a.Health())
	}
}

func TestAdapter_ExperimentalDirectWithAckIsDegraded(t *testing.T) {
	a := New(Config{Mode: ModeExperimentalDirect, ExperimentalDirectAck: true}, newTestLogger())
	if err := a.Start(context.Background()); err != nil {
		t.Fatalf("Start: %v", err)
	}
	if a.Health() != model.HealthDegraded {
		t.Errorf("expected degraded (stub), got %s", a.Health())
	}
}

func TestAdapter_WriteProgress_ManualLogsButSucceeds(t *testing.T) {
	a := New(Config{Mode: ModeManualPlugin}, newTestLogger())
	job := &model.OutboxJob{BookID: 7, TargetSource: model.SourceGoodreadsBridge}
	if err := a.WriteProgress(context.Background(), job); err != nil {
		t.Errorf("manual mode WriteProgress should not error: %v", err)
	}
}

func TestAdapter_WriteProgress_DisabledErrors(t *testing.T) {
	a := New(Config{Mode: ModeDisabled}, newTestLogger())
	job := &model.OutboxJob{BookID: 7}
	if err := a.WriteProgress(context.Background(), job); err == nil {
		t.Error("disabled mode WriteProgress must error")
	}
}

func TestAdapter_WriteProgress_ExperimentalRefuses(t *testing.T) {
	a := New(Config{Mode: ModeExperimentalDirect, ExperimentalDirectAck: true}, newTestLogger())
	job := &model.OutboxJob{BookID: 7}
	if err := a.WriteProgress(context.Background(), job); err == nil {
		t.Error("experimental-direct WriteProgress must refuse (stub)")
	}
}


// ─── Plugin-simulator helpers (cover integration_test.go logic without
//     needing calibredb to be runnable) ─────────────────────────────────

func TestBuildMirrorFromRow_PercentInferShelf(t *testing.T) {
	row := map[string]interface{}{
		"title":              "The Test Book",
		"#readsync_progress": float64(47),
		// no #readsync_gr_shelf set
	}
	m := buildMirrorFromRow("42", row)
	if m.BookID != "42" {
		t.Errorf("BookID: got %q", m.BookID)
	}
	if m.Title != "The Test Book" {
		t.Errorf("Title: got %q", m.Title)
	}
	if m.Progress != 47 {
		t.Errorf("Progress: got %v", m.Progress)
	}
	if m.Shelf != "currently-reading" {
		t.Errorf("Shelf: want currently-reading (inferred from 47%%), got %q", m.Shelf)
	}
}

func TestBuildMirrorFromRow_FinishedInferred(t *testing.T) {
	row := map[string]interface{}{
		"title":              "Done",
		"#readsync_progress": float64(100),
	}
	m := buildMirrorFromRow("9", row)
	if m.Shelf != "read" {
		t.Errorf("Shelf: want read (inferred from 100%%), got %q", m.Shelf)
	}
}

func TestBuildMirrorFromRow_ExplicitShelfWins(t *testing.T) {
	row := map[string]interface{}{
		"title":              "Halfway",
		"#readsync_progress": float64(50),
		"#readsync_gr_shelf": "to-read", // user override
	}
	m := buildMirrorFromRow("3", row)
	if m.Shelf != "to-read" {
		t.Errorf("explicit shelf must win, got %q", m.Shelf)
	}
}

func TestBuildMirrorFromRow_ProgressAsString(t *testing.T) {
	// Some calibredb builds emit numeric custom columns as strings.
	row := map[string]interface{}{
		"title":              "Stringy",
		"#readsync_progress": "33",
	}
	m := buildMirrorFromRow("1", row)
	if m.Progress != 33 {
		t.Errorf("string-form progress not parsed: got %v", m.Progress)
	}
}

func TestBuildMirrorFromRow_EmptyRow(t *testing.T) {
	m := buildMirrorFromRow("0", map[string]interface{}{})
	if m.Progress != 0 || m.Shelf != "" {
		t.Errorf("empty row should yield zero mirror, got %+v", m)
	}
}

func TestExtractAddedBookID(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want string
	}{
		{"single", "Added book ids: 7\n", "7"},
		{"with_prefix_lines", "warning: foo\nAdded book ids: 42\n", "42"},
		{"comma_list", "Added book ids: 11, 12, 13\n", "11"},
		{"trailing_space", "Added book ids:   99   \n", "99"},
		{"empty", "", ""},
		{"no_marker", "nothing here\n", ""},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := extractAddedBookID(c.in); got != c.want {
				t.Errorf("extractAddedBookID(%q) = %q, want %q", c.in, got, c.want)
			}
		})
	}
}

func pct(v float64) *float64 { return &v }
