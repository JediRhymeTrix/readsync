//go:build !windows
// +build !windows

// internal/secrets/dpapi_other.go
//
// Non-Windows fallback: PlatformStore returns a MemStore.
// In production this branch is for dev/CI on Linux & macOS only.

package secrets

// PlatformStore returns the platform-native secret store.
// On non-Windows platforms this is the in-memory store.
func PlatformStore() Store {
	return NewMemStore()
}
