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
		{"alice", "616c696365"},
		{"user\ninjected", `757365720a696e6a6563746564`},
		{"user\rinjected", `757365720d696e6a6563746564`},
		{"multi\r\nline", `6d756c74690d0a6c696e65`},
		{"device\nname\rvalue", `6465766963650a6e616d650d76616c7565`},
		{"", ""},
	}
	for _, tt := range tests {
		got := sanitizeLog(tt.input)
		if got != tt.want {
			t.Errorf("sanitizeLog(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}
