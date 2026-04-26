// internal/adapters/goodreads_bridge/detect.go
//
// Plugin detection. We never load or execute the plugin — we only inspect
// the surrounding metadata files Calibre writes:
//
//   %APPDATA%\calibre\plugins\Goodreads Sync.zip          (or *_sync.zip)
//   %APPDATA%\calibre\plugins\pluginsCustomization.json   (config JSON)
//
// Reading these files is data, not code, so GPL-3.0 of the plugin does
// not impose obligations on ReadSync (see docs/research/goodreads-bridge.md
// section 7).

package goodreads_bridge

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// ExpectedProgressColumn is the canonical column that the user must point
// Goodreads Sync at so the two systems share the same data lane.
const ExpectedProgressColumn = "#readsync_progress"

// ExpectedShelfColumn is the canonical column the user should point the
// plugin's "Reading List column" at.
const ExpectedShelfColumn = "#readsync_gr_shelf"

// PluginConfigKey is the top-level JSON key in pluginsCustomization.json
// used by the Goodreads Sync plugin.
const PluginConfigKey = "Goodreads Sync"

// Detection captures everything the setup wizard needs to know about the
// Goodreads Sync plugin's installation and configuration.
type Detection struct {
	// PluginsDir is the directory that was scanned.
	PluginsDir string

	// Installed is true when a plugin .zip identifying itself as
	// "goodreads_sync" or named "Goodreads Sync.zip" is present.
	Installed bool

	// PluginZipPath is the absolute path to the detected zip (empty if
	// not installed).
	PluginZipPath string

	// CustomizationPath is the absolute path to pluginsCustomization.json
	// (set whether or not the file exists).
	CustomizationPath string

	// ConfigFound is true when the JSON file existed and had a
	// "Goodreads Sync" key.
	ConfigFound bool

	// ProgressColumn is the column the plugin is currently configured to
	// use for reading progress (e.g. "#readsync_progress" or another
	// user-chosen value). Empty when the config is missing.
	ProgressColumn string

	// ShelfColumn is the column configured for the reading-list shelf
	// (e.g. "#readsync_gr_shelf"). Empty when the config is missing.
	ShelfColumn string

	// SyncProgressEnabled mirrors the plugin's "sync_reading_progress" flag.
	SyncProgressEnabled bool

	// Notes contains any informational messages produced during detection
	// (used by the setup wizard).
	Notes []string
}

// ProgressColumnConfigured reports whether the plugin is pointed at the
// canonical #readsync_progress column.
func (d *Detection) ProgressColumnConfigured() bool {
	if d == nil {
		return false
	}
	return strings.EqualFold(d.ProgressColumn, ExpectedProgressColumn)
}

// ShelfColumnConfigured reports whether the plugin is pointed at the
// canonical #readsync_gr_shelf column.
func (d *Detection) ShelfColumnConfigured() bool {
	if d == nil {
		return false
	}
	return strings.EqualFold(d.ShelfColumn, ExpectedShelfColumn)
}

// DetectPlugin runs the detection algorithm against pluginsDir. When
// pluginsDir is empty the function falls back to the standard Windows
// location (%APPDATA%\calibre\plugins).
func DetectPlugin(pluginsDir string) (*Detection, error) {
	if pluginsDir == "" {
		pluginsDir = defaultPluginsDir()
	}
	d := &Detection{PluginsDir: pluginsDir}
	if pluginsDir == "" {
		d.Notes = append(d.Notes, "no plugins directory could be discovered (APPDATA unset?)")
		return d, nil
	}
	if _, err := os.Stat(pluginsDir); err != nil {
		// Directory missing is expected when Calibre isn't installed:
		// surface as "not installed" rather than as an error.
		d.Notes = append(d.Notes, fmt.Sprintf("plugins directory %q not found", pluginsDir))
		return d, nil
	}

	zipPath, err := findGoodreadsSyncZip(pluginsDir)
	if err != nil {
		return d, fmt.Errorf("detect plugin zip: %w", err)
	}
	if zipPath != "" {
		d.Installed = true
		d.PluginZipPath = zipPath
	}

	d.CustomizationPath = filepath.Join(pluginsDir, "pluginsCustomization.json")
	if err := loadPluginConfig(d.CustomizationPath, d); err != nil {
		// Missing file or absent key is expected; log into Notes only.
		d.Notes = append(d.Notes, fmt.Sprintf("config: %s", err))
	}
	return d, nil
}

// defaultPluginsDir returns %APPDATA%\calibre\plugins on Windows, or
// $HOME/.config/calibre/plugins as a fallback for tests on other OSes.
func defaultPluginsDir() string {
	if appdata := os.Getenv("APPDATA"); appdata != "" {
		return filepath.Join(appdata, "calibre", "plugins")
	}
	if home, err := os.UserHomeDir(); err == nil {
		// Linux default: ~/.config/calibre/plugins
		return filepath.Join(home, ".config", "calibre", "plugins")
	}
	return ""
}

// findGoodreadsSyncZip enumerates *.zip in pluginsDir and returns the path
// of the Goodreads Sync plugin if present. Detection is conservative: we
// match by the conventional file name (case-insensitive). We do NOT read
// or extract any *.py code from the archive.
func findGoodreadsSyncZip(pluginsDir string) (string, error) {
	entries, err := os.ReadDir(pluginsDir)
	if err != nil {
		return "", err
	}
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name := e.Name()
		lower := strings.ToLower(name)
		if !strings.HasSuffix(lower, ".zip") {
			continue
		}
		// Filename-based detection (cheap and reliable for the official build).
		if strings.Contains(lower, "goodreads") {
			return filepath.Join(pluginsDir, name), nil
		}
	}
	return "", nil
}

// loadPluginConfig parses pluginsCustomization.json and populates the
// progress-column / shelf-column / sync-progress fields on d.
func loadPluginConfig(path string, d *Detection) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	// The file is a flat object keyed by plugin display name. Each value
	// is itself an object with the plugin's settings. We tolerate both
	// raw JSON values (newer Calibre) and stringified JSON (older).
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		return fmt.Errorf("parse %s: %w", filepath.Base(path), err)
	}
	cfg, ok := raw[PluginConfigKey]
	if !ok {
		return fmt.Errorf("no %q key in customization file", PluginConfigKey)
	}
	// Try direct object first.
	var settings map[string]interface{}
	if err := json.Unmarshal(cfg, &settings); err != nil {
		// Try stringified JSON.
		var asStr string
		if jerr := json.Unmarshal(cfg, &asStr); jerr == nil {
			if jerr2 := json.Unmarshal([]byte(asStr), &settings); jerr2 != nil {
				return fmt.Errorf("parse plugin settings: %w", jerr2)
			}
		} else {
			return fmt.Errorf("parse plugin settings: %w", err)
		}
	}
	d.ConfigFound = true
	if v, ok := settings["progress_column"].(string); ok {
		d.ProgressColumn = v
	}
	if v, ok := settings["reading_list_column"].(string); ok {
		d.ShelfColumn = v
	}
	if v, ok := settings["sync_reading_progress"].(bool); ok {
		d.SyncProgressEnabled = v
	}
	return nil
}
