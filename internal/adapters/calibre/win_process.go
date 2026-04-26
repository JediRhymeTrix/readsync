// internal/adapters/calibre/win_process.go
//
// Windows GUI process detection via `tasklist`.

package calibre

import (
	"bytes"
	"os/exec"
	"strings"
)

// calibreGUIProcessNames are the possible process names for the Calibre GUI.
var calibreGUIProcessNames = []string{
	"calibre.exe",
	"calibre-debug.exe",
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
