// internal/adapters/goodreads_bridge/modes.go
//
// Bridge operating modes. v1 ships ModeDisabled, ModeManualPlugin and
// ModeGuidedPlugin as fully-supported. ModeCompanionPlugin and
// ModeExperimentalDirect are design hooks only: they validate config,
// log clear "not implemented" markers, and surface visible health states
// so users can never silently end up depending on them.

package goodreads_bridge

// BridgeMode is the typed enumeration of supported bridge modes.
type BridgeMode string

const (
	// ModeDisabled means ReadSync ignores Goodreads entirely. Detection
	// still runs so the setup wizard can offer to enable the bridge later.
	ModeDisabled BridgeMode = "disabled"

	// ModeManualPlugin means ReadSync writes the canonical progress into
	// Calibre's #readsync_progress column (via the Calibre adapter) and
	// the user is expected to manually open Calibre and trigger the
	// Goodreads Sync plugin's "Upload reading progress" command.
	//
	// This is the default and safest mode for v1.
	ModeManualPlugin BridgeMode = "manual-plugin"

	// ModeGuidedPlugin extends manual mode with a UI checklist + a deep
	// link that opens Calibre directly to the right plugin dialog.
	// Functionally identical for the data path; only the UX differs.
	ModeGuidedPlugin BridgeMode = "guided-plugin"

	// ModeCompanionPlugin is a v2 hook for an in-Calibre companion plugin
	// authored by the ReadSync project. The companion plugin would expose
	// a small RPC surface that ReadSync could call to invoke Goodreads
	// Sync programmatically. v1 ships only the configuration plumbing.
	ModeCompanionPlugin BridgeMode = "companion-plugin"

	// ModeExperimentalDirect is a stub for a future direct-Goodreads-API
	// integration. It is GATED behind Config.ExperimentalDirectAck and
	// emits prominent warnings; v1 performs no network calls.
	ModeExperimentalDirect BridgeMode = "experimental-direct"
)

// AllModes returns every defined mode (used by the setup wizard).
func AllModes() []BridgeMode {
	return []BridgeMode{
		ModeDisabled,
		ModeManualPlugin,
		ModeGuidedPlugin,
		ModeCompanionPlugin,
		ModeExperimentalDirect,
	}
}

// IsValid returns true if the mode string is one of the known values.
func (m BridgeMode) IsValid() bool {
	switch m {
	case ModeDisabled, ModeManualPlugin, ModeGuidedPlugin,
		ModeCompanionPlugin, ModeExperimentalDirect:
		return true
	}
	return false
}

// IsActive returns true if this mode emits/consumes events from Calibre's
// #readsync_progress column. Disabled and the v2/experimental hooks do not.
func (m BridgeMode) IsActive() bool {
	return m == ModeManualPlugin || m == ModeGuidedPlugin
}

// HumanLabel returns a short label used in the setup wizard / activity log.
func (m BridgeMode) HumanLabel() string {
	switch m {
	case ModeDisabled:
		return "Disabled"
	case ModeManualPlugin:
		return "Manual (Goodreads Sync plugin, user-triggered)"
	case ModeGuidedPlugin:
		return "Guided (Goodreads Sync plugin with ReadSync checklist)"
	case ModeCompanionPlugin:
		return "Companion plugin (v2, not yet available)"
	case ModeExperimentalDirect:
		return "Experimental — direct Goodreads (NOT IMPLEMENTED, opt-in stub)"
	default:
		return string(m)
	}
}
