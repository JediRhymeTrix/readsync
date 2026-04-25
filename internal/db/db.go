// internal/db/db.go
//
// Database layer: opens SQLite with WAL mode and runs migrations.

package db

import (
	"database/sql"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	_ "github.com/mattn/go-sqlite3"
)

const dsnParams = "?_journal_mode=WAL&_synchronous=NORMAL&_busy_timeout=5000&_foreign_keys=on&_txlock=immediate"

// DB wraps *sql.DB and provides ReadSync-specific helpers.
type DB struct {
	sql *sql.DB
}

// buildDSN constructs a go-sqlite3 DSN from a file path.
// Uses the file: URI scheme so that query parameters work on all platforms,
// including Windows paths containing backslashes.
func buildDSN(path string) string {
	// go-sqlite3 accepts file: URIs with forward-slash separators.
	// On Windows, filepath.ToSlash converts backslashes.
	clean := filepath.ToSlash(path)
	return "file:" + clean + dsnParams
}

// Open opens (or creates) the SQLite file at path and configures it for
// ReadSync: WAL mode, NORMAL sync, 5 s busy timeout, foreign keys enabled.
func Open(path string) (*DB, error) {
	dsn := buildDSN(path)
	sqlDB, err := sql.Open("sqlite3", dsn)
	if err != nil {
		return nil, fmt.Errorf("db.Open: %w", err)
	}
	// Single writer to avoid busy-lock storms.
	sqlDB.SetMaxOpenConns(1)
	sqlDB.SetConnMaxLifetime(0)
	sqlDB.SetConnMaxIdleTime(0)

	db := &DB{sql: sqlDB}
	if err := db.ping(); err != nil {
		_ = sqlDB.Close()
		return nil, err
	}
	return db, nil
}

// Close releases the underlying connection.
func (d *DB) Close() error { return d.sql.Close() }

// SQL returns the raw *sql.DB for callers that need it.
func (d *DB) SQL() *sql.DB { return d.sql }

func (d *DB) ping() error {
	var v int
	return d.sql.QueryRow("SELECT 1").Scan(&v)
}

// Migrate applies all unapplied migrations in order.
// It is safe to call on an already-migrated database (idempotent).
func (d *DB) Migrate() error {
	// Bootstrap: run PRAGMA first, then create migration table.
	// We apply migration 1 in two steps: pragmas first (can't be in a tx),
	// then the DDL in a transaction.
	if err := d.applyPragmas(); err != nil {
		return err
	}

	// Ensure tracking table exists.
	if _, err := d.sql.Exec(`CREATE TABLE IF NOT EXISTS schema_migrations (
		version    INTEGER PRIMARY KEY,
		applied_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ','now'))
	)`); err != nil {
		return fmt.Errorf("db.Migrate bootstrap: %w", err)
	}

	for _, m := range migrations {
		applied, err := d.isApplied(m.version)
		if err != nil {
			return err
		}
		if applied {
			continue
		}
		if err := d.apply(m); err != nil {
			return fmt.Errorf("db.Migrate v%d: %w", m.version, err)
		}
	}
	return nil
}

// applyPragmas runs the connection-level PRAGMAs outside a transaction.
func (d *DB) applyPragmas() error {
	pragmas := []string{
		"PRAGMA journal_mode=WAL",
		"PRAGMA synchronous=NORMAL",
		"PRAGMA busy_timeout=5000",
		"PRAGMA foreign_keys=ON",
	}
	for _, p := range pragmas {
		if _, err := d.sql.Exec(p); err != nil {
			return fmt.Errorf("db pragma %q: %w", p, err)
		}
	}
	return nil
}

func (d *DB) isApplied(version int) (bool, error) {
	var count int
	err := d.sql.QueryRow(
		"SELECT COUNT(*) FROM schema_migrations WHERE version=?", version,
	).Scan(&count)
	if err != nil {
		if strings.Contains(err.Error(), "no such table") {
			return false, nil
		}
		return false, err
	}
	return count > 0, nil
}

func (d *DB) apply(m migration) error {
	tx, err := d.sql.Begin()
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()

	// Strip PRAGMA lines from the DDL body (already applied above).
	ddl := stripPragmas(m.sql)
	if _, err := tx.Exec(ddl); err != nil {
		return fmt.Errorf("exec ddl: %w", err)
	}
	if _, err := tx.Exec(
		"INSERT OR IGNORE INTO schema_migrations(version, applied_at) VALUES (?, ?)",
		m.version, time.Now().UTC().Format(time.RFC3339Nano),
	); err != nil {
		return fmt.Errorf("record migration: %w", err)
	}
	return tx.Commit()
}

// stripPragmas removes PRAGMA lines so they are not re-executed inside a tx.
func stripPragmas(sql string) string {
	lines := strings.Split(sql, "\n")
	var out []string
	for _, l := range lines {
		trimmed := strings.TrimSpace(l)
		if strings.HasPrefix(strings.ToUpper(trimmed), "PRAGMA") {
			continue
		}
		out = append(out, l)
	}
	return strings.Join(out, "\n")
}

// WithTx executes fn inside a serialised write transaction with automatic
// commit or rollback.
func (d *DB) WithTx(fn func(tx *sql.Tx) error) error {
	tx, err := d.sql.Begin()
	if err != nil {
		return err
	}
	if err := fn(tx); err != nil {
		_ = tx.Rollback()
		return err
	}
	return tx.Commit()
}
