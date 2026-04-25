// internal/api/server.go
//
// Local admin HTTP API, bound to 127.0.0.1 only.
// CSRF token required on all state-mutating endpoints.

package api

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"time"
)

const (
	defaultPort = 7201
	csrfHeader  = "X-ReadSync-CSRF"
)

// Server is the local admin HTTP server.
type Server struct {
	handler    *http.ServeMux
	httpServer *http.Server
	csrfToken  string
	port       int
}

// Deps holds the dependencies injected into the server.
type Deps struct {
	DB          interface{} // *sql.DB, used by handlers
	Diagnostics interface{} // *diagnostics.Collector
	Port        int
}

// New creates a new admin API server.
func New(deps Deps) (*Server, error) {
	tok, err := generateCSRFToken()
	if err != nil {
		return nil, fmt.Errorf("api: generate csrf token: %w", err)
	}
	port := deps.Port
	if port == 0 {
		port = defaultPort
	}
	s := &Server{
		handler:   http.NewServeMux(),
		csrfToken: tok,
		port:      port,
	}
	s.registerRoutes(deps)
	return s, nil
}

// CSRFToken returns the token callers must send in X-ReadSync-CSRF for writes.
func (s *Server) CSRFToken() string { return s.csrfToken }

// Start begins listening on 127.0.0.1:<port>.
func (s *Server) Start(ctx context.Context) error {
	addr := fmt.Sprintf("127.0.0.1:%d", s.port)
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		return fmt.Errorf("api.Start: %w", err)
	}
	s.httpServer = &http.Server{
		Handler:      s.handler,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 10 * time.Second,
		IdleTimeout:  60 * time.Second,
	}
	go func() {
		_ = s.httpServer.Serve(ln)
	}()
	go func() {
		<-ctx.Done()
		shutCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = s.httpServer.Shutdown(shutCtx)
	}()
	return nil
}

// Stop shuts down the server gracefully.
func (s *Server) Stop() error {
	if s.httpServer == nil {
		return nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	return s.httpServer.Shutdown(ctx)
}

func (s *Server) registerRoutes(deps Deps) {
	s.handler.HandleFunc("/healthz", s.handleHealthz)
	s.handler.HandleFunc("/status", s.handleStatus)
	s.handler.HandleFunc("/adapters", s.handleAdapters)
	s.handler.HandleFunc("/conflicts", s.handleConflicts)
	s.handler.HandleFunc("/outbox", s.handleOutbox)
	s.handler.HandleFunc("/events", s.handleEvents)
}

// csrfMiddleware checks the CSRF token on non-GET requests.
func (s *Server) csrfMiddleware(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet && r.Method != http.MethodHead {
			tok := r.Header.Get(csrfHeader)
			if tok != s.csrfToken {
				http.Error(w, `{"error":"invalid csrf token"}`, http.StatusForbidden)
				return
			}
		}
		next(w, r)
	}
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func (s *Server) handleHealthz(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok", "ts": time.Now().UTC().Format(time.RFC3339)})
}

func (s *Server) handleStatus(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{
		"service": "readsync",
		"status":  "running",
		"ts":      time.Now().UTC().Format(time.RFC3339),
	})
}

func (s *Server) handleAdapters(w http.ResponseWriter, r *http.Request) {
	// TODO Phase 2: query adapter_health table.
	writeJSON(w, http.StatusOK, map[string]any{"adapters": []any{}})
}

func (s *Server) handleConflicts(w http.ResponseWriter, r *http.Request) {
	// TODO Phase 2: query conflicts table.
	writeJSON(w, http.StatusOK, map[string]any{"conflicts": []any{}})
}

func (s *Server) handleOutbox(w http.ResponseWriter, r *http.Request) {
	// TODO Phase 2: query sync_outbox table.
	writeJSON(w, http.StatusOK, map[string]any{"outbox": []any{}})
}

func (s *Server) handleEvents(w http.ResponseWriter, r *http.Request) {
	// TODO Phase 2: query progress_events table.
	writeJSON(w, http.StatusOK, map[string]any{"events": []any{}})
}

func generateCSRFToken() (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}
