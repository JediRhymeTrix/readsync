// internal/setup/persist.go
//
// File-backed persistence for wizard state.

package setup

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
)

// load reads w.path into w.state. Returns ErrStateNotFound if the file
// does not yet exist; any other error is propagated.
func (w *Wizard) load() error {
	if w.path == "" {
		return nil
	}
	data, err := os.ReadFile(w.path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return ErrStateNotFound
		}
		return err
	}
	w.mu.Lock()
	defer w.mu.Unlock()
	var s State
	if err := json.Unmarshal(data, &s); err != nil {
		return err
	}
	if s.Pages == nil {
		s.Pages = map[PageSlug]PageState{}
	}
	// Ensure every canonical page exists in the loaded state.
	for _, slug := range Pages {
		if _, ok := s.Pages[slug]; !ok {
			s.Pages[slug] = PageState{Slug: slug, Status: StatusPending}
		}
	}
	w.state = s
	return nil
}

// persistLocked writes w.state to w.path. The caller must already hold w.mu.
func (w *Wizard) persistLocked() error {
	if w.path == "" {
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(w.path), 0o700); err != nil {
		return err
	}
	data, err := json.MarshalIndent(w.state, "", "  ")
	if err != nil {
		return err
	}
	tmp := w.path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, w.path)
}

// Save writes the current state to disk explicitly. Mostly useful for
// tests and external callers.
func (w *Wizard) Save() error {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.persistLocked()
}
