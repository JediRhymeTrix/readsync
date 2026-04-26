// internal/adapters/moon/writeback_test.go
//
// Round-trip writer tests gated by the verified-fixture flag.
//
// Layer 4 invariant: writeback is enabled only when (a) a verified writer
// fixture exists for the format AND (b) parse → mutate → serialize →
// reparse equals the expected mutated value.  We test (b) here over both
// synthetic and real-device fixtures.  We assert (a) — IsWriterVerified —
// is currently FALSE, ensuring the adapter falls back to Calibre/KOReader
// writeback as the spec requires until a fixture set is committed.

package moon

import (
	"math"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// TestWriteback_NotVerifiedByDefault confirms FormatV1Plain is gated.
func TestWriteback_NotVerifiedByDefault(t *testing.T) {
	if IsWriterVerified(FormatV1Plain) {
		t.Fatal("FormatV1Plain writer must NOT be verified by default — " +
			"a writer fixture set is required before flipping the flag.")
	}
	if IsWriterVerified(FormatUnknown) {
		t.Error("FormatUnknown should never be verified")
	}
}

// TestWriteback_RoundTrip exercises the round-trip invariant on every
// synthetic + real-device fixture: parse → serialize with mutated pct →
// reparse → compare.
func TestWriteback_RoundTrip(t *testing.T) {
	roots := []string{syntheticDir, captureDir}
	for _, root := range roots {
		entries, err := os.ReadDir(root)
		if err != nil {
			continue // skipped; the parser test handles missing fixtures
		}
		for _, e := range entries {
			if e.IsDir() || !strings.HasSuffix(strings.ToLower(e.Name()), ".po") {
				continue
			}
			path := filepath.Join(root, e.Name())
			t.Run(filepath.Base(root)+"/"+e.Name(), func(t *testing.T) {
				body, err := os.ReadFile(path)
				if err != nil {
					t.Fatalf("read: %v", err)
				}
				orig, err := Parse(e.Name(), body, time.Unix(1700000000, 0))
				if err != nil {
					t.Fatalf("parse orig: %v", err)
				}

				// Mutate the percent to a deterministic new value.
				newPct := 0.5
				if orig.Percent < 0.5 {
					newPct = 0.75
				}
				out, err := SerializeV1Plain(orig, newPct)
				if err != nil {
					t.Fatalf("serialize: %v", err)
				}

				reparsed, err := Parse(e.Name(), out, time.Unix(1700000000, 0))
				if err != nil {
					t.Fatalf("reparse: %v\nout=%q", err, out)
				}
				if reparsed.Format != FormatV1Plain {
					t.Errorf("reparse format: %q", reparsed.Format)
				}
				if math.Abs(reparsed.Percent-newPct) > 0.0011 {
					t.Errorf("reparse pct: want %.4f got %.4f",
						newPct, reparsed.Percent)
				}
				// The book key must be preserved across the round-trip
				// (file_id and basename are unchanged).
				if reparsed.BookKey != orig.BookKey {
					t.Errorf("BookKey changed: %q -> %q",
						orig.BookKey, reparsed.BookKey)
				}
			})
		}
	}
}

// TestSerialize_Edge confirms the writer guards against bad inputs.
func TestSerialize_Edge(t *testing.T) {
	bad := Result{Format: FormatUnknown}
	if _, err := SerializeV1Plain(bad, 0.5); err == nil {
		t.Error("expected error for FormatUnknown")
	}
	good := Result{Format: FormatV1Plain, Position: "12*0@0#0:0.0%"}
	if _, err := SerializeV1Plain(good, -0.1); err == nil {
		t.Error("expected error for negative pct")
	}
	if _, err := SerializeV1Plain(good, 2.0); err == nil {
		t.Error("expected error for pct > 1")
	}
	out, err := SerializeV1Plain(good, 0.5)
	if err != nil {
		t.Fatalf("good serialize: %v", err)
	}
	if !strings.HasSuffix(string(out), ":50%") {
		t.Errorf("expected suffix :50%%, got %q", out)
	}
}
