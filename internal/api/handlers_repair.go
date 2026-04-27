// internal/api/handlers_repair.go
//
// One-click repair endpoints (master spec section 13). All are
// CSRF-protected via the routes wrapper.

package api

import (
	"bytes"
	"html/template"
	"net/http"
	"strings"

	"github.com/readsync/readsync/internal/repair"
)

// repairSnippetTmpl renders a safe HTML snippet for repair action results.
// html/template provides contextual auto-escaping of all interpolated values.
var repairSnippetTmpl = template.Must(template.New("repair-snippet").Parse(
	`<span class="{{.Class}}">{{.Body}}</span>`,
))

type repairSnippetData struct {
	Class string
	Body  string
}

// RepairAction lists the actions the UI exposes on /ui/repair.
type RepairAction struct {
	Slug        string
	Title       string
	Description string
	Endpoint    string
}

func allRepairActions() []RepairAction {
	return []RepairAction{
		{"find_calibredb", "Find calibredb",
			"Locate calibredb.exe on PATH or known install dirs.",
			"/api/repair/find_calibredb"},
		{"backup_library", "Backup Calibre library",
			"Copy metadata.db to a timestamped .bak.",
			"/api/repair/backup_library"},
		{"create_columns", "Create custom columns",
			"Create the #readsync_* columns in the configured library.",
			"/api/repair/create_columns"},
		{"goodreads_plugin", "Open Goodreads plugin instructions",
			"Show the MobileRead URL for Goodreads Sync.",
			"/api/repair/goodreads_plugin"},
		{"missing_id_report", "Generate missing-ID report",
			"List books missing a Goodreads identifier.",
			"/api/repair/missing_id_report"},
		{"enable_koreader", "Enable KOReader endpoint",
			"Turn on the KOSync HTTP endpoint (LAN-only).",
			"/api/repair/enable_koreader"},
		{"rotate_creds", "Rotate adapter credentials",
			"Regenerate the password for the named adapter.",
			"/api/repair/rotate_creds"},
		{"open_firewall", "Open firewall rule",
			"Add a Windows Firewall rule for inbound LAN-only TCP.",
			"/api/repair/open_firewall"},
		{"restart_service", "Restart service",
			"Stop and start ReadSync via the SCM.",
			"/api/repair/restart_service"},
		{"rebuild_index", "Rebuild resolver index",
			"REINDEX + ANALYZE on the books / aliases tables.",
			"/api/repair/rebuild_index"},
		{"clear_deadletter", "Clear deadletter",
			"Delete all sync_outbox rows in deadletter status.",
			"/api/repair/clear_deadletter"},
		{"export_diagnostics", "Export diagnostics",
			"Write a redacted diagnostic bundle to a file.",
			"/api/repair/export_diagnostics"},
	}
}

// secretSetter is the contract every secrets store satisfies.
type secretSetter interface {
	Set(k, v string) error
}

var defaultSecretsStore secretSetter

// SetSecretsStore registers the secrets store used by rotate_creds.
func SetSecretsStore(s secretSetter) { defaultSecretsStore = s }

func (s *Server) handleRepairAction(w http.ResponseWriter, r *http.Request) {
	slug := strings.TrimPrefix(r.URL.Path, "/api/repair/")
	if slug == "" {
		http.Error(w, "missing slug", http.StatusBadRequest)
		return
	}
	res := s.dispatchRepair(r, slug)
	if strings.Contains(r.Header.Get("Accept"), "text/html") ||
		r.URL.Query().Get("html") == "1" {
		s.renderRepairSnippet(w, res)
		return
	}
	writeJSON(w, http.StatusOK, res)
}

func (s *Server) dispatchRepair(r *http.Request, slug string) repair.ActionResult {
	q := r.URL.Query()
	switch slug {
	case "find_calibredb":
		return repair.FindCalibredb()
	case "backup_library":
		return repair.BackupLibrary(q.Get("library"))
	case "create_columns":
		return repair.CreateCustomColumns(r.Context(), q.Get("calibredb"), q.Get("library"))
	case "goodreads_plugin":
		return repair.OpenGoodreadsPluginInstructions()
	case "missing_id_report":
		return repair.WriteMissingIDReport(map[string]any{
			"note": "missing-ID list not yet wired (Phase 5 helper)"}, "")
	case "enable_koreader":
		return repair.EnableKOReaderEndpoint(q.Get("config"))
	case "rotate_creds":
		store := defaultSecretsStore
		if store == nil {
			return repair.ActionResult{Action: "rotate_adapter_creds",
				OK: false, Message: "no secrets store registered"}
		}
		return repair.RotateAdapterCreds(q.Get("adapter"), store)
	case "open_firewall":
		port := 0
		for _, c := range q.Get("port") {
			if c < '0' || c > '9' {
				break
			}
			port = port*10 + int(c-'0')
		}
		return repair.OpenFirewallRule(q.Get("name"), port)
	case "restart_service":
		return repair.RestartService()
	case "rebuild_index":
		if s.deps.DB == nil {
			return repair.ActionResult{Action: "rebuild_resolver_index",
				OK: false, Message: "no database"}
		}
		return repair.RebuildResolverIndex(r.Context(), s.deps.DB)
	case "clear_deadletter":
		if s.deps.DB == nil {
			return repair.ActionResult{Action: "clear_deadletter",
				OK: false, Message: "no database"}
		}
		return repair.ClearDeadletter(r.Context(), s.deps.DB)
	case "export_diagnostics":
		report := map[string]any{"version": s.deps.Version}
		if s.deps.Diagnostics != nil {
			if rep, err := s.deps.Diagnostics.Collect(r.Context()); err == nil {
				report = map[string]any{"report": rep}
			}
		}
		return repair.ExportDiagnostics(report, "")
	}
	return repair.ActionResult{Action: slug, OK: false,
		Message: "unknown repair action"}
}

func (s *Server) renderRepairSnippet(w http.ResponseWriter, res repair.ActionResult) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	klass := "rs-status-ok"
	if !res.OK {
		klass = "rs-status-error"
	}
	body := res.Message
	if res.Detail != "" {
		body += "\n" + res.Detail
	}
	var buf bytes.Buffer
	if err := repairSnippetTmpl.Execute(&buf, repairSnippetData{Class: klass, Body: body}); err != nil {
		http.Error(w, "render error", http.StatusInternalServerError)
		return
	}
	_, _ = w.Write(buf.Bytes())
}
