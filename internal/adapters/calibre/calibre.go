// internal/adapters/calibre/calibre.go
//
// Calibre adapter – Phase 2.

package calibre

import (
	"context"
	"fmt"
	"os"
	"sync"
	"time"

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

// Config holds runtime configuration for the Calibre adapter.
type Config struct {
	PollInterval  time.Duration
	WriteDelay    time.Duration
	LibraryPath   string
	CalibredbPath string
}

// DefaultConfig returns sensible defaults.
func DefaultConfig() Config {
	return Config{PollInterval: 60 * time.Second, WriteDelay: 10 * time.Second}
}

// Adapter is the Calibre adapter.
type Adapter struct {
	cfg Config
	log *logging.Logger

	mu            sync.Mutex
	calibredbPath string
	libraryPath   string
	health        model.AdapterHealthState
	healthNote    string
	pipeline      *core.Pipeline
	lastMtime     time.Time

	ticker   *time.Ticker
	stopCh   chan struct{}
	stopOnce sync.Once

	debounceMu sync.Mutex
	debounce   map[string]time.Time

	writeQueueMu sync.Mutex
	writeQueue   []*model.OutboxJob
}

// New creates a Calibre adapter.
func New(cfg Config, log *logging.Logger) *Adapter {
	if cfg.PollInterval == 0 {
		cfg.PollInterval = 60 * time.Second
	}
	if cfg.WriteDelay == 0 {
		cfg.WriteDelay = 10 * time.Second
	}
	return &Adapter{
		cfg: cfg, log: log,
		health:   model.HealthDisabled,
		debounce: make(map[string]time.Time),
		stopCh:   make(chan struct{}),
	}
}

func (a *Adapter) Source() model.Source { return model.SourceCalibre }
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

// SetPipeline wires the pipeline (implements adapters.EventEmitter).
func (a *Adapter) SetPipeline(p *core.Pipeline) {
	a.mu.Lock()
	a.pipeline = p
	a.mu.Unlock()
}
func (a *Adapter) Start(ctx context.Context) error {
	a.mu.Lock()
	if a.cfg.CalibredbPath != "" {
		a.calibredbPath = a.cfg.CalibredbPath
	} else {
		p, err := findCalibredb()
		if err != nil {
			a.health = model.HealthNeedsUserAction
			a.healthNote = fmt.Sprintf("calibredb not found: %v", err)
			a.mu.Unlock()
			return fmt.Errorf("calibre: calibredb not found: %w", err)
		}
		a.calibredbPath = p
	}
	if a.cfg.LibraryPath != "" {
		a.libraryPath = a.cfg.LibraryPath
	} else {
		libs, err := discoverLibraries()
		if err != nil || len(libs) == 0 {
			a.health = model.HealthNeedsUserAction
			a.healthNote = "no Calibre library found; run setup wizard"
			a.mu.Unlock()
			if err != nil {
				return fmt.Errorf("calibre: no library found: %w", err)
			}
			return fmt.Errorf("calibre: no library found")
		}
		a.libraryPath = libs[0]
	}
	a.health = model.HealthOK
	a.healthNote = ""
	a.mu.Unlock()
	a.log.Info("calibre adapter started",
		logging.F("calibredb", a.calibredbPath),
		logging.F("library", a.libraryPath),
	)
	a.ticker = time.NewTicker(a.cfg.PollInterval)
	go a.pollLoop(ctx)
	go a.writeQueueLoop(ctx)
	return nil
}
func (a *Adapter) Stop() error {
	a.stopOnce.Do(func() {
		if a.ticker != nil {
			a.ticker.Stop()
		}
		close(a.stopCh)
	})
	return nil
}
func (a *Adapter) WriteProgress(ctx context.Context, job *model.OutboxJob) error {
	a.mu.Lock()
	cdb, lib := a.calibredbPath, a.libraryPath
	a.mu.Unlock()
	if cdb == "" || lib == "" {
		return fmt.Errorf("calibre: adapter not ready")
	}
	key := fmt.Sprintf("%d", job.BookID)
	a.debounceMu.Lock()
	if time.Since(a.debounce[key]) < a.cfg.WriteDelay {
		a.debounceMu.Unlock()
		return nil
	}
	a.debounce[key] = time.Now()
	a.debounceMu.Unlock()
	if isGUIRunning() {
		a.writeQueueMu.Lock()
		a.writeQueue = append(a.writeQueue, job)
		a.writeQueueMu.Unlock()
		a.log.Info("calibre: GUI running, write queued", logging.F("book_id", job.BookID))
		return nil
	}
	return a.applyWrite(ctx, cdb, lib, job)
}

// CalibredbPath returns the discovered calibredb path (setup UI).
func (a *Adapter) CalibredbPath() string {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.calibredbPath
}

// LibraryPath returns the active library path (setup UI).
func (a *Adapter) LibraryPath() string {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.libraryPath
}

// MissingColumns returns required columns not yet present.
func (a *Adapter) MissingColumns() ([]ColumnDef, error) {
	a.mu.Lock()
	cdb, lib := a.calibredbPath, a.libraryPath
	a.mu.Unlock()
	if cdb == "" || lib == "" {
		return nil, fmt.Errorf("calibre: adapter not ready")
	}
	return missingColumns(cdb, lib)
}

// EnsureColumns creates any missing required #readsync_* columns.
func (a *Adapter) EnsureColumns(ctx context.Context) error {
	a.mu.Lock()
	cdb, lib := a.calibredbPath, a.libraryPath
	a.mu.Unlock()
	if cdb == "" || lib == "" {
		return fmt.Errorf("calibre: adapter not ready")
	}
	if isGUIRunning() {
		return fmt.Errorf("calibre: close Calibre GUI before modifying columns")
	}
	missing, err := missingColumns(cdb, lib)
	if err != nil {
		return fmt.Errorf("calibre: listing columns: %w", err)
	}
	for _, col := range missing {
		if cerr := createColumn(ctx, cdb, lib, col); cerr != nil {
			return fmt.Errorf("calibre: creating column %s: %w", col.Name, cerr)
		}
		a.log.Info("calibre: created column", logging.F("column", col.Name))
	}
	if m2, e2 := missingColumns(cdb, lib); e2 == nil && len(m2) == 0 {
		a.mu.Lock()
		a.health, a.healthNote = model.HealthOK, ""
		a.mu.Unlock()
	}
	return nil
}

// ScanResult is returned by SystemScan for the setup wizard UI.
type ScanResult struct {
	CalibredbPath  string
	Libraries      []string
	MissingColumns []ColumnDef
	GUIRunning     bool
	Health         model.AdapterHealthState
	Note           string
}

// SystemScan discovers Calibre and reports status for the setup wizard.
func SystemScan() ScanResult {
	r := ScanResult{}
	p, err := findCalibredb()
	if err != nil {
		r.Health = model.HealthNeedsUserAction
		r.Note = fmt.Sprintf("calibredb not found: %v", err)
		return r
	}
	r.CalibredbPath = p
	libs, _ := discoverLibraries()
	r.Libraries = libs
	r.GUIRunning = isGUIRunning()
	if len(libs) > 0 {
		if missing, merr := missingColumns(p, libs[0]); merr == nil {
			r.MissingColumns = missing
		}
		if len(r.MissingColumns) > 0 {
			r.Health = model.HealthNeedsUserAction
			r.Note = fmt.Sprintf("%d required column(s) missing", len(r.MissingColumns))
			return r
		}
	}
	r.Health = model.HealthOK
	return r
}

func (a *Adapter) pollLoop(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		case <-a.stopCh:
			return
		case <-a.ticker.C:
			a.doPoll(ctx)
		}
	}
}

