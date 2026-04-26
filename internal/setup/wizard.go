// internal/setup/wizard.go
//
// Wizard orchestrator: thread-safe state machine with optional file
// persistence. UI/HTTP layers consume the State struct directly.

package setup

import (
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"time"
)

// Wizard orchestrates first-run setup. Safe for concurrent use.
type Wizard struct {
	mu    sync.RWMutex
	state State
	path  string
}

// ErrStateNotFound is returned by Load when no persisted state exists.
var ErrStateNotFound = errors.New("setup: state file not found")

// New creates a wizard initialised at the welcome page.
func New() *Wizard {
	w := &Wizard{
		state: State{
			StartedAt:   time.Now().UTC(),
			CurrentPage: PageWelcome,
			Pages:       map[PageSlug]PageState{},
		},
	}
	for _, slug := range Pages {
		w.state.Pages[slug] = PageState{
			Slug: slug, Status: StatusPending,
			UpdatedAt: time.Now().UTC()}
	}
	return w
}

// NewWithPath creates a Wizard backed by a file.
func NewWithPath(path string) (*Wizard, error) {
	w := New()
	w.path = path
	if err := w.load(); err != nil && !errors.Is(err, ErrStateNotFound) {
		return nil, err
	}
	return w, nil
}

// NeedsSetup returns true when no completed-at timestamp is recorded.
func (w *Wizard) NeedsSetup() bool {
	w.mu.RLock()
	defer w.mu.RUnlock()
	return w.state.CompletedAt == nil
}

// State returns a copy of the current state.
func (w *Wizard) State() State {
	w.mu.RLock()
	defer w.mu.RUnlock()
	cpy := w.state
	pages := make(map[PageSlug]PageState, len(w.state.Pages))
	for k, v := range w.state.Pages {
		pages[k] = v
	}
	cpy.Pages = pages
	return cpy
}

// Page returns the state for slug.
func (w *Wizard) Page(slug PageSlug) (PageState, bool) {
	w.mu.RLock()
	defer w.mu.RUnlock()
	p, ok := w.state.Pages[slug]
	return p, ok
}

// SetCurrent moves the wizard to slug.
func (w *Wizard) SetCurrent(slug PageSlug) error {
	if !validSlug(slug) {
		return fmt.Errorf("setup: unknown page %q", slug)
	}
	w.mu.Lock()
	defer w.mu.Unlock()
	w.state.CurrentPage = slug
	return w.persistLocked()
}

// Update sets the page status, message, and optional data.
func (w *Wizard) Update(slug PageSlug, status Status, msg string, data map[string]any) error {
	if !validSlug(slug) {
		return fmt.Errorf("setup: unknown page %q", slug)
	}
	w.mu.Lock()
	defer w.mu.Unlock()
	w.state.Pages[slug] = PageState{
		Slug: slug, Status: status, Message: msg, Data: data,
		UpdatedAt: time.Now().UTC()}
	return w.persistLocked()
}

// Complete marks the wizard finished. Idempotent.
func (w *Wizard) Complete() error {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.state.CompletedAt != nil {
		return nil
	}
	now := time.Now().UTC()
	w.state.CompletedAt = &now
	w.state.CurrentPage = PageFinish
	return w.persistLocked()
}

// Reset clears all state.
func (w *Wizard) Reset() error {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.state = State{
		StartedAt:   time.Now().UTC(),
		CurrentPage: PageWelcome,
		Pages:       map[PageSlug]PageState{},
	}
	for _, slug := range Pages {
		w.state.Pages[slug] = PageState{Slug: slug, Status: StatusPending,
			UpdatedAt: time.Now().UTC()}
	}
	return w.persistLocked()
}

// NextPage returns the slug after current.
func (w *Wizard) NextPage() (PageSlug, bool) {
	w.mu.RLock()
	cur := w.state.CurrentPage
	w.mu.RUnlock()
	for i, slug := range Pages {
		if slug == cur && i+1 < len(Pages) {
			return Pages[i+1], true
		}
	}
	return "", false
}

// PrevPage returns the slug before current.
func (w *Wizard) PrevPage() (PageSlug, bool) {
	w.mu.RLock()
	cur := w.state.CurrentPage
	w.mu.RUnlock()
	for i, slug := range Pages {
		if slug == cur && i > 0 {
			return Pages[i-1], true
		}
	}
	return "", false
}

// MarshalJSON serialises the entire wizard state.
func (w *Wizard) MarshalJSON() ([]byte, error) {
	w.mu.RLock()
	defer w.mu.RUnlock()
	return json.Marshal(w.state)
}
