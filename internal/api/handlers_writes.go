// internal/api/handlers_writes.go
//
// State-mutating handlers (CSRF-protected by routes.go's csrf wrapper).

package api

import (
	"bytes"
	"html/template"
	"net/http"
	"strconv"
	"strings"

	"github.com/readsync/readsync/internal/setup"
)

// wizardSnippetTmpl renders a safe HTML snippet for wizard run results.
// html/template provides contextual auto-escaping of all interpolated values.
var wizardSnippetTmpl = template.Must(template.New("wizard-snippet").Parse(
	`<div class="rs-wizard-message {{.Class}}">{{.Message}}</div>` +
		`{{if .DataItems}}<dl class="rs-data">` +
		`{{range .DataItems}}<dt>{{.Key}}</dt><dd>{{.Val}}</dd>{{end}}` +
		`</dl>{{end}}`,
))

type wizardSnippetData struct {
	Class     string
	Message   string
	DataItems []struct{ Key, Val string }
}

// SyncTrigger is the contract for "sync now".
type SyncTrigger interface {
	TriggerSync() error
}

var defaultSyncTrigger SyncTrigger
var defaultRestartHook func() error

// SetSyncTrigger registers a SyncTrigger implementation.
func SetSyncTrigger(t SyncTrigger) { defaultSyncTrigger = t }

// SetRestartHook registers the service-restart implementation.
func SetRestartHook(fn func() error) { defaultRestartHook = fn }

func (s *Server) handleSyncNow(w http.ResponseWriter, r *http.Request) {
	if defaultSyncTrigger == nil {
		writeJSON(w, http.StatusOK, map[string]any{
			"ok": false, "message": "no sync trigger registered (manual mode only)",
		})
		return
	}
	if err := defaultSyncTrigger.TriggerSync(); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{
			"ok": false, "message": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "message": "sync triggered"})
}

func (s *Server) handleRestartService(w http.ResponseWriter, r *http.Request) {
	if defaultRestartHook == nil {
		writeJSON(w, http.StatusOK, map[string]any{
			"ok": false, "message": "no restart hook registered"})
		return
	}
	if err := defaultRestartHook(); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{
			"ok": false, "message": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "message": "restart requested"})
}

func (s *Server) handleWizardRun(w http.ResponseWriter, r *http.Request) {
	if s.deps.Wizard == nil {
		http.Error(w, "wizard not configured", http.StatusServiceUnavailable)
		return
	}
	slug := strings.TrimPrefix(r.URL.Path, "/api/wizard/run/")
	if slug == "" {
		http.Error(w, "missing slug", http.StatusBadRequest)
		return
	}
	page := setup.PageSlug(slug)
	res := s.runWizardPage(r, page)
	status := setup.StatusOK
	if !res.OK {
		status = setup.StatusError
	}
	_ = s.deps.Wizard.Update(page, status, res.Message, res.Data)

	if strings.Contains(r.Header.Get("Accept"), "text/html") ||
		r.URL.Query().Get("html") == "1" {
		s.renderWizardSnippet(w, res)
		return
	}
	writeJSON(w, http.StatusOK, res)
}

func (s *Server) renderWizardSnippet(w http.ResponseWriter, res WizardRunResult) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	klass := "rs-status-ok"
	if !res.OK {
		klass = "rs-status-error"
	}
	data := wizardSnippetData{Class: klass, Message: res.Message}
	for k, v := range res.Data {
		data.DataItems = append(data.DataItems, struct{ Key, Val string }{k, toString(v)})
	}
	var buf bytes.Buffer
	if err := wizardSnippetTmpl.Execute(&buf, data); err != nil {
		http.Error(w, "render error", http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(buf.Bytes())
}

func toString(v any) string {
	switch x := v.(type) {
	case string:
		return x
	case bool:
		if x {
			return "true"
		}
		return "false"
	case int:
		return strconv.Itoa(x)
	case int64:
		return strconv.FormatInt(x, 10)
	case float64:
		return strconv.FormatFloat(x, 'f', 2, 64)
	default:
		return ""
	}
}

func (s *Server) handleWizardComplete(w http.ResponseWriter, r *http.Request) {
	if s.deps.Wizard == nil {
		http.Error(w, "wizard not configured", http.StatusServiceUnavailable)
		return
	}
	if err := s.deps.Wizard.Complete(); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"ok": false, "message": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

func (s *Server) handleWizardReset(w http.ResponseWriter, r *http.Request) {
	if s.deps.Wizard == nil {
		http.Error(w, "wizard not configured", http.StatusServiceUnavailable)
		return
	}
	if err := s.deps.Wizard.Reset(); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"ok": false, "message": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

func (s *Server) handleConflictAction(w http.ResponseWriter, r *http.Request) {
	parts := strings.Split(strings.TrimPrefix(r.URL.Path, "/api/conflicts/"), "/")
	if len(parts) != 2 {
		http.Error(w, "bad path", http.StatusBadRequest)
		return
	}
	id, err := strconv.ParseInt(parts[0], 10, 64)
	if err != nil {
		http.Error(w, "bad id", http.StatusBadRequest)
		return
	}
	switch parts[1] {
	case "resolve":
		s.resolveConflict(w, r, id, r.URL.Query().Get("winner"))
	case "dismiss":
		s.dismissConflict(w, r, id)
	default:
		http.Error(w, "unknown verb", http.StatusBadRequest)
	}
}

func (s *Server) handleOutboxAction(w http.ResponseWriter, r *http.Request) {
	parts := strings.Split(strings.TrimPrefix(r.URL.Path, "/api/outbox/"), "/")
	if len(parts) != 2 {
		http.Error(w, "bad path", http.StatusBadRequest)
		return
	}
	id, err := strconv.ParseInt(parts[0], 10, 64)
	if err != nil {
		http.Error(w, "bad id", http.StatusBadRequest)
		return
	}
	switch parts[1] {
	case "retry":
		s.retryOutbox(w, r, id)
	case "drop":
		s.dropOutbox(w, r, id)
	default:
		http.Error(w, "unknown verb", http.StatusBadRequest)
	}
}
