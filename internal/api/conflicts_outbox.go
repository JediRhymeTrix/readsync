// internal/api/conflicts_outbox.go
//
// Conflict resolution and outbox retry/drop handlers.

package api

import (
	"context"
	"net/http"
	"time"
)

func (s *Server) resolveConflict(w http.ResponseWriter, r *http.Request, id int64, winner string) {
	if s.deps.DB == nil {
		writeJSON(w, http.StatusOK, map[string]any{"ok": false, "message": "no database"})
		return
	}
	if winner != "a" && winner != "b" {
		http.Error(w, "winner must be a or b", http.StatusBadRequest)
		return
	}
	col := "event_a_id"
	if winner == "b" {
		col = "event_b_id"
	}
	q := `UPDATE conflicts
		    SET winner_event_id = ` + col + `,
		        status='auto_resolved',
		        resolved_at = ?,
		        resolved_by = 'user'
		  WHERE id = ?`
	if _, err := s.deps.DB.ExecContext(r.Context(), q,
		time.Now().UTC().Format(time.RFC3339), id); err != nil {
		writeJSON(w, http.StatusInternalServerError,
			map[string]any{"ok": false, "message": err.Error()})
		return
	}
	// Unblock any outbox jobs that were waiting on this conflict.
	_, _ = s.deps.DB.ExecContext(r.Context(),
		`UPDATE sync_outbox SET status='queued', blocking_conflict_id=NULL
		   WHERE blocking_conflict_id = ?`, id)
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "winner": winner})
}

func (s *Server) dismissConflict(w http.ResponseWriter, r *http.Request, id int64) {
	if s.deps.DB == nil {
		writeJSON(w, http.StatusOK, map[string]any{"ok": false, "message": "no database"})
		return
	}
	if _, err := s.deps.DB.ExecContext(r.Context(),
		`UPDATE conflicts SET status='dismissed', resolved_at=?, resolved_by='user'
		   WHERE id=?`,
		time.Now().UTC().Format(time.RFC3339), id); err != nil {
		writeJSON(w, http.StatusInternalServerError,
			map[string]any{"ok": false, "message": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

func (s *Server) retryOutbox(w http.ResponseWriter, r *http.Request, id int64) {
	if s.deps.DB == nil {
		writeJSON(w, http.StatusOK, map[string]any{"ok": false, "message": "no database"})
		return
	}
	q := `UPDATE sync_outbox
		    SET status='queued', attempts=0, next_retry_at=NULL,
		        last_error=NULL, updated_at=?
		  WHERE id=?`
	res, err := s.deps.DB.ExecContext(r.Context(), q,
		time.Now().UTC().Format(time.RFC3339), id)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError,
			map[string]any{"ok": false, "message": err.Error()})
		return
	}
	n, _ := res.RowsAffected()
	writeJSON(w, http.StatusOK,
		map[string]any{"ok": n > 0, "rows": n})
}

func (s *Server) dropOutbox(w http.ResponseWriter, r *http.Request, id int64) {
	if s.deps.DB == nil {
		writeJSON(w, http.StatusOK, map[string]any{"ok": false, "message": "no database"})
		return
	}
	res, err := s.deps.DB.ExecContext(r.Context(),
		`DELETE FROM sync_outbox WHERE id=?`, id)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError,
			map[string]any{"ok": false, "message": err.Error()})
		return
	}
	n, _ := res.RowsAffected()
	writeJSON(w, http.StatusOK, map[string]any{"ok": n > 0, "rows": n})
}

// dbHandle is a minimal shim so handler files compile against either a
// real *sql.DB or a test fake that implements ExecContext.
type dbHandle interface {
	ExecContext(ctx context.Context, q string, args ...any) (any, error)
}

var _ = dbHandle(nil) // keep the linter happy when no test fake registered
