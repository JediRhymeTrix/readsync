// internal/secrets/secrets.go
//
// Secrets manager: retrieves credentials from the Windows Credential Manager
// (DPAPI-protected) or from environment variables (dev/CI).
// Secrets are NEVER written to logs; the redact package enforces this at the
// logging layer. This package is the sole place where raw secret values exist.

package secrets

import (
	"errors"
	"fmt"
	"os"
	"sync"
)

// ErrNotFound is returned when a secret cannot be located.
var ErrNotFound = errors.New("secret not found")

// Store is an interface for secret retrieval.
type Store interface {
	// Get retrieves the secret identified by key.
	// Returns ErrNotFound if the secret does not exist.
	Get(key string) (string, error)

	// Set stores or updates a secret.
	Set(key, value string) error

	// Delete removes a secret.
	Delete(key string) error
}

// EnvStore reads secrets from environment variables.
// Key is looked up as the env var name (uppercase).
// This is the fallback for dev / CI environments.
type EnvStore struct{}

// Get reads from the environment.
func (e *EnvStore) Get(key string) (string, error) {
	v := os.Getenv(key)
	if v == "" {
		return "", fmt.Errorf("%w: %s", ErrNotFound, key)
	}
	return v, nil
}

// Set is not supported for EnvStore.
func (e *EnvStore) Set(_, _ string) error {
	return errors.New("secrets: EnvStore is read-only")
}

// Delete is not supported for EnvStore.
func (e *EnvStore) Delete(_ string) error {
	return errors.New("secrets: EnvStore is read-only")
}

// MemStore is an in-memory store for testing.
type MemStore struct {
	mu     sync.RWMutex
	values map[string]string
}

// NewMemStore creates an empty in-memory secret store.
func NewMemStore() *MemStore {
	return &MemStore{values: make(map[string]string)}
}

func (m *MemStore) Get(key string) (string, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	v, ok := m.values[key]
	if !ok {
		return "", fmt.Errorf("%w: %s", ErrNotFound, key)
	}
	return v, nil
}

func (m *MemStore) Set(key, value string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.values[key] = value
	return nil
}

func (m *MemStore) Delete(key string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.values, key)
	return nil
}

// ChainStore tries each Store in order, returning the first successful result.
type ChainStore struct {
	stores []Store
}

// NewChainStore creates a chain of stores (first has highest priority).
func NewChainStore(stores ...Store) *ChainStore {
	return &ChainStore{stores: stores}
}

func (c *ChainStore) Get(key string) (string, error) {
	for _, s := range c.stores {
		v, err := s.Get(key)
		if err == nil {
			return v, nil
		}
		if !errors.Is(err, ErrNotFound) {
			return "", err
		}
	}
	return "", fmt.Errorf("%w: %s", ErrNotFound, key)
}

func (c *ChainStore) Set(key, value string) error {
	if len(c.stores) == 0 {
		return errors.New("secrets: no stores configured")
	}
	return c.stores[0].Set(key, value)
}

func (c *ChainStore) Delete(key string) error {
	var lastErr error
	for _, s := range c.stores {
		if err := s.Delete(key); err != nil {
			lastErr = err
		}
	}
	return lastErr
}
