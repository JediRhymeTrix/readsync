// internal/api/handlers_html.go
//
// Server-rendered HTML page handlers.

package api

import (
	"html/template"
	"net/http"

	"github.com/readsync/readsync/internal/setup"
)

type baseVM struct {
	Title         string
	Version       string
	CSRFToken     string
	OverallHealth string
}

func (s *Server) baseModel(title string) baseVM {
	return baseVM{
		Title:         title,
		Version:       s.deps.Version,
		CSRFToken:     s.CSRFToken(),
		OverallHealth: s.overallHealth(),
	}
}

func (s *Server) overallHealth() string {
	chips := []AdapterChip{}
	if s.healthProvider != nil {
		chips = s.healthProvider()
	}
	if len(chips) == 0 {
		return "ok"
	}
	worst := "ok"
	rank := func(s string) int {
		switch s {
		case "ok":
			return 0
		case "disabled":
			return 1
		case "degraded":
			return 2
		case "needs_user_action":
			return 3
		case "failed":
			return 4
		}
		return 0
	}
	for _, c := range chips {
		if rank(c.State) > rank(worst) {
			worst = c.State
		}
	}
	return worst
}

func (s *Server) renderPage(w http.ResponseWriter, contentTmpl string, vm any) {
	// Each page template defines `{{define "content"}}`, but because
	// every template uses the same block name only the last-parsed
	// content survives in s.tmpl. We work around this by parsing a
	// fresh template tree per request: base.html + the requested
	// content template only. This is cheap because the source HTML
	// is in-memory (embed.FS).
	baseSrc, err := templatesFS.ReadFile("templates/base.html")
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	contentSrc, err := templatesFS.ReadFile("templates/" + contentTmpl)
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	t, err := template.New("base.html").Parse(string(baseSrc))
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	if _, err := t.Parse(string(contentSrc)); err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := t.ExecuteTemplate(w, "base.html", vm); err != nil {
		http.Error(w, err.Error(), 500)
	}
}

type dashboardVM struct {
	baseVM
	Adapters    []AdapterChip
	OutboxStats map[string]int
}

func (s *Server) handleDashboard(w http.ResponseWriter, r *http.Request) {
	chips := []AdapterChip{}
	if s.healthProvider != nil {
		chips = s.healthProvider()
	} else if s.deps.DB != nil {
		chips = s.queryAdapterChips(r.Context())
	}
	stats := map[string]int{}
	if s.deps.DB != nil {
		rows, err := s.deps.DB.QueryContext(r.Context(),
			`SELECT status, COUNT(*) FROM sync_outbox GROUP BY status`)
		if err == nil {
			defer rows.Close()
			for rows.Next() {
				var status string
				var count int
				if err := rows.Scan(&status, &count); err == nil {
					stats[status] = count
				}
			}
		}
	}
	s.renderPage(w, "dashboard.html", dashboardVM{
		baseVM:      s.baseModel("Dashboard"),
		Adapters:    chips, OutboxStats: stats,
	})
}

type wizardPageVM struct {
	Slug       setup.PageSlug
	Title      string
	Summary    string
	PageStatus setup.Status
}

type wizardVM struct {
	baseVM
	Pages          []wizardPageVM
	CurrentSlug    setup.PageSlug
	CurrentTitle   string
	CurrentSummary string
	CurrentStatus  setup.Status
	CurrentMessage string
	PrevSlug       setup.PageSlug
	NextSlug       setup.PageSlug
	CalibredbPath  string
	LibraryPath    string
	ScanResultHTML string
}

func (s *Server) handleWizardHTML(w http.ResponseWriter, r *http.Request) {
	if s.deps.Wizard == nil {
		http.Error(w, "wizard not configured", http.StatusServiceUnavailable)
		return
	}
	wz := s.deps.Wizard
	st := wz.State()
	cur := setup.PageSlug(r.URL.Query().Get("page"))
	if cur == "" {
		cur = st.CurrentPage
	}
	_ = wz.SetCurrent(cur)
	pages := make([]wizardPageVM, 0, len(setup.AllPages()))
	var curMeta setup.PageMeta
	for _, m := range setup.AllPages() {
		ps := st.Pages[m.Slug]
		pages = append(pages, wizardPageVM{m.Slug, m.Title, m.Summary, ps.Status})
		if m.Slug == cur {
			curMeta = m
		}
	}
	curState := st.Pages[cur]
	prev, _ := wz.PrevPage()
	next, _ := wz.NextPage()
	vm := wizardVM{
		baseVM:         s.baseModel("Setup"),
		Pages:          pages,
		CurrentSlug:    cur,
		CurrentTitle:   curMeta.Title,
		CurrentSummary: curMeta.Summary,
		CurrentStatus:  curState.Status,
		CurrentMessage: curState.Message,
		PrevSlug:       prev,
		NextSlug:       next,
	}
	if data := curState.Data; data != nil {
		if v, ok := data["calibredb"].(string); ok {
			vm.CalibredbPath = v
		}
		if v, ok := data["library"].(string); ok {
			vm.LibraryPath = v
		}
	}
	s.renderPage(w, "wizard.html", vm)
}

type conflictsVM struct {
	baseVM
	Conflicts []ConflictRow
}

func (s *Server) handleConflictsHTML(w http.ResponseWriter, r *http.Request) {
	rows := []ConflictRow{}
	if s.deps.DB != nil {
		rows = s.queryConflicts(r.Context())
	}
	s.renderPage(w, "conflicts.html",
		conflictsVM{baseVM: s.baseModel("Conflicts"), Conflicts: rows})
}

type outboxVM struct {
	baseVM
	Jobs []OutboxRow
}

func (s *Server) handleOutboxHTML(w http.ResponseWriter, r *http.Request) {
	rows := []OutboxRow{}
	if s.deps.DB != nil {
		rows = s.queryOutbox(r.Context())
	}
	s.renderPage(w, "outbox.html",
		outboxVM{baseVM: s.baseModel("Outbox"), Jobs: rows})
}

type activityVM struct {
	baseVM
	Entries []ActivityEntry
	Limit   int
}

func (s *Server) handleActivityHTML(w http.ResponseWriter, r *http.Request) {
	entries := []ActivityEntry{}
	if s.activityProvider != nil {
		entries = s.activityProvider()
	} else if s.deps.DB != nil {
		entries = s.queryActivity(r.Context(), 100)
	}
	s.renderPage(w, "activity.html",
		activityVM{baseVM: s.baseModel("Activity"), Entries: entries, Limit: 100})
}

type repairVM struct {
	baseVM
	Actions []RepairAction
}

func (s *Server) handleRepairHTML(w http.ResponseWriter, r *http.Request) {
	s.renderPage(w, "repair.html",
		repairVM{baseVM: s.baseModel("Repair"), Actions: allRepairActions()})
}
