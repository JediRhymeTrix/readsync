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
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return failD("missing_id_report", "could not create dir", err.Error())
	}
	ts := time.Now().UTC().Format("20060102-150405")
	path := filepath.Join(dir, "missing_goodreads_ids-"+ts+".json")
	data, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		return failD("missing_id_report", "marshal failed", err.Error())
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
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
	if data, err := os.ReadFile(configPath); err == nil {
		_ = json.Unmarshal(data, &cfg)
	}
	cfg["koreader_enabled"] = true
	cfg["koreader_lan_only"] = true
	data, _ := json.MarshalIndent(cfg, "", "  ")
	if err := os.MkdirAll(filepath.Dir(configPath), 0o755); err != nil {
		return failD("enable_koreader_endpoint", "mkdir failed", err.Error())
	}
	if err := os.WriteFile(configPath, data, 0o644); err != nil {
		return failD("enable_koreader_endpoint", "write failed", err.Error())
	}
	return okR("enable_koreader_endpoint", "KOReader endpoint enabled (LAN-only)")
}

func safeConfigFilePath(configFile string) (string, error) {
	if configFile == "" {
		return "", errUnsafePath
	}
	abs, err := safeAbsPath(configFile)
	if err != nil {
		return "", err
	}
	if filepath.Base(abs) == string(filepath.Separator) || filepath.Ext(abs) == "" {
		return "", fmt.Errorf("config path must name a file")
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
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return failD("export_diagnostics", "mkdir failed", err.Error())
	}
	ts := time.Now().UTC().Format("20060102-150405")
	path := filepath.Join(dir, "readsync-diag-"+ts+".json")
	data, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		return failD("export_diagnostics", "marshal failed", err.Error())
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		return failD("export_diagnostics", "write failed", err.Error())
	}
	return okR("export_diagnostics", path)
}
