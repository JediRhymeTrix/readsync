// tools/moon-fixture-recorder/main.go
//
// Moon+ WebDAV Fixture Recorder
//
// Runs an embedded WebDAV server that accepts Moon+ Pro sync uploads.
// Every PUT is saved with a timestamp suffix for versioning.
// All PROPFIND/PUT/GET/MKCOL operations are logged.
//
// Usage:
//   go run . [--port 8765] [--capture-dir DIR] [--verbose]
//
// Default capture dir: ../../fixtures/moonplus/captures
//
// Point Moon+ Pro to: http://<YOUR_IP>:8765/dav/

package main

import (
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
)

func main() {
	port := flag.Int("port", 8765, "Port to listen on")
	captureDir := flag.String("capture-dir",
		filepath.Join("..", "..", "fixtures", "moonplus", "captures"),
		"Directory to save captured .po files")
	verbose := flag.Bool("verbose", false, "Log all WebDAV request details")
	flag.Parse()

	// Ensure capture directory exists.
	if err := os.MkdirAll(*captureDir, 0755); err != nil {
		log.Fatalf("Failed to create capture dir %q: %v", *captureDir, err)
	}

	// WebDAV root directory (served files live here).
	davRoot := filepath.Join(os.TempDir(), "readsync-moon-dav")
	if err := os.MkdirAll(davRoot, 0755); err != nil {
		log.Fatalf("Failed to create DAV root %q: %v", davRoot, err)
	}

	recorder := NewRecorder(*captureDir, davRoot, *verbose)
	handler := recorder.Handler()

	addr := fmt.Sprintf(":%d", *port)
	log.Printf("Moon+ WebDAV Fixture Recorder")
	log.Printf("  Listening:   http://0.0.0.0%s/dav/", addr)
	log.Printf("  Capture dir: %s", *captureDir)
	log.Printf("  DAV root:    %s", davRoot)
	log.Printf("")
	log.Printf("Configure Moon+ Pro:")
	log.Printf("  Settings → Sync → WebDAV → URL: http://<YOUR_IP>%s/dav/", addr)
	log.Printf("  Sync folder: /moonreader/")
	log.Printf("")
	log.Printf("Waiting for Moon+ connections...")

	if err := http.ListenAndServe(addr, handler); err != nil {
		log.Fatal(err)
	}
}
