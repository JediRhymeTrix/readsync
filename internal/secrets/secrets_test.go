// internal/secrets/secrets_test.go

package secrets

import (
	"errors"
	"testing"
)

func TestMemStore_RoundTrip(t *testing.T) {
	s := NewMemStore()
	if err := s.Set("foo", "bar"); err != nil {
		t.Fatalf("Set: %v", err)
	}
	got, err := s.Get("foo")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got != "bar" {
		t.Errorf("got=%q want=bar", got)
	}
	if err := s.Delete("foo"); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if _, err := s.Get("foo"); !errors.Is(err, ErrNotFound) {
		t.Errorf("after delete: err=%v want ErrNotFound", err)
	}
}

func TestEnvStore_NotFound(t *testing.T) {
	e := &EnvStore{}
	if _, err := e.Get("READSYNC_DEFINITELY_NOT_SET_xyz123"); !errors.Is(err, ErrNotFound) {
		t.Errorf("got %v, want ErrNotFound", err)
	}
}

func TestEnvStore_ReadOnly(t *testing.T) {
	e := &EnvStore{}
	if err := e.Set("k", "v"); err == nil {
		t.Error("Set should return error on read-only store")
	}
	if err := e.Delete("k"); err == nil {
		t.Error("Delete should return error on read-only store")
	}
}

func TestChainStore_FallsThrough(t *testing.T) {
	primary := NewMemStore()
	secondary := NewMemStore()
	_ = secondary.Set("k", "fallback")
	chain := NewChainStore(primary, secondary)

	got, err := chain.Get("k")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got != "fallback" {
		t.Errorf("got=%q want=fallback", got)
	}
}

func TestChainStore_NotFound(t *testing.T) {
	chain := NewChainStore(NewMemStore(), NewMemStore())
	if _, err := chain.Get("missing"); !errors.Is(err, ErrNotFound) {
		t.Errorf("got %v, want ErrNotFound", err)
	}
}

func TestChainStore_WritesPrimary(t *testing.T) {
	primary := NewMemStore()
	secondary := NewMemStore()
	chain := NewChainStore(primary, secondary)

	if err := chain.Set("k", "v"); err != nil {
		t.Fatalf("Set: %v", err)
	}
	got, err := primary.Get("k")
	if err != nil || got != "v" {
		t.Errorf("primary did not receive write: got=%q err=%v", got, err)
	}
	if _, err := secondary.Get("k"); !errors.Is(err, ErrNotFound) {
		t.Errorf("secondary should not have value, err=%v", err)
	}
}

func TestPlatformStore_NotNil(t *testing.T) {
	s := PlatformStore()
	if s == nil {
		t.Fatal("PlatformStore returned nil")
	}
}
