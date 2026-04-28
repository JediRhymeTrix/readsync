// internal/repair/actions_more.go
//
// Continuation of actions.go (repair actions for the admin UI).

package repair

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

// WriteMissingIDReport dumps the missing-Goodreads-ID list to a JSON file.
func WriteMissingIDReport(report any, dir string) ActionResult {
	if dir == "" {
		dir = filepath.Join(os.TempDir(), "readsync-reports")
	}
	reportDir, err := safeExistingOrCreatableDir(dir)
	if err != nil {
		return failD("missing_id_report", "invalid report dir", err.Error())
	}
	if err := mkdirAllValidatedDir(reportDir, 0o755); err != nil {
		return failD("missing_id_report", "could not create dir", err.Error())
	}
	ts := time.Now().UTC().Format("20060102-150405")
	path, err := safeChildPath(reportDir, "missing_goodreads_ids-"+ts+".json")
	if err != nil {
		return failD("missing_id_report", "invalid report path", err.Error())
	}
	data, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		return failD("missing_id_report", "marshal failed", err.Error())
	}
	if err := writeValidatedFile(path, data, 0o644); err != nil {
		return failD("missing_id_report", "write failed", err.Error())
	}
	return okR("missing_id_report", path)
}

// EnableKOReaderEndpoint flips the local config flag the service watches.
func EnableKOReaderEndpoint(configFile string) ActionResult {
	configPath, err := safeConfigFilePath(configFile)
	if err != nil {
		return failD("enable_koreader_endpoint", "invalid config path", err.Error())
	}
	cfg := map[string]any{}
	if data, err := readValidatedFile(configPath); err == nil {
		_ = json.Unmarshal(data, &cfg)
	}
	cfg["koreader_enabled"] = true
	cfg["koreader_lan_only"] = true
	data, _ := json.MarshalIndent(cfg, "", "  ")
	if err := mkdirAllValidatedDir(filepath.Dir(configPath), 0o755); err != nil {
		return failD("enable_koreader_endpoint", "mkdir failed", err.Error())
	}
	if err := writeValidatedFile(configPath, data, 0o644); err != nil {
		return failD("enable_koreader_endpoint", "write failed", err.Error())
	}
	return okR("enable_koreader_endpoint", "KOReader endpoint enabled (LAN-only)")
}

func safeConfigFilePath(configFile string) (string, error) {
	if configFile == "" || filepath.IsAbs(configFile) {
		return "", errUnsafePath
	}
	abs, err := safeChildPath(".", configFile)
	if err != nil {
		return "", err
	}
	safeRoot, err := safeAbsPath(filepath.Join(os.TempDir(), "readsync-config"))
	if err != nil {
		return "", err
	}
	candidate := abs
	if filepath.IsAbs(configFile) {
		candidate = filepath.Join(safeRoot, filepath.Base(abs))
	} else {
		candidate = filepath.Join(safeRoot, configFile)
	}
	candidate, err = safeAbsPath(candidate)
	if err != nil {
		return "", err
	}
	if !isPathWithin(safeRoot, candidate) {
		return "", errUnsafePath
	}
	if filepath.Base(candidate) == string(filepath.Separator) || filepath.Ext(candidate) == "" {
		return "", fmt.Errorf("config path must name a file")
	}
	return candidate, nil
}

func safeExistingOrCreatableDir(dir string) (string, error) {
	if dir == "" {
		return "", errUnsafePath
	}
	abs, err := safeAbsPath(dir)
	if err != nil {
		return "", err
	}
	if filepath.Ext(abs) != "" {
		return "", fmt.Errorf("directory path must not name a file")
	}
	return abs, nil
}

// RotateAdapterCreds generates a new random credential and writes it
// to the supplied secrets store.
func RotateAdapterCreds(adapter string, store interface{ Set(k, v string) error }) ActionResult {
	if adapter == "" {
		return failR("rotate_adapter_creds", "adapter name is required")
	}
	pw, err := generateSecret(24)
	if err != nil {
		return failD("rotate_adapter_creds", "rng failed", err.Error())
	}
	key := strings.ToLower(adapter) + "_password"
	if err := store.Set(key, pw); err != nil {
		return failD("rotate_adapter_creds", "store failed", err.Error())
	}
	return okR("rotate_adapter_creds",
		fmt.Sprintf("rotated %s credentials (length %d) - re-pair devices", adapter, len(pw)))
}

