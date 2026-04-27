// tools/koreader-sim/handlers_test.go

package main

import (
	"testing"
)

// TestSanitizeLog_KOSim checks that CR and LF are replaced with escape sequences.
func TestSanitizeLog_KOSim(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"alice", "alice"},
		{"user\ninjected", `user\ninjected`},
		{"user\rinjected", `user\rinjected`},
		{"multi\r\nline", `multi\r\nline`},
		{"device\nname\rvalue", `device\nname\rvalue`},
		{"", ""},
	}
	for _, tt := range tests {
		got := sanitizeLog(tt.input)
		if got != tt.want {
			t.Errorf("sanitizeLog(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}
