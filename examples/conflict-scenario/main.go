// examples/conflict-scenario/main.go
//
// Demonstrates the ReadSync conflict detection engine (internal/conflicts).
//
// Shows the 5 suspicious-jump detectors defined in spec §6:
//   1. Backward jump > 10%
//   2. Goodreads reports "finished" from < 85% canonical
//   3. Page count changed
//   4. Identity confidence < 60
//   5. Locator type changed
//
// Run: go run ./examples/conflict-scenario/
//
// This example does NOT require CGO or a running service.

package main

import (
	"fmt"
	"time"

	"github.com/readsync/readsync/internal/conflicts"
	"github.com/readsync/readsync/internal/model"
)

func pct(v float64) *float64 { return &v }
func pages(n int32) *int32   { return &n }

// makeCanon builds a minimal canonical progress record.
func makeCanon(pctComplete float64, locType model.LocationType, totalPages *int32) *model.CanonicalProgress {
	return &model.CanonicalProgress{
		PercentComplete: pct(pctComplete),
		LocatorType:     locType,
		TotalPages:      totalPages,
	}
}

// makeEvent builds a minimal progress event.
func makeEvent(src model.Source, pctComplete float64, status model.ReadStatus,
	locType model.LocationType, confidence int, totalPages *int32) *model.ProgressEvent {
	now := time.Now()
	return &model.ProgressEvent{
		Source:             src,
		PercentComplete:    pct(pctComplete),
		ReadStatus:         status,
		LocatorType:        locType,
		IdentityConfidence: confidence,
		TotalPages:         totalPages,
		ReceivedAt:         now,
	}
}

func main() {
	fmt.Println("==> ReadSync Conflict Detection Example")
	fmt.Println()

	// --- Detector 1: Backward jump > 10% ---
	fmt.Println("--- Detector 1: Backward jump > 10% ---")
	canon1 := makeCanon(0.72, model.LocationPercent, nil)
	ev1 := makeEvent(model.SourceKOReader, 0.55, model.StatusReading, model.LocationPercent, 80, nil)
	result1 := conflicts.DetectSuspiciousJump(canon1, ev1)
	printResult(result1, "canon=72%, new=55%")

	// Not suspicious: small backward jump (5%)
	ev1b := makeEvent(model.SourceKOReader, 0.67, model.StatusReading, model.LocationPercent, 80, nil)
	result1b := conflicts.DetectSuspiciousJump(canon1, ev1b)
	printResult(result1b, "canon=72%, new=67% (small backward — not suspicious)")

	// Not suspicious: backward jump but status=abandoned
	ev1c := makeEvent(model.SourceKOReader, 0.10, model.StatusAbandoned, model.LocationPercent, 80, nil)
	result1c := conflicts.DetectSuspiciousJump(canon1, ev1c)
	printResult(result1c, "canon=72%, new=10% but Abandoned (not suspicious)")

	// --- Detector 2: Goodreads says "finished" but canonical is < 85% ---
	fmt.Println("--- Detector 2: Goodreads finished < 85% canonical ---")
	canon2 := makeCanon(0.38, model.LocationPercent, nil)
	ev2 := makeEvent(model.SourceGoodreadsBridge, 1.0, model.StatusFinished, model.LocationPercent, 90, nil)
	result2 := conflicts.DetectSuspiciousJump(canon2, ev2)
	printResult(result2, "Goodreads=finished, canonical=38%")

	// --- Detector 3: Page count changed ---
	fmt.Println("--- Detector 3: Page count changed ---")
	canon3 := makeCanon(0.50, model.LocationPage, pages(450))
	ev3 := makeEvent(model.SourceCalibre, 0.52, model.StatusReading, model.LocationPage, 95, pages(512))
	result3 := conflicts.DetectSuspiciousJump(canon3, ev3)
	printResult(result3, "pages: 450 → 512")

	// --- Detector 4: Low identity confidence ---
	fmt.Println("--- Detector 4: Identity confidence < 60 ---")
	canon4 := makeCanon(0.60, model.LocationPercent, nil)
	ev4 := makeEvent(model.SourceMoon, 0.62, model.StatusReading, model.LocationPercent, 30, nil)
	result4 := conflicts.DetectSuspiciousJump(canon4, ev4)
	printResult(result4, "confidence=30 (title-only match)")

	// --- Detector 5: Locator type changed ---
	fmt.Println("--- Detector 5: Locator type changed ---")
	canon5 := makeCanon(0.40, model.LocationKOReaderXPtr, nil)
	ev5 := makeEvent(model.SourceMoon, 0.42, model.StatusReading, model.LocationMoonPosition, 70, nil)
	result5 := conflicts.DetectSuspiciousJump(canon5, ev5)
	printResult(result5, "locator: koreader_xpointer → moon_position")

	// --- Spec §6 three-way scenario ---
	fmt.Println("--- Spec §6 scenario: KOReader 72% / Calibre 70% / Goodreads 38% ---")
	canon6 := makeCanon(0.72, model.LocationKOReaderXPtr, nil)
	evGR := makeEvent(model.SourceGoodreadsBridge, 0.38, model.StatusFinished, model.LocationPercent, 90, nil)
	result6 := conflicts.DetectSuspiciousJump(canon6, evGR)
	printResult(result6, "Goodreads=finished(38%), KOReader canonical=72%")

	// --- Auto-resolve gate ---
	fmt.Println("--- Auto-resolve gate ---")
	params := conflicts.AutoResolveParams{
		TrustworthyTimestamps: true,
		ConfidenceHigh:        true,
		PlausibleMovement:     true,
		WritebackEnabled:      true,
		NoUserPin:             true,
	}
	fmt.Printf("  All conditions met: CanAutoResolve=%v\n", conflicts.CanAutoResolve(params))

	params.PlausibleMovement = false
	fmt.Printf("  SuspiciousJump detected: CanAutoResolve=%v\n", conflicts.CanAutoResolve(params))
}

func printResult(r conflicts.SuspiciousJump, label string) {
	if r.Suspicious {
		fmt.Printf("  ⚠ SUSPICIOUS  %s\n    Reason: %s\n", label, r.Reason)
	} else {
		fmt.Printf("  ✓ OK          %s\n", label)
	}
	fmt.Println()
}