// OpenFirewallRule adds a Windows Firewall rule for inbound TCP from LAN.
func OpenFirewallRule(name string, port int) ActionResult {
	if runtime.GOOS != "windows" {
		return failD("open_firewall_rule", "not supported on this OS", runtime.GOOS)
	}
	if name == "" {
		name = "ReadSync"
	}
	if port <= 0 {
		return failR("open_firewall_rule", "port must be > 0")
	}
	args := []string{
		"advfirewall", "firewall", "add", "rule",
		"name=" + name, "dir=in", "action=allow",
		"protocol=TCP",
		"localport=" + fmt.Sprintf("%d", port),
		"profile=private", "remoteip=LocalSubnet",
	}
	out, err := exec.Command("netsh", args...).CombinedOutput()
	if err != nil {
		return failD("open_firewall_rule",
			"netsh failed (admin required?)", string(out))
	}
	return okR("open_firewall_rule",
		fmt.Sprintf("firewall rule %q added for TCP/%d (LAN-only)", name, port))
}

// RestartService asks the SCM to restart ReadSync.
func RestartService() ActionResult {
	if runtime.GOOS != "windows" {
		return failD("restart_service", "not supported on this OS", runtime.GOOS)
	}
	_, _ = exec.Command("sc", "stop", "ReadSync").CombinedOutput()
	time.Sleep(2 * time.Second)
	startOut, err := exec.Command("sc", "start", "ReadSync").CombinedOutput()
	if err != nil {
		return failD("restart_service", "sc start failed", string(startOut))
	}
	return okR("restart_service", "service restarted")
}

// RebuildResolverIndex re-runs SQLite REINDEX + ANALYZE on the resolver tables.
func RebuildResolverIndex(ctx context.Context, db *sql.DB) ActionResult {
	stmts := []string{"REINDEX books", "REINDEX book_aliases",
		"ANALYZE books", "ANALYZE book_aliases"}
	for _, s := range stmts {
		if _, err := db.ExecContext(ctx, s); err != nil {
			return failD("rebuild_resolver_index", "exec failed: "+s, err.Error())
		}
	}
	return okR("rebuild_resolver_index", "resolver indexes rebuilt")
}

// ClearDeadletter removes deadletter rows so the outbox can retry.
func ClearDeadletter(ctx context.Context, db *sql.DB) ActionResult {
	res, err := db.ExecContext(ctx,
		"DELETE FROM sync_outbox WHERE status='deadletter'")
	if err != nil {
		return failD("clear_deadletter", "delete failed", err.Error())
	}
	n, _ := res.RowsAffected()
	return okR("clear_deadletter", fmt.Sprintf("removed %d deadletter rows", n))
}

// ExportDiagnostics writes a redacted snapshot of the system to a file.
func ExportDiagnostics(report any, dir string) ActionResult {
	if dir == "" {
		dir = filepath.Join(os.TempDir(), "readsync-diagnostics")
	}
	diagDir, err := safeExistingOrCreatableDir(dir)
	if err != nil {
		return failD("export_diagnostics", "invalid diagnostics dir", err.Error())
	}
	if err := mkdirAllValidatedDir(diagDir, 0o755); err != nil {
		return failD("export_diagnostics", "mkdir failed", err.Error())
	}
	ts := time.Now().UTC().Format("20060102-150405")
	path, err := safeChildPath(diagDir, "readsync-diag-"+ts+".json")
	if err != nil {
		return failD("export_diagnostics", "invalid diagnostics path", err.Error())
	}
	data, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		return failD("export_diagnostics", "marshal failed", err.Error())
	}
	if err := writeValidatedFile(path, data, 0o644); err != nil {
		return failD("export_diagnostics", "write failed", err.Error())
	}
	return okR("export_diagnostics", path)
}
