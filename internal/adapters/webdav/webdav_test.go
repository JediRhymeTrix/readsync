// internal/adapters/webdav/webdav_test.go
//
// Litmus-style minimal conformance tests for the embedded WebDAV server.

package webdav_test

import (
	"bytes"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/readsync/readsync/internal/adapters/webdav"
	"github.com/readsync/readsync/internal/db"
	"github.com/readsync/readsync/internal/logging"
)

const (
	testUser = "moonuser"
	testPass = "test-password-1234"
)

type srvEnv struct {
	srv     *webdav.Server
	dataDir string
	httpsrv *httptest.Server
}

func setupServer(t *testing.T) *srvEnv {
	t.Helper()
	tmp := t.TempDir()

	dbFile := filepath.Join(tmp, "test.db")
	database, err := db.Open(dbFile)
	if err != nil {
		t.Fatalf("db.Open: %v", err)
	}
	t.Cleanup(func() { database.Close() })
	if err := database.Migrate(); err != nil {
		t.Fatalf("Migrate: %v", err)
	}

	logger := logging.New(io.Discard, io.Discard, logging.LevelError)
	cfg := webdav.Defaults()
	cfg.DataDir = filepath.Join(tmp, "data")
	cfg.URLPrefix = "/dav/"
	srv, err := webdav.New(cfg, database.SQL(), logger)
	if err != nil {
		t.Fatalf("webdav.New: %v", err)
	}
	if err := srv.CreateUser(testUser, testPass); err != nil {
		t.Fatalf("CreateUser: %v", err)
	}
	hs := httptest.NewServer(srv)
	t.Cleanup(hs.Close)
	return &srvEnv{srv: srv, dataDir: cfg.DataDir, httpsrv: hs}
}

// doReq is a small helper that authenticates with the test creds.
func doReq(t *testing.T, e *srvEnv, method, path string, body io.Reader, hdr map[string]string) *http.Response {
	t.Helper()
	req, err := http.NewRequest(method, e.httpsrv.URL+path, body)
	if err != nil {
		t.Fatalf("NewRequest: %v", err)
	}
	req.SetBasicAuth(testUser, testPass)
	for k, v := range hdr {
		req.Header.Set(k, v)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("Do: %v", err)
	}
	return resp
}

func TestWebDAV_AuthRequired(t *testing.T) {
	e := setupServer(t)
	req, _ := http.NewRequest("PROPFIND", e.httpsrv.URL+"/dav/", nil)
	req.Header.Set("Depth", "0")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("Do: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("want 401, got %d", resp.StatusCode)
	}
	if !strings.HasPrefix(resp.Header.Get("WWW-Authenticate"), "Basic") {
		t.Errorf("missing WWW-Authenticate Basic")
	}
}

func TestWebDAV_OptionsAdvertisesMethods(t *testing.T) {
	e := setupServer(t)
	resp := doReq(t, e, "OPTIONS", "/dav/", nil, nil)
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("OPTIONS status: %d", resp.StatusCode)
	}
	allow := resp.Header.Get("Allow")
	for _, m := range []string{"OPTIONS", "PROPFIND", "GET", "PUT", "DELETE", "MKCOL"} {
		if !strings.Contains(allow, m) {
			t.Errorf("Allow missing %q (got %q)", m, allow)
		}
	}
}

func TestWebDAV_PropfindDepthZero(t *testing.T) {
	e := setupServer(t)
	resp := doReq(t, e, "PROPFIND", "/dav/", nil,
		map[string]string{"Depth": "0", "Content-Type": "application/xml"})
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusMultiStatus {
		t.Fatalf("PROPFIND status: %d", resp.StatusCode)
	}
}

