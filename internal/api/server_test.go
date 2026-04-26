// internal/api/server_test.go

package api

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/readsync/readsync/internal/setup"
)

func newTestServer(t *testing.T) *Server {
	t.Helper()
	wz := setup.New()
	s, err := New(Deps{Wizard: wz, Version: "test", Port: 0, BindAddr: "127.0.0.1"})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	return s
}

func do(t *testing.T, s *Server, method, path string, body io.Reader, headers map[string]string) *http.Response {
	t.Helper()
	r := httptest.NewRequest(method, path, body)
	for k, v := range headers {
		r.Header.Set(k, v)
	}
	w := httptest.NewRecorder()
	s.mux.ServeHTTP(w, r)
	return w.Result()
}

func TestHealthz(t *testing.T) {
	s := newTestServer(t)
	resp := do(t, s, "GET", "/healthz", nil, nil)
	if resp.StatusCode != 200 {
		t.Errorf("status=%d", resp.StatusCode)
	}
	body, _ := io.ReadAll(resp.Body)
	if !strings.Contains(string(body), `"status":"ok"`) {
		t.Errorf("body=%s", body)
	}
}

func TestStatus_HasVersion(t *testing.T) {
	s := newTestServer(t)
	resp := do(t, s, "GET", "/status", nil, nil)
	body, _ := io.ReadAll(resp.Body)
	if !strings.Contains(string(body), `"version"`) {
		t.Errorf("missing version: %s", body)
	}
}

func TestCSRF_GetEndpointsBypass(t *testing.T) {
	s := newTestServer(t)
	for _, p := range []string{"/healthz", "/csrf", "/status", "/api/adapters"} {
		resp := do(t, s, "GET", p, nil, nil)
		if resp.StatusCode != 200 {
			t.Errorf("%s GET should not require CSRF, got %d", p, resp.StatusCode)
		}
	}
}

func TestCSRF_PostMissingTokenForbidden(t *testing.T) {
	s := newTestServer(t)
	endpoints := []string{
		"/api/sync_now",
		"/api/restart_service",
		"/api/wizard/run/welcome",
		"/api/wizard/complete",
		"/api/wizard/reset",
		"/api/conflicts/1/resolve?winner=a",
		"/api/conflicts/1/dismiss",
		"/api/outbox/1/retry",
		"/api/outbox/1/drop",
		"/api/repair/find_calibredb",
		"/api/repair/restart_service",
	}
	for _, p := range endpoints {
		resp := do(t, s, "POST", p, nil, nil)
		if resp.StatusCode != http.StatusForbidden {
			body, _ := io.ReadAll(resp.Body)
			t.Errorf("%s without csrf: status=%d body=%s", p, resp.StatusCode, body)
		}
	}
}

func TestCSRF_PostWithTokenAccepted(t *testing.T) {
	s := newTestServer(t)
	tok := s.CSRFToken()
	resp := do(t, s, "POST", "/api/wizard/complete", nil,
		map[string]string{CSRFHeader: tok})
	if resp.StatusCode != 200 {
		body, _ := io.ReadAll(resp.Body)
		t.Errorf("status=%d body=%s", resp.StatusCode, body)
	}
}

func TestCSRF_FormFieldFallback(t *testing.T) {
	s := newTestServer(t)
	tok := s.CSRFToken()
	body := strings.NewReader("csrf=" + tok)
	resp := do(t, s, "POST", "/api/wizard/reset", body,
		map[string]string{"Content-Type": "application/x-www-form-urlencoded"})
	if resp.StatusCode != 200 {
		t.Errorf("status=%d", resp.StatusCode)
	}
}

func TestCSRF_DifferentTokenRejected(t *testing.T) {
	s := newTestServer(t)
	resp := do(t, s, "POST", "/api/wizard/complete", nil,
		map[string]string{CSRFHeader: "deadbeef-not-a-real-token"})
	if resp.StatusCode != http.StatusForbidden {
		t.Errorf("status=%d want 403", resp.StatusCode)
	}
}

func TestRedirectRoot_NeedsSetup(t *testing.T) {
	s := newTestServer(t)
	resp := do(t, s, "GET", "/", nil, nil)
	if resp.StatusCode != http.StatusFound {
		t.Errorf("status=%d", resp.StatusCode)
	}
	loc := resp.Header.Get("Location")
	if !strings.Contains(loc, "/ui/wizard") {
		t.Errorf("location=%s", loc)
	}
}

func TestRedirectRoot_Completed(t *testing.T) {
	s := newTestServer(t)
	_ = s.deps.Wizard.Complete()
	resp := do(t, s, "GET", "/", nil, nil)
	loc := resp.Header.Get("Location")
	if !strings.Contains(loc, "/ui/dashboard") {
		t.Errorf("location=%s", loc)
	}
}

func TestStartStop(t *testing.T) {
	s := newTestServer(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	if err := s.Start(ctx); err != nil {
		t.Fatal(err)
	}
	defer s.Stop()
	if s.Addr() == "" {
		t.Error("Addr empty after start")
	}
}

func TestGenerateCSRFToken_Unique(t *testing.T) {
	a, _ := generateCSRFToken()
	b, _ := generateCSRFToken()
	if a == b {
		t.Error("tokens not unique")
	}
	if len(a) != 48 {
		t.Errorf("token length=%d want 48", len(a))
	}
}

func TestWizardJSON(t *testing.T) {
	s := newTestServer(t)
	resp := do(t, s, "GET", "/api/wizard", nil, nil)
	body, _ := io.ReadAll(resp.Body)
	var st setup.State
	if err := json.Unmarshal(body, &st); err != nil {
		t.Fatalf("unmarshal: %v body=%s", err, body)
	}
	if st.CurrentPage != setup.PageWelcome {
		t.Errorf("current=%s", st.CurrentPage)
	}
}
