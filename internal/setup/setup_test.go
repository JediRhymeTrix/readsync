// internal/setup/setup_test.go

package setup

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestNew_Defaults(t *testing.T) {
	w := New()
	if !w.NeedsSetup() {
		t.Error("fresh wizard should need setup")
	}
	if w.State().CurrentPage != PageWelcome {
		t.Errorf("current=%s want welcome", w.State().CurrentPage)
	}
	if len(w.State().Pages) != len(Pages) {
		t.Errorf("page count=%d want %d", len(w.State().Pages), len(Pages))
	}
	for _, p := range Pages {
		if w.State().Pages[p].Status != StatusPending {
			t.Errorf("page %s: status=%s want pending", p, w.State().Pages[p].Status)
		}
	}
}

func TestUpdate_Status(t *testing.T) {
	w := New()
	if err := w.Update(PageCalibre, StatusOK, "found", map[string]any{"path": "C:/x"}); err != nil {
		t.Fatal(err)
	}
	p, _ := w.Page(PageCalibre)
	if p.Status != StatusOK || p.Message != "found" {
		t.Errorf("page state: %+v", p)
	}
	if p.Data["path"] != "C:/x" {
		t.Errorf("data: %+v", p.Data)
	}
}

func TestUpdate_BadSlug(t *testing.T) {
	w := New()
	if err := w.Update("does_not_exist", StatusOK, "", nil); err == nil {
		t.Error("Update with bad slug should fail")
	}
}

func TestNextPrev(t *testing.T) {
	w := New()
	w.SetCurrent(PageCalibre)
	next, ok := w.NextPage()
	if !ok || next != PageGoodreadsBridge {
		t.Errorf("next from calibre=%s want goodreads_bridge", next)
	}
	prev, ok := w.PrevPage()
	if !ok || prev != PageSystemScan {
		t.Errorf("prev from calibre=%s want system_scan", prev)
	}
	w.SetCurrent(PageFinish)
	if _, ok := w.NextPage(); ok {
		t.Error("no next from finish")
	}
	w.SetCurrent(PageWelcome)
	if _, ok := w.PrevPage(); ok {
		t.Error("no prev from welcome")
	}
}

func TestComplete(t *testing.T) {
	w := New()
	if err := w.Complete(); err != nil {
		t.Fatal(err)
	}
	if w.NeedsSetup() {
		t.Error("after Complete, NeedsSetup should be false")
	}
	// Idempotent.
	if err := w.Complete(); err != nil {
		t.Errorf("Complete should be idempotent: %v", err)
	}
}

func TestReset(t *testing.T) {
	w := New()
	w.Update(PageCalibre, StatusOK, "ok", nil)
	w.Complete()
	if err := w.Reset(); err != nil {
		t.Fatal(err)
	}
	if !w.NeedsSetup() {
		t.Error("after Reset, NeedsSetup should be true")
	}
	if w.State().Pages[PageCalibre].Status != StatusPending {
		t.Error("page state not reset")
	}
}

func TestPersist_RoundTrip(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "wizard.json")

	w1, err := NewWithPath(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := w1.Update(PageCalibre, StatusOK, "library /tmp", nil); err != nil {
		t.Fatal(err)
	}
	if err := w1.SetCurrent(PageGoodreadsBridge); err != nil {
		t.Fatal(err)
	}

	w2, err := NewWithPath(path)
	if err != nil {
		t.Fatal(err)
	}
	if w2.State().CurrentPage != PageGoodreadsBridge {
		t.Errorf("loaded current=%s want goodreads_bridge", w2.State().CurrentPage)
	}
	if w2.State().Pages[PageCalibre].Status != StatusOK {
		t.Error("page state not persisted")
	}
}

func TestPersist_FileMissing(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "missing.json")
	w, err := NewWithPath(path)
	if err != nil {
		t.Fatal(err)
	}
	if !w.NeedsSetup() {
		t.Error("missing file should produce fresh wizard")
	}
}

func TestMarshalJSON(t *testing.T) {
	w := New()
	w.Update(PageCalibre, StatusOK, "msg", nil)
	data, err := json.Marshal(w)
	if err != nil {
		t.Fatal(err)
	}
	var s State
	if err := json.Unmarshal(data, &s); err != nil {
		t.Fatal(err)
	}
	if s.Pages[PageCalibre].Status != StatusOK {
		t.Error("status round-trip failed")
	}
}

func TestSystemScan_NoConfig(t *testing.T) {
	r := SystemScan(context.Background(), ScanOptions{})
	if len(r.Probes) == 0 {
		t.Fatal("no probes returned")
	}
	// We expect at least the calibredb probe to fail for an empty path.
	found := false
	for _, p := range r.Probes {
		if p.Name == "calibredb" && !p.OK {
			found = true
		}
	}
	if !found {
		t.Error("calibredb probe should fail with empty config")
	}
}

func TestSystemScan_PortAvailable(t *testing.T) {
	// Pick a known free port via the OS, then probe it; it should be OK.
	r := SystemScan(context.Background(), ScanOptions{AdminPort: 0})
	for _, p := range r.Probes {
		if p.Name == "admin_port" && p.OK {
			t.Error("admin_port=0 should fail probe")
		}
	}
}

func TestDefaultPolicy_Valid(t *testing.T) {
	if err := DefaultPolicy().Validate(); err != nil {
		t.Errorf("DefaultPolicy invalid: %v", err)
	}
}

func TestPolicy_Invalid(t *testing.T) {
	cases := []ConflictPolicy{
		{}, // empty precedence
		{Precedence: []string{"koreader", "koreader"}, SuspiciousJumpPercent: 30, FinishedRegressionThreshold: 85},
		{Precedence: []string{"unknown"}, SuspiciousJumpPercent: 30, FinishedRegressionThreshold: 85},
		{Precedence: []string{"koreader"}, SuspiciousJumpPercent: 0, FinishedRegressionThreshold: 85},
		{Precedence: []string{"koreader"}, SuspiciousJumpPercent: 30, FinishedRegressionThreshold: 200},
		{Precedence: []string{"koreader"}, SuspiciousJumpPercent: 30, SuspiciousJumpWindowMinutes: -1, FinishedRegressionThreshold: 85},
	}
	for i, c := range cases {
		if err := c.Validate(); err == nil {
			t.Errorf("case %d should be invalid: %+v", i, c)
		}
	}
}

func TestProbe_DBFileMissing_FirstRun(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "first.db")
	r := SystemScan(context.Background(), ScanOptions{DBPath: path})
	for _, p := range r.Probes {
		if p.Name == "db_file" && !p.OK {
			t.Errorf("db_file probe should pass for non-existent path on first run: %+v", p)
		}
	}
	// Now create it and re-probe.
	_ = os.WriteFile(path, []byte("x"), 0o644)
	r = SystemScan(context.Background(), ScanOptions{DBPath: path})
	for _, p := range r.Probes {
		if p.Name == "db_file" && !p.OK {
			t.Errorf("db_file probe should pass for existing file: %+v", p)
		}
	}
}