func (a *Adapter) doPoll(ctx context.Context) {
	a.mu.Lock()
	lib, cdb, last, pipeline := a.libraryPath, a.calibredbPath, a.lastMtime, a.pipeline
	a.mu.Unlock()
	if lib == "" || cdb == "" {
		return
	}
	dbPath := lib + string(os.PathSeparator) + "metadata.db"
	info, err := os.Stat(dbPath)
	if err != nil {
		a.log.Warn("calibre: stat metadata.db failed", logging.F("err", err))
		a.mu.Lock()
		a.health = model.HealthDegraded
		a.healthNote = fmt.Sprintf("cannot stat metadata.db: %v", err)
		a.mu.Unlock()
		return
	}
	if !info.ModTime().After(last) {
		return
	}
	a.mu.Lock()
	a.lastMtime = info.ModTime()
	a.mu.Unlock()
	a.log.Debug("calibre: metadata.db changed", logging.F("mtime", info.ModTime()))
	events, err := readAllProgress(ctx, cdb, lib)
	if err != nil {
		a.log.Error("calibre: readAllProgress failed", logging.F("err", err))
		return
	}
	if pipeline == nil {
		return
	}
	for _, ev := range events {
		if err2 := pipeline.Submit(ctx, ev); err2 != nil {
			a.log.Warn("calibre: pipeline submit failed",
				logging.F("err", err2), logging.F("title", ev.BookEvidence.Title))
		}
	}
}

func (a *Adapter) writeQueueLoop(ctx context.Context) {
	tick := time.NewTicker(30 * time.Second)
	defer tick.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-a.stopCh:
			return
		case <-tick.C:
			if !isGUIRunning() {
				a.drainWriteQueue(ctx)
			}
		}
	}
}

func (a *Adapter) drainWriteQueue(ctx context.Context) {
	a.writeQueueMu.Lock()
	queue := a.writeQueue
	a.writeQueue = nil
	a.writeQueueMu.Unlock()
	a.mu.Lock()
	cdb, lib := a.calibredbPath, a.libraryPath
	a.mu.Unlock()
	for _, job := range queue {
		if err := a.applyWrite(ctx, cdb, lib, job); err != nil {
			a.log.Error("calibre: queued write failed",
				logging.F("book_id", job.BookID), logging.F("err", err))
			a.writeQueueMu.Lock()
			a.writeQueue = append(a.writeQueue, job)
			a.writeQueueMu.Unlock()
		}
	}
} // drainWriteQueue


}
