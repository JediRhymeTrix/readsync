// internal/api/routes.go
//
// Route registration. Pulled out of server.go to keep the constructor
// short and the route map easy to audit (master spec section 16).

package api

import (
	"crypto/subtle"
	"io/fs"
	"net/http"
	"time"
)

func (s *Server) registerRoutes() {
	// JSON API (read-only).
	s.mux.HandleFunc("/healthz", s.handleHealthz)
	s.mux.HandleFunc("/csrf", s.handleCSRF)
	s.mux.HandleFunc("/status", s.handleStatus)
	s.mux.HandleFunc("/api/adapters", s.handleAdapters)
	s.mux.HandleFunc("/api/conflicts", s.handleConflicts)
	s.mux.HandleFunc("/api/outbox", s.handleOutbox)
	s.mux.HandleFunc("/api/events", s.handleEvents)
	s.mux.HandleFunc("/api/wizard", s.handleWizardJSON)
	s.mux.HandleFunc("/api/diagnostics", s.handleDiagnosticsJSON)

	// State-mutating endpoints (CSRF-protected).
	s.mux.HandleFunc("/api/sync_now", s.csrf(s.handleSyncNow))
	s.mux.HandleFunc("/api/restart_service", s.csrf(s.handleRestartService))
	s.mux.HandleFunc("/api/wizard/run/", s.csrf(s.handleWizardRun))
	s.mux.HandleFunc("/api/wizard/complete", s.csrf(s.handleWizardComplete))
	s.mux.HandleFunc("/api/wizard/reset", s.csrf(s.handleWizardReset))
	s.mux.HandleFunc("/api/conflicts/", s.csrf(s.handleConflictAction))
	s.mux.HandleFunc("/api/outbox/", s.csrf(s.handleOutboxAction))
	s.mux.HandleFunc("/api/repair/", s.csrf(s.handleRepairAction))

	// HTML UI.
	s.mux.HandleFunc("/", s.redirectRoot)
	s.mux.HandleFunc("/ui/dashboard", s.handleDashboard)
	s.mux.HandleFunc("/ui/wizard", s.handleWizardHTML)
	s.mux.HandleFunc("/ui/conflicts", s.handleConflictsHTML)
	s.mux.HandleFunc("/ui/outbox", s.handleOutboxHTML)
	s.mux.HandleFunc("/ui/activity", s.handleActivityHTML)
	s.mux.HandleFunc("/ui/repair", s.handleRepairHTML)

	// Static.
	staticSub, _ := fs.Sub(staticFS, "static")
	s.mux.Handle("/static/", http.StripPrefix("/static/",
		http.FileServer(http.FS(staticSub))))
}

// csrf wraps a handler in a CSRF check. GET/HEAD pass through unchanged.
// All other methods MUST present a valid X-ReadSync-CSRF header (or
// "csrf" form field, for browser form fallbacks).
func (s *Server) csrf(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet && r.Method != http.MethodHead {
			tok := r.Header.Get(CSRFHeader)
			if tok == "" {
				tok = r.FormValue("csrf")
			}
			expected := s.CSRFToken()
			// Constant-time compare to avoid token-recovery via timing.
			// subtle.ConstantTimeCompare returns 0 when lengths differ
			// or content mismatches, so an empty token is rejected too.
			if subtle.ConstantTimeCompare([]byte(tok), []byte(expected)) != 1 {
				http.Error(w, `{"error":"invalid csrf token"}`, http.StatusForbidden)
				return
			}
		}
		next(w, r)
	}
}

// redirectRoot sends the user to the wizard if setup is incomplete,
// otherwise to the dashboard.
func (s *Server) redirectRoot(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}
	if s.deps.Wizard != nil && s.deps.Wizard.NeedsSetup() {
		http.Redirect(w, r, "/ui/wizard", http.StatusFound)
		return
	}
	http.Redirect(w, r, "/ui/dashboard", http.StatusFound)
}

// handleHealthz: lightweight liveness probe; never CSRF-checked.
func (s *Server) handleHealthz(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{
		"status": "ok",
		"ts":     time.Now().UTC().Format(time.RFC3339),
	})
}

// handleCSRF returns the current CSRF token. Useful for the tray app.
func (s *Server) handleCSRF(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"csrf": s.CSRFToken()})
}

// handleStatus returns service version and uptime info.
func (s *Server) handleStatus(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{
		"service": "readsync",
		"status":  "running",
		"version": s.deps.Version,
		"ts":      time.Now().UTC().Format(time.RFC3339),
	})
}
