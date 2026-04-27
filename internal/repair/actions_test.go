// internal/repair/actions_test.go

package repair

import (
	"context"
	"net"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestActionResult_Helpers(t *testing.T) {
	if r := okR("foo", "msg"); !r.OK || r.Action != "foo" || r.Message != "msg" {
		t.Errorf("okR mismatched: %+v", r)
	}
	if r := failR("foo", "bad"); r.OK || r.Action != "foo" {
		t.Errorf("failR mismatched: %+v", r)
	}
	if r := failD("foo", "bad", "detail"); r.OK || r.Detail != "detail" {
		t.Errorf("failD mismatched: %+v", r)
	}
}

func TestCheckPort_Available(t *testing.T) {
	// Pick a free port via OS.
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	port := ln.Addr().(*net.TCPAddr).Port
	_ = ln.Close()

	r := CheckPort(port)
	if !r.OK {
		t.Errorf("CheckPort(%d) should be available, got %+v", port, r)
	}
}

func TestCheckPort_InUse(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()
	port := ln.Addr().(*net.TCPAddr).Port

	r := CheckPort(port)
	if r.OK {
		t.Errorf("CheckPort(%d) should report in-use", port)
	}
}

func TestCheckPort_Invalid(t *testing.T) {
	if r := CheckPort(0); r.OK {
		t.Error("CheckPort(0) should fail")
	}
}

func TestBackupLibrary_Missing(t *testing.T) {
	dir := t.TempDir()
	r := BackupLibrary(dir)
	if r.OK {
		t.Error("BackupLibrary on empty dir should fail")
	}
}

func TestBackupLibrary_OK(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "metadata.db")
	if err := os.WriteFile(src, []byte("fake"), 0o644); err != nil {
		t.Fatal(err)
	}
	r := BackupLibrary(dir)
	if !r.OK {
		t.Fatalf("BackupLibrary failed: %+v", r)
	}
	if !strings.Contains(r.Message, "backup created") {
		t.Errorf("unexpected msg: %q", r.Message)
	}
}

func TestBackupLibrary_EmptyPath(t *testing.T) {
	if r := BackupLibrary(""); r.OK {
		t.Error("BackupLibrary('') should fail")
	}
}

func TestEnableKOReaderEndpoint(t *testing.T) {
	dir := t.TempDir()
	cfg := filepath.Join(dir, "subdir", "config.json")
	r := EnableKOReaderEndpoint(cfg)
	if !r.OK {
		t.Fatalf("EnableKOReaderEndpoint failed: %+v", r)
	}
	data, err := os.ReadFile(cfg)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), "koreader_enabled") {
		t.Errorf("config missing key: %s", data)
	}
}

func TestRotateAdapterCreds_NoName(t *testing.T) {
	if r := RotateAdapterCreds("", &fakeStore{}); r.OK {
		t.Error("RotateAdapterCreds('') should fail")
	}
}

func TestRotateAdapterCreds_OK(t *testing.T) {
	s := &fakeStore{m: map[string]string{}}
	r := RotateAdapterCreds("koreader", s)
	if !r.OK {
		t.Fatalf("RotateAdapterCreds failed: %+v", r)
	}
	if v := s.m["koreader_password"]; v == "" {
		t.Error("password not stored")
	}
	// Secret must NOT appear in the returned message.
	if strings.Contains(r.Message, s.m["koreader_password"]) {
		t.Error("secret leaked into message")
	}
}

type fakeStore struct{ m map[string]string }

func (f *fakeStore) Set(k, v string) error {
	if f.m == nil {
		f.m = map[string]string{}
	}
	f.m[k] = v
	return nil
}

func TestExportDiagnostics(t *testing.T) {
	dir := t.TempDir()
	report := map[string]any{"version": "1.0.0", "ok": true}
	r := ExportDiagnostics(report, dir)
	if !r.OK {
		t.Fatalf("ExportDiagnostics failed: %+v", r)
	}
	if _, err := os.Stat(r.Message); err != nil {
		t.Errorf("output file missing: %v", err)
	}
}

func TestWriteMissingIDReport(t *testing.T) {
	dir := t.TempDir()
	report := map[string]any{"missing": []string{"book1"}}
	r := WriteMissingIDReport(report, dir)
	if !r.OK {
		t.Fatalf("WriteMissingIDReport failed: %+v", r)
	}
	data, err := os.ReadFile(r.Message)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), "missing") {
		t.Errorf("report content missing: %s", data)
	}
}

func TestOpenGoodreadsPluginInstructions(t *testing.T) {
	r := OpenGoodreadsPluginInstructions()
	if !r.OK {
		t.Error("should always succeed")
	}
	if !strings.HasPrefix(r.Message, "https://") {
		t.Errorf("expected URL, got %q", r.Message)
	}
}

func TestGenerateSecret_Length(t *testing.T) {
	s, err := generateSecret(24)
	if err != nil {
		t.Fatal(err)
	}
	if len(s) < 30 {
		t.Errorf("generateSecret too short: %d", len(s))
	}
}

// ── Validation helpers ──────────────────────────────────────────────────────

func TestCleanAbsPath_Empty(t *testing.T) {
	if _, err := cleanAbsPath(""); err == nil {
		t.Error("cleanAbsPath('') should return error")
	}
}

func TestCleanAbsPath_Traversal(t *testing.T) {
	for _, p := range []string{"../../etc/passwd", "/tmp/../etc/shadow", "foo/../../bar"} {
		if _, err := cleanAbsPath(p); err == nil {
			t.Errorf("cleanAbsPath(%q) should reject traversal", p)
		}
	}
}

