// internal/adapters/webdav/versioned_fs.go
//
// versionedFS is a webdav.FileSystem that delegates to an inner FileSystem
// (an in-memory MemFS) for normal WebDAV operations and additionally
// intercepts every successful PUT (= a write-mode OpenFile + Write + Close
// cycle) to archive the original bytes immutably under DataDir.
//
// The wrapped inner FS is partitioned per authenticated user: file paths
// served to Moon+ are prefixed with "/u/{user}/…" inside the FS so two users
// never see each other's files. The user is taken from the request context
// set by Server.ServeHTTP.

package webdav

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path"
	"strings"
	"sync"
	"time"

	"github.com/readsync/readsync/internal/logging"
	"github.com/readsync/readsync/internal/model"
	xwd "golang.org/x/net/webdav"
)

// versionedFS implements xwd.FileSystem.
type versionedFS struct {
	inner  xwd.FileSystem
	server *Server
}

// scopedName returns the inner FS path for the given external name, scoped to
// the authenticated user.  An empty user is rejected.
func scopedName(ctx context.Context, name string) (string, string, error) {
	user := userFromCtx(ctx)
	if user == "" {
		return "", "", errors.New("webdav: anonymous request")
	}
	clean := path.Clean("/" + name)
	scoped := path.Join("/u", user, clean)
	return scoped, clean, nil
}

func (v *versionedFS) Mkdir(ctx context.Context, name string, perm os.FileMode) error {
	scoped, _, err := scopedName(ctx, name)
	if err != nil {
		return err
	}
	if err := v.ensureUserRoot(ctx); err != nil {
		return err
	}
	if err := v.ensureParent(ctx, scoped); err != nil {
		return err
	}
	return v.inner.Mkdir(ctx, scoped, perm)
}

func (v *versionedFS) RemoveAll(ctx context.Context, name string) error {
	scoped, _, err := scopedName(ctx, name)
	if err != nil {
		return err
	}
	return v.inner.RemoveAll(ctx, scoped)
}

func (v *versionedFS) Rename(ctx context.Context, oldName, newName string) error {
	oldScoped, _, err := scopedName(ctx, oldName)
	if err != nil {
		return err
	}
	newScoped, _, err := scopedName(ctx, newName)
	if err != nil {
		return err
	}
	return v.inner.Rename(ctx, oldScoped, newScoped)
}

func (v *versionedFS) Stat(ctx context.Context, name string) (os.FileInfo, error) {
	scoped, _, err := scopedName(ctx, name)
	if err != nil {
		return nil, err
	}
	if err := v.ensureUserRoot(ctx); err != nil {
		return nil, err
	}
	return v.inner.Stat(ctx, scoped)
}

func (v *versionedFS) OpenFile(ctx context.Context, name string, flag int, perm os.FileMode) (xwd.File, error) {
	scoped, clean, err := scopedName(ctx, name)
	if err != nil {
		return nil, err
	}
	if err := v.ensureUserRoot(ctx); err != nil {
		return nil, err
	}
	// Make sure all intermediate directories exist for writes (Moon+ may
	// PUT to a deep path without preceding MKCOLs).
	if isWrite(flag) {
		if err := v.ensureParent(ctx, scoped); err != nil {
			return nil, err
		}
	}
	f, err := v.inner.OpenFile(ctx, scoped, flag, perm)
	if err != nil {
		return nil, err
	}
	if !isWrite(flag) {
		return f, nil
	}
	user := userFromCtx(ctx)
	return &capturingFile{
		File:    f,
		server:  v.server,
		user:    user,
		relPath: clean,
	}, nil
}

func isWrite(flag int) bool {
	return flag&(os.O_WRONLY|os.O_RDWR|os.O_CREATE|os.O_TRUNC|os.O_APPEND) != 0
}

// ensureUserRoot lazily creates the per-user namespace directories so the
// inner FS Stat/OpenFile calls find a parent.
func (v *versionedFS) ensureUserRoot(ctx context.Context) error {
	user := userFromCtx(ctx)
	if user == "" {
		return errors.New("webdav: anonymous request")
	}
	for _, p := range []string{"/u", "/u/" + user} {
		if _, err := v.inner.Stat(ctx, p); err == nil {
			continue
		}
		if err := v.inner.Mkdir(ctx, p, 0o755); err != nil &&
			!errors.Is(err, os.ErrExist) &&
			!strings.Contains(err.Error(), "exists") {
			return err
		}
	}
	return nil
}

// ensureParent ensures the parent directory of a write target exists by
// creating intermediate dirs (mkdir -p semantics).
func (v *versionedFS) ensureParent(ctx context.Context, scoped string) error {
	dir := path.Dir(scoped)
	if dir == "/" || dir == "." || dir == "" {
		return nil
	}
	// Walk down from "/" creating each segment.
	parts := strings.Split(strings.TrimPrefix(dir, "/"), "/")
	cur := ""
	for _, p := range parts {
		if p == "" {
			continue
		}
		cur = cur + "/" + p
		if _, err := v.inner.Stat(ctx, cur); err == nil {
			continue
		}
		if err := v.inner.Mkdir(ctx, cur, 0o755); err != nil &&
			!errors.Is(err, os.ErrExist) &&
			!strings.Contains(err.Error(), "exists") {
			return err
		}
	}
	return nil
}

// capturingFile wraps an inner File and copies every Write into an in-memory
// buffer.  When Close is called, if any write occurred, the buffer is
// archived as an immutable, versioned blob via Server.archiveUpload.
type capturingFile struct {
	xwd.File
	server  *Server
	user    string
	relPath string

	mu     sync.Mutex
	buf    bytes.Buffer
	dirty  bool
	closed bool
	wErr   error
}

// Write tees writes into both the inner File (so subsequent reads return
// the new content) and an in-memory buffer (so on Close we have the exact
// bytes to archive).
func (c *capturingFile) Write(p []byte) (int, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	n, err := c.File.Write(p)
	if n > 0 {
		c.buf.Write(p[:n])
		c.dirty = true
	}
	if err != nil {
		c.wErr = err
	}
	return n, err
}

// Close finalises the inner file and, if any data was written, hands the
// captured bytes off to Server.archiveUpload for versioned, immutable
// archival.  An archive failure is logged but does NOT fail the WebDAV
// response: Moon+ can retry, and a future repair sweep may re-archive.
func (c *capturingFile) Close() error {
	c.mu.Lock()
	already := c.closed
	c.closed = true
	c.mu.Unlock()
	if already {
		return nil
	}
	cerr := c.File.Close()
	if c.dirty && c.wErr == nil {
		body := append([]byte(nil), c.buf.Bytes()...)
		// Archive using a fresh context: the request context may be on
		// the verge of cancellation as the handler unwinds, but archival
		// MUST complete (Layer 1 invariant).  We bound it ourselves.
		archCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if _, err := c.server.archiveUpload(archCtx, c.user, c.relPath, body); err != nil {
			// Surface the failure but never fail the WebDAV response —
			// Moon+ can retry, and a repair sweep may re-archive.
			// Log the path/size only; never the body or any credentials.
			if c.server.log != nil {
				c.server.log.Error("webdav: archive failed",
					logging.F("user", c.user),
					logging.F("path", c.relPath),
					logging.F("size", len(body)),
					logging.F("error", err.Error()))
			}
			c.server.SetHealth(model.HealthDegraded)
		}
	}
	return cerr
}

// Read, Seek, Readdir, Stat are inherited from the embedded xwd.File.
// We deliberately only override Write and Close so the captured-bytes
// path is the only one that mutates state.
