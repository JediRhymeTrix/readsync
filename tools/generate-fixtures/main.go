// tools/generate-fixtures/main.go
//
// Generates all synthetic fixtures needed for CI testing:
// - fixtures/moonplus/synthetic/*.po  (Moon+ plain-text progress files)
//
// This is idempotent: re-running produces identical output.
//
// Usage:
//   go run . [--root ROOT]
//   ROOT defaults to ../.. (the ReadSync repo root)

package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"
)

func main() {
	root := flag.String("root", filepath.Join("..", ".."), "ReadSync repo root")
	flag.Parse()

	if err := generateMoonPlusSynthetic(*root); err != nil {
		log.Fatalf("generate moonplus: %v", err)
	}
	log.Println("All fixtures generated successfully.")
}

func generateMoonPlusSynthetic(root string) error {
	outDir := filepath.Join(root, "fixtures", "moonplus", "synthetic")
	if err := os.MkdirAll(outDir, 0755); err != nil {
		return fmt.Errorf("mkdir %q: %w", outDir, err)
	}

	// Real format confirmed from live Moon+ Pro v9 captures 2026-04-25.
	// Plain UTF-8 text: {file_id}*{position}@{chapter}#{scroll}:{percentage}%
	const fileID = "1703471974608"

	levels := []struct {
		label      string
		position   int
		chapter    int
		scroll     int
		percentage string
	}{
		{"010pct", 5, 0, 8192, "10.0"},
		{"025pct", 12, 0, 33241, "25.0"},
		{"050pct", 28, 1, 16384, "50.0"},
		{"075pct", 42, 2, 4096, "75.0"},
		{"100pct", 52, 0, 9486, "100"},
	}

	for _, lv := range levels {
		content := fmt.Sprintf("%s*%d@%d#%d:%s%%",
			fileID, lv.position, lv.chapter, lv.scroll, lv.percentage)
		path := filepath.Join(outDir, lv.label+".po")
		if err := os.WriteFile(path, []byte(content), 0644); err != nil {
			return fmt.Errorf("write %q: %w", path, err)
		}
		log.Printf("  moonplus/synthetic/%s.po  %q", lv.label+".po", content)
	}
	return nil
}
