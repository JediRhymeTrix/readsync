// internal/api/server.go
//
// Local admin HTTP server. Always binds to 127.0.0.1; exposes both a
// JSON API and a server-rendered HTML UI. Optional self-signed TLS.
//
// Security invariants (master spec section 16):
//   - Listener address is 127.0.0.1 only.
//   - Every state-mutating endpoint requires a valid X-ReadSync-CSRF
//     header. The token is generated per-server-start and exposed in
//     each HTML page as <meta name="csrf-token">.

package api

import (
	"context"
	"crypto/rand"
	"crypto/tls"
	"database/sql"
	"embed"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"html/template"
	"io/fs"
	"net"
	"net/http"
	"sync"
	"time"

	"github.com/readsync/readsync/internal/setup"
)

const (
	// DefaultPort is the canonical admin UI port.
	DefaultPort = 7201

	// CSRFHeader is the HTTP header carrying the CSRF token.
	CSRFHeader = "X-ReadSync-CSRF"
)

//go:embed templates/*.html
var templatesFS embed.FS

//go:embed static/*
var staticFS embed.FS

// Deps holds all the dependencies the API server needs.
type Deps struct {
	DB          *sql.DB
	Wizard      *setup.Wizard
	Diagnostics DiagnosticsCollector
	Version     string
	Port        int
	BindAddr    string // overrides 127.0.0.1; tests use ":0" or 127.0.0.1
	TLSCert     *tls.Certificate
}

// DiagnosticsCollector is the contract between the API and the diagnostics
// package. Defined as an interface so server tests need not import that pkg.
type DiagnosticsCollector interface {
	Collect(ctx context.Context) (any, error)
}

// Server is the local admin HTTP server.
type Server struct {
	deps      Deps
	mu        sync.Mutex
	csrfToken string
	mux       *http.ServeMux
	httpSrv   *http.Server
	listener  net.Listener
	tmpl      *template.Template

	activityProvider func() []ActivityEntry
	healthProvider   func() []AdapterChip
}

// New creates a fully wired admin server.
func New(deps Deps) (*Server, error) {
	tok, err := generateCSRFToken()
	if err != nil {
		return nil, fmt.Errorf("api: generate csrf: %w", err)
	}
	if deps.Port == 0 {
		deps.Port = DefaultPort
	}
	if deps.BindAddr == "" {
		deps.BindAddr = "127.0.0.1"
	}
	if deps.Version == "" {
		deps.Version = "0.6.0-phase6"
	}
	tmpl, err := loadTemplates()
	if err != nil {
		return nil, fmt.Errorf("api: load templates: %w", err)
	}
	s := &Server{
		deps:      deps,
		csrfToken: tok,
		mux:       http.NewServeMux(),
		tmpl:      tmpl,
	}
	s.registerRoutes()
	return s, nil
}

func loadTemplates() (*template.Template, error) {
	t := template.New("readsync")
	entries, err := fs.ReadDir(templatesFS, "templates")
	if err != nil {
		return nil, err
	}
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		data, err := templatesFS.ReadFile("templates/" + e.Name())
		if err != nil {
			return nil, err
		}
		if _, err := t.New(e.Name()).Parse(string(data)); err != nil {
			return nil, fmt.Errorf("parse %s: %w", e.Name(), err)
		}
	}
	return t, nil
}

// CSRFToken returns the current per-session token.
func (s *Server) CSRFToken() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.csrfToken
}

// SetActivityProvider registers a callback returning recent activity log entries.
func (s *Server) SetActivityProvider(fn func() []ActivityEntry) {
	s.activityProvider = fn
}

// SetHealthProvider registers a callback returning adapter freshness chips.
func (s *Server) SetHealthProvider(fn func() []AdapterChip) {
	s.healthProvider = fn
}

// Addr returns the actual listening address (useful in tests).
func (s *Server) Addr() string {
	if s.listener == nil {
		return ""
	}
	return s.listener.Addr().String()
}

// Handler exposes the underlying mux for integration tests.
func (s *Server) Handler() http.Handler { return s.mux }

// Start begins listening. Non-blocking: serves in a goroutine.
func (s *Server) Start(ctx context.Context) error {
	addr := fmt.Sprintf("%s:%d", s.deps.BindAddr, s.deps.Port)
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		return fmt.Errorf("api.Start: %w", err)
	}
	s.listener = ln
	s.httpSrv = &http.Server{
		Handler:      s.mux,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 30 * time.Second,
		IdleTimeout:  60 * time.Second,
	}
	go func() {
		if s.deps.TLSCert != nil {
			s.httpSrv.TLSConfig = &tls.Config{Certificates: []tls.Certificate{*s.deps.TLSCert}}
			_ = s.httpSrv.ServeTLS(ln, "", "")
			return
		}
		_ = s.httpSrv.Serve(ln)
	}()
	go func() {
		<-ctx.Done()
		shutCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = s.httpSrv.Shutdown(shutCtx)
	}()
	return nil
}

// Stop shuts down the server gracefully.
func (s *Server) Stop() error {
	if s.httpSrv == nil {
		return nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	return s.httpSrv.Shutdown(ctx)
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func generateCSRFToken() (string, error) {
	b := make([]byte, 24)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}
