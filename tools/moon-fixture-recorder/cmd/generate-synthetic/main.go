// tools/moon-fixture-recorder/cmd/generate-synthetic/main.go
//
// Generates synthetic Moon+ Reader Pro .po files in the real plain-text format
// confirmed from live device captures on 2026-04-25.
//
// Format: {file_id}*{position}@{chapter}#{scroll}:{percentage}%
//
// Usage: go run . [--out DIR]
// Output: 010pct.po, 025pct.po, 050pct.po, 075pct.po, 100pct.po

package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"
)

func main() {
	outDir := flag.String("out", filepath.Join("..", "..", "..", "..", "fixtures", "moonplus", "synthetic"),
		"Output directory for synthetic .po files")
	flag.Parse()

	if err := os.MkdirAll(*outDir, 0755); err != nil {
		log.Fatalf("mkdir %q: %v", *outDir, err)
	}

	// file_id: millisecond mtime of a synthetic test EPUB (fixed for reproducibility).
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
		name := lv.label + ".po"
		path := filepath.Join(*outDir, name)
		if err := os.WriteFile(path, []byte(content), 0644); err != nil {
			log.Fatalf("write %q: %v", path, err)
		}
		log.Printf("Generated %-12s  %q", name, content)
	}
	log.Printf("Done. Synthetic .po files written to %s", *outDir)
}
