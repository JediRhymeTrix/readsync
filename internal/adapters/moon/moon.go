// internal/adapters/moon/moon.go
//
// Moon+ Reader Pro adapter (Phase 4 — master spec section 11).
//
// Orchestrator that wires together four safety layers:
//   Layer 1 (storage)   : embedded WebDAV server with versioned, immutable
//                         archival of every PUT (internal/adapters/webdav).
//   Layer 2 (capture)   : opt-in fixture recorder (capture.go).
//   Layer 3 (parser)    : read-only progress extractor restricted to
//                         fixture-verified formats (parser.go).
//   Layer 4 (writeback) : safe writeback gated on a verified writer
//                         fixture matrix and round-trip self-test
//                         (writeback.go).

package moon

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"

	"github.com/readsync/readsync/internal/adapters/webdav"
	"github.com/readsync/readsync/internal/core"
	"github.com/readsync/readsync/internal/logging"
	"github.com/readsync/readsync/internal/model"
	"github.com/readsync/readsync/internal/secrets"
)

// Config configures the Moon+ adapter.
type Config struct {
	WebDAV         webdav.Config
	CaptureDir     string
	AllowWriteback bool
}

// Defaults returns sensible defaults.  WebDAV.DataDir must be set.
func Defaults() Config { return Config{WebDAV: webdav.Defaults()} }

// Adapter implements adapters.EventEmitter and adapters.WriteTarget.
type Adapter struct {
	cfg     Config
	db      *sql.DB
	log     *logging.Logger
	secrets secrets.Store

	webdav   *webdav.Server
	pipeline *core.Pipeline

	mu               sync.RWMutex
	health           model.AdapterHealthState
	lastDegradedHint string
	lastUploadAt     atomic.Int64

	capture captureMode
}

// New creates a Moon+ adapter.
func New(cfg Config, db *sql.DB, log *logging.Logger, sec secrets.Store) (*Adapter, error) {
	if cfg.WebDAV.DataDir == "" {
		return nil, errors.New("moon: WebDAV.DataDir is required")
	}
	srv, err := webdav.New(cfg.WebDAV, db, log)
	if err != nil {
		return nil, fmt.Errorf("moon: webdav.New: %w", err)
	}
	a := &Adapter{
		cfg: cfg, db: db, log: log, secrets: sec,
		webdav: srv, health: model.HealthDisabled,
	}
	srv.AddUploadObserver(a.onUpload)
	return a, nil
}

func (a *Adapter) Source() model.Source { return model.SourceMoon }

func (a *Adapter) Health() model.AdapterHealthState {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.health
}

// HealthHint returns the most recent degraded-state hint (empty if healthy).
func (a *Adapter) HealthHint() string {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.lastDegradedHint
}

// SetPipeline wires the core event pipeline (EventEmitter contract).
func (a *Adapter) SetPipeline(p *core.Pipeline) { a.pipeline = p }

// WebDAVServer exposes the embedded server for setup wizard / external
// HTTP mounting.
func (a *Adapter) WebDAVServer() *webdav.Server { return a.webdav }

// Start launches the embedded WebDAV server in a goroutine.
func (a *Adapter) Start(ctx context.Context) error {
	if a.pipeline == nil {
		return errors.New("moon: pipeline not set")
	}
	if a.cfg.CaptureDir != "" {
		if err := a.EnableCapture(a.cfg.CaptureDir); err != nil {
			return fmt.Errorf("moon: enable capture: %w", err)
		}
	}
	a.setHealth(model.HealthOK, "")
	go func() {
		if err := a.webdav.Start(ctx); err != nil {
			a.setHealth(model.HealthFailed, err.Error())
			if a.log != nil {
				a.log.Error("moon: webdav serve failed",
					logging.F("error", err.Error()))
			}
		}
	}()
	return nil
}

// Stop is a no-op (the embedded webdav server stops when its context is cancelled).
func (a *Adapter) Stop() error { return nil }

// WriteProgress honours the Layer-4 invariant: explicit error when
// writeback is not verified.
func (a *Adapter) WriteProgress(_ context.Context, _ *model.OutboxJob) error {
	if !a.cfg.AllowWriteback {
		return fmt.Errorf("moon: writeback disabled (no verified writer fixture)")
	}
	if !IsWriterVerified(FormatV1Plain) {
		return fmt.Errorf("moon: writeback unsafe — verified=false in writer registry")
	}
	return errors.New("moon: writeback path not implemented in this build")
}

func (a *Adapter) setHealth(s model.AdapterHealthState, hint string) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.health = s
	a.lastDegradedHint = hint
}
