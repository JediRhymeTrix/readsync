// internal/adapters/webdav/webdav.go
//
// Embedded WebDAV server (Phase 4 — Moon+ Reader Pro).
//
// Layer 1 of the Moon+ adapter: provides WebDAV storage compatibility for
// Moon+ Pro on Android. Implements PROPFIND / GET / PUT / MKCOL / DELETE /
// MOVE / LOCK with the strict invariant that *every* PUT is versioned: the
// raw uploaded bytes are written immutably to a per-user, per-path archive
// directory and NEVER overwritten or mutated in-place.
//
// The live FileSystem that Moon+ sees is a thin in-memory view backed by
// `webdav.NewMemFS()`; the archive on disk under
//   {DataDir}/raw/{user}/{path}/{version}.bin
// is the source of truth.  Per-user credentials are HTTP Basic.

package webdav

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/readsync/readsync/internal/logging"
	"github.com/readsync/readsync/internal/model"
	"golang.org/x/crypto/bcrypt"
	xwd "golang.org/x/net/webdav"
)

// Config configures the embedded WebDAV server.
type Config struct {
	BindAddr  string // default "0.0.0.0:8765"
	URLPrefix string // default "/moon-webdav/" (trailing slash required)
	DataDir   string // versioned archive root
	Realm     string // HTTP Basic realm
}

// Defaults returns sensible defaults.  DataDir must be set by the caller.
func Defaults() Config {
	return Config{
		BindAddr:  "0.0.0.0:8765",
		URLPrefix: "/moon-webdav/",
		Realm:     "ReadSync Moon+",
	}
}

// Server is the embedded WebDAV HTTP handler.
type Server struct {
	cfg Config
	db  *sql.DB
	log *logging.Logger

	handler *xwd.Handler

	mu     sync.RWMutex
	health model.AdapterHealthState

	obsMu     sync.RWMutex
	observers []UploadObserver
}

// UploadObserver is invoked once per successful PUT, after the bytes are
// safely archived.
type UploadObserver func(ctx context.Context, ev UploadEvent)

// UploadEvent describes a single archived upload version.
type UploadEvent struct {
	User        string
	RelPath     string
	Version     int
	ReceivedAt  time.Time
	SizeBytes   int64
	SHA256      string
	ArchivePath string
}

// New creates a new embedded WebDAV Server.  db must be migrated; logger may
// be nil.
func New(cfg Config, db *sql.DB, log *logging.Logger) (*Server, error) {
	if cfg.DataDir == "" {
		return nil, errors.New("webdav: Config.DataDir is required")
	}
	if !strings.HasSuffix(cfg.URLPrefix, "/") {
		cfg.URLPrefix += "/"
	}
	if cfg.Realm == "" {
		cfg.Realm = "ReadSync Moon+"
	}
	rawDir := filepath.Join(cfg.DataDir, "raw")
	if err := os.MkdirAll(rawDir, 0o755); err != nil {
		return nil, fmt.Errorf("webdav: mkdir %s: %w", rawDir, err)
	}

	s := &Server{cfg: cfg, db: db, log: log, health: model.HealthDisabled}

	fs := &versionedFS{
		inner:  xwd.NewMemFS(),
		server: s,
	}
	s.handler = &xwd.Handler{
		Prefix:     strings.TrimSuffix(cfg.URLPrefix, "/"),
		FileSystem: fs,
		LockSystem: xwd.NewMemLS(),
		Logger: func(r *http.Request, err error) {
			if err == nil || log == nil {
				return
			}
			log.Debug("webdav handler",
				logging.F("method", r.Method),
				logging.F("path", r.URL.Path),
				logging.F("error", err.Error()),
			)
		},
	}
	return s, nil
}

// Source identifies this adapter for the framework.
func (s *Server) Source() model.Source { return model.SourceMoon }

// Health reports the adapter health.
func (s *Server) Health() model.AdapterHealthState {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.health
}

// SetHealth updates the adapter health.
func (s *Server) SetHealth(state model.AdapterHealthState) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.health = state
}

