// internal/conflicts/engine.go
//
// Conflict detection and resolution engine (spec §6).
//
// Default precedence (lower number = higher trust):
//   1. calibre         (ground truth for metadata)
//   2. koreader        (reliable timestamps, continuous sync)
//   3. moon            (reliable but WebDAV is fire-and-forget)
//   4. goodreads_bridge (delayed, proxy-based)
//   5. kindle_via_goodreads (least trusted: data via Goodreads API)

package conflicts

import (
	"fmt"
	"time"

	"github.com/readsync/readsync/internal/model"
)

// Precedence returns the trust rank of a source (lower = more trusted).
// Sources not in the list get rank 99.
func Precedence(s model.Source) int {
	switch s {
	case model.SourceCalibre:
		return 1
	case model.SourceKOReader:
		return 2
	case model.SourceMoon:
		return 3
	case model.SourceGoodreadsBridge:
		return 4
	case model.SourceKindleViaGoodreads:
		return 5
	default:
		return 99
	}
}

// SuspiciousJump holds the result of jump detection.
type SuspiciousJump struct {
	Suspicious bool
	Reason     string
}

// DetectSuspiciousJump checks whether a new event is suspicious relative to
// the current canonical progress.
//
// Suspicious conditions (spec §6):
//  1. Progress decreases by > 10% (backward jump without abandon/restart).
//  2. Goodreads reports finished from < 85% canonical.
//  3. Page count changed (different total_pages).
//  4. Identity confidence dropped significantly (> 15 points).
//  5. Raw locator type changed (format inconsistency).
func DetectSuspiciousJump(canon *model.CanonicalProgress, ev *model.ProgressEvent) SuspiciousJump {
	if canon == nil {
		return SuspiciousJump{}
	}

	// 1. Backward jump > 10%.
	if canon.PercentComplete != nil && ev.PercentComplete != nil {
		delta := *canon.PercentComplete - *ev.PercentComplete
		if delta > 0.10 &&
			ev.ReadStatus != model.StatusAbandoned &&
			ev.ReadStatus != model.StatusNotStarted {
			return SuspiciousJump{
				Suspicious: true,
				Reason: fmt.Sprintf("backward_jump: %.1f%% → %.1f%%",
					*canon.PercentComplete*100, *ev.PercentComplete*100),
			}
		}
	}

	// 2. Goodreads finished from < 85%.
	if ev.Source == model.SourceGoodreadsBridge &&
		ev.ReadStatus == model.StatusFinished &&
		canon.PercentComplete != nil && *canon.PercentComplete < 0.85 {
		return SuspiciousJump{
			Suspicious: true,
			Reason: fmt.Sprintf("goodreads_finished_early: canonical_pct=%.1f%%",
				*canon.PercentComplete*100),
		}
	}

	// 3. Page count changed.
	if canon.TotalPages != nil && ev.TotalPages != nil &&
		*canon.TotalPages != *ev.TotalPages && *canon.TotalPages > 0 {
		return SuspiciousJump{
			Suspicious: true,
			Reason: fmt.Sprintf("page_count_change: %d → %d",
				*canon.TotalPages, *ev.TotalPages),
		}
	}

	// 4. Identity confidence too low to trust this event.
	if ev.IdentityConfidence > 0 && ev.IdentityConfidence < 60 {
		return SuspiciousJump{
			Suspicious: true,
			Reason:     fmt.Sprintf("low_confidence: %d", ev.IdentityConfidence),
		}
	}

	// 5. Locator type changed.
	if canon.LocatorType != "" && ev.LocatorType != "" &&
		canon.LocatorType != ev.LocatorType &&
		canon.LocatorType != model.LocationRaw && ev.LocatorType != model.LocationRaw {
		return SuspiciousJump{
			Suspicious: true,
			Reason: fmt.Sprintf("locator_type_change: %s → %s",
				canon.LocatorType, ev.LocatorType),
		}
	}

	return SuspiciousJump{}
}

// AutoResolveParams captures all conditions needed to decide whether a
// conflict can be auto-resolved without user intervention.
type AutoResolveParams struct {
	// TrustworthyTimestamps: both events have device timestamps and they are
	// monotonically ordered (the newer one actually happened later).
	TrustworthyTimestamps bool

	// ConfidenceHigh: identity confidence >= 80 for the winning event.
	ConfidenceHigh bool

	// PlausibleMovement: no suspicious jump detected.
	PlausibleMovement bool

	// WritebackEnabled: confidence band permits writeback.
	WritebackEnabled bool

	// NoUserPin: the canonical row is not user-pinned.
	NoUserPin bool
}

// CanAutoResolve returns true when all gating conditions are met.
func CanAutoResolve(p AutoResolveParams) bool {
	return p.TrustworthyTimestamps &&
		p.ConfidenceHigh &&
		p.PlausibleMovement &&
		p.WritebackEnabled &&
		p.NoUserPin
}

// ChooseWinner picks which of two events should be canonical when there is a
// conflict. It returns the winning event and the reason.
//
// Resolution order:
//  1. User-pinned value always wins (handled upstream, not here).
//  2. Higher source precedence wins.
//  3. If equal precedence, newer device timestamp wins.
//  4. If timestamps unavailable, newer received_at wins.
func ChooseWinner(a, b *model.ProgressEvent) (*model.ProgressEvent, string) {
	pa, pb := Precedence(a.Source), Precedence(b.Source)
	if pa < pb {
		return a, fmt.Sprintf("source_precedence: %s(%d) > %s(%d)", a.Source, pa, b.Source, pb)
	}
	if pb < pa {
		return b, fmt.Sprintf("source_precedence: %s(%d) > %s(%d)", b.Source, pb, a.Source, pa)
	}

	// Same precedence: use device timestamp if available.
	aTS := effectiveTime(a)
	bTS := effectiveTime(b)
	if aTS.After(bTS) {
		return a, fmt.Sprintf("newer_timestamp: %s vs %s", aTS.Format(time.RFC3339), bTS.Format(time.RFC3339))
	}
	return b, fmt.Sprintf("newer_timestamp: %s vs %s", bTS.Format(time.RFC3339), aTS.Format(time.RFC3339))
}

func effectiveTime(ev *model.ProgressEvent) time.Time {
	if ev.DeviceTS != nil {
		return *ev.DeviceTS
	}
	return ev.ReceivedAt
}
