// tools/koreader-sim/handlers_test.go

package main

import (
	"testing"
)

// TestSanitizeLog_KOSim checks that CR and LF are stripped from log strings.
func TestSanitizeLog_KOSim(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"alice", "alice"},
		{"user\ninjected", "user injected"},
		{"user\rinjected", "user injected"},
		{"multi\r\nline", "multi  line"},
		{"device\nname\rvalue", "device name value"},
		{"", ""},
	}
	for _, tt := range tests {
		got := sanitizeLog(tt.input)
		if got != tt.want {
			t.Errorf("sanitizeLog(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}
