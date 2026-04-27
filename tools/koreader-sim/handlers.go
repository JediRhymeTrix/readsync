// tools/koreader-sim/handlers.go
// HTTP handler implementations for the KOSync simulator.

package main

import (
	"encoding/json"
	"log"
	"net/http"
	"strings"
	"time"
)

// POST /users/create
func (s *Server) handleUsersCreate(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"message": "Method Not Allowed"})
		return
	}
	var req struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Username == "" || req.Password == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"message": "username and password required"})
		return
	}

	s.state.mu.Lock()
	defer s.state.mu.Unlock()

	if _, exists := s.state.Users[req.Username]; exists {
		s.logf("register: %q already taken", req.Username)
		writeJSON(w, http.StatusPaymentRequired,
			map[string]string{"message": "Username is already registered."})
		return
	}
	s.state.Users[req.Username] = User{Username: req.Username, Password: req.Password}
	s.saveStateUnlocked()
	s.logf("register: created %q", req.Username)
	writeJSON(w, http.StatusCreated, map[string]string{"username": req.Username})
}

// GET /users/auth
func (s *Server) handleUsersAuth(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"message": "Method Not Allowed"})
		return
	}
	u, k := r.Header.Get("x-auth-user"), r.Header.Get("x-auth-key")
	if !s.authenticate(u, k) {
		s.logf("auth: fail for %q", u)
		writeJSON(w, http.StatusUnauthorized, map[string]string{"message": "Unauthorized"})
		return
	}
	s.logf("auth: ok for %q", u)
	writeJSON(w, http.StatusOK, map[string]string{"authorized": "OK"})
}

// PUT /syncs/progress — push progress
func (s *Server) handleSyncsPush(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPut {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"message": "Method Not Allowed"})
		return
	}
	u, k := r.Header.Get("x-auth-user"), r.Header.Get("x-auth-key")
	if !s.authenticate(u, k) {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"message": "Unauthorized"})
		return
	}

	var req ProgressEntry
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Document == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"message": "document field required"})
		return
	}
	if !docKeyRe.MatchString(req.Document) {
		writeJSON(w, http.StatusBadRequest,
			map[string]string{"message": "document must be 64-char hex SHA256"})
		return
	}
	if req.Percentage < 0 || req.Percentage > 1.0 {
		writeJSON(w, http.StatusBadRequest,
			map[string]string{"message": "percentage must be 0.0–1.0"})
		return
	}

	stateKey := u + ":" + req.Document
	now := time.Now().Unix()

	s.state.mu.Lock()
	defer s.state.mu.Unlock()

	if existing, exists := s.state.Progress[stateKey]; exists && existing.Timestamp >= now {
		s.logf("push: stale %s/%s (ts=%d)", sanitizeLog(u), req.Document[:8], existing.Timestamp)
		writeJSON(w, http.StatusPreconditionFailed, map[string]interface{}{
			"message":   "Document update is not newer.",
			"document":  req.Document,
			"timestamp": existing.Timestamp,
		})
		return
	}

	req.Timestamp = now
	s.state.Progress[stateKey] = req
	s.saveStateUnlocked()
	s.logf("push: %s/%s pct=%.2f device=%s", sanitizeLog(u), req.Document[:8], req.Percentage, sanitizeLog(req.Device))
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"document":  req.Document,
		"timestamp": now,
	})
}

// GET /syncs/progress/:document — pull progress
func (s *Server) handleSyncsPull(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"message": "Method Not Allowed"})
		return
	}
	u, k := r.Header.Get("x-auth-user"), r.Header.Get("x-auth-key")
	if !s.authenticate(u, k) {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"message": "Unauthorized"})
		return
	}

	docHash := strings.TrimPrefix(r.URL.Path, "/syncs/progress/")
	if docHash == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"message": "document hash required"})
		return
	}

	stateKey := u + ":" + docHash

	s.state.mu.RLock()
	entry, exists := s.state.Progress[stateKey]
	s.state.mu.RUnlock()

	if !exists {
		s.logf("pull: %s/%s not found", sanitizeLog(u), first8(docHash))
		writeJSON(w, http.StatusOK, map[string]interface{}{})
		return
	}
	s.logf("pull: %s/%s pct=%.2f", sanitizeLog(u), first8(docHash), entry.Percentage)
	writeJSON(w, http.StatusOK, entry)
}

// ---- Helpers ----

func (s *Server) authenticate(username, key string) bool {
	if username == "" || key == "" {
		return false
	}
	s.state.mu.RLock()
	user, ok := s.state.Users[username]
	s.state.mu.RUnlock()
	return ok && user.Password == key
}

func (s *Server) logf(format string, args ...interface{}) {
	if s.verbose {
		log.Printf("[kosync] "+format, args...)
	}
}

// logSanitizer replaces newline and carriage-return characters with their
// escape sequences so user-controlled strings cannot inject forged log lines.
var logSanitizer = strings.NewReplacer("\n", `\n`, "\r", `\r`)

// sanitizeLog applies logSanitizer to s.
func sanitizeLog(s string) string {
	return logSanitizer.Replace(s)
}

func writeJSON(w http.ResponseWriter, status int, body interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(body); err != nil {
		log.Printf("warn: encode response: %v", err)
	}
}

func first8(s string) string {
	if len(s) > 8 {
		return s[:8]
	}
	return s
}
