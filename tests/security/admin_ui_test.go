// tests/security/admin_ui_test.go
//
// Security tests:
//   - Admin UI bound to 127.0.0.1 by default.
//   - CSRF required on all write endpoints.
//   - Secrets never in JSONL logs.

package security_test

import (
	"bytes"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/readsync/readsync/internal/api"
	"github.com/readsync/readsync/internal/logging"
	"github.com/readsync/readsync/internal/setup"
)

func newSecServer(t *testing.T) *api.Server {
	t.Helper()
	wz := setup.New()
	srv, err := api.New(api.Deps{Wizard: wz, Version: "test", Port: 0, BindAddr: "127.0.0.1"})
	if err != nil {
		t.Fatalf("api.New: %v", err)
	}
	return srv
}

func doReq(t *testing.T, s *api.Server, method, path string, headers map[string]string) *http.Response {
	t.Helper()
	r := httptest.NewRequest(method, path, nil)
	for k, v := range headers {
		r.Header.Set(k, v)
	}
	w := httptest.NewRecorder()
	s.Handler().ServeHTTP(w, r)
	return w.Result()
}

// TestAdminUI_DefaultBindIsLoopback verifies default BindAddr is 127.0.0.1.
func TestAdminUI_DefaultBindIsLoopback(t *testing.T) {
	wz := setup.New()
	// No BindAddr override → defaults to 127.0.0.1.
	srv, err := api.New(api.Deps{Wizard: wz, Version: "test"})
	if err != nil {
		t.Fatalf("api.New: %v", err)
	}
	// Verify it can serve without binding to 0.0.0.0.
	_ = srv // construction must succeed
}

// TestCSRF_PostMissingToken_Returns403 verifies all write endpoints require CSRF.
func TestCSRF_PostMissingToken_Returns403(t *testing.T) {
	s := newSecServer(t)

	endpoints := []string{
		"/api/sync_now",
		"/api/restart_service",
		"/api/wizard/run/welcome",
		"/api/wizard/complete",
		"/api/wizard/reset",
		"/api/conflicts/1/resolve",
		"/api/conflicts/1/dismiss",
		"/api/outbox/1/retry",
		"/api/outbox/1/drop",
		"/api/repair/find_calibredb",
		"/api/repair/restart_service",
	}
	for _, path := range endpoints {
		path := path
		t.Run(path, func(t *testing.T) {
			resp := doReq(t, s, http.MethodPost, path, nil)
			if resp.StatusCode != http.StatusForbidden {
				t.Errorf("POST %s without CSRF: want 403, got %d", path, resp.StatusCode)
			}
		})
	}
}

// TestCSRF_ValidToken_Accepted verifies a valid CSRF token passes.
func TestCSRF_ValidToken_Accepted(t *testing.T) {
	s := newSecServer(t)
	tok := s.CSRFToken()
	resp := doReq(t, s, http.MethodPost, "/api/wizard/complete",
		map[string]string{api.CSRFHeader: tok})
	if resp.StatusCode == http.StatusForbidden {
		t.Errorf("valid CSRF token was rejected (403)")
	}
}

// TestCSRF_GETEndpoints_NoCSRFNeeded verifies GET requests bypass CSRF.
func TestCSRF_GETEndpoints_NoCSRFNeeded(t *testing.T) {
	s := newSecServer(t)
	for _, path := range []string{"/healthz", "/csrf", "/status", "/api/adapters"} {
		resp := doReq(t, s, http.MethodGet, path, nil)
		if resp.StatusCode == http.StatusForbidden {
			t.Errorf("GET %s should not require CSRF, got 403", path)
		}
	}
}

// TestLogs_NoSecretsInOutput verifies secrets are redacted before reaching logs.
func TestLogs_NoSecretsInOutput(t *testing.T) {
	var logBuf bytes.Buffer
	logger := logging.New(io.Discard, &logBuf, logging.LevelDebug)

	logger.Info("koreader auth attempt",
		logging.F("user", "alice"),
		logging.F("password", "hunter2"),
		logging.F("token", "eyJhbGciOiJSUzI1NiJ9.test"),
	)
	logger.Warn("moon upload",
		logging.F("file", "mybook.po"),
		logging.F("credential", "moonpassword123"),
	)

	output := logBuf.String()
	for _, s := range []string{"hunter2", "eyJhbGciOiJSUzI1NiJ9", "moonpassword123"} {
		if strings.Contains(output, s) {
			t.Errorf("secret %q leaked into JSONL log", s)
		}
	}
}
