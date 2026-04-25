// internal/diagnostics/diagnostics.go
//
// Diagnostics: collects system state for export and troubleshooting.

package diagnostics

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"runtime"
	"strings"
	"time"

	"github.com/readsync/readsync/internal/model"
)

// Report is a snapshot of system state.
type Report struct {
	GeneratedAt time.Time              `json:"generated_at"`
	Version     string                 `json:"version"`
	Go          string                 `json:"go"`
	OS          string                 `json:"os"`
	Arch        string                 `json:"arch"`
	AdapterHealth []AdapterHealthEntry `json:"adapter_health"`
	OutboxStats   OutboxStats          `json:"outbox_stats"`
	DBStats       DBStats              `json:"db_stats"`
}

// AdapterHealthEntry captures health state for one adapter.
type AdapterHealthEntry struct {
	Source    string `json:"source"`
	State     string `json:"state"`
	LastError string `json:"last_error,omitempty"`
}

// OutboxStats summarises sync_outbox state.
type OutboxStats struct {
	Queued     int `json:"queued"`
	Running    int `json:"running"`
	Retrying   int `json:"retrying"`
	DeadLetter int `json:"dead_letter"`
	Blocked    int `json:"blocked"`
}

// DBStats captures SQLite statistics.
type DBStats struct {
	OpenConns    int   `json:"open_conns"`
	IdleConns    int   `json:"idle_conns"`
	WALFrames    int64 `json:"wal_frames"`
}

// Collector assembles diagnostic reports.
type Collector struct {
	db      *sql.DB
	health  map[model.Source]model.AdapterHealthState
	version string
}

// New creates a Collector.
func New(db *sql.DB, version string) *Collector {
	return &Collector{db: db, version: version,
		health: make(map[model.Source]model.AdapterHealthState)}
}

// SetAdapterHealth records the health state for an adapter.
func (c *Collector) SetAdapterHealth(source model.Source, state model.AdapterHealthState) {
	c.health[source] = state
}

// Collect assembles a diagnostic report.
func (c *Collector) Collect(ctx context.Context) (*Report, error) {
	r := &Report{
		GeneratedAt: time.Now().UTC(),
		Version:     c.version,
		Go:          runtime.Version(),
		OS:          runtime.GOOS,
		Arch:        runtime.GOARCH,
	}

	// Adapter health from map.
	for src, state := range c.health {
		r.AdapterHealth = append(r.AdapterHealth, AdapterHealthEntry{
			Source: string(src),
			State:  string(state),
		})
	}

	// Outbox stats.
	rows, err := c.db.QueryContext(ctx,
		`SELECT status, COUNT(*) FROM sync_outbox GROUP BY status`)
	if err != nil {
		// Ignore error if the table doesn't exist yet (before first migrate).
		if !isNoTable(err) {
			return nil, fmt.Errorf("diagnostics: outbox query: %w", err)
		}
	}
	if rows != nil {
		defer rows.Close()
		for rows.Next() {
			var status string
			var count int
			if err := rows.Scan(&status, &count); err != nil {
				continue
			}
			switch model.OutboxStatus(status) {
			case model.OutboxQueued:
				r.OutboxStats.Queued = count
			case model.OutboxRunning:
				r.OutboxStats.Running = count
			case model.OutboxRetrying:
				r.OutboxStats.Retrying = count
			case model.OutboxDeadLetter:
				r.OutboxStats.DeadLetter = count
			case model.OutboxBlockedByConflict, model.OutboxBlockedByLowConfidence,
				model.OutboxBlockedByAdapterHealth:
				r.OutboxStats.Blocked += count
			}
		}
	}

	// DB stats.
	s := c.db.Stats()
	r.DBStats = DBStats{
		OpenConns: s.OpenConnections,
		IdleConns: s.Idle,
	}

	return r, nil
}

// Export serialises the report to JSON.
func Export(r *Report) ([]byte, error) {
	return json.MarshalIndent(r, "", "  ")
}

// isNoTable returns true if the error indicates the table does not exist.
// Used to gracefully handle diagnostics queries before migration is run.
func isNoTable(err error) bool {
	if err == nil {
		return false
	}
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "no such table") ||
		strings.Contains(msg, "table does not exist")
}
