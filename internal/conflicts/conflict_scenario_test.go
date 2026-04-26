// internal/conflicts/conflict_scenario_test.go
//
// Tests the spec §6 conflict scenario:
//   KOReader 72%, Calibre 70%, Goodreads 38% (claims finished).

package conflicts

import (
	"testing"
	"time"

	"github.com/readsync/readsync/internal/model"
)

func fp(f float64) *float64 { return &f }

// TestSpecSection6_ConflictResolution tests the scenario from spec §6.
func TestSpecSection6_ConflictResolution(t *testing.T) {
	now := time.Now()
	t1 := now.Add(-3 * time.Hour)
	t2 := now.Add(-2 * time.Hour)
	t3 := now.Add(-1 * time.Hour)

	calibreEv := &model.ProgressEvent{
		Source:             model.SourceCalibre,
		PercentComplete:    fp(0.70),
		ReadStatus:         model.StatusReading,
		IdentityConfidence: 95,
		ReceivedAt:         t1,
		LocatorType:        model.LocationPercent,
	}
	koreaderEv := &model.ProgressEvent{
		Source:             model.SourceKOReader,
		PercentComplete:    fp(0.72),
		ReadStatus:         model.StatusReading,
		IdentityConfidence: 70,
		ReceivedAt:         t2,
		LocatorType:        model.LocationPercent,
	}
	goodreadsEv := &model.ProgressEvent{
		Source:             model.SourceGoodreadsBridge,
		PercentComplete:    fp(1.0), // says finished
		ReadStatus:         model.StatusFinished,
		IdentityConfidence: 90,
		ReceivedAt:         t3,
		LocatorType:        model.LocationPercent,
	}

	// Calibre wins over KOReader by precedence.
	winner, reason := ChooseWinner(calibreEv, koreaderEv)
	if winner != calibreEv {
		t.Errorf("calibre should beat koreader by precedence; got %s (%s)", winner.Source, reason)
	}

	// KOReader wins over Goodreads by precedence.
	winner2, reason2 := ChooseWinner(koreaderEv, goodreadsEv)
	if winner2 != koreaderEv {
		t.Errorf("koreader should beat goodreads; got %s (%s)", winner2.Source, reason2)
	}

	// Goodreads "finished" from 38% is suspicious.
	canon38 := &model.CanonicalProgress{PercentComplete: fp(0.38), LocatorType: model.LocationPercent}
	jump := DetectSuspiciousJump(canon38, goodreadsEv)
	if !jump.Suspicious {
		t.Error("goodreads finished from 38% must be suspicious (below 85% threshold)")
	}

	// 72% vs 70% is NOT suspicious (only 2% forward advance).
	canon70 := &model.CanonicalProgress{PercentComplete: fp(0.70), LocatorType: model.LocationPercent}
	jumpSmall := DetectSuspiciousJump(canon70, koreaderEv)
	if jumpSmall.Suspicious {
		t.Errorf("2%% difference should not be suspicious: %s", jumpSmall.Reason)
	}

	// Auto-resolve blocked for Goodreads (suspicious movement).
	if CanAutoResolve(AutoResolveParams{
		TrustworthyTimestamps: true,
		ConfidenceHigh:        true,
		PlausibleMovement:     false, // suspicious
		WritebackEnabled:      true,
		NoUserPin:             true,
	}) {
		t.Error("auto-resolve must be blocked when PlausibleMovement=false")
	}

	// Auto-resolve blocked for KOReader (confidence 70 < 80).
	if CanAutoResolve(AutoResolveParams{
		TrustworthyTimestamps: true,
		ConfidenceHigh:        false, // 70 < 80
		PlausibleMovement:     true,
		WritebackEnabled:      true,
		NoUserPin:             true,
	}) {
		t.Error("auto-resolve must be blocked when ConfidenceHigh=false")
	}
}

// TestSuspiciousJump_AllDetectors covers all 5 spec §6 detectors.
func TestSuspiciousJump_AllDetectors(t *testing.T) {
	t.Run("backward_jump>10pct", func(t *testing.T) {
		canon := &model.CanonicalProgress{PercentComplete: fp(0.80)}
		ev := &model.ProgressEvent{
			Source: model.SourceKOReader, PercentComplete: fp(0.60),
			ReadStatus: model.StatusReading, IdentityConfidence: 95,
		}
		if r := DetectSuspiciousJump(canon, ev); !r.Suspicious {
			t.Error("80%→60% must be suspicious")
		}
	})

	t.Run("goodreads_finished_early", func(t *testing.T) {
		canon := &model.CanonicalProgress{PercentComplete: fp(0.50)}
		ev := &model.ProgressEvent{
			Source: model.SourceGoodreadsBridge, PercentComplete: fp(1.0),
			ReadStatus: model.StatusFinished, IdentityConfidence: 90,
		}
		if r := DetectSuspiciousJump(canon, ev); !r.Suspicious {
			t.Error("goodreads finished from 50% must be suspicious")
		}
	})

	t.Run("page_count_change", func(t *testing.T) {
		p300, p350 := int32(300), int32(350)
		canon := &model.CanonicalProgress{TotalPages: &p300, PercentComplete: fp(0.5)}
		ev := &model.ProgressEvent{
			Source: model.SourceKOReader, TotalPages: &p350,
			PercentComplete: fp(0.5), ReadStatus: model.StatusReading, IdentityConfidence: 90,
		}
		if r := DetectSuspiciousJump(canon, ev); !r.Suspicious {
			t.Error("page count change 300→350 must be suspicious")
		}
	})

	t.Run("low_confidence<60", func(t *testing.T) {
		canon := &model.CanonicalProgress{PercentComplete: fp(0.5)}
		ev := &model.ProgressEvent{
			Source: model.SourceMoon, PercentComplete: fp(0.52),
			ReadStatus: model.StatusReading, IdentityConfidence: 55,
		}
		if r := DetectSuspiciousJump(canon, ev); !r.Suspicious {
			t.Error("identity confidence 55 (<60) must be suspicious")
		}
	})

	t.Run("locator_type_change", func(t *testing.T) {
		canon := &model.CanonicalProgress{
			PercentComplete: fp(0.5), LocatorType: model.LocationPercent,
		}
		ev := &model.ProgressEvent{
			Source: model.SourceKOReader, PercentComplete: fp(0.52),
			ReadStatus: model.StatusReading, IdentityConfidence: 90,
			LocatorType: model.LocationKOReaderXPtr,
		}
		if r := DetectSuspiciousJump(canon, ev); !r.Suspicious {
			t.Error("locator type change percent→koreader_xpointer must be suspicious")
		}
	})

	t.Run("abandoned_bypasses_backward_check", func(t *testing.T) {
		canon := &model.CanonicalProgress{PercentComplete: fp(0.80)}
		ev := &model.ProgressEvent{
			Source: model.SourceKOReader, PercentComplete: fp(0.10),
			ReadStatus: model.StatusAbandoned, IdentityConfidence: 95,
		}
		r := DetectSuspiciousJump(canon, ev)
		if r.Suspicious && r.Reason == "backward_jump: 80.0% → 10.0%" {
			t.Error("abandoned event must not trigger backward_jump check")
		}
	})
}
