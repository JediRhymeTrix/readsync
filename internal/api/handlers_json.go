// internal/api/handlers_json.go
//
// JSON API handlers (read-only).

package api

import (
	"context"
	"net/http"
	"strconv"
	"time"
)

// AdapterChip is the per-adapter health summary surfaced on the dashboard.
type AdapterChip struct {
	Source    string `json:"source"`
	State     string `json:"state"`
	Freshness string `json:"freshness"`
	LastError string `json:"last_error,omitempty"`
	UpdatedAt string `json:"updated_at,omitempty"`
}

// ActivityEntry is a redacted progress event for the activity log view.
type ActivityEntry struct {
	ReceivedAt string `json:"received_at"`
	Source     string `json:"source"`
	BookID     int64  `json:"book_id"`
	BookTitle  string `json:"book_title"`
	Percent    string `json:"percent"`
	ReadStatus string `json:"read_status"`
}

// ConflictRow models a row of /api/conflicts.
type ConflictRow struct {
	ID         int64  `json:"id"`
	BookID     int64  `json:"book_id"`
	BookTitle  string `json:"book_title"`
	Reason     string `json:"reason"`
	DetectedAt string `json:"detected_at"`
	Status     string `json:"status"`
}

// OutboxRow models a row of /api/outbox.
type OutboxRow struct {
	ID           int64  `json:"id"`
	BookID       int64  `json:"book_id"`
	TargetSource string `json:"target_source"`
	Status       string `json:"status"`
	Attempts     int    `json:"attempts"`
	LastError    string `json:"last_error,omitempty"`
}

// freshnessFor returns the canonical freshness chip per spec section 6.
func freshnessFor(source string) string {
	switch source {
	case "koreader":
		return "live"
	case "moon":
		return "event-driven"
	case "calibre":
		return "near-real-time"
	case "goodreads_bridge":
		return "scheduled"
	case "kindle_via_goodreads":
		return "manual"
	default:
		return "unsupported"
	}
}

func (s *Server) handleAdapters(w http.ResponseWriter, r *http.Request) {
	chips := []AdapterChip{}
	if s.healthProvider != nil {
		chips = s.healthProvider()
	} else if s.deps.DB != nil {
		chips = s.queryAdapterChips(r.Context())
	}
	writeJSON(w, http.StatusOK, map[string]any{"adapters": chips})
}

func (s *Server) queryAdapterChips(ctx context.Context) []AdapterChip {
	var chips []AdapterChip
	rows, err := s.deps.DB.QueryContext(ctx,
		`SELECT source, state, COALESCE(last_error,''), updated_at
		   FROM adapter_health ORDER BY source`)
	if err != nil {
		return chips
	}
	defer rows.Close()
	for rows.Next() {
		var c AdapterChip
		if err := rows.Scan(&c.Source, &c.State, &c.LastError, &c.UpdatedAt); err != nil {
			continue
		}
		c.Freshness = freshnessFor(c.Source)
		chips = append(chips, c)
	}
	return chips
}

func (s *Server) handleConflicts(w http.ResponseWriter, r *http.Request) {
	rows := []ConflictRow{}
	if s.deps.DB != nil {
		rows = s.queryConflicts(r.Context())
	}
	writeJSON(w, http.StatusOK, map[string]any{"conflicts": rows})
}

func (s *Server) queryConflicts(ctx context.Context) []ConflictRow {
	var out []ConflictRow
	q := `SELECT c.id, c.book_id, COALESCE(b.title,''), c.reason, c.detected_at, c.status
		   FROM conflicts c
		   LEFT JOIN books b ON b.id = c.book_id
		   ORDER BY c.detected_at DESC LIMIT 200`
	rows, err := s.deps.DB.QueryContext(ctx, q)
	if err != nil {
		return out
	}
	defer rows.Close()
	for rows.Next() {
		var c ConflictRow
		if err := rows.Scan(&c.ID, &c.BookID, &c.BookTitle, &c.Reason, &c.DetectedAt, &c.Status); err != nil {
			continue
		}
		out = append(out, c)
	}
	return out
}

func (s *Server) handleOutbox(w http.ResponseWriter, r *http.Request) {
	rows := []OutboxRow{}
	if s.deps.DB != nil {
		rows = s.queryOutbox(r.Context())
	}
	writeJSON(w, http.StatusOK, map[string]any{"outbox": rows})
}

func (s *Server) queryOutbox(ctx context.Context) []OutboxRow {
	var out []OutboxRow
	q := `SELECT id, book_id, target_source, status, attempts, COALESCE(last_error,'')
		   FROM sync_outbox ORDER BY updated_at DESC LIMIT 200`
	rows, err := s.deps.DB.QueryContext(ctx, q)
	if err != nil {
		return out
	}
	defer rows.Close()
	for rows.Next() {
		var o OutboxRow
		if err := rows.Scan(&o.ID, &o.BookID, &o.TargetSource, &o.Status, &o.Attempts, &o.LastError); err != nil {
			continue
		}
		out = append(out, o)
	}
	return out
}

func (s *Server) handleEvents(w http.ResponseWriter, r *http.Request) {
	entries := []ActivityEntry{}
	if s.activityProvider != nil {
		entries = s.activityProvider()
	} else if s.deps.DB != nil {
		entries = s.queryActivity(r.Context(), 100)
	}
	writeJSON(w, http.StatusOK, map[string]any{"events": entries})
}

func (s *Server) queryActivity(ctx context.Context, limit int) []ActivityEntry {
	var out []ActivityEntry
	q := `SELECT pe.received_at, pe.source, pe.book_id,
				COALESCE(b.title,''), COALESCE(pe.percent_complete,0),
				pe.read_status
		   FROM progress_events pe
		   LEFT JOIN books b ON b.id = pe.book_id
		   ORDER BY pe.received_at DESC LIMIT ?`
	rows, err := s.deps.DB.QueryContext(ctx, q, limit)
	if err != nil {
		return out
	}
	defer rows.Close()
	for rows.Next() {
		var e ActivityEntry
		var pct float64
		if err := rows.Scan(&e.ReceivedAt, &e.Source, &e.BookID,
			&e.BookTitle, &pct, &e.ReadStatus); err != nil {
			continue
		}
		e.Percent = strconv.FormatFloat(pct*100, 'f', 1, 64) + "%"
		out = append(out, e)
	}
	return out
}

func (s *Server) handleWizardJSON(w http.ResponseWriter, r *http.Request) {
	if s.deps.Wizard == nil {
		writeJSON(w, http.StatusOK, map[string]any{"wizard": nil})
		return
	}
	writeJSON(w, http.StatusOK, s.deps.Wizard.State())
}

func (s *Server) handleDiagnosticsJSON(w http.ResponseWriter, r *http.Request) {
	if s.deps.Diagnostics == nil {
		writeJSON(w, http.StatusOK, map[string]any{
			"version": s.deps.Version,
			"ts":      time.Now().UTC().Format(time.RFC3339),
		})
		return
	}
	rep, err := s.deps.Diagnostics.Collect(r.Context())
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, rep)
}
