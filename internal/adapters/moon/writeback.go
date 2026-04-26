// internal/adapters/moon/writeback.go
//
// Layer 4 of the Moon+ adapter: safe writeback generator.
//
// Policy: Moon+ writeback is enabled only when BOTH conditions hold:
//   1. The exact format version has a verified writer fixture committed
//      under fixtures/moonplus/writers/<format>/{input.po,expected.po,
//      mutate.json}, marked verified=true in the writer registry below.
//   2. The round-trip self-test passes at adapter Start time:
//        parse(input)  -> Result A
//        Serialize(A.with(mutated_pct))   -> bytes B
//        parse(B)      -> Result B'
//        B' must equal A with the mutated percent (within tolerance).
//
// If either condition fails the adapter sets MoonWriteback = false and
// records a clear setup warning so the wizard can fall back to Calibre /
// KOReader writebacks. The default in this Phase 4 release is
// writeback DISABLED for FormatV1Plain because we do not ship a verified
// writer fixture matrix yet (round-trip parser tests prove the format is
// reversible, but spec section 11 requires a separate writer fixture
// gate). This conservatism is the explicit Layer 4 invariant.

package moon

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
)

// WriterRegistryEntry describes a Moon+ writer that has been verified by
// fixture round-trip tests and may be used for outbox writeback.
type WriterRegistryEntry struct {
	Format   Format
	Verified bool
	// FixtureDir is the path of the fixtures directory that gates this
	// writer. The adapter does NOT load the fixtures at runtime; the
	// gating is enforced at compile/test time and surfaced as a flag.
	FixtureDir string
	// Notes is a human-readable comment.
	Notes string
}

// writerRegistry is the static list of writers ReadSync knows how to emit.
//
// IMPORTANT: a writer is considered verified ONLY when:
//   - A fixture set lives under FixtureDir, AND
//   - `go test ./internal/adapters/moon/... -run TestWriteback_RoundTrip`
//     passes against those fixtures.
//
// We default Verified=false for FormatV1Plain in Phase 4: the round-trip
// is known reversible (see writeback_test.go) but per the master spec we
// require an explicit fixture-driven writer gate before allowing the
// adapter to emit writebacks. This is the documented degraded fallback.
var writerRegistry = []WriterRegistryEntry{
	{
		Format:     FormatV1Plain,
		Verified:   false,
		FixtureDir: "fixtures/moonplus/writers/po-v1-plain",
		Notes:      "Round-trip safe but requires committed writer fixture set before enable.",
	},
}

// IsWriterVerified reports whether the given format has a verified writer.
func IsWriterVerified(f Format) bool {
	for _, w := range writerRegistry {
		if w.Format == f {
			return w.Verified
		}
	}
	return false
}

// SerializeV1Plain produces the exact byte representation of a
// FormatV1Plain payload from a parsed Result with an updated percent.
//
// The function preserves the file_id / position / chapter / scroll fields
// from the original raw locator (Result.Position) and only mutates the
// trailing ":{pct}%" suffix.  This is the round-trip invariant the writer
// fixtures verify.
//
// Returns an error if Result.Position does not match the FormatV1Plain
// shape — a defensive guard against being asked to serialize a payload
// whose origin we did not parse.
func SerializeV1Plain(r Result, newPercent float64) ([]byte, error) {
	if r.Format != FormatV1Plain {
		return nil, fmt.Errorf("moon: serialize: format mismatch %q", r.Format)
	}
	if newPercent < 0 || newPercent > 1.0001 {
		return nil, fmt.Errorf("moon: serialize: percent out of range %f", newPercent)
	}
	if newPercent > 1.0 {
		newPercent = 1.0
	}
	// Pull the colon position from the original locator.
	colon := strings.LastIndex(r.Position, ":")
	if colon < 0 {
		return nil, errors.New("moon: serialize: no colon in original locator")
	}
	prefix := r.Position[:colon] // includes trailing slot before ":pct%"
	pctNew := strconv.FormatFloat(newPercent*100.0, 'f', -1, 64)
	return []byte(prefix + ":" + pctNew + "%"), nil
}
