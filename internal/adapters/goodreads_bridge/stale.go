// internal/adapters/goodreads_bridge/stale.go
//
// Stale-state detection and writeback safety gates for Goodreads-derived
// progress events (master spec sections 6, 8 and 9).
//
// The Goodreads Sync plugin can write back into Calibre's columns with
// data that originated on Kindle / the Goodreads website. Those events
// are LOW CONFIDENCE because:
//
//   1. Timestamps are rough (Goodreads timeline granularity is days).
//   2. The user may have updated their shelf to "read" without actually
//      finishing the book on a local reader.
//   3. The Goodreads progress is subject to manual edits and offline lag.
//
// We therefore treat Goodreads-derived events as advisory: we surface a
// CONFLICT instead of overwriting a fresher local reader event.

package goodreads_bridge

import (
	"fmt"
	"time"

	"github.com/readsync/readsync/internal/model"
)

// LocalChangeRecency is the window during which a *local* reader event is
// considered "fresh" — Goodreads-derived data that arrives during this
// window must NEVER overwrite the local value.
const LocalChangeRecency = 24 * time.Hour

// MinConfidenceForWriteback is the confidence threshold required for a
// Goodreads-derived event to be written into canonical (spec §8).
const MinConfidenceForWriteback = 90

// FinishedRegressionThreshold is the canonical-progress floor below
// which a Goodreads "finished" claim must be treated as suspicious
// (spec §6 example).
const FinishedRegressionThreshold = 0.85

// GoodreadsObservation is the subset of fields the bridge extracts from
// Calibre's #readsync_progress / #readsync_gr_shelf columns when the
// Goodreads Sync plugin has written into them.
type GoodreadsObservation struct {
	BookID int64

	// PercentComplete is 0.0–1.0 (nil when unknown).
	PercentComplete *float64

	// Shelf is "currently-reading" / "read" / "to-read" / "" (none).
	Shelf string

	// DeviceTS is the timestamp the plugin recorded for this observation.
	DeviceTS *time.Time

	// IdentityConfidence is the resolver confidence for the book mapping.
	IdentityConfidence int
}

// StaleResult is the outcome of stale-state detection.
type StaleResult struct {
	// Stale is true when a conflict must be raised instead of writing
	// the canonical record.
	Stale bool

	// Reason is a short machine-readable label for the activity log.
	Reason string

	// Detail is a longer human description (used for the conflict row).
	Detail string
}

// DetectStaleFinished implements the spec section 6 example: "Goodreads
// says finished, local readers say <85%". When the canonical record
// shows the book is below 85% and a Goodreads-derived event marks it as
// finished (shelf="read" or PercentComplete >= 1.0), we DO NOT auto-finish
// — instead the caller must raise a conflict.
func DetectStaleFinished(canon *model.CanonicalProgress, obs GoodreadsObservation) StaleResult {
	if canon == nil {
		return StaleResult{}
	}
	finishedClaim := obs.Shelf == "read"
	if obs.PercentComplete != nil && *obs.PercentComplete >= 1.0 {
		finishedClaim = true
	}
	if !finishedClaim {
		return StaleResult{}
	}
	if canon.PercentComplete == nil {
		// We have no local progress to compare against — defer to the
		// generic confidence gate rather than raising a stale conflict.
		return StaleResult{}
	}
	if *canon.PercentComplete >= FinishedRegressionThreshold {
		return StaleResult{}
	}
	return StaleResult{
		Stale:  true,
		Reason: "goodreads_bridge_stale",
		Detail: fmt.Sprintf("Goodreads bridge stale: shelf reports finished but local progress is %.1f%% (< %.0f%%)",
			*canon.PercentComplete*100, FinishedRegressionThreshold*100),
	}
}

// WritebackDecision describes whether a Goodreads-derived event may
// update the canonical record.
type WritebackDecision struct {
	// Allow is true only when ALL gates of spec §8 pass.
	Allow bool

	// Reason captures which gate (if any) blocked the writeback.
	Reason string
}

// EvaluateWriteback applies the spec §8 safety rules:
//
//   - Confidence ≥ 90.
//   - DeviceTS is present and not implausibly far in the future.
//   - The canonical record was NOT updated by a non-Goodreads source
//     within LocalChangeRecency.
//   - The event is not a stale regression (DetectStaleFinished negative).
//
// canon may be nil (no prior canonical record); in that case only the
// confidence + timestamp gates apply.
func EvaluateWriteback(canon *model.CanonicalProgress, obs GoodreadsObservation, now time.Time) WritebackDecision {
	if obs.IdentityConfidence < MinConfidenceForWriteback {
		return WritebackDecision{Reason: fmt.Sprintf(
			"confidence_below_gate: %d < %d", obs.IdentityConfidence, MinConfidenceForWriteback)}
	}
	if obs.DeviceTS == nil {
		return WritebackDecision{Reason: "missing_device_ts"}
	}
	// Reject device timestamps more than 1 hour in the future to defend
	// against clock-skew based regressions.
	if obs.DeviceTS.After(now.Add(1 * time.Hour)) {
		return WritebackDecision{Reason: "device_ts_in_future"}
	}
	if canon != nil {
		// If a *non*-Goodreads source updated canonical recently, that
		// local reader event takes precedence (spec §8 default precedence:
		// local reader > Calibre manual > Goodreads/Kindle-derived).
		if canon.UpdatedBy != model.SourceGoodreadsBridge &&
			canon.UpdatedBy != model.SourceKindleViaGoodreads &&
			now.Sub(canon.UpdatedAt) < LocalChangeRecency {
			return WritebackDecision{Reason: fmt.Sprintf(
				"recent_local_change: canonical updated by %s %.0fm ago",
				canon.UpdatedBy, now.Sub(canon.UpdatedAt).Minutes())}
		}
		if stale := DetectStaleFinished(canon, obs); stale.Stale {
			return WritebackDecision{Reason: stale.Reason}
		}
	}
	return WritebackDecision{Allow: true, Reason: "ok"}
}
