// internal/adapters/calibre/discovery.go
//
// Discovery of calibredb.exe and Calibre library paths on Windows.

package calibre

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// commonCalibrePaths lists the standard Windows install locations for calibredb.
var commonCalibrePaths = []string{
	`C:\Program Files\Calibre2\calibredb.exe`,
	`C:\Program Files (x86)\Calibre2\calibredb.exe`,
	`C:\Program Files\calibre\calibredb.exe`,
	`C:\Program Files (x86)\calibre\calibredb.exe`,
	`C:\Calibre2\calibredb.exe`,
	`C:\calibre\calibredb.exe`,
}

// findCalibredb locates calibredb.exe.
// Checks PATH first, then common Windows install directories.
func findCalibredb() (string, error) {
	// Check PATH.
	if p, err := exec.LookPath("calibredb"); err == nil {
		return p, nil
	}
	if p, err := exec.LookPath("calibredb.exe"); err == nil {
		return p, nil
	}
	// Check known install paths.
	for _, p := range commonCalibrePaths {
		if _, err := os.Stat(p); err == nil {
			return p, nil
		}
	}
	return "", fmt.Errorf("calibredb not found in PATH or common install dirs")
}

// discoverLibraries returns a list of Calibre library paths.
// It checks: CALIBRE_LIBRARY_PATH env, the default ~/Calibre Library,
// and parses gui.json if available.
func discoverLibraries() ([]string, error) {
	var libs []string
	seen := map[string]bool{}

	add := func(p string) {
		p = filepath.Clean(p)
		if !seen[p] {
			if info, err := os.Stat(p); err == nil && info.IsDir() {
				seen[p] = true
				libs = append(libs, p)
			}
		}
	}

	// 1. Environment override.
	if p := os.Getenv("CALIBRE_LIBRARY_PATH"); p != "" {
		for _, part := range strings.Split(p, string(os.PathListSeparator)) {
			add(part)
		}
	}

	// 2. Default ~/Documents/Calibre Library and ~/Calibre Library.
	home, err := os.UserHomeDir()
	if err == nil {
		add(filepath.Join(home, "Documents", "Calibre Library"))
		add(filepath.Join(home, "Calibre Library"))
	}

	// 3. Parse Calibre gui.json / global.py.json for library_usage_stats.
	guiPaths := calibreConfigPaths()
	for _, cfgPath := range guiPaths {
		if paths, err := parseLibraryListFromConfig(cfgPath); err == nil {
			for _, p := range paths {
				add(p)
			}
		}
	}

	if len(libs) == 0 {
		return nil, fmt.Errorf("no Calibre libraries found")
	}
	return libs, nil
}

// calibreConfigPaths returns the paths of Calibre GUI config files to try.
func calibreConfigPaths() []string {
	home, _ := os.UserHomeDir()
	appdata := os.Getenv("APPDATA")
	localAppdata := os.Getenv("LOCALAPPDATA")

	var paths []string
	for _, base := range []string{appdata, localAppdata, filepath.Join(home, "AppData", "Roaming")} {
		if base != "" {
			paths = append(paths,
				filepath.Join(base, "calibre", "gui.json"),
				filepath.Join(base, "calibre", "global.py.json"),
			)
		}
	}
	return paths
}

// parseLibraryListFromConfig reads library paths from a Calibre JSON config file.
// Both gui.json and global.py.json store an object at the top level; we look
// for keys named "library_usage_stats" (object with lib paths as keys) or
// "library_path" (string).
func parseLibraryListFromConfig(path string) ([]string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var obj map[string]json.RawMessage
	if err := json.Unmarshal(data, &obj); err != nil {
		return nil, fmt.Errorf("json parse %s: %w", path, err)
	}

	var libs []string

	// library_usage_stats: {"path": count, ...}
	if raw, ok := obj["library_usage_stats"]; ok {
		var stats map[string]int
		if err := json.Unmarshal(raw, &stats); err == nil {
			for p := range stats {
				libs = append(libs, p)
			}
		}
	}

	// library_path: "path"
	if raw, ok := obj["library_path"]; ok {
		var p string
		if err := json.Unmarshal(raw, &p); err == nil && p != "" {
			libs = append(libs, p)
		}
	}

	return libs, nil
}
