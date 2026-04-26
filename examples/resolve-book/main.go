// examples/resolve-book/main.go
//
// Demonstrates the ReadSync identity resolver (internal/resolver).
//
// The resolver maps adapter-supplied evidence (file hash, ISBN, title, etc.)
// to a canonical book ID using a 10-signal confidence ladder.
//
// Run: go run ./examples/resolve-book/
//
// This example does NOT require CGO or a running service.

package main

import (
	"fmt"

	"github.com/readsync/readsync/internal/resolver"
)

func main() {
	fmt.Println("==> ReadSync Identity Resolver Example")
	fmt.Println()

	// Simulated "stored" book record (would come from the DB in production).
	stored := resolver.Evidence{
		FileHash:    "abc123def456abc123def456abc123def456abc123def456abc123def456abc1",
		ISBN13:      "9780441013593",
		Title:       "Dune",
		AuthorSort:  "Herbert, Frank",
		CalibreID:   "42",
	}

	// --- Example 1: Exact file hash match (highest confidence) ---
	fmt.Println("--- Example 1: File hash match ---")
	ev1 := resolver.Evidence{
		FileHash: "abc123def456abc123def456abc123def456abc123def456abc123def456abc1",
	}
	m1 := resolver.Score(ev1, stored)
	printMatch(ev1, m1)

	// --- Example 2: ISBN-13 match ---
	fmt.Println("--- Example 2: ISBN-13 match ---")
	ev2 := resolver.Evidence{
		ISBN13: "9780441013593",
	}
	m2 := resolver.Score(ev2, stored)
	printMatch(ev2, m2)

	// --- Example 3: Fuzzy title + author match ---
	fmt.Println("--- Example 3: Fuzzy title + author match ---")
	ev3 := resolver.Evidence{
		Title:      "Dune",
		AuthorSort: "Herbert, Frank",
	}
	m3 := resolver.Score(ev3, stored)
	printMatch(ev3, m3)

	// --- Example 4: Title only (low confidence) ---
	fmt.Println("--- Example 4: Title only (low confidence) ---")
	ev4 := resolver.Evidence{
		Title: "Dune",
	}
	m4 := resolver.Score(ev4, stored)
	printMatch(ev4, m4)

	// --- Example 5: No match ---
	fmt.Println("--- Example 5: No matching evidence ---")
	ev5 := resolver.Evidence{
		Title: "Foundation",
		ISBN13: "9780553293357",
	}
	m5 := resolver.Score(ev5, stored)
	printMatch(ev5, m5)

	// --- Band explanations ---
	fmt.Println()
	fmt.Println("--- Confidence bands ---")
	for _, score := range []int{100, 95, 85, 70, 55, 30, 0} {
		band := resolver.Band(score)
		wb := resolver.WritebackEnabled(score)
		fmt.Printf("  Score %3d → band %-20s  writeback=%v\n",
			score, bandName(band), wb)
	}
}

func printMatch(ev resolver.Evidence, m resolver.Match) {
	fmt.Printf("  Score: %d  Reason: %s  Band: %s  Writeback: %v\n",
		m.Confidence, m.Reason,
		bandName(resolver.Band(m.Confidence)),
		resolver.WritebackEnabled(m.Confidence))
	fmt.Println()
}

func bandName(b resolver.ConfidenceBand) string {
	switch b {
	case resolver.BandAutoResolve:
		return "AutoResolve (95-100)"
	case resolver.BandWritebackSafe:
		return "WritebackSafe (80-94)"
	case resolver.BandWritebackWary:
		return "WritebackWary (60-79)"
	case resolver.BandUserReview:
		return "UserReview (40-59)"
	default:
		return "Quarantine (0-39)"
	}
}