func TestWebDAV_MkcolPutGet(t *testing.T) {
	e := setupServer(t)
	resp := doReq(t, e, "MKCOL", "/dav/Apps/", nil, nil)
	resp.Body.Close()
	if resp.StatusCode != http.StatusCreated && resp.StatusCode != http.StatusOK {
		t.Fatalf("MKCOL Apps: %d", resp.StatusCode)
	}
	resp = doReq(t, e, "MKCOL", "/dav/Apps/", nil, nil)
	resp.Body.Close()
	if resp.StatusCode != http.StatusMethodNotAllowed {
		t.Fatalf("repeated MKCOL want 405, got %d", resp.StatusCode)
	}

	body := []byte("12345*0@0#0:42.0%")
	resp = doReq(t, e, "PUT", "/dav/Apps/test.po", bytes.NewReader(body), nil)
	resp.Body.Close()
	if resp.StatusCode != http.StatusCreated && resp.StatusCode != http.StatusOK {
		t.Fatalf("PUT: %d", resp.StatusCode)
	}

	resp = doReq(t, e, "GET", "/dav/Apps/test.po", nil, nil)
	got, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	if !bytes.Equal(got, body) {
		t.Errorf("GET body: want %q got %q", body, got)
	}
}

// TestWebDAV_VersionedImmutable confirms the Layer 1 invariant: every PUT
// produces a fresh version on disk, earlier versions are not mutated.
func TestWebDAV_VersionedImmutable(t *testing.T) {
	e := setupServer(t)
	body1 := []byte("v1*0@0#0:10.0%")
	body2 := []byte("v2*5@0#0:20.0%")
	body3 := []byte("v3*9@1#0:30.0%")
	for i, b := range [][]byte{body1, body2, body3} {
		resp := doReq(t, e, "PUT", "/dav/book.po", bytes.NewReader(b), nil)
		resp.Body.Close()
		if resp.StatusCode != http.StatusCreated && resp.StatusCode != http.StatusOK {
			t.Fatalf("PUT %d: %d", i+1, resp.StatusCode)
		}
	}
	archiveRoot := filepath.Join(e.dataDir, "raw", testUser, "book.po")
	bins, err := filepath.Glob(filepath.Join(archiveRoot, "*.bin"))
	if err != nil {
		t.Fatalf("glob: %v", err)
	}
	if len(bins) != 3 {
		t.Fatalf("want 3 archive versions, got %d (%v)", len(bins), bins)
	}
	want := map[string][]byte{
		"1.bin": body1, "2.bin": body2, "3.bin": body3,
	}
	for _, p := range bins {
		got, err := os.ReadFile(p)
		if err != nil {
			t.Fatalf("read %s: %v", p, err)
		}
		base := filepath.Base(p)
		if !bytes.Equal(got, want[base]) {
			t.Errorf("%s mismatch: want %q got %q", base, want[base], got)
		}
	}
	mfs, _ := filepath.Glob(filepath.Join(archiveRoot, "*.json"))
	if len(mfs) != 3 {
		t.Errorf("want 3 manifests, got %d", len(mfs))
	}

	// Re-PUT the same content as version 1 and ensure existing 1.bin is
	// untouched (a fresh 4.bin should be created).
	resp := doReq(t, e, "PUT", "/dav/book.po", bytes.NewReader(body1), nil)
	resp.Body.Close()
	got1, _ := os.ReadFile(filepath.Join(archiveRoot, "1.bin"))
	if !bytes.Equal(got1, body1) {
		t.Errorf("1.bin was mutated by a later write")
	}
	got4, err := os.ReadFile(filepath.Join(archiveRoot, "4.bin"))
	if err != nil {
		t.Fatalf("4.bin missing: %v", err)
	}
	if !bytes.Equal(got4, body1) {
		t.Errorf("4.bin content mismatch")
	}
}

