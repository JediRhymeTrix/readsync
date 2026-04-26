// internal/setup/scan.go
//
// SystemScan: aggregates Calibre, KOReader port, Moon+ port, SQLite,
// firewall, and Goodreads-plugin probes into a single ScanReport
// the UI renders on PageSystemScan.

package setup

import (
	"context"
	"database/sql"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"runtime"
	"time"
)

// Probe is a single named check.
type Probe struct {
	Name    string `json:"name"`
	OK      bool   `json:"ok"`
	Detail  string `json:"detail,omitempty"`
	Hint    string `json:"hint,omitempty"`
}

// ScanReport bundles all probes for the system_scan page.
type ScanReport struct {
	GeneratedAt time.Time `json:"generated_at"`
	Probes      []Probe   `json:"probes"`
}

// AnyFailed reports whether at least one probe failed.
func (r ScanReport) AnyFailed() bool {
	for _, p := range r.Probes {
		if !p.OK {
			return true
		}
	}
	return false
}

// ScanOptions configures which probes to run.
type ScanOptions struct {
	CalibredbPath    string
	KOReaderPort     int
	MoonPort         int
	AdminPort        int
	DBPath           string
	GoodreadsPlugins string // path to %APPDATA%\calibre\plugins
	DB               *sql.DB
}

// SystemScan executes all known probes and returns a ScanReport.
// The function is best-effort: failures of individual probes do not
// abort the scan; they are reflected in their respective Probe.OK.
func SystemScan(ctx context.Context, opt ScanOptions) ScanReport {
	r := ScanReport{GeneratedAt: time.Now().UTC()}

	r.Probes = append(r.Probes, probeCalibredb(opt.CalibredbPath))
	r.Probes = append(r.Probes, probePort("admin", opt.AdminPort))
	r.Probes = append(r.Probes, probePort("koreader", opt.KOReaderPort))
	r.Probes = append(r.Probes, probePort("moon_webdav", opt.MoonPort))
	r.Probes = append(r.Probes, probeSQLiteHealth(ctx, opt.DB))
	r.Probes = append(r.Probes, probeDBFile(opt.DBPath))
	r.Probes = append(r.Probes, probeGoodreadsPlugin(opt.GoodreadsPlugins))
	r.Probes = append(r.Probes, probeFirewall())

	return r
}

func probeCalibredb(path string) Probe {
	if path != "" {
		if _, err := os.Stat(path); err == nil {
			return Probe{Name: "calibredb", OK: true, Detail: path}
		}
	}
	return Probe{Name: "calibredb", OK: false,
		Detail: "calibredb path not configured or missing",
		Hint:   "Run repair action 'Find calibredb' or set the path explicitly."}
}

func probePort(name string, port int) Probe {
	if port <= 0 {
		return Probe{Name: name + "_port", OK: false, Detail: "no port configured"}
	}
	ln, err := net.Listen("tcp", fmt.Sprintf("127.0.0.1:%d", port))
	if err != nil {
		return Probe{Name: name + "_port", OK: false,
			Detail: fmt.Sprintf("port %d in use: %v", port, err),
			Hint:   "Stop the conflicting process or pick a different port."}
	}
	_ = ln.Close()
	return Probe{Name: name + "_port", OK: true,
		Detail: fmt.Sprintf("port %d available", port)}
}

func probeSQLiteHealth(ctx context.Context, db *sql.DB) Probe {
	if db == nil {
		return Probe{Name: "sqlite_health", OK: false,
			Detail: "no database handle",
			Hint:   "Service did not open the database; check logs."}
	}
	c, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	row := db.QueryRowContext(c, "PRAGMA integrity_check")
	var v string
	if err := row.Scan(&v); err != nil {
		return Probe{Name: "sqlite_health", OK: false,
			Detail: err.Error(),
			Hint:   "Run repair action 'Rebuild resolver index'."}
	}
	if v != "ok" {
		return Probe{Name: "sqlite_health", OK: false,
			Detail: "integrity_check returned " + v,
			Hint:   "Restore from a recent backup; contact support."}
	}
	return Probe{Name: "sqlite_health", OK: true, Detail: "integrity_check ok"}
}

func probeDBFile(path string) Probe {
	if path == "" {
		return Probe{Name: "db_file", OK: false, Detail: "DB path unset"}
	}
	info, err := os.Stat(path)
	if err != nil {
		if os.IsNotExist(err) {
			// First run is fine; service creates it on Migrate.
			return Probe{Name: "db_file", OK: true,
				Detail: "DB will be created at " + path}
		}
		return Probe{Name: "db_file", OK: false, Detail: err.Error()}
	}
	return Probe{Name: "db_file", OK: true,
		Detail: fmt.Sprintf("%s (%d bytes)", path, info.Size())}
}

func probeGoodreadsPlugin(pluginsDir string) Probe {
	if pluginsDir == "" {
		appdata := os.Getenv("APPDATA")
		if appdata != "" {
			pluginsDir = filepath.Join(appdata, "calibre", "plugins")
		}
	}
	if pluginsDir == "" {
		return Probe{Name: "goodreads_plugin", OK: false,
			Detail: "could not determine plugins dir",
			Hint:   "This is normal on non-Windows hosts."}
	}
	if _, err := os.Stat(pluginsDir); err != nil {
		return Probe{Name: "goodreads_plugin", OK: false,
			Detail: "plugins dir not found: " + pluginsDir,
			Hint:   "Install Calibre and the Goodreads Sync plugin."}
	}
	return Probe{Name: "goodreads_plugin", OK: true,
		Detail: "plugins dir present at " + pluginsDir}
}

func probeFirewall() Probe {
	if runtime.GOOS != "windows" {
		return Probe{Name: "firewall", OK: true,
			Detail: "non-Windows host; firewall check skipped"}
	}
	// We can't reliably probe Windows Firewall state without admin;
	// surface a soft "unknown" with hint to use the repair action.
	return Probe{Name: "firewall", OK: true,
		Detail: "Windows Firewall state not probed",
		Hint:   "Use repair action 'Open firewall rule' if LAN reader endpoints are unreachable."}
}
