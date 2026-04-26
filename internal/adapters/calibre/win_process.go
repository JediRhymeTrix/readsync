// internal/adapters/calibre/win_process.go
//
// Windows GUI process detection via `tasklist`.

package calibre

import (
	"bytes"
	"os/exec"
	"strings"
)

// calibreGUIProcessNames lists every Calibre process whose presence makes
// calibredb refuse to mutate a library. The full set, observed from real
// installs:
//
//   - calibre.exe            : the desktop GUI
//   - calibre-debug.exe      : debug build of the GUI
//   - calibre-server.exe     : standalone Content server
//   - calibre-parallel.exe   : worker process spawned by GUI/server
//
// If any of these is running, `calibredb <mutating-cmd> --library-path X`
// fails with "Another calibre program ... is running" — even if the
// running process targets a *different* library.
var calibreGUIProcessNames = []string{
	"calibre.exe",
	"calibre-debug.exe",
	"calibre-server.exe",
	"calibre-parallel.exe",
}

// isGUIRunning returns true if any Calibre GUI process is currently running.
// Uses `tasklist /FI "IMAGENAME eq calibre.exe" /NH /FO CSV` for reliable
// Windows process enumeration without requiring admin rights.
func isGUIRunning() bool {
	for _, name := range calibreGUIProcessNames {
		if processRunning(name) {
			return true
		}
	}
	return false
}

// processRunning checks if a process with the given image name is running.
func processRunning(imageName string) bool {
	cmd := exec.Command("tasklist",
		"/FI", "IMAGENAME eq "+imageName,
		"/NH", "/FO", "CSV",
	)
	out, err := cmd.Output()
	if err != nil {
		return false
	}
	// tasklist outputs a line per process; if the image name appears, it's running.
	lower := strings.ToLower(string(bytes.TrimSpace(out)))
	lowerName := strings.ToLower(imageName)
	return strings.Contains(lower, lowerName)
}

// IsCalibreRunning is the exported variant of isGUIRunning. It returns true
// when any Calibre process (calibre.exe, calibre-debug.exe, calibre-server.exe,
// or calibre-parallel.exe) is currently running on the host. Tests in other
// packages use it to skip integration scenarios that require calibredb to
// mutate a library, since calibredb refuses such commands while any of those
// processes are alive.
func IsCalibreRunning() bool { return isGUIRunning() }
