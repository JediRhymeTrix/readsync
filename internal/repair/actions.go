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
	"fmt"
	"io"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

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

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	if _, err = io.Copy(out, in); err != nil {
		_ = out.Close()
		return err
	}
	if err = out.Sync(); err != nil {
		_ = out.Close()
		return err
	}
	return out.Close()
}

// BackupLibrary copies metadata.db to a timestamped .bak file.
func BackupLibrary(libraryPath string) ActionResult {
	if libraryPath == "" {
		return failR("backup_library", "library path is required")
	}
	absLib, err := cleanAbsPath(libraryPath)
	if err != nil {
		return failD("backup_library", "invalid library path", err.Error())
	}
	src := filepath.Join(absLib, "metadata.db")
	// Verify the resolved source file is inside the resolved library directory.
	if !strings.HasPrefix(src, absLib+string(filepath.Separator)) {
		return failR("backup_library", "invalid library path")
	}
	if _, err := os.Stat(src); err != nil {
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
	safeCalib, err := validateCalibredbPath(calibredbPath)
	if err != nil {
		return failD("create_custom_columns", "invalid calibredb path", err.Error())
	}
	safeLib, err := cleanAbsPath(libraryPath)
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
		args := []string{"add_custom_column", "--library-path", safeLib,
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
		out, err := exec.CommandContext(ctx, safeCalib, args...).CombinedOutput()
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
