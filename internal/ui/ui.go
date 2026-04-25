// internal/ui/ui.go
//
// System tray UI stub. Phase 4 will implement the full tray icon + menu.

package ui

// Tray manages the system tray icon and context menu.
// Stub: Phase 4 implementation.
type Tray struct{}

// New creates a tray UI stub.
func New() *Tray { return &Tray{} }

// Run starts the tray event loop. Blocks until the tray is dismissed.
func (t *Tray) Run() {}

// Stop removes the tray icon.
func (t *Tray) Stop() {}
