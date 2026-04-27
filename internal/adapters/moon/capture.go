// internal/adapters/moon/capture.go
//
// Layer 2 of the Moon+ adapter: in-process fixture recorder.
//
// When capture mode is enabled (via Config.CaptureDir or AdapterAPI),
// every successful WebDAV upload is hard-linked (or copied as a fallback
// on filesystems that do not support hardlinks across volumes) into the
// configured capture directory using a timestamped, slug-safe filename.
//
// This is the same fixture concept produced by the standalone
// tools/moon-fixture-recorder used in Phase 0, but available inside the
// running service so users can flip a switch ("record next 5 minutes")
// when reproducing a parsing issue. Captures are never auto-deleted.

package moon

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"

	"github.com/readsync/readsync/internal/adapters/webdav"
	"github.com/readsync/readsync/internal/logging"
)

// captureMode is a tri-state flag: 0 = off, 1 = on.
type captureMode struct {
	on  atomic.Bool
	dir atomic.Value // string
}

// EnableCapture switches capture mode on, recording uploads into dir.
// dir must be writeable; it is created on demand.
func (a *Adapter) EnableCapture(dir string) error {
	if dir == "" {
		return fmt.Errorf("moon: capture: empty dir")
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("moon: capture: mkdir %s: %w", dir, err)
	}
	a.capture.dir.Store(dir)
	a.capture.on.Store(true)
	if a.log != nil {
		a.log.Info("moon: capture mode enabled", logging.F("dir", dir))
	}
	return nil
}

// DisableCapture turns capture mode off.
func (a *Adapter) DisableCapture() {
	a.capture.on.Store(false)
	if a.log != nil {
		a.log.Info("moon: capture mode disabled")
	}
}

// CaptureEnabled reports the current state.
func (a *Adapter) CaptureEnabled() bool { return a.capture.on.Load() }

// captureUpload is invoked from the webdav UploadObserver path.  It is a
// best-effort copy: failure is logged but never propagated to the WebDAV
// response.
func (a *Adapter) captureUpload(_ context.Context, ev webdav.UploadEvent) {
	if !a.capture.on.Load() {
		return
	}
	dirAny := a.capture.dir.Load()
	dir, _ := dirAny.(string)
	if dir == "" {
		return
	}
	base := filepath.Base(ev.RelPath)
	if base == "" || base == "/" {
		base = "upload.bin"
	}
	stamp := ev.ReceivedAt.UTC().Format("20060102T150405Z")
	name := fmt.Sprintf("%s_%s.fixture", sanitiseSlug(base), stamp)
	dst := filepath.Join(dir, name)

	// Try hardlink first (zero-copy, preserves the immutable archive).
	if err := os.Link(ev.ArchivePath, dst); err == nil {
		a.logCaptureSaved(dst, ev)
		return
	}
	// Fallback: copy.
	if err := copyFile(ev.ArchivePath, dst); err != nil {
		if a.log != nil {
			a.log.Warn("moon: capture copy failed",
				logging.F("src", ev.ArchivePath),
				logging.F("dst", dst),
				logging.F("error", err.Error()))
		}
		return
	}
	a.logCaptureSaved(dst, ev)
}

func (a *Adapter) logCaptureSaved(dst string, ev webdav.UploadEvent) {
	if a.log == nil {
		return
	}
	a.log.Info("moon: capture saved",
		logging.F("path", dst),
		logging.F("size", ev.SizeBytes),
		logging.F("source_version", ev.Version))
}

// sanitiseSlug replaces characters that cause portability issues across
// Windows/macOS/Linux filesystems with underscores while keeping the
// original token intact for human inspection.
func sanitiseSlug(s string) string {
	for _, ch := range []string{"/", "\\", ":", "*", "?", `"`, "<", ">", "|"} {
		s = strings.ReplaceAll(s, ch, "_")
	}
	if s == "" {
		s = "_"
	}
	return s
}

// copyFile copies src to dst, creating dst if absent.  Used as a hardlink
// fallback.
func copyFile(src, dst string) (retErr error) {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.OpenFile(dst, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o444)
	if err != nil {
		return err
	}
	defer func() {
		if cerr := out.Close(); cerr != nil && retErr == nil {
			retErr = cerr
		}
	}()
	_, retErr = io.Copy(out, in)
	return retErr
}

