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

func TestBackupLibrary_RejectsTraversal(t *testing.T) {
	dir := t.TempDir()
	bad := filepath.Join(dir, "..", filepath.Base(dir))
	if r := BackupLibrary(bad); r.OK {
		t.Fatalf("BackupLibrary should reject traversal path: %+v", r)
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

func TestEnableKOReaderEndpoint_RejectsTraversal(t *testing.T) {
	dir := t.TempDir()
	cfg := dir + string(filepath.Separator) + ".." + string(filepath.Separator) + "config.json"
	if r := EnableKOReaderEndpoint(cfg); r.OK {
		t.Fatalf("EnableKOReaderEndpoint should reject traversal path: %+v", r)
	}
}

func TestWriteMissingIDReport_RejectsTraversalDir(t *testing.T) {
	bad := t.TempDir() + string(filepath.Separator) + ".." + string(filepath.Separator) + "readsync-report-escape"
	if r := WriteMissingIDReport(map[string]any{"missing": []string{"book1"}}, bad); r.OK {
		t.Fatalf("WriteMissingIDReport should reject traversal dir: %+v", r)
	}
}

func TestExportDiagnostics_RejectsTraversalDir(t *testing.T) {
	bad := t.TempDir() + string(filepath.Separator) + ".." + string(filepath.Separator) + "readsync-diag-escape"
	if r := ExportDiagnostics(map[string]any{"version": "1.0.0"}, bad); r.OK {
		t.Fatalf("ExportDiagnostics should reject traversal dir: %+v", r)
	}
}

func TestSafeCalibredbPath_RejectsUnsafeExecutables(t *testing.T) {
	bad := []string{"cmd.exe", "calibredb.exe --bad", "-calibredb", ".." + string(filepath.Separator) + "calibredb.exe"}
	for _, p := range bad {
		if _, err := safeCalibredbCommand(p); err == nil {
			t.Fatalf("safeCalibredbCommand(%q) should fail", p)
		}
	}
}

func TestCreateCustomColumns_RejectsUnsafeInputs(t *testing.T) {
	dir := t.TempDir()
	if r := CreateCustomColumns(context.Background(), "cmd.exe", dir); r.OK {
		t.Fatalf("CreateCustomColumns should reject non-calibredb executable: %+v", r)
	}
	if r := CreateCustomColumns(context.Background(), "calibredb.exe", filepath.Join(dir, "..", filepath.Base(dir))); r.OK {
		t.Fatalf("CreateCustomColumns should reject traversal library path: %+v", r)
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
