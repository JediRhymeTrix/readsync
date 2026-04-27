// tools/moon-fixture-recorder/recorder_test.go

package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestSafeDiskPath_ValidPaths ensures normal Moon+ paths are accepted.
func TestSafeDiskPath_ValidPaths(t *testing.T) {
	davRoot := t.TempDir()
	rec := NewRecorder(t.TempDir(), davRoot, false)

	tests := []struct {
		name    string
		urlPath string
	}{
		{"dav root", "/dav/"},
		{"moonreader dir", "/dav/moonreader/"},
		{"po file", "/dav/moonreader/book.po"},
		{"nested file", "/dav/moonreader/sub/book.po"},
		{"no dav prefix", "/files/data.po"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := rec.safeDiskPath(tt.urlPath)
			if err != nil {
				t.Fatalf("safeDiskPath(%q) unexpected error: %v", tt.urlPath, err)
			}
			absRoot, _ := filepath.Abs(davRoot)
			if got != absRoot && !strings.HasPrefix(got, absRoot+string(filepath.Separator)) {
				t.Errorf("safeDiskPath(%q) = %q escapes davRoot %q", tt.urlPath, got, absRoot)
			}
		})
	}
}

// TestSafeDiskPath_InvalidPaths ensures traversal and injection attempts are rejected.
func TestSafeDiskPath_InvalidPaths(t *testing.T) {
	davRoot := t.TempDir()
	rec := NewRecorder(t.TempDir(), davRoot, false)

	tests := []struct {
		name    string
		urlPath string
	}{
		{"dotdot traversal", "/dav/../../../etc/passwd"},
		{"dotdot in middle", "/dav/moonreader/../../etc/passwd"},
		{"dotdot only", "/../../../etc"},
		{"single dotdot", "/dav/.."},
		{"backslash injection", "/dav/moonreader\\..\\evil"},
		{"dotdot with encoded-like text", "/dav/foo/.."},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := rec.safeDiskPath(tt.urlPath)
			if err == nil {
				t.Errorf("safeDiskPath(%q) expected error, got nil", tt.urlPath)
			}
		})
	}
}

// TestSafeDiskPath_NoEscape verifies the abs-prefix check as defense-in-depth.
func TestSafeDiskPath_NoEscape(t *testing.T) {
	davRoot := t.TempDir()
	rec := NewRecorder(t.TempDir(), davRoot, false)

	// All valid paths must resolve inside davRoot.
	validPaths := []string{
		"/dav/",
		"/dav/moonreader/progress.po",
		"/dav/a/b/c/d.po",
	}
	absRoot, _ := filepath.Abs(davRoot)
	for _, p := range validPaths {
		got, err := rec.safeDiskPath(p)
		if err != nil {
			t.Errorf("safeDiskPath(%q) error: %v", p, err)
			continue
		}
		if got != absRoot && !strings.HasPrefix(got, absRoot+string(filepath.Separator)) {
			t.Errorf("safeDiskPath(%q) = %q escapes davRoot %q", p, got, absRoot)
		}
	}
}

// TestSanitizeLog checks that CR and LF are replaced with escape sequences.
func TestSanitizeLog(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"normal", "normal"},
		{"with\nnewline", `with\nnewline`},
		{"with\rreturn", `with\rreturn`},
		{"multi\r\nline", `multi\r\nline`},
		{"/dav/moonreader/book.po", "/dav/moonreader/book.po"},
		{"user\ninjected\rlog", `user\ninjected\rlog`},
		{"", ""},
	}
	for _, tt := range tests {
		got := sanitizeLog(tt.input)
		if got != tt.want {
			t.Errorf("sanitizeLog(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

// TestSanitizeFilename checks that only safe characters are kept.
func TestSanitizeFilename(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"book.po", "book.po"},
		{"my-book_v1.po", "my-book_v1.po"},
		{"../../evil.po", ".._.._evil.po"},
		{"path/to/file.po", "path_to_file.po"},
		{"book\nname.po", "book_name.po"},
		{"book\rname.po", "book_name.po"},
		{"", ""},
	}
	for _, tt := range tests {
		got := sanitizeFilename(tt.input)
		if got != tt.want {
			t.Errorf("sanitizeFilename(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

// TestSaveCapturedPO_PathTraversalContained verifies captured files stay in captureDir.
func TestSaveCapturedPO_PathTraversalContained(t *testing.T) {
	captureDir := t.TempDir()
	rec := NewRecorder(captureDir, t.TempDir(), false)

	// Traversal attempts in the URL path must not escape captureDir.
	rec.saveCapturedPO("/dav/../../evil.po", []byte("data"))
	rec.saveCapturedPO("/dav/moonreader/normal.po", []byte("data2"))

	absCapture, _ := filepath.Abs(captureDir)
	entries, err := os.ReadDir(captureDir)
	if err != nil {
		t.Fatalf("ReadDir: %v", err)
	}
	for _, e := range entries {
		absFile, _ := filepath.Abs(filepath.Join(captureDir, e.Name()))
		if !strings.HasPrefix(absFile, absCapture+string(filepath.Separator)) {
			t.Errorf("captured file outside captureDir: %s", absFile)
		}
	}
}
