// internal/repair/validate.go
//
// Input-validation helpers used by repair actions to prevent path traversal
// and command injection before untrusted values reach the filesystem or exec.

package repair

import (
	"fmt"
	"path/filepath"
	"regexp"
	"strings"
)

// firewallNameRE matches names that are safe to embed in a Windows Firewall rule.
// Only ASCII letters, digits, spaces, hyphens, and underscores are allowed.
var firewallNameRE = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9 _\-]*$`)

// cleanAbsPath resolves p to an absolute, clean filesystem path.
// It rejects empty strings and paths that contain ".." components before
// normalization (explicit traversal guard).
func cleanAbsPath(p string) (string, error) {
	if p == "" {
		return "", fmt.Errorf("path is empty")
	}
	// Reject explicit traversal sequences before any normalization.
	if strings.Contains(filepath.ToSlash(p), "..") {
		return "", fmt.Errorf("path traversal not allowed")
	}
	abs, err := filepath.Abs(p)
	if err != nil {
		return "", fmt.Errorf("cannot resolve path: %w", err)
	}
	return abs, nil
}

// validateCalibredbPath checks that p is a safe path to the calibredb
// executable: it must be absolute and its base name must be "calibredb" or
// "calibredb.exe" (case-insensitive). This prevents an attacker from
// substituting an arbitrary binary via the query parameter.
func validateCalibredbPath(p string) (string, error) {
	if p == "" {
		return "", fmt.Errorf("calibredb path is empty")
	}
	clean := filepath.Clean(p)
	if !filepath.IsAbs(clean) {
		return "", fmt.Errorf("calibredb path must be absolute")
	}
	base := strings.ToLower(filepath.Base(clean))
	if base != "calibredb" && base != "calibredb.exe" {
		return "", fmt.Errorf("invalid calibredb executable %q: base name must be calibredb or calibredb.exe", base)
	}
	return clean, nil
}

// validateFirewallName ensures name is safe to embed as a Windows Firewall
// rule name argument to netsh. An empty name is replaced by the default
// "ReadSync". Names with disallowed characters are rejected.
func validateFirewallName(name string) (string, error) {
	if name == "" {
		return "ReadSync", nil
	}
	if !firewallNameRE.MatchString(name) {
		return "", fmt.Errorf("firewall rule name %q contains invalid characters: use letters, digits, spaces, hyphens, or underscores", name)
	}
	return name, nil
}
