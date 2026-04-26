// internal/repair/repair.go
//
// Self-healing helpers: SQLite busy retry, port conflict auto-pick,
// deadletter repair hints.

package repair

import (
	"context"
	"database/sql"
	"fmt"
	"math/rand"
	"net"
	"strings"
	"time"
)

// RetryOnBusy retries fn up to maxRetries times when SQLite returns SQLITE_BUSY.
// It applies exponential backoff with jitter between attempts.
func RetryOnBusy(ctx context.Context, maxRetries int, fn func() error) error {
	var lastErr error
	for i := 0; i <= maxRetries; i++ {
		err := fn()
		if err == nil {
			return nil
		}
		if !isBusy(err) {
			return err
		}
		lastErr = err
		if i >= maxRetries {
			break
		}
		// Exponential backoff: 50ms * 2^i, plus up to 20ms jitter.
		// Use an explicit int shift so go vet does not flag a float shift.
		shift := int64(1) << uint(i)
		delay := time.Duration(int64(50*time.Millisecond)*shift) +
			time.Duration(rand.Int63n(int64(20*time.Millisecond)))
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(delay):
		}
	}
	return fmt.Errorf("repair.RetryOnBusy: max retries exceeded: %w", lastErr)
}

func isBusy(err error) bool {
	if err == nil {
		return false
	}
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "database is locked") ||
		strings.Contains(msg, "sqlite_busy") ||
		strings.Contains(msg, "busy")
}

// PickFreePort finds an available TCP port near preferred.
// If preferred is 0 or taken, it picks a random free port.
func PickFreePort(preferred int) (int, error) {
	if preferred > 0 {
		ln, err := net.Listen("tcp", fmt.Sprintf("127.0.0.1:%d", preferred))
		if err == nil {
			_ = ln.Close()
			return preferred, nil
		}
	}
	// Let the OS assign a port.
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return 0, fmt.Errorf("repair.PickFreePort: %w", err)
	}
	port := ln.Addr().(*net.TCPAddr).Port
	_ = ln.Close()
	return port, nil
}

// DeadLetterHint returns a human-readable troubleshooting hint for a deadletter job.
func DeadLetterHint(lastError string) string {
	lower := strings.ToLower(lastError)
	switch {
	case strings.Contains(lower, "calibredb"):
		return "Check that Calibre is installed and calibredb is in PATH. " +
			"Run: readsyncctl adapters calibre check"
	case strings.Contains(lower, "connection refused") || strings.Contains(lower, "dial"):
		return "Adapter endpoint is unreachable. Check network connectivity and adapter configuration."
	case strings.Contains(lower, "timeout"):
		return "Operation timed out. Check adapter health: readsyncctl adapters"
	case strings.Contains(lower, "not implemented"):
		return "This adapter is not yet implemented (pending Phase 2+)."
	case strings.Contains(lower, "conflict"):
		return "Job blocked by a conflict. Resolve via: readsyncctl conflicts list"
	case strings.Contains(lower, "low_confidence") || strings.Contains(lower, "identity"):
		return "Book identity confidence too low for writeback. " +
			"Improve book identifiers and re-try: readsyncctl outbox retry <id>"
	default:
		return "Check engineering logs for details. Run: readsyncctl diagnostics export"
	}
}

// WALCheckpoint forces a WAL checkpoint on the SQLite database.
func WALCheckpoint(ctx context.Context, db *sql.DB) error {
	_, err := db.ExecContext(ctx, "PRAGMA wal_checkpoint(FULL)")
	return err
}

// Vacuum runs a VACUUM on the database to reclaim space.
func Vacuum(ctx context.Context, db *sql.DB) error {
	_, err := db.ExecContext(ctx, "VACUUM")
	return err
}
