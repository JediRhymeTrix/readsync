// internal/setup/policy.go
//
// Conflict policy data structure and validation. Surfaces the
// precedence list and suspicious-jump thresholds the wizard's
// PageConflictPolicy collects.

package setup

import (
	"errors"
	"strings"
)

// ConflictPolicy bundles user-configurable conflict-handling settings.
type ConflictPolicy struct {
	// Precedence is the ordered list of source names. Earlier sources
	// win over later ones. Default canonical order is local readers
	// first, manual second, derived (Goodreads/Kindle) last.
	Precedence []string `json:"precedence"`

	// SuspiciousJumpPercent: if a single event moves canonical progress
	// by more than this many percentage points (in absolute terms),
	// raise a conflict instead of auto-applying. Default 30.
	SuspiciousJumpPercent float64 `json:"suspicious_jump_percent"`

	// SuspiciousJumpWindow: minimum time delta between two events
	// where a large jump still counts as "fast" reading. Default 1h.
	SuspiciousJumpWindowMinutes int `json:"suspicious_jump_window_minutes"`

	// FinishedRegressionThreshold: a Goodreads "finished" claim is
	// rejected if local progress is below this percent. Default 85.
	FinishedRegressionThreshold int `json:"finished_regression_threshold"`
}

// DefaultPolicy returns a sensible default precedence + thresholds.
func DefaultPolicy() ConflictPolicy {
	return ConflictPolicy{
		Precedence: []string{
			"koreader", "moon", "calibre", "goodreads_bridge", "kindle_via_goodreads",
		},
		SuspiciousJumpPercent:       30.0,
		SuspiciousJumpWindowMinutes: 60,
		FinishedRegressionThreshold: 85,
	}
}

// ValidPolicySources is the set of source names accepted by Validate.
var ValidPolicySources = map[string]bool{
	"koreader":              true,
	"moon":                  true,
	"calibre":               true,
	"goodreads_bridge":      true,
	"kindle_via_goodreads":  true,
}

// Validate reports any structural problems with the policy.
func (p ConflictPolicy) Validate() error {
	if len(p.Precedence) == 0 {
		return errors.New("policy: precedence must list at least one source")
	}
	seen := map[string]bool{}
	for _, s := range p.Precedence {
		s = strings.TrimSpace(strings.ToLower(s))
		if !ValidPolicySources[s] {
			return errors.New("policy: unknown source: " + s)
		}
		if seen[s] {
			return errors.New("policy: duplicate source: " + s)
		}
		seen[s] = true
	}
	if p.SuspiciousJumpPercent <= 0 || p.SuspiciousJumpPercent > 100 {
		return errors.New("policy: suspicious_jump_percent must be in (0,100]")
	}
	if p.SuspiciousJumpWindowMinutes < 0 {
		return errors.New("policy: suspicious_jump_window_minutes must be >= 0")
	}
	if p.FinishedRegressionThreshold < 0 || p.FinishedRegressionThreshold > 100 {
		return errors.New("policy: finished_regression_threshold must be in [0,100]")
	}
	return nil
}
