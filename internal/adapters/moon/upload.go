// internal/adapters/moon/upload.go
//
// Upload handler glue: invoked by the embedded WebDAV server whenever a
// PUT is successfully archived.  Drives Layer 2 (capture) and Layer 3
// (parse + emit) without ever blocking the WebDAV response path.

package moon

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/readsync/readsync/internal/adapters/webdav"
	"github.com/readsync/readsync/internal/logging"
	"github.com/readsync/readsync/internal/model"
)

// onUpload is registered as a webdav.UploadObserver: every successful PUT
// triggers Layer 2 (capture) and Layer 3 (parse + emit).
func (a *Adapter) onUpload(ctx context.Context, ev webdav.UploadEvent) {
	a.lastUploadAt.Store(time.Now().UnixNano())
	a.captureUpload(ctx, ev) // Layer 2

	body, err := readArchive(ev.ArchivePath)
	if err != nil {
		if a.log != nil {
			a.log.Warn("moon: read archive", logging.F("error", err.Error()))
		}
		return
	}
	filename := filepath.Base(ev.RelPath)
	res, perr := Parse(filename, body, ev.ReceivedAt)
	if perr != nil {
		// Unknown format: keep raw bytes (already on disk), mark degraded
		// with a clear hint, never fail the WebDAV response.
		a.markUnknownFormat(filename, perr)
		_, _ = a.db.ExecContext(ctx,
			`UPDATE moon_uploads SET parse_error=? WHERE archive_path=?`,
			perr.Error(), ev.ArchivePath)
		return
	}

	pev := res.ToAdapterEvent()
	if a.pipeline != nil {
		subCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
		defer cancel()
		if err := a.pipeline.Submit(subCtx, pev); err != nil && a.log != nil {
			a.log.Warn("moon: pipeline submit",
				logging.F("error", err.Error()))
		}
	}
	_, _ = a.db.ExecContext(ctx,
		`UPDATE moon_uploads SET parsed=1 WHERE archive_path=?`,
		ev.ArchivePath)
	a.setHealth(model.HealthOK, "")
}

func (a *Adapter) markUnknownFormat(filename string, perr error) {
	hint := fmt.Sprintf(
		"New Moon+ format observed: %s. Raw bytes are safely stored. "+
			"To enable parsing, capture a fixture (Settings → Diagnostics → "+
			"Capture Moon+ uploads) and add it under fixtures/moonplus/.",
		filename)
	a.setHealth(model.HealthDegraded, hint)
	if a.log != nil {
		a.log.Warn("moon: unknown upload format",
			logging.F("file", filename),
			logging.F("error", perr.Error()))
	}
}

// readArchive reads the immutable archive file at path.
func readArchive(path string) ([]byte, error) {
	return os.ReadFile(path)
}
