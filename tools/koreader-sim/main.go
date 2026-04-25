// tools/koreader-sim/main.go
//
// KOReader Sync Server Simulator — implements KOSync protocol.
// Endpoints:
//   POST /users/create        Register user
//   GET  /users/auth          Authenticate
//   PUT  /syncs/progress      Push progress
//   GET  /syncs/progress/:doc Pull progress
//
// Usage: go run . [--port 7200] [--state state.json] [--verbose]

package main

import (
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"regexp"
	"sync"
)

// User represents a registered user.
type User struct {
	Username string `json:"username"`
	Password string `json:"password"` // MD5 hex
}

// ProgressEntry stores reading progress for a document.
type ProgressEntry struct {
	Document   string  `json:"document"`
	Progress   string  `json:"progress"`
	Percentage float64 `json:"percentage"`
	Device     string  `json:"device"`
	DeviceID   string  `json:"device_id"`
	Timestamp  int64   `json:"timestamp"`
}

// ServerState is the shared in-memory state.
type ServerState struct {
	mu       sync.RWMutex
	Users    map[string]User          `json:"users"`
	Progress map[string]ProgressEntry `json:"progress"` // "username:document"
}

type Server struct {
	state     *ServerState
	stateFile string
	verbose   bool
}

var docKeyRe = regexp.MustCompile(`^[0-9a-fA-F]{64}$`)

func main() {
	port := flag.Int("port", 7200, "Port to listen on")
	stateFile := flag.String("state", "", "JSON state file for persistence")
	verbose := flag.Bool("verbose", false, "Log every request")
	flag.Parse()

	state := &ServerState{
		Users:    make(map[string]User),
		Progress: make(map[string]ProgressEntry),
	}
	if *stateFile != "" {
		if err := loadState(state, *stateFile); err != nil && !os.IsNotExist(err) {
			log.Fatalf("load state: %v", err)
		}
	}

	srv := &Server{state: state, stateFile: *stateFile, verbose: *verbose}
	mux := http.NewServeMux()
	mux.HandleFunc("/users/create", srv.handleUsersCreate)
	mux.HandleFunc("/users/auth", srv.handleUsersAuth)
	mux.HandleFunc("/syncs/progress", srv.handleSyncsPush)
	mux.HandleFunc("/syncs/progress/", srv.handleSyncsPull)

	addr := fmt.Sprintf(":%d", *port)
	log.Printf("KOReader Sync Simulator  http://0.0.0.0%s", addr)
	if err := http.ListenAndServe(addr, mux); err != nil {
		log.Fatal(err)
	}
}
