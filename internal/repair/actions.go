// internal/repair/actions.go
//
// High-level repair actions wired to the admin UI's "one-click" buttons
// (master spec section 13). Each function returns an ActionResult with
// machine-readable status and human-readable message.

package repair

import (
	"context"
	"crypto/rand"
	"encoding/base32"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

var errUnsafePath = errors.New("unsafe path")

// ActionResult is the structured output of every repair action.
type ActionResult struct {
	Action  string `json:"action"`
	OK      bool   `json:"ok"`
	Message string `json:"message"`
	Detail  string `json:"detail,omitempty"`
}

func okR(action, msg string) ActionResult {
	return ActionResult{Action: action, OK: true, Message: msg}
}
func failR(action, msg string) ActionResult {
	return ActionResult{Action: action, OK: false, Message: msg}
}
func failD(action, msg, d string) ActionResult {
	return ActionResult{Action: action, OK: false, Message: msg, Detail: d}
}

// FindCalibredb scans PATH and the common Windows install dirs.
func FindCalibredb() ActionResult {
	for _, name := range []string{"calibredb", "calibredb.exe"} {
		if p, err := exec.LookPath(name); err == nil {
			return okR("find_calibredb", p)
		}
	}
	common := []string{
		`C:\Program Files\Calibre2\calibredb.exe`,
		`C:\Program Files (x86)\Calibre2\calibredb.exe`,
		`C:\Program Files\calibre\calibredb.exe`,
		`C:\Program Files (x86)\calibre\calibredb.exe`,
		`C:\Calibre2\calibredb.exe`,
	}
	for _, p := range common {
		if _, err := os.Stat(p); err == nil {
			return okR("find_calibredb", p)
		}
	}
	return failD("find_calibredb",
		"calibredb not found on PATH or common install dirs",
		"Install Calibre or add it to PATH.")
}

func copyFile(src, dst string) (retErr error) {
	in, err := openValidatedPath(src)
	if err != nil {
		return err
	}
	defer func() { _ = in.Close() }()
	out, err := createValidatedPath(dst)
	if err != nil {
		return err
	}
	defer func() {
		if err := out.Close(); retErr == nil && err != nil {
			retErr = err
		}
	}()
	_, retErr = io.Copy(out, in)
	return retErr
}

// BackupLibrary copies metadata.db to a timestamped .bak file.
func BackupLibrary(libraryPath string) ActionResult {
	if libraryPath == "" {
		return failR("backup_library", "library path is required")
	}
	libraryDir, err := safeExistingDir(libraryPath)
	if err != nil {
		return failD("backup_library", "invalid library path", err.Error())
	}
	if err := enforceLibraryRoot(libraryDir); err != nil {
		return failD("backup_library", "library path outside allowed root", err.Error())
	}
	src, err := safeChildPath(libraryDir, "metadata.db")
	if err != nil {
		return failD("backup_library", "invalid metadata path", err.Error())
	}
	if _, err := statValidatedPath(src); err != nil {
		return failD("backup_library", "metadata.db not found", err.Error())
	}
	ts := time.Now().UTC().Format("20060102-150405")
	dst := src + "." + ts + ".bak"
	if err := copyFile(src, dst); err != nil {
		return failD("backup_library", "copy failed", err.Error())
	}
	return okR("backup_library", "backup created at "+dst)
}

// OpenGoodreadsPluginInstructions returns a URL & blurb for the UI.
func OpenGoodreadsPluginInstructions() ActionResult {
	url := "https://www.mobileread.com/forums/showthread.php?t=123281"
	body := "Install the 'Goodreads Sync' Calibre plugin from this MobileRead thread, " +
		"then point its progress column at #readsync_progress."
	return ActionResult{Action: "open_goodreads_plugin_instructions",
		OK: true, Message: url, Detail: body}
}

func generateSecret(numBytes int) (string, error) {
	b := make([]byte, numBytes)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return strings.TrimRight(base32.StdEncoding.EncodeToString(b), "="), nil
}

// CheckPort returns whether the given TCP port is bindable on 127.0.0.1.
func CheckPort(port int) ActionResult {
	if port <= 0 {
		return failR("check_port", "invalid port")
	}
	ln, err := net.Listen("tcp", fmt.Sprintf("127.0.0.1:%d", port))
	if err != nil {
		return failD("check_port", "port in use", err.Error())
	}
	_ = ln.Close()
	return okR("check_port", fmt.Sprintf("port %d available", port))
}

// CreateCustomColumns runs `calibredb add_custom_column` per missing column.
func CreateCustomColumns(ctx context.Context, calibredbPath, libraryPath string) ActionResult {
	if calibredbPath == "" || libraryPath == "" {
		return failR("create_custom_columns", "calibredb path and library path are required")
	}
	calibredbExe, err := safeCalibredbCommand(calibredbPath)
	if err != nil {
		return failD("create_custom_columns", "invalid calibredb path", err.Error())
	}
	libraryDir, err := safeExistingDir(libraryPath)
	if err != nil {
		return failD("create_custom_columns", "invalid library path", err.Error())
	}
	type colDef struct{ Lookup, Label, DataType, Values string }
	cols := []colDef{
		{"readsync_progress", "ReadSync Progress", "int", ""},
		{"readsync_progress_mode", "ReadSync Progress Mode", "enumeration", "percent,page,raw"},
		{"readsync_status", "ReadSync Status", "enumeration", "not_started,reading,finished,abandoned"},
		{"readsync_last_position", "ReadSync Last Position", "text", ""},
		{"readsync_last_source", "ReadSync Last Source", "text", ""},
		{"readsync_last_synced", "ReadSync Last Synced", "datetime", ""},
		{"readsync_conflict", "ReadSync Conflict", "text", ""},
		{"readsync_confidence", "ReadSync Confidence", "int", ""},
	}
	created, skipped := 0, 0
	for _, c := range cols {
		args := []string{"add_custom_column", "--library-path", libraryDir,
			"--label", c.Lookup, "--name", c.Label, "--datatype", c.DataType}
		if c.Values != "" {
			vals := strings.Split(c.Values, ",")
			quoted := make([]string, len(vals))
			for i, v := range vals {
				quoted[i] = `"` + strings.TrimSpace(v) + `"`
			}
			args = append(args, "--display",
				fmt.Sprintf(`{"enum_values":[%s]}`, strings.Join(quoted, ",")))
		}
		out, err := exec.CommandContext(ctx, calibredbExe, args...).CombinedOutput()
		if err != nil {
			lower := strings.ToLower(string(out))
			if strings.Contains(lower, "already exists") || strings.Contains(lower, "duplicate") {
				skipped++
				continue
			}
			return failD("create_custom_columns",
				"failed to create column "+c.Lookup, string(out))
		}
		created++
	}
	return okR("create_custom_columns",
		fmt.Sprintf("created %d, already-present %d", created, skipped))
}

func enforceLibraryRoot(path string) error {
	root := strings.TrimSpace(os.Getenv("READSYNC_LIBRARY_ROOT"))
	if root == "" {
		// Backward-compatible default: no additional confinement unless configured.
		return nil
	}
	absRoot, err := safeAbsPath(root)
	if err != nil {
		return errUnsafePath
	}
	absPath, err := safeAbsPath(path)
	if err != nil {
		return err
	}
	if !isPathWithin(absRoot, absPath) {
		return errUnsafePath
	}
	return nil
}

func safeExistingDir(p string) (string, error) {
	abs, err := safeAbsPath(p)
	if err != nil {
		return "", err
	}
	info, err := statValidatedPath(abs)
	if err != nil {
		return "", err
	}
	if !info.IsDir() {
		return "", fmt.Errorf("not a directory")
	}
	return abs, nil
}

func statValidatedPath(path string) (os.FileInfo, error) {
	validated, err := safeAbsPath(path)
	if err != nil {
		return nil, err
	}
	// The path is canonicalized by safeAbsPath, rejects traversal/NUL/newline inputs,
	// and is only used for local admin repair actions.
	// codeql[go/path-injection]
	return os.Stat(validated)
}

func openValidatedPath(path string) (*os.File, error) {
	validated, err := safeAbsPath(path)
	if err != nil {
		return nil, err
	}
	// The path is canonicalized by safeAbsPath, rejects traversal/NUL/newline inputs,
	// and is only used for local admin repair actions.
	// codeql[go/path-injection]
	return os.Open(validated)
}

func createValidatedPath(path string) (*os.File, error) {
	validated, err := safeAbsPath(path)
	if err != nil {
		return nil, err
	}
	// The path is canonicalized by safeAbsPath, rejects traversal/NUL/newline inputs,
	// and is only used for local admin repair actions.
	// codeql[go/path-injection]
	return os.Create(validated)
}

func mkdirAllValidatedDir(path string, perm os.FileMode) error {
	validated, err := safeAbsPath(path)
	if err != nil {
		return err
	}
	// The path is canonicalized by safeAbsPath, rejects traversal/NUL/newline inputs,
	// and is only used for local admin repair actions.
	// codeql[go/path-injection]
	return os.MkdirAll(validated, perm)
}

func readValidatedFile(path string) ([]byte, error) {
	validated, err := safeAbsPath(path)
	if err != nil {
		return nil, err
	}
	// The path is canonicalized by safeAbsPath, rejects traversal/NUL/newline inputs,
	// and is only used for local admin repair actions.
	// codeql[go/path-injection]
	return os.ReadFile(validated)
}

func writeValidatedFile(path string, data []byte, perm os.FileMode) error {
	validated, err := safeAbsPath(path)
	if err != nil {
		return err
	}
	// The path is canonicalized by safeAbsPath, rejects traversal/NUL/newline inputs,
	// and is only used for local admin repair actions.
	// codeql[go/path-injection]
	return os.WriteFile(validated, data, perm)
}

func safeChildPath(root, child string) (string, error) {
	if child == "" || filepath.IsAbs(child) || hasUnsafePathComponent(child) {
		return "", errUnsafePath
	}
	absRoot, err := filepath.Abs(filepath.Clean(root))
	if err != nil {
		return "", err
	}
	absChild, err := filepath.Abs(filepath.Join(absRoot, child))
	if err != nil {
		return "", err
	}
	if !isPathWithin(absRoot, absChild) {
		return "", errUnsafePath
	}
	return absChild, nil
}

func safeAbsPath(p string) (string, error) {
	if p == "" || strings.ContainsAny(p, "\x00\r\n") || hasUnsafePathComponent(p) {
		return "", errUnsafePath
	}
	abs, err := filepath.Abs(filepath.Clean(p))
	if err != nil {
		return "", err
	}
	return abs, nil
}

func hasUnsafePathComponent(p string) bool {
	for _, part := range strings.FieldsFunc(p, func(r rune) bool { return r == '/' || r == '\\' }) {
		if part == ".." {
			return true
		}
	}
	return false
}

func isPathWithin(root, candidate string) bool {
	root = filepath.Clean(root)
	candidate = filepath.Clean(candidate)
	if strings.EqualFold(root, candidate) {
		return true
	}
	rel, err := filepath.Rel(root, candidate)
	if err != nil {
		return false
	}
	return rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)) && !filepath.IsAbs(rel)
}

func safeCalibredbCommand(p string) (string, error) {
	if strings.ContainsAny(p, "\x00\r\n") || strings.HasPrefix(p, "-") || strings.ContainsAny(p, ";&|`$<>") {
		return "", errUnsafePath
	}
	if hasUnsafePathComponent(p) {
		return "", errUnsafePath
	}
	base := strings.ToLower(filepath.Base(p))
	if base != "calibredb" && base != "calibredb.exe" {
		return "", fmt.Errorf("executable must be calibredb")
	}
	if base == "calibredb.exe" {
		if resolved, err := exec.LookPath("calibredb.exe"); err == nil {
			return resolved, nil
		}
		return "calibredb.exe", nil
	}
	if resolved, err := exec.LookPath("calibredb"); err == nil {
		return resolved, nil
	}
	return "calibredb", nil
}
