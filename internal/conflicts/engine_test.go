// internal/conflicts/engine_test.go

package conflicts

import (
	"testing"
	"time"

	"github.com/readsync/readsync/internal/model"
)

func pf64(f float64) *float64      { return &f }
func pi32(i int32) *int32          { return &i }
func ptime(t time.Time) *time.Time { return &t }

func TestPrecedence(t *testing.T) {
	tests := []struct {
		source model.Source
		want   int
	}{
		{model.SourceCalibre, 1},
		{model.SourceKOReader, 2},
		{model.SourceMoon, 3},
		{model.SourceGoodreadsBridge, 4},
		{model.SourceKindleViaGoodreads, 5},
		{"unknown_source", 99},
	}
	for _, tt := range tests {
		got := Precedence(tt.source)
		if got != tt.want {
			t.Errorf("Precedence(%s)=%d want %d", tt.source, got, tt.want)
		}
	}
}

func TestDetectSuspiciousJump_BackwardJump(t *testing.T) {
	canon := &model.CanonicalProgress{
		PercentComplete: pf64(0.80),
		LocatorType:     model.LocationPercent,
	}
	ev := &model.ProgressEvent{
		Source:             model.SourceKOReader,
		PercentComplete:    pf64(0.60),
		LocatorType:        model.LocationPercent,
		ReadStatus:         model.StatusReading,
		IdentityConfidence: 95,
	}
	result := DetectSuspiciousJump(canon, ev)
	if !result.Suspicious {
		t.Fatal("expected suspicious jump for 80%->60%")
	}
	t.Log("reason:", result.Reason)
}

func TestDetectSuspiciousJump_SmallDecrease_OK(t *testing.T) {
	canon := &model.CanonicalProgress{
		PercentComplete: pf64(0.80),
		LocatorType:     model.LocationPercent,
	}
	ev := &model.ProgressEvent{
		Source:             model.SourceKOReader,
		PercentComplete:    pf64(0.75), // only 5% drop
		LocatorType:        model.LocationPercent,
		ReadStatus:         model.StatusReading,
		IdentityConfidence: 95,
	}
	result := DetectSuspiciousJump(canon, ev)
	if result.Suspicious {
		t.Errorf("5%% drop should not be suspicious, got: %s", result.Reason)
	}
}

func TestDetectSuspiciousJump_GoodreadsFinishedEarly(t *testing.T) {
	canon := &model.CanonicalProgress{
		PercentComplete: pf64(0.50),
		LocatorType:     model.LocationPercent,
	}
	ev := &model.ProgressEvent{
		Source:             model.SourceGoodreadsBridge,
		PercentComplete:    pf64(1.0),
		ReadStatus:         model.StatusFinished,
		LocatorType:        model.LocationPercent,
		IdentityConfidence: 90,
	}
	result := DetectSuspiciousJump(canon, ev)
	if !result.Suspicious {
		t.Fatal("goodreads finished from 50% should be suspicious")
	}
	t.Log("reason:", result.Reason)
}

func TestDetectSuspiciousJump_PageCountChange(t *testing.T) {
	canon := &model.CanonicalProgress{
		PercentComplete: pf64(0.50),
		TotalPages:      pi32(300),
		LocatorType:     model.LocationPage,
	}
	ev := &model.ProgressEvent{
		Source:             model.SourceKOReader,
		PercentComplete:    pf64(0.50),
		TotalPages:         pi32(350),
		LocatorType:        model.LocationPage,
		ReadStatus:         model.StatusReading,
		IdentityConfidence: 90,
	}
	result := DetectSuspiciousJump(canon, ev)
	if !result.Suspicious {
		t.Fatal("page count change should be suspicious")
	}
}

func TestDetectSuspiciousJump_AbandonedOK(t *testing.T) {
	canon := &model.CanonicalProgress{
		PercentComplete: pf64(0.80),
		LocatorType:     model.LocationPercent,
	}
	ev := &model.ProgressEvent{
		Source:             model.SourceKOReader,
		PercentComplete:    pf64(0.10), // big drop but abandoned
		LocatorType:        model.LocationPercent,
		ReadStatus:         model.StatusAbandoned,
		IdentityConfidence: 95,
	}
	result := DetectSuspiciousJump(canon, ev)
	// Abandoned status bypasses backward jump check
	if result.Suspicious && result.Reason == "backward_jump: 80.0% → 10.0%" {
		t.Error("abandoned event should not trigger backward_jump check")
	}
}

func TestChooseWinner_ByPrecedence(t *testing.T) {
	now := time.Now()
	calibre := &model.ProgressEvent{
		Source:     model.SourceCalibre,
		ReceivedAt: now,
	}
	koreader := &model.ProgressEvent{
		Source:     model.SourceKOReader,
		ReceivedAt: now.Add(time.Second),
	}
	winner, reason := ChooseWinner(calibre, koreader)
	if winner != calibre {
		t.Errorf("calibre should beat koreader; reason: %s", reason)
	}
}

func TestChooseWinner_ByTimestamp(t *testing.T) {
	t1 := time.Now()
	t2 := t1.Add(time.Hour)
	a := &model.ProgressEvent{
		Source:     model.SourceKOReader,
		ReceivedAt: t1,
		DeviceTS:   ptime(t1),
	}
	b := &model.ProgressEvent{
		Source:     model.SourceKOReader,
		ReceivedAt: t2,
		DeviceTS:   ptime(t2),
	}
	winner, reason := ChooseWinner(a, b)
	if winner != b {
		t.Errorf("newer timestamp should win; reason: %s", reason)
	}
}

func TestCanAutoResolve(t *testing.T) {
	full := AutoResolveParams{
		TrustworthyTimestamps: true,
		ConfidenceHigh:        true,
		PlausibleMovement:     true,
		WritebackEnabled:      true,
		NoUserPin:             true,
	}
	if !CanAutoResolve(full) {
		t.Error("all conditions met should auto-resolve")
	}

	// Missing one condition.
	p := full
	p.TrustworthyTimestamps = false
	if CanAutoResolve(p) {
		t.Error("missing TrustworthyTimestamps should block auto-resolve")
	}

	p = full
	p.NoUserPin = false
	if CanAutoResolve(p) {
		t.Error("user pin should block auto-resolve")
	}
}
