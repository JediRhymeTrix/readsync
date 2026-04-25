// internal/setup/setup.go
//
// First-run setup wizard stub. Phase 4 will implement the full setup UI.

package setup

// Wizard orchestrates the first-run setup flow.
// Stub: Phase 4 implementation.
type Wizard struct{}

// New creates a setup wizard stub.
func New() *Wizard { return &Wizard{} }

// NeedsSetup returns true if first-run configuration is incomplete.
func (w *Wizard) NeedsSetup() bool { return false }

// Run executes the interactive setup flow.
func (w *Wizard) Run() error { return nil }
