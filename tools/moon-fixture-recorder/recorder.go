// tools/moon-fixture-recorder/recorder.go
//
// Minimal stdlib-only WebDAV recorder.
// Handles the exact subset Moon+ Pro uses: MKCOL, PROPFIND, PUT, GET.
// No external dependencies — pure net/http.

package main

import (
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// Recorder handles WebDAV requests, saves every PUT *.po to captureDir.
type Recorder struct {
	captureDir string
	davRoot    string
	verbose    bool
}

// NewRecorder creates a Recorder. davRoot is where files are stored on disk.
func NewRecorder(captureDir, davRoot string, verbose bool) *Recorder {
	return &Recorder{captureDir: captureDir, davRoot: davRoot, verbose: verbose}
}

// Handler returns the http.Handler to mount at /dav/.
func (rec *Recorder) Handler() http.Handler {
	return http.HandlerFunc(rec.serveWebDAV)
}

func (rec *Recorder) serveWebDAV(w http.ResponseWriter, r *http.Request) {
	if rec.verbose {
		log.Printf("[moon] %-10s %s  Content-Length=%d", r.Method, r.URL.Path, r.ContentLength)
	}
	switch r.Method {
	case "MKCOL":
		rec.handleMKCOL(w, r)
	case "PROPFIND":
		rec.handlePROPFIND(w, r)
	case http.MethodPut:
		rec.handlePUT(w, r)
	case http.MethodGet:
		rec.handleGET(w, r)
	case http.MethodOptions:
		w.Header().Set("DAV", "1")
		w.Header().Set("Allow", "OPTIONS, GET, PUT, MKCOL, PROPFIND")
		w.WriteHeader(http.StatusOK)
	default:
		http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
	}
}

// MKCOL — create a collection (directory). Moon+ sends this for /dav/moonreader/.
func (rec *Recorder) handleMKCOL(w http.ResponseWriter, r *http.Request) {
	dir := rec.diskPath(r.URL.Path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		log.Printf("[webdav] MKCOL %s error: %v", r.URL.Path, err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}
	if rec.verbose {
		log.Printf("[webdav] MKCOL %s → 201", r.URL.Path)
	}
	w.WriteHeader(http.StatusCreated)
}

// PROPFIND — return minimal file properties. Moon+ uses Depth: 0.
func (rec *Recorder) handlePROPFIND(w http.ResponseWriter, r *http.Request) {
	diskPath := rec.diskPath(r.URL.Path)
	info, err := os.Stat(diskPath)

	w.Header().Set("Content-Type", "application/xml; charset=utf-8")
	w.WriteHeader(207) // 207 Multi-Status

	if err != nil {
		// File doesn't exist yet — return empty multistatus so Moon+ proceeds with PUT.
		fmt.Fprintf(w, `<?xml version="1.0" encoding="utf-8"?>`+
			`<D:multistatus xmlns:D="DAV:"></D:multistatus>`)
		return
	}

	etag := fmt.Sprintf(`"%x-%x"`, info.ModTime().UnixNano(), info.Size())
	modTime := info.ModTime().UTC().Format(http.TimeFormat)
	href := r.URL.Path

	fmt.Fprintf(w, `<?xml version="1.0" encoding="utf-8"?>
<D:multistatus xmlns:D="DAV:">
  <D:response>
    <D:href>%s</D:href>
    <D:propstat>
      <D:prop>
        <D:getlastmodified>%s</D:getlastmodified>
        <D:getetag>%s</D:getetag>
        <D:getcontentlength>%d</D:getcontentlength>
        <D:resourcetype/>
      </D:prop>
      <D:status>HTTP/1.1 200 OK</D:status>
    </D:propstat>
  </D:response>
</D:multistatus>`, href, modTime, etag, info.Size())
}

// PUT — receive a file upload. Captures .po files to captureDir as well.
func (rec *Recorder) handlePUT(w http.ResponseWriter, r *http.Request) {
	body, err := io.ReadAll(r.Body)
	if err != nil {
		log.Printf("[webdav] PUT read error: %v", err)
		http.Error(w, "Bad Request", http.StatusBadRequest)
		return
	}

	// Capture .po files with a timestamped filename.
	if strings.HasSuffix(r.URL.Path, ".po") {
		rec.saveCapturedPO(r.URL.Path, body)
	}

	// Write to the DAV root so GET can retrieve it.
	diskPath := rec.diskPath(r.URL.Path)
	if err := os.MkdirAll(filepath.Dir(diskPath), 0755); err == nil {
		_ = os.WriteFile(diskPath, body, 0644)
	}

	w.WriteHeader(http.StatusCreated)
}

// GET — serve a file from davRoot.
func (rec *Recorder) handleGET(w http.ResponseWriter, r *http.Request) {
	diskPath := rec.diskPath(r.URL.Path)
	data, err := os.ReadFile(diskPath)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Content-Type", "application/octet-stream")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(data)
}

// saveCapturedPO saves a .po body to captureDir with a timestamp suffix.
func (rec *Recorder) saveCapturedPO(urlPath string, body []byte) {
	base := filepath.Base(urlPath)
	ts := time.Now().UTC().Format("20060102T150405Z")
	name := strings.TrimSuffix(base, ".po") + "_" + ts + ".po"
	dst := filepath.Join(rec.captureDir, name)

	if err := os.WriteFile(dst, body, 0644); err != nil {
		log.Printf("[recorder] failed to save %q: %v", dst, err)
		return
	}
	log.Printf("[recorder] captured %-30s  %d bytes", name, len(body))
	if len(body) >= 4 {
		log.Printf("[recorder] magic bytes: % x", body[:min4(len(body), 16)])
	}
}

// diskPath maps a URL path under /dav/ to an absolute filesystem path.
func (rec *Recorder) diskPath(urlPath string) string {
	rel := strings.TrimPrefix(urlPath, "/dav")
	rel = filepath.FromSlash(rel)
	return filepath.Join(rec.davRoot, rel)
}

func min4(a, b int) int {
	if a < b {
		return a
	}
	return b
}
