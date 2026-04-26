// internal/adapters/koreader/users.go
//
// Database helpers for koreader_users and koreader_devices tables.

package koreader

import (
	"database/sql"
	"errors"
	"strings"
	"time"
)

// createUser inserts a new user, returning ErrUserExists if the username is
// already taken.  The md5Key is the value sent by KOReader; we immediately
// re-hash it with bcrypt before storage.
func (a *Adapter) createUser(username, md5Key string) error {
	hash, err := hashMD5Key(md5Key)
	if err != nil {
		return err
	}
	now := time.Now().UTC().Format(time.RFC3339Nano)
	_, err = a.db.Exec(
		`INSERT INTO koreader_users(username, password_hash, created_at, updated_at) VALUES (?,?,?,?)`,
		username, hash, now, now,
	)
	if err != nil {
		if isUniqueConstraint(err) {
			return ErrUserExists
		}
		return err
	}
	return nil
}

// upsertDevice records (or refreshes) a device association for a user.
func (a *Adapter) upsertDevice(userID int64, deviceID, deviceName string) error {
	now := time.Now().UTC().Format(time.RFC3339Nano)
	_, err := a.db.Exec(`
		INSERT INTO koreader_devices(user_id, device_id, device_name, last_seen)
		VALUES (?,?,?,?)
		ON CONFLICT(user_id, device_id) DO UPDATE SET
			device_name = excluded.device_name,
			last_seen   = excluded.last_seen
	`, userID, deviceID, deviceName, now)
	return err
}

// lookupDevice returns the device_name for a given user and device_id,
// or empty string if not found.
func (a *Adapter) lookupDevice(userID int64, deviceID string) string {
	var name string
	err := a.db.QueryRow(
		`SELECT device_name FROM koreader_devices WHERE user_id=? AND device_id=?`,
		userID, deviceID,
	).Scan(&name)
	if errors.Is(err, sql.ErrNoRows) {
		return ""
	}
	return name
}

// ErrUserExists is returned when a registration uses an already-taken username.
var ErrUserExists = errors.New("username already registered")

// isUniqueConstraint returns true for SQLite UNIQUE constraint violations.
func isUniqueConstraint(err error) bool {
	if err == nil {
		return false
	}
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "unique constraint") ||
		strings.Contains(msg, "unique violation") ||
		strings.Contains(msg, "uq_") // some drivers prefix column
}
