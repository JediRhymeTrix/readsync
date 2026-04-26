// internal/adapters/goodreads_bridge/goodreads.go
//
// Goodreads Bridge adapter — Phase 5.
//
// IMPORTANT design constraints (master spec sections 8 & 9):
//
//   - We do NOT call the Goodreads API.
//   - We do NOT scrape Goodreads.
//   - We do NOT vendor any code from the GPL-3.0 Goodreads Sync plugin.
//
// Instead, we treat the Calibre Goodreads Sync plugin as an OPTIONAL
// companion that the user runs manually inside Calibre. ReadSync only
// reads/writes Calibre custom columns (the plugin does that too,
// independently). This file orchestrates plugin detection, surfaces
// configuration to the setup wizard, and provides safety gates for
// Goodreads-derived (low-confidence) progress events.

package goodreads_bridge

import (
	"context"
	"fmt"
	"sync"

	"github.com/readsync/readsync/internal/adapters"
	"github.com/readsync/readsync/internal/core"
	"github.com/readsync/readsync/internal/logging"
	"github.com/readsync/readsync/internal/model"
)

// Compile-time interface compliance assertions.
var (
	_ adapters.Adapter      = (*Adapter)(nil)
	_ adapters.EventEmitter = (*Adapter)(nil)
	_ adapters.WriteTarget  = (*Adapter)(nil)
)

// Config holds runtime configuration for the Goodreads bridge adapter.
type Config struct {
	// Mode is the bridge operating mode. See modes.go.
	Mode BridgeMode

	// PluginsDir is the path to %APPDATA%\calibre\plugins (the directory
	// that contains the plugin .zip files and pluginsCustomization.json).
	// If empty, the adapter discovers it via the standard Windows location.
	PluginsDir string

	// CalibredbPath / LibraryPath are inherited from the Calibre adapter
	// to support reading identifiers and emitting low-confidence events.
	// When empty the adapter operates in passive mode (no event emission).
	CalibredbPath string
	LibraryPath   string

	// ExperimentalDirectAck must be true when Mode == ModeExperimentalDirect.
	// This is a *gate* — without it, the adapter refuses to start in that
	// mode. Direct Goodreads API access is not implemented in v1; the
	// gate exists so that the configuration can be wired up but the actual
	// network call surface remains stubbed and visibly opt-in.
	ExperimentalDirectAck bool
}

// DefaultConfig returns sensible defaults — the bridge starts disabled.
func DefaultConfig() Config { return Config{Mode: ModeDisabled} }

// Adapter is the Goodreads bridge adapter.
type Adapter struct {
	cfg Config
	log *logging.Logger

	mu         sync.Mutex
	health     model.AdapterHealthState
	healthNote string
	pipeline   *core.Pipeline

	// detection holds the most recent plugin detection result.
	detection *Detection
}

// New creates a Goodreads bridge adapter. The adapter does not perform
// any I/O until Start is called.
func New(cfg Config, log *logging.Logger) *Adapter {
	if cfg.Mode == "" {
		cfg.Mode = ModeDisabled
	}
	return &Adapter{cfg: cfg, log: log, health: model.HealthDisabled}
}

// Source identifies this adapter's events.
func (a *Adapter) Source() model.Source { return model.SourceGoodreadsBridge }

// Health returns the current health state.
func (a *Adapter) Health() model.AdapterHealthState {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.health
}

// HealthNote returns a human-readable description of the health state.
func (a *Adapter) HealthNote() string {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.healthNote
}

// SetPipeline implements adapters.EventEmitter; the bridge will push
// low-confidence Goodreads-derived events into the pipeline only after
// the safety gates (confidence ≥ 90, trustworthy timestamp, no recent
// local change, not a stale regression) are satisfied.
func (a *Adapter) SetPipeline(p *core.Pipeline) {
	a.mu.Lock()
	a.pipeline = p
	a.mu.Unlock()
}

