// internal/repair/actions_test.go

package repair

import (
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
