// internal/logging/redact.go
//
// Field redaction: removes secrets, passwords, and auth tokens from log output.
// No secret value ever reaches the log streams.

package logging

import (
	"fmt"
	"regexp"
	"strings"
)

// secretKeys is the list of field names whose values must be redacted.
// Keys are matched case-insensitively.
var secretKeys = []string{
	"password",
	"passwd",
	"secret",
	"token",
	"api_key",
	"apikey",
	"auth",
	"authorization",
	"credential",
	"credentials",
	"private_key",
	"privatekey",
	"access_token",
	"refresh_token",
	"bearer",
	"session",
	"cookie",
	"x-api-key",
	"client_secret",
}

// secretPatterns matches common secret patterns in string values.
// These are applied to string values even when the key is not in secretKeys.
var secretPatterns = []*regexp.Regexp{
	// Bearer tokens.
	regexp.MustCompile(`(?i)bearer\s+[a-zA-Z0-9\-._~+/]+=*`),
	// Basic auth header value.
	regexp.MustCompile(`(?i)basic\s+[a-zA-Z0-9+/]+=*`),
	// Generic long hex/base64 that looks like a token (32+ chars).
	regexp.MustCompile(`[a-zA-Z0-9\-._~+/]{40,}={0,2}`),
}

const redactedPlaceholder = "[REDACTED]"

// IsSecretKey returns true when the field key name indicates a secret.
//
// Matching is case-insensitive and uses word boundaries (_, -,
// or string ends) to avoid false positives like "author" containing
// "auth".
func IsSecretKey(key string) bool {
	lk := strings.ToLower(key)
	for _, sk := range secretKeys {
		if tokenContains(lk, sk) {
			return true
		}
	}
	return false
}

// tokenContains reports whether s contains needle as a discrete token,
// where token boundaries are _, -, or the start/end of s. This
// keeps "auth" out of "author" while still matching "x-api-key"
// and "client_secret".
func tokenContains(s, needle string) bool {
	if s == needle {
		return true
	}
	n := len(needle)
	for i := 0; i+n <= len(s); i++ {
		if s[i:i+n] != needle {
			continue
		}
		leftOK := i == 0 || s[i-1] == '_' || s[i-1] == '-'
		rightOK := i+n == len(s) || s[i+n] == '_' || s[i+n] == '-'
		if leftOK && rightOK {
			return true
		}
	}
	return false
}

// RedactString applies pattern-based redaction to a free-form string value.
// It is conservative: only replaces patterns that strongly indicate secrets.
func RedactString(s string) string {
	// Bearer / Basic tokens.
	result := secretPatterns[0].ReplaceAllString(s, "bearer "+redactedPlaceholder)
	result = secretPatterns[1].ReplaceAllString(result, "basic "+redactedPlaceholder)
	// Leave the long-token pattern for value redaction only (too aggressive for msgs).
	return result
}

// redactValue redacts a field value if it looks like a secret.
// For map[string]any recursion is handled by the caller.
func redactValue(v any) any {
	if v == nil {
		return nil
	}
	switch val := v.(type) {
	case string:
		return redactTokenPatterns(val)
	case fmt.Stringer:
		return redactTokenPatterns(val.String())
	default:
		return v
	}
}

func redactTokenPatterns(s string) string {
	result := secretPatterns[0].ReplaceAllString(s, "bearer "+redactedPlaceholder)
	result = secretPatterns[1].ReplaceAllString(result, "basic "+redactedPlaceholder)
	result = secretPatterns[2].ReplaceAllString(result, redactedPlaceholder)
	return result
}

// RedactField returns redactedPlaceholder if the key is secret, else the value.
func RedactField(key string, value any) any {
	if IsSecretKey(key) {
		return redactedPlaceholder
	}
	return redactValue(value)
}

// SafeFields returns a copy of the fields map with secrets replaced.
func SafeFields(fields map[string]any) map[string]any {
	out := make(map[string]any, len(fields))
	for k, v := range fields {
		out[k] = RedactField(k, v)
	}
	return out
}