// AddUploadObserver registers a callback fired after every successful PUT.
func (s *Server) AddUploadObserver(o UploadObserver) {
	if o == nil {
		return
	}
	s.obsMu.Lock()
	defer s.obsMu.Unlock()
	s.observers = append(s.observers, o)
}

// URLPrefix returns the configured prefix (always trailing slash).
func (s *Server) URLPrefix() string { return s.cfg.URLPrefix }

// BindAddr returns the configured bind address.
func (s *Server) BindAddr() string { return s.cfg.BindAddr }

// DataDir returns the configured data directory.
func (s *Server) DataDir() string { return s.cfg.DataDir }

// ServeHTTP wraps the underlying WebDAV handler with HTTP Basic auth that
// scopes the FileSystem to a per-user namespace.
func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	user, ok := s.authenticate(r)
	if !ok {
		w.Header().Set("WWW-Authenticate",
			fmt.Sprintf(`Basic realm=%q`, s.cfg.Realm))
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}
	if r.Method == http.MethodOptions {
		w.Header().Set("Allow", "OPTIONS, PROPFIND, GET, PUT, DELETE, MKCOL, MOVE, LOCK, UNLOCK")
		w.WriteHeader(http.StatusOK)
		return
	}
	ctx := context.WithValue(r.Context(), userCtxKey{}, user)
	s.handler.ServeHTTP(w, r.WithContext(ctx))
}

// Start binds to BindAddr and serves HTTP until ctx is cancelled.
func (s *Server) Start(ctx context.Context) error {
	srv := &http.Server{
		Addr:         s.cfg.BindAddr,
		Handler:      s,
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 30 * time.Second,
		IdleTimeout:  90 * time.Second,
	}
	go func() {
		<-ctx.Done()
		shCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = srv.Shutdown(shCtx)
	}()
	s.SetHealth(model.HealthOK)
	if s.log != nil {
		s.log.Info("webdav: started",
			logging.F("addr", s.cfg.BindAddr),
			logging.F("prefix", s.cfg.URLPrefix))
	}
	if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		s.SetHealth(model.HealthFailed)
		return err
	}
	return nil
}

// -- Auth ---------------------------------------------------------------------

type userCtxKey struct{}

// userFromCtx returns the authenticated user, or empty string if the request
// was unauthenticated.
func userFromCtx(ctx context.Context) string {
	if v, ok := ctx.Value(userCtxKey{}).(string); ok {
		return v
	}
	return ""
}

// authenticate validates HTTP Basic credentials against moon_users.
func (s *Server) authenticate(r *http.Request) (string, bool) {
	user, pass, ok := r.BasicAuth()
	if !ok || user == "" || pass == "" {
		return "", false
	}
	var hash string
	err := s.db.QueryRow(
		`SELECT password_hash FROM moon_users WHERE username=?`, user,
	).Scan(&hash)
	if err != nil {
		return "", false
	}
	// bcrypt.CompareHashAndPassword performs a constant-time comparison
	// internally, so we do not need an additional subtle.ConstantTimeCompare.
	if bcrypt.CompareHashAndPassword([]byte(hash), []byte(pass)) != nil {
		return "", false
	}
	return user, true
}

// CreateUser registers a new WebDAV user.  Plaintext password is bcrypt-
// hashed before storage.  Returns ErrUserExists if the username is taken.
func (s *Server) CreateUser(username, password string) error {
	if username == "" || password == "" {
		return errors.New("webdav: username and password required")
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(password), 12)
	if err != nil {
		return err
	}
	now := time.Now().UTC().Format(time.RFC3339Nano)
	_, err = s.db.Exec(
		`INSERT INTO moon_users(username,password_hash,created_at,updated_at)
		 VALUES(?,?,?,?)`,
		username, string(hash), now, now,
	)
	if err != nil {
		msg := strings.ToLower(err.Error())
		if strings.Contains(msg, "unique") {
			return ErrUserExists
		}
		return err
	}
	return nil
}

// ErrUserExists is returned by CreateUser when the username collides.
var ErrUserExists = errors.New("webdav: username already exists")