func TestCleanAbsPath_Valid(t *testing.T) {
	dir := t.TempDir()
	got, err := cleanAbsPath(dir)
	if err != nil {
		t.Fatalf("cleanAbsPath(%q) unexpected error: %v", dir, err)
	}
	if got != dir {
		t.Errorf("cleanAbsPath(%q) = %q; want %q", dir, got, dir)
	}
}

func TestValidateCalibredbPath_Empty(t *testing.T) {
	if _, err := validateCalibredbPath(""); err == nil {
		t.Error("validateCalibredbPath('') should return error")
	}
}

func TestValidateCalibredbPath_NotAbsolute(t *testing.T) {
	if _, err := validateCalibredbPath("calibredb"); err == nil {
		t.Error("validateCalibredbPath('calibredb') (relative) should return error")
	}
}

func TestValidateCalibredbPath_WrongBasename(t *testing.T) {
	for _, p := range []string{"/usr/bin/sh", "/bin/bash", "/tmp/evil.exe"} {
		if _, err := validateCalibredbPath(p); err == nil {
			t.Errorf("validateCalibredbPath(%q) should reject wrong basename", p)
		}
	}
}

func TestValidateCalibredbPath_Valid(t *testing.T) {
	for _, p := range []string{"/usr/bin/calibredb", "/opt/calibre/calibredb.exe"} {
		got, err := validateCalibredbPath(p)
		if err != nil {
			t.Errorf("validateCalibredbPath(%q) unexpected error: %v", p, err)
		}
		if got == "" {
			t.Errorf("validateCalibredbPath(%q) returned empty string", p)
		}
	}
}

func TestValidateFirewallName_Empty(t *testing.T) {
	got, err := validateFirewallName("")
	if err != nil {
		t.Fatalf("validateFirewallName('') unexpected error: %v", err)
	}
	if got != "ReadSync" {
		t.Errorf("empty name should default to ReadSync, got %q", got)
	}
}

func TestValidateFirewallName_Invalid(t *testing.T) {
	for _, n := range []string{"foo;bar", "name=evil", "rule\x00null", "foo&bar", "foo|bar"} {
		if _, err := validateFirewallName(n); err == nil {
			t.Errorf("validateFirewallName(%q) should reject invalid name", n)
		}
	}
}

func TestValidateFirewallName_Valid(t *testing.T) {
	for _, n := range []string{"ReadSync", "ReadSync-7200", "My Service 1"} {
		got, err := validateFirewallName(n)
		if err != nil {
			t.Errorf("validateFirewallName(%q) unexpected error: %v", n, err)
		}
		if got != n {
			t.Errorf("validateFirewallName(%q) = %q; want %q", n, got, n)
		}
	}
}

// ── BackupLibrary ───────────────────────────────────────────────────────────

func TestBackupLibrary_TraversalRejected(t *testing.T) {
	for _, p := range []string{"../../tmp", "/a/../b/c"} {
		r := BackupLibrary(p)
		if r.OK {
			t.Errorf("BackupLibrary(%q) should fail with traversal path", p)
		}
	}
}

// ── CreateCustomColumns ─────────────────────────────────────────────────────

func TestCreateCustomColumns_EmptyArgs(t *testing.T) {
	r := CreateCustomColumns(context.Background(), "", "")
	if r.OK {
		t.Error("CreateCustomColumns with empty args should fail")
	}
}

func TestCreateCustomColumns_InvalidCalibredbPath(t *testing.T) {
	dir := t.TempDir()
	// Wrong executable name (not calibredb/calibredb.exe).
	r := CreateCustomColumns(context.Background(), "/usr/bin/sh", dir)
	if r.OK {
		t.Error("CreateCustomColumns with /usr/bin/sh should fail validation")
	}
	if !strings.Contains(strings.ToLower(r.Message+r.Detail), "calibredb") {
		t.Errorf("error should mention calibredb, got: %s %s", r.Message, r.Detail)
	}
}

func TestCreateCustomColumns_RelativeCalibredbPath(t *testing.T) {
	dir := t.TempDir()
	r := CreateCustomColumns(context.Background(), "calibredb", dir)
	if r.OK {
		t.Error("CreateCustomColumns with relative calibredb path should fail")
	}
}

func TestCreateCustomColumns_TraversalInLibraryPath(t *testing.T) {
	// Valid calibredb path but traversal in library path.
	r := CreateCustomColumns(context.Background(), "/usr/bin/calibredb", "../../etc")
	if r.OK {
		t.Error("CreateCustomColumns with traversal library path should fail")
	}
}

// ── EnableKOReaderEndpoint ──────────────────────────────────────────────────

func TestEnableKOReaderEndpoint_EmptyPath(t *testing.T) {
	r := EnableKOReaderEndpoint("")
	if r.OK {
		t.Error("EnableKOReaderEndpoint('') should fail")
	}
}

func TestEnableKOReaderEndpoint_TraversalRejected(t *testing.T) {
	for _, p := range []string{"../../etc/readsync.json", "/tmp/../etc/config.json"} {
		r := EnableKOReaderEndpoint(p)
		if r.OK {
			t.Errorf("EnableKOReaderEndpoint(%q) should fail with traversal path", p)
		}
	}
}

// ── OpenFirewallRule ────────────────────────────────────────────────────────

func TestOpenFirewallRule_InvalidName(t *testing.T) {
	// On non-Windows the "not supported" error fires before name validation,
	// so we test the validator directly.
	for _, n := range []string{"foo;drop all", "evil|cmd", "name=bad"} {
		if _, err := validateFirewallName(n); err == nil {
			t.Errorf("validateFirewallName(%q) should reject unsafe name", n)
		}
	}
}
