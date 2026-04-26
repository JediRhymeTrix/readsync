// internal/setup/pages.go
//
// First-run setup wizard page metadata (master spec section 13).

package setup

import "time"

// PageSlug is a stable identifier for a wizard step.
type PageSlug string

const (
	PageWelcome         PageSlug = "welcome"
	PageSystemScan      PageSlug = "system_scan"
	PageCalibre         PageSlug = "calibre"
	PageGoodreadsBridge PageSlug = "goodreads_bridge"
	PageKOReader        PageSlug = "koreader"
	PageMoonReader      PageSlug = "moon"
	PageConflictPolicy  PageSlug = "conflict_policy"
	PageTestSync        PageSlug = "test_sync"
	PageDiagnostics     PageSlug = "diagnostics"
	PageFinish          PageSlug = "finish"
)

// Pages is the canonical ordered list of wizard slugs.
var Pages = []PageSlug{
	PageWelcome, PageSystemScan, PageCalibre, PageGoodreadsBridge,
	PageKOReader, PageMoonReader, PageConflictPolicy, PageTestSync,
	PageDiagnostics, PageFinish,
}

// PageMeta describes a single wizard page for UI rendering.
type PageMeta struct {
	Slug    PageSlug `json:"slug"`
	Title   string   `json:"title"`
	Summary string   `json:"summary"`
}

// AllPages returns the page metadata in canonical order.
func AllPages() []PageMeta {
	return []PageMeta{
		{PageWelcome, "Welcome", "Introduction to ReadSync setup."},
		{PageSystemScan, "System Scan", "Detect Calibre, KOReader, Moon+, ports, SQLite."},
		{PageCalibre, "Calibre", "Pick a library, create #readsync_* columns, back up metadata."},
		{PageGoodreadsBridge, "Goodreads Bridge", "Mode picker: disabled / manual / guided plugin."},
		{PageKOReader, "KOReader", "Generate creds, choose LAN bind, show device URL + QR."},
		{PageMoonReader, "Moon+ Reader Pro", "Generate WebDAV creds + URL + QR + connection test."},
		{PageConflictPolicy, "Conflict Policy", "Precedence picker, suspicious-jump thresholds."},
		{PageTestSync, "Test Sync", "End-to-end smoke against each enabled adapter."},
		{PageDiagnostics, "Diagnostics", "Export bundle for support."},
		{PageFinish, "Finish", "All done."},
	}
}

// Status is the per-page state carried between requests.
type Status string

const (
	StatusPending Status = "pending"
	StatusRunning Status = "running"
	StatusOK      Status = "ok"
	StatusWarn    Status = "warn"
	StatusError   Status = "error"
	StatusSkipped Status = "skipped"
)

// PageState captures the outcome for a single page.
type PageState struct {
	Slug      PageSlug       `json:"slug"`
	Status    Status         `json:"status"`
	Message   string         `json:"message,omitempty"`
	UpdatedAt time.Time      `json:"updated_at"`
	Data      map[string]any `json:"data,omitempty"`
}

// State is the wizard's overall state, persisted between runs.
type State struct {
	StartedAt   time.Time              `json:"started_at"`
	CompletedAt *time.Time             `json:"completed_at,omitempty"`
	CurrentPage PageSlug               `json:"current_page"`
	Pages       map[PageSlug]PageState `json:"pages"`
}

// validSlug reports whether slug is a known wizard page.
func validSlug(slug PageSlug) bool {
	for _, s := range Pages {
		if s == slug {
			return true
		}
	}
	return false
}
