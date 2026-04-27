// internal/adapters/moon/capture_test.go
//
// Unit tests for copyFile (no CGO required).

package moon

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

// TestCopyFile_Basic verifies that copyFile copies file content correctly.
func TestCopyFile_Basic(t *testing.T) {
	tmp := t.TempDir()
	src := filepath.Join(tmp, "src.bin")
	dst := filepath.Join(tmp, "dst.bin")

	want := []byte("hello moon capture")
	if err := os.WriteFile(src, want, 0o644); err != nil {
		t.Fatalf("write src: %v", err)
	}

	if err := copyFile(src, dst); err != nil {
		t.Fatalf("copyFile: %v", err)
	}

	got, err := os.ReadFile(dst)
	if err != nil {
		t.Fatalf("read dst: %v", err)
	}
	if string(got) != string(want) {
		t.Errorf("content mismatch: got %q want %q", got, want)
	}
}

// TestCopyFile_MissingSrc verifies that a missing source returns an error.
func TestCopyFile_MissingSrc(t *testing.T) {
	tmp := t.TempDir()
	err := copyFile(filepath.Join(tmp, "nosuchfile"), filepath.Join(tmp, "dst.bin"))
	if err == nil {
		t.Fatal("expected error for missing src, got nil")
	}
}

// TestCopyFile_DstAlreadyExists verifies that copyFile fails when dst exists
// (O_EXCL semantics).
func TestCopyFile_DstAlreadyExists(t *testing.T) {
	tmp := t.TempDir()
	src := filepath.Join(tmp, "src.bin")
	dst := filepath.Join(tmp, "dst.bin")
	if err := os.WriteFile(src, []byte("data"), 0o644); err != nil {
		t.Fatalf("write src: %v", err)
	}
	if err := os.WriteFile(dst, []byte("existing"), 0o644); err != nil {
		t.Fatalf("write dst: %v", err)
	}
	err := copyFile(src, dst)
	if err == nil {
		t.Fatal("expected error when dst already exists, got nil")
	}
	if !errors.Is(err, os.ErrExist) {
		t.Errorf("expected ErrExist, got %v", err)
	}
}