// Start performs detection and validates the configured mode. Long-running
// background work happens on demand (via SystemScan / EmitFromCalibre)
// rather than via a polling goroutine; the Calibre adapter already polls
// metadata.db, and the bridge piggy-backs off that poll output.
func (a *Adapter) Start(ctx context.Context) error {
	a.mu.Lock()
	mode := a.cfg.Mode
	a.mu.Unlock()

	// Validate experimental mode gate (spec §8 safety).
	if mode == ModeExperimentalDirect && !a.cfg.ExperimentalDirectAck {
		a.setHealth(model.HealthNeedsUserAction,
			"experimental-direct mode requires explicit user opt-in (ExperimentalDirectAck=true)")
		return fmt.Errorf("goodreads_bridge: experimental-direct mode without ack")
	}

	// In disabled mode we still run detection so the setup wizard can
	// surface accurate state; we just refuse to emit/write.
	det, err := DetectPlugin(a.cfg.PluginsDir)
	if err != nil {
		// Detection failure is non-fatal: surface degraded health.
		a.log.Warn("goodreads_bridge: plugin detection failed", logging.F("err", err))
	}
	a.mu.Lock()
	a.detection = det
	a.mu.Unlock()

	switch mode {
	case ModeDisabled:
		a.setHealth(model.HealthDisabled, "Goodreads bridge disabled")
	case ModeManualPlugin, ModeGuidedPlugin:
		if det == nil || !det.Installed {
			a.setHealth(model.HealthNeedsUserAction,
				"Goodreads Sync plugin not installed in Calibre")
		} else if !det.ProgressColumnConfigured() {
			a.setHealth(model.HealthNeedsUserAction,
				fmt.Sprintf("Goodreads Sync plugin progress column is %q, expected %q",
					det.ProgressColumn, ExpectedProgressColumn))
		} else {
			a.setHealth(model.HealthOK, "")
		}
	case ModeCompanionPlugin:
		// v2 RPC — health stays disabled (hooks only).
		a.setHealth(model.HealthDisabled, "companion-plugin mode: v2 hook (not yet implemented)")
	case ModeExperimentalDirect:
		a.setHealth(model.HealthDegraded,
			"experimental-direct mode is a stub; no network calls performed")
	default:
		return fmt.Errorf("goodreads_bridge: unknown mode %q", mode)
	}

	a.log.Info("goodreads_bridge started",
		logging.F("mode", string(mode)),
		logging.F("plugin_installed", det != nil && det.Installed),
	)
	_ = ctx
	return nil
}

// Stop is a no-op for the bridge: it owns no goroutines.
func (a *Adapter) Stop() error { return nil }


// WriteProgress is the WriteTarget implementation for outbox jobs whose
// target is SourceGoodreadsBridge. In v1, ReadSync does NOT write to
// Goodreads directly — the canonical progress is written into Calibre's
// #readsync_progress column by the Calibre adapter, and the user runs
// the Goodreads Sync plugin manually.
//
// In all v1 modes this method:
//   - In manual-plugin/guided-plugin: emits a friendly "skipped" log line;
//     the caller relies on the Calibre adapter's writeback to land the
//     progress in #readsync_progress.
//   - In disabled: errors out so the outbox marks the job blocked.
//   - In experimental-direct: refuses (stub only) with a clear error.
func (a *Adapter) WriteProgress(ctx context.Context, job *model.OutboxJob) error {
	a.mu.Lock()
	mode := a.cfg.Mode
	a.mu.Unlock()
	_ = ctx

	switch mode {
	case ModeDisabled:
		return fmt.Errorf("goodreads_bridge: disabled (configure mode to enable)")
	case ModeManualPlugin:
		a.log.Info("Goodreads bridge skipped: manual mode",
			logging.F("book_id", job.BookID),
			logging.F("hint", "open Calibre and run Goodreads Sync ➜ Upload reading progress"),
		)
		return nil
	case ModeGuidedPlugin:
		a.log.Info("Goodreads bridge skipped: guided mode (checklist will be shown to user)",
			logging.F("book_id", job.BookID),
		)
		// Guided mode reuses the manual data path (writes #readsync_progress
		// via the Calibre adapter); the UI layer is responsible for the
		// checklist + opening Calibre. The bridge itself does not perform
		// any extra write.
		return nil
	case ModeCompanionPlugin:
		// v2 RPC hook: defer to a future companion plugin.
		return fmt.Errorf("goodreads_bridge: companion-plugin RPC not yet implemented (v2)")
	case ModeExperimentalDirect:
		return fmt.Errorf("goodreads_bridge: experimental-direct write is a stub; not implemented")
	default:
		return fmt.Errorf("goodreads_bridge: unknown mode %q", mode)
	}
}

// Detection returns the most recent plugin detection result (may be nil
// before Start has run).
func (a *Adapter) Detection() *Detection {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.detection
}

// Mode returns the configured bridge mode.
func (a *Adapter) Mode() BridgeMode {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.cfg.Mode
}

// setHealth updates the adapter's health state under the lock.
func (a *Adapter) setHealth(state model.AdapterHealthState, note string) {
	a.mu.Lock()
	a.health = state
	a.healthNote = note
	a.mu.Unlock()
}
