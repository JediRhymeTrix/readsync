// internal/api/wizard_run.go
//
// Per-page wizard run handlers. Heavy lifting is delegated to
// internal/repair and the relevant adapters.

package api

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/readsync/readsync/internal/repair"
	"github.com/readsync/readsync/internal/setup"
)

// WizardRunResult is the standard shape every step runner returns.
type WizardRunResult struct {
	OK      bool           `json:"ok"`
	Slug    string         `json:"slug"`
	Message string         `json:"message,omitempty"`
	Data    map[string]any `json:"data,omitempty"`
}

func (s *Server) runWizardPage(r *http.Request, slug setup.PageSlug) WizardRunResult {
	res := WizardRunResult{Slug: string(slug)}
	switch slug {
	case setup.PageWelcome:
		res.OK = true
		res.Message = "welcome"
	case setup.PageSystemScan:
		return s.runSystemScan(r, slug)
	case setup.PageCalibre:
		return s.runCalibreSetup(r, slug)
	case setup.PageGoodreadsBridge:
		return s.runGoodreadsSetup(r, slug)
	case setup.PageKOReader:
		return s.runKOReaderSetup(r, slug)
	case setup.PageMoonReader:
		return s.runMoonSetup(r, slug)
	case setup.PageConflictPolicy:
		return s.runPolicySetup(r, slug)
	case setup.PageTestSync:
		return s.runTestSync(r, slug)
	case setup.PageDiagnostics:
		return s.runDiagnosticsBundle(r, slug)
	case setup.PageFinish:
		res.OK = true
		res.Message = "finished"
	default:
		res.Message = "unknown page"
	}
	return res
}

func (s *Server) runSystemScan(r *http.Request, slug setup.PageSlug) WizardRunResult {
	rep := setup.SystemScan(r.Context(), setup.ScanOptions{
		AdminPort: s.deps.Port, DB: s.deps.DB,
	})
	data := map[string]any{}
	allOK := !rep.AnyFailed()
	for _, p := range rep.Probes {
		data[p.Name+"_ok"] = p.OK
		if p.Detail != "" {
			data[p.Name+"_detail"] = p.Detail
		}
	}
	msg := "system scan complete"
	if !allOK {
		msg = "system scan reported issues; see details"
	}
	return WizardRunResult{OK: allOK, Slug: string(slug), Message: msg, Data: data}
}

func (s *Server) runCalibreSetup(r *http.Request, slug setup.PageSlug) WizardRunResult {
	_ = r.ParseForm()
	calibredb := strings.TrimSpace(r.FormValue("calibredb"))
	library := strings.TrimSpace(r.FormValue("library"))
	if calibredb == "" {
		find := repair.FindCalibredb()
		if !find.OK {
			return WizardRunResult{OK: false, Slug: string(slug),
				Message: find.Message, Data: map[string]any{"detail": find.Detail}}
		}
		calibredb = find.Message
	}
	if library == "" {
		return WizardRunResult{OK: false, Slug: string(slug),
			Message: "library path is required"}
	}
	if !isSafeUserPathInput(library) {
		return WizardRunResult{OK: false, Slug: string(slug),
			Message: "invalid library path"}
	}
	if bk := repair.BackupLibrary(library); !bk.OK {
		return WizardRunResult{OK: false, Slug: string(slug),
			Message: "backup failed: " + bk.Message,
			Data:    map[string]any{"detail": bk.Detail}}
	}
	cc := repair.CreateCustomColumns(r.Context(), calibredb, library)
	return WizardRunResult{
		OK: cc.OK, Slug: string(slug),
		Message: "calibre configured: " + cc.Message,
		Data:    map[string]any{"calibredb": calibredb, "library": library},
	}
}

func (s *Server) runGoodreadsSetup(r *http.Request, slug setup.PageSlug) WizardRunResult {
	_ = r.ParseForm()
	mode := strings.TrimSpace(r.FormValue("mode"))
	if mode == "" {
		mode = "manual"
	}
	switch mode {
	case "disabled", "manual", "guided":
	default:
		return WizardRunResult{OK: false, Slug: string(slug),
			Message: "unknown mode: " + mode}
	}
	return WizardRunResult{OK: true, Slug: string(slug),
		Message: "goodreads bridge mode set to " + mode,
		Data:    map[string]any{"mode": mode}}
}

func (s *Server) runKOReaderSetup(r *http.Request, slug setup.PageSlug) WizardRunResult {
	return WizardRunResult{OK: true, Slug: string(slug),
		Message: "KOReader endpoint stub: configure via service config",
		Data:    map[string]any{"note": "use repair: enable_koreader_endpoint"}}
}

func (s *Server) runMoonSetup(r *http.Request, slug setup.PageSlug) WizardRunResult {
	return WizardRunResult{OK: true, Slug: string(slug),
		Message: "Moon+ WebDAV stub: see Moon+ adapter setup bundle",
		Data:    map[string]any{"note": "/api/repair/rotate_adapter_creds?adapter=moon"}}
}

func (s *Server) runPolicySetup(r *http.Request, slug setup.PageSlug) WizardRunResult {
	_ = r.ParseForm()
	policy := setup.DefaultPolicy()
	if v := r.FormValue("jump_pct"); v != "" {
		if f, err := strconv.ParseFloat(v, 64); err == nil {
			policy.SuspiciousJumpPercent = f
		}
	}
	if v := r.FormValue("finished_pct"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			policy.FinishedRegressionThreshold = n
		}
	}
	if err := policy.Validate(); err != nil {
		return WizardRunResult{OK: false, Slug: string(slug),
			Message: "policy invalid: " + err.Error()}
	}
	return WizardRunResult{OK: true, Slug: string(slug),
		Message: "policy saved",
		Data: map[string]any{
			"jump_pct":     policy.SuspiciousJumpPercent,
			"finished_pct": policy.FinishedRegressionThreshold,
		}}
}

func (s *Server) runTestSync(r *http.Request, slug setup.PageSlug) WizardRunResult {
	if defaultSyncTrigger == nil {
		return WizardRunResult{OK: true, Slug: string(slug),
			Message: "test-sync skipped: no SyncTrigger registered (manual mode)"}
	}
	if err := defaultSyncTrigger.TriggerSync(); err != nil {
		return WizardRunResult{OK: false, Slug: string(slug),
			Message: "test-sync failed: " + err.Error()}
	}
	return WizardRunResult{OK: true, Slug: string(slug),
		Message: "test-sync triggered"}
}

func (s *Server) runDiagnosticsBundle(r *http.Request, slug setup.PageSlug) WizardRunResult {
	report := map[string]any{"version": s.deps.Version, "ts": "now"}
	if s.deps.Diagnostics != nil {
		if rep, err := s.deps.Diagnostics.Collect(r.Context()); err == nil {
			report = map[string]any{"report": rep}
		}
	}
	res := repair.ExportDiagnostics(report, "")
	return WizardRunResult{OK: res.OK, Slug: string(slug),
		Message: res.Message, Data: map[string]any{"detail": res.Detail}}
}
