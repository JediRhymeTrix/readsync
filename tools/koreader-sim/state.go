// tools/koreader-sim/state.go
// State persistence helpers.

package main

import (
	"encoding/json"
	"log"
	"os"
)

func loadState(state *ServerState, path string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	return json.Unmarshal(data, state)
}

func (s *Server) saveStateUnlocked() {
	if s.stateFile == "" {
		return
	}
	type stateExport struct {
		Users    map[string]User          `json:"users"`
		Progress map[string]ProgressEntry `json:"progress"`
	}
	data, err := json.MarshalIndent(stateExport{
		Users:    s.state.Users,
		Progress: s.state.Progress,
	}, "", "  ")
	if err != nil {
		log.Printf("warn: marshal state: %v", err)
		return
	}
	if err := os.WriteFile(s.stateFile, data, 0600); err != nil {
		log.Printf("warn: write state: %v", err)
	}
}