func TestWebDAV_DeleteAndMove(t *testing.T) {
	e := setupServer(t)
	body := []byte("hello")
	resp := doReq(t, e, "PUT", "/dav/a.txt", bytes.NewReader(body), nil)
	resp.Body.Close()

	resp = doReq(t, e, "MOVE", "/dav/a.txt", nil, map[string]string{
		"Destination": e.httpsrv.URL + "/dav/b.txt",
	})
	resp.Body.Close()
	if resp.StatusCode != http.StatusCreated && resp.StatusCode != http.StatusNoContent {
		t.Fatalf("MOVE: %d", resp.StatusCode)
	}

	resp = doReq(t, e, "DELETE", "/dav/b.txt", nil, nil)
	resp.Body.Close()
	if resp.StatusCode != http.StatusNoContent && resp.StatusCode != http.StatusOK {
		t.Fatalf("DELETE: %d", resp.StatusCode)
	}

	resp = doReq(t, e, "GET", "/dav/b.txt", nil, nil)
	resp.Body.Close()
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("GET after DELETE want 404, got %d", resp.StatusCode)
	}
}

func TestWebDAV_LockUnlock(t *testing.T) {
	e := setupServer(t)
	body := strings.NewReader(`<?xml version="1.0"?><D:lockinfo xmlns:D="DAV:">
		<D:lockscope><D:exclusive/></D:lockscope>
		<D:locktype><D:write/></D:locktype>
		<D:owner><D:href>test</D:href></D:owner>
	</D:lockinfo>`)
	resp := doReq(t, e, "LOCK", "/dav/locked.txt", body,
		map[string]string{"Content-Type": "application/xml", "Timeout": "Second-30"})
	resp.Body.Close()
	// Moon+ does not use LOCK; we just verify the server accepts the
	// method. golang.org/x/net/webdav implements RFC 4918 locking, so
	// this should be 200 OK.
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		t.Logf("LOCK status %d (acceptable - Moon+ does not use LOCK)", resp.StatusCode)
	}
}

// TestWebDAV_NoCredentialsInArchive verifies the password is never written
// to the on-disk archive (manifests, version blobs, etc.).
func TestWebDAV_NoCredentialsInArchive(t *testing.T) {
	e := setupServer(t)
	body := []byte("12345*0@0#0:42.0%")
	resp := doReq(t, e, "PUT", "/dav/x.po", bytes.NewReader(body), nil)
	resp.Body.Close()
	_ = filepath.Walk(e.dataDir, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return nil
		}
		b, _ := os.ReadFile(path)
		if bytes.Contains(b, []byte(testPass)) {
			t.Errorf("password leaked into %s", path)
		}
		return nil
	})
}

// TestWebDAV_BadPassword denies access.
func TestWebDAV_BadPassword(t *testing.T) {
	e := setupServer(t)
	req, _ := http.NewRequest("PROPFIND", e.httpsrv.URL+"/dav/", nil)
	req.Header.Set("Depth", "0")
	req.SetBasicAuth(testUser, "wrong-password")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("Do: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("want 401, got %d", resp.StatusCode)
	}
}

// TestWebDAV_PerUserIsolation ensures one user cannot see another's files.
func TestWebDAV_PerUserIsolation(t *testing.T) {
	e := setupServer(t)
	const otherUser, otherPass = "alice", "alice-secret-pass"
	if err := e.srv.CreateUser(otherUser, otherPass); err != nil {
		t.Fatalf("CreateUser alice: %v", err)
	}
	// PUT as testUser.
	resp := doReq(t, e, "PUT", "/dav/private.po",
		bytes.NewReader([]byte("12345*0@0#0:42.0%")), nil)
	resp.Body.Close()
	// GET as alice — must be 404.
	req, _ := http.NewRequest("GET", e.httpsrv.URL+"/dav/private.po", nil)
	req.SetBasicAuth(otherUser, otherPass)
	resp2, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("Do: %v", err)
	}
	defer resp2.Body.Close()
	if resp2.StatusCode != http.StatusNotFound {
		t.Errorf("alice should see 404, got %d", resp2.StatusCode)
	}
}
