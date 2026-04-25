// internal/logging/redact_test.go
//
// Unit tests ensuring that no secret values survive logging.

package logging

import (
	"bytes"
	"strings"
	"testing"
)

func TestIsSecretKey(t *testing.T) {
	mustBeSecret := []string{
		"password", "PASSWORD", "Password",
		"passwd",
		"secret", "client_secret",
		"token", "access_token", "refresh_token", "api_token",
		"api_key", "apikey",
		"authorization", "Authorization",
		"auth",
		"credential", "credentials",
		"private_key", "privatekey",
		"bearer",
		"session",
		"cookie",
		"x-api-key",
	}
	for _, k := range mustBeSecret {
		if !IsSecretKey(k) {
			t.Errorf("IsSecretKey(%q) = false, want true", k)
		}
	}

	mustNotBeSecret := []string{
		"title", "author", "book_id", "percent", "source", "page", "notes",
		"updated_at", "user_id",
	}
	for _, k := range mustNotBeSecret {
		if IsSecretKey(k) {
			t.Errorf("IsSecretKey(%q) = true, want false", k)
		}
	}
}

func TestRedactField_SecretKey(t *testing.T) {
	v := RedactField("password", "supersecret")
	if v != "[REDACTED]" {
		t.Errorf("RedactField(password) = %v, want [REDACTED]", v)
	}

	v = RedactField("token", "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.abc")
	if v != "[REDACTED]" {
		t.Errorf("RedactField(token) = %v, want [REDACTED]", v)
	}
}

func TestRedactField_NonSecret(t *testing.T) {
	v := RedactField("title", "The Pragmatic Programmer")
	if v != "The Pragmatic Programmer" {
		t.Errorf("RedactField(title) = %v, want original", v)
	}
}

func TestRedactString_BearerToken(t *testing.T) {
	msg := "Authorization: Bearer eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9"
	out := RedactString(msg)
	if strings.Contains(out, "eyJ") {
		t.Errorf("bearer token not redacted in: %s", out)
	}
}

func TestSafeFields(t *testing.T) {
	fields := map[string]any{
		"book_id":  42,
		"password": "hunter2",
		"token":    "tok_abc123def456ghi789jkl012mno345pqr",
		"title":    "Dune",
	}
	safe := SafeFields(fields)
	if safe["password"] != "[REDACTED]" {
		t.Error("password should be redacted")
	}
	if safe["token"] != "[REDACTED]" {
		t.Error("token should be redacted")
	}
	if safe["title"] != "Dune" {
		t.Error("title should not be redacted")
	}
	if safe["book_id"] != 42 {
		t.Error("book_id should not be redacted")
	}
}

func TestLogger_NoSecretsInOutput(t *testing.T) {
	var activity, jsonl bytes.Buffer
	log := New(&activity, &jsonl, LevelDebug)

	log.Info("user logged in",
		F("user", "alice"),
		F("password", "hunter2"),
		F("api_token", "tok_super_secret_value_1234567890"),
	)

	for _, stream := range []string{activity.String(), jsonl.String()} {
		if strings.Contains(stream, "hunter2") {
			t.Errorf("password leaked into log: %s", stream)
		}
		if strings.Contains(stream, "tok_super_secret_value_1234567890") {
			t.Errorf("api_token leaked into log: %s", stream)
		}
	}
}

func TestLogger_SecretInMessage_PatternRedacted(t *testing.T) {
	var activity, jsonl bytes.Buffer
	log := New(&activity, &jsonl, LevelDebug)
	log.Info("calling API with Bearer eyJhbGciOiJSUzI1NiJ9.test.signature")

	for _, stream := range []string{activity.String(), jsonl.String()} {
		if strings.Contains(stream, "eyJhbGciOiJSUzI1NiJ9") {
			t.Errorf("bearer token leaked into log: %s", stream)
		}
	}
}
