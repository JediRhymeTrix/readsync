// internal/adapters/webdav/archive.go
//
// Versioned, immutable archive of every uploaded file.  The invariant is
// that the original PUT body is never overwritten and never mutated in
// place.  Each PUT allocates a fresh integer version under
//   {DataDir}/raw/{user}/{path}/{version}.bin
// with a sibling JSON manifest, and the file is created with O_EXCL so a
// race between two writers cannot clobber an earlier version.

package webdav

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/readsync/readsync/internal/logging"
)

// archiveUpload is invoked by versionedFS after the inner FS has accepted a
// complete write.  It persists the raw bytes immutably under DataDir, writes
// a JSON manifest, records a row in moon_uploads, and fires UploadObservers.
func (s *Server) archiveUpload(ctx context.Context, user, relPath string, body []byte) (UploadEvent, error) {
	if user == "" {
		return UploadEvent{}, errors.New("webdav: archive without user")
	}
	cleanPath := strings.Trim(filepath.ToSlash(relPath), "/")
	if cleanPath == "" {
		return UploadEvent{}, errors.New("webdav: archive empty path")
	}

	var userID int64
	if err := s.db.QueryRowContext(ctx,
		`SELECT id FROM moon_users WHERE username=?`, user).Scan(&userID); err != nil {
		return UploadEvent{}, fmt.Errorf("webdav: user lookup: %w", err)
	}

	var nextVersion int
	if err := s.db.QueryRowContext(ctx, `
		SELECT COALESCE(MAX(version), 0) + 1
		FROM moon_uploads
		WHERE user_id=? AND rel_path=?`, userID, cleanPath).Scan(&nextVersion); err != nil {
		return UploadEvent{}, fmt.Errorf("webdav: version probe: %w", err)
	}

	archiveDir := filepath.Join(s.cfg.DataDir, "raw", safeName(user),
		filepath.FromSlash(safeRelPath(cleanPath)))
	if err := os.MkdirAll(archiveDir, 0o755); err != nil {
		return UploadEvent{}, fmt.Errorf("webdav: mkdir archive: %w", err)
	}

	binPath := filepath.Join(archiveDir, fmt.Sprintf("%d.bin", nextVersion))
	bf, err := os.OpenFile(binPath, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o444)
	if err != nil {
		return UploadEvent{}, fmt.Errorf("webdav: archive create: %w", err)
	}
	if _, err := bf.Write(body); err != nil {
		_ = bf.Close()
		_ = os.Remove(binPath)
		return UploadEvent{}, fmt.Errorf("webdav: archive write: %w", err)
	}
	if err := bf.Close(); err != nil {
		return UploadEvent{}, fmt.Errorf("webdav: archive close: %w", err)
	}

	sum := sha256.Sum256(body)
	hexSum := hex.EncodeToString(sum[:])
	now := time.Now().UTC()
	mfBytes, _ := json.MarshalIndent(map[string]any{
		"user":         user,
		"rel_path":     "/" + cleanPath,
		"version":      nextVersion,
		"received_at":  now.Format(time.RFC3339Nano),
		"size_bytes":   len(body),
		"sha256":       hexSum,
		"content_type": "application/octet-stream",
	}, "", "  ")
	mfPath := filepath.Join(archiveDir, fmt.Sprintf("%d.json", nextVersion))
	if err := os.WriteFile(mfPath, mfBytes, 0o444); err != nil {
		return UploadEvent{}, fmt.Errorf("webdav: manifest: %w", err)
	}

	if _, err := s.db.ExecContext(ctx, `
		INSERT INTO moon_uploads(user_id, rel_path, version, received_at,
		                          size_bytes, sha256, archive_path)
		VALUES(?,?,?,?,?,?,?)
	`, userID, cleanPath, nextVersion, now.Format(time.RFC3339Nano),
		int64(len(body)), hexSum, binPath); err != nil {
		return UploadEvent{}, fmt.Errorf("webdav: upload row: %w", err)
	}

	ev := UploadEvent{
		User: user, RelPath: "/" + cleanPath, Version: nextVersion,
		ReceivedAt: now, SizeBytes: int64(len(body)),
		SHA256: hexSum, ArchivePath: binPath,
	}
	s.fireObservers(ctx, ev)
	if s.log != nil {
		short := hexSum
		if len(short) > 16 {
			short = short[:16] + "…"
		}
		s.log.Info("webdav: archived",
			logging.F("user", user), logging.F("path", ev.RelPath),
			logging.F("version", nextVersion), logging.F("size", ev.SizeBytes),
			logging.F("sha256", short))
	}
	return ev, nil
}

func (s *Server) fireObservers(ctx context.Context, ev UploadEvent) {
	s.obsMu.RLock()
	obs := append([]UploadObserver(nil), s.observers...)
	s.obsMu.RUnlock()
	for _, o := range obs {
		func() {
			defer func() {
				if r := recover(); r != nil && s.log != nil {
					s.log.Error("webdav: observer panic",
						logging.F("recover", fmt.Sprint(r)))
				}
			}()
			o(ctx, ev)
		}()
	}
}

// LatestVersion returns the latest archived version for (user, relPath).
// Returns (UploadEvent{}, false, nil) when no upload has been recorded yet
// for the given path; (_, false, err) for genuine database errors.
func (s *Server) LatestVersion(ctx context.Context, user, relPath string) (UploadEvent, bool, error) {
	cleanPath := strings.Trim(filepath.ToSlash(relPath), "/")
	var userID int64
	if err := s.db.QueryRowContext(ctx,
		`SELECT id FROM moon_users WHERE username=?`, user).Scan(&userID); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return UploadEvent{}, false, nil
		}
		return UploadEvent{}, false, err
	}
	var ev UploadEvent
	var receivedAt string
	err := s.db.QueryRowContext(ctx, `
		SELECT version, received_at, size_bytes, sha256, archive_path
		FROM moon_uploads
		WHERE user_id=? AND rel_path=?
		ORDER BY version DESC
		LIMIT 1
	`, userID, cleanPath).Scan(
		&ev.Version, &receivedAt, &ev.SizeBytes, &ev.SHA256, &ev.ArchivePath)
	if errors.Is(err, sql.ErrNoRows) {
		return UploadEvent{}, false, nil
	}
	if err != nil {
		return UploadEvent{}, false, err
	}
	if t, perr := time.Parse(time.RFC3339Nano, receivedAt); perr == nil {
		ev.ReceivedAt = t
	}
	ev.User = user
	ev.RelPath = "/" + cleanPath
	return ev, true, nil
}

// safeName replaces filesystem-unsafe characters in a single segment.
func safeName(s string) string {
	for _, ch := range []string{"..", "\\", ":", "*", "?", `"`, "<", ">", "|"} {
		s = strings.ReplaceAll(s, ch, "_")
	}
	if s == "" {
		s = "_"
	}
	return s
}

// safeRelPath sanitises every path segment.
func safeRelPath(p string) string {
	parts := strings.Split(p, "/")
	for i, seg := range parts {
		parts[i] = safeName(seg)
	}
	return strings.Join(parts, "/")
}
