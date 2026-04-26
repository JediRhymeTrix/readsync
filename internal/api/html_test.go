// internal/api/html_test.go
//
// HTML rendering and integration tests.

package api

import (
	"encoding/json"
	"io"
	"strings"
	"testing"

	"github.com/readsync/readsync/internal/setup"
)

func TestDashboardHTML(t *testing.T) {
	s := newTestServer(t)
	resp := do(t, s, "GET", "/ui/dashboard", nil, nil)
	if resp.StatusCode != 200 {
		t.Errorf("status=%d", resp.StatusCode)
	}
	body, _ := io.ReadAll(resp.Body)
	for _, want := range []string{"Dashboard", "csrf-token", "ReadSync"} {
		if !strings.Contains(string(body), want) {
			t.Errorf("body missing %q", want)
		}
	}
}

func TestWizardHTML_HasAllPages(t *testing.T) {
	s := newTestServer(t)
	resp := do(t, s, "GET", "/ui/wizard", nil, nil)
	body, _ := io.ReadAll(resp.Body)
	for _, slug := range setup.Pages {
		if !strings.Contains(string(body), string(slug)) {
			t.Errorf("wizard HTML missing slug %q", slug)
		}
	}
}

func TestWizardHTML_NavigateToPage(t *testing.T) {
	s := newTestServer(t)
	resp := do(t, s, "GET", "/ui/wizard?page=calibre", nil, nil)
	body, _ := io.ReadAll(resp.Body)
	if !strings.Contains(string(body), "calibredb path") {
		t.Errorf("calibre page not rendered")
	}
}

func TestRepairHTML_ListsAllActions(t *testing.T) {
	s := newTestServer(t)
	resp := do(t, s, "GET", "/ui/repair", nil, nil)
	body, _ := io.ReadAll(resp.Body)
	for _, a := range allRepairActions() {
		if !strings.Contains(string(body), a.Title) {
			t.Errorf("repair page missing %q", a.Title)
		}
	}
}

func TestStaticAssets(t *testing.T) {
	s := newTestServer(t)
	for _, p := range []string{"/static/app.css", "/static/app.js"} {
		resp := do(t, s, "GET", p, nil, nil)
		if resp.StatusCode != 200 {
			t.Errorf("%s status=%d", p, resp.StatusCode)
		}
	}
}

func TestConflictsHTML_Empty(t *testing.T) {
	s := newTestServer(t)
	resp := do(t, s, "GET", "/ui/conflicts", nil, nil)
	body, _ := io.ReadAll(resp.Body)
	if !strings.Contains(string(body), "No conflicts") {
		t.Errorf("expected empty placeholder, got %s", body)
	}
}

func TestOutboxHTML_Empty(t *testing.T) {
	s := newTestServer(t)
	resp := do(t, s, "GET", "/ui/outbox", nil, nil)
	body, _ := io.ReadAll(resp.Body)
	if !strings.Contains(string(body), "No outbox jobs") {
		t.Errorf("expected empty placeholder, got %s", body)
	}
}

func TestActivityHTML_Empty(t *testing.T) {
	s := newTestServer(t)
	resp := do(t, s, "GET", "/ui/activity", nil, nil)
	body, _ := io.ReadAll(resp.Body)
	if !strings.Contains(string(body), "No recent activity") {
		t.Errorf("expected empty placeholder, got %s", body)
	}
}

func TestTLS_GenerateSelfSigned(t *testing.T) {
	cert, err := GenerateSelfSigned("", "", 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(cert.Certificate) == 0 {
		t.Error("empty certificate")
	}
}

func TestFreshnessFor_AllSources(t *testing.T) {
	cases := map[string]string{
		"koreader":             "live",
		"moon":                 "event-driven",
		"calibre":              "near-real-time",
		"goodreads_bridge":     "scheduled",
		"kindle_via_goodreads": "manual",
		"unknown":              "unsupported",
	}
	for src, want := range cases {
		if got := freshnessFor(src); got != want {
			t.Errorf("freshnessFor(%q)=%q want %q", src, got, want)
		}
	}
}

type fakeTrigger struct{ called bool }

func (f *fakeTrigger) TriggerSync() error { f.called = true; return nil }

func TestSyncNow_TriggerCalled(t *testing.T) {
	ft := &fakeTrigger{}
	SetSyncTrigger(ft)
	defer func() { defaultSyncTrigger = nil }()

	s := newTestServer(t)
	tok := s.CSRFToken()
	resp := do(t, s, "POST", "/api/sync_now", nil,
		map[string]string{CSRFHeader: tok})
	if resp.StatusCode != 200 {
		t.Errorf("status=%d", resp.StatusCode)
	}
	if !ft.called {
		t.Error("trigger not called")
	}
}

func TestWizardRun_Welcome_OK(t *testing.T) {
	s := newTestServer(t)
	tok := s.CSRFToken()
	resp := do(t, s, "POST", "/api/wizard/run/welcome", nil,
		map[string]string{CSRFHeader: tok})
	if resp.StatusCode != 200 {
		t.Errorf("status=%d", resp.StatusCode)
	}
	var res WizardRunResult
	_ = json.NewDecoder(resp.Body).Decode(&res)
	if !res.OK {
		t.Errorf("welcome run not OK: %+v", res)
	}
}

func TestWizardRun_BadSlug(t *testing.T) {
	s := newTestServer(t)
	tok := s.CSRFToken()
	resp := do(t, s, "POST", "/api/wizard/run/does_not_exist", nil,
		map[string]string{CSRFHeader: tok})
	body, _ := io.ReadAll(resp.Body)
	var res WizardRunResult
	_ = json.Unmarshal(body, &res)
	if res.OK {
		t.Errorf("bad slug should not OK: %+v", res)
	}
}

func TestRepair_GoodreadsPlugin(t *testing.T) {
	s := newTestServer(t)
	tok := s.CSRFToken()
	resp := do(t, s, "POST", "/api/repair/goodreads_plugin", nil,
		map[string]string{CSRFHeader: tok})
	if resp.StatusCode != 200 {
		t.Errorf("status=%d", resp.StatusCode)
	}
	body, _ := io.ReadAll(resp.Body)
	if !strings.Contains(string(body), "https://") {
		t.Errorf("expected URL in response: %s", body)
	}
}

func TestRepair_HTMLAcceptHeader(t *testing.T) {
	s := newTestServer(t)
	tok := s.CSRFToken()
	resp := do(t, s, "POST", "/api/repair/goodreads_plugin", nil,
		map[string]string{CSRFHeader: tok, "Accept": "text/html"})
	body, _ := io.ReadAll(resp.Body)
	if !strings.Contains(string(body), "rs-status-ok") {
		t.Errorf("expected HTML snippet, got %s", body)
	}
}

func TestRepair_UnknownSlug(t *testing.T) {
	s := newTestServer(t)
	tok := s.CSRFToken()
	resp := do(t, s, "POST", "/api/repair/totally_made_up", nil,
		map[string]string{CSRFHeader: tok})
	body, _ := io.ReadAll(resp.Body)
	if !strings.Contains(string(body), "unknown repair action") {
		t.Errorf("expected unknown-action error: %s", body)
	}
}
