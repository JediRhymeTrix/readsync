//go:build windows
// +build windows

// internal/secrets/dpapi_windows.go
//
// Windows Credential Manager / DPAPI-backed secret store.
// Uses CredRead / CredWrite / CredDelete from advapi32.dll via x/sys/windows.
// Each secret is stored as a CRED_TYPE_GENERIC entry with TargetName
// "ReadSync:<key>". The credential blob is encrypted at rest by the OS
// using the current user's DPAPI master key.

package secrets

import (
	"errors"
	"fmt"
	"syscall"
	"unsafe"

	"golang.org/x/sys/windows"
)

const (
	credTargetPrefix = "ReadSync:"
	credTypeGeneric  = 1
	credPersistLocal = 2
)

type credential struct {
	Flags              uint32
	Type               uint32
	TargetName         *uint16
	Comment            *uint16
	LastWritten        windows.Filetime
	CredentialBlobSize uint32
	CredentialBlob     *byte
	Persist            uint32
	AttributeCount     uint32
	Attributes         uintptr
	TargetAlias        *uint16
	UserName           *uint16
}

var (
	advapi32       = windows.NewLazySystemDLL("advapi32.dll")
	procCredReadW  = advapi32.NewProc("CredReadW")
	procCredWriteW = advapi32.NewProc("CredWriteW")
	procCredDelete = advapi32.NewProc("CredDeleteW")
	procCredFree   = advapi32.NewProc("CredFree")
)

// DPAPIStore is the Windows-backed Store implementation.
type DPAPIStore struct{}

// NewDPAPIStore constructs the Windows credential store.
func NewDPAPIStore() *DPAPIStore { return &DPAPIStore{} }

// Get retrieves a secret from Windows Credential Manager.
func (d *DPAPIStore) Get(key string) (string, error) {
	target, err := syscall.UTF16PtrFromString(credTargetPrefix + key)
	if err != nil {
		return "", err
	}
	var pcred *credential
	r, _, callErr := procCredReadW.Call(
		uintptr(unsafe.Pointer(target)),
		uintptr(credTypeGeneric),
		0,
		uintptr(unsafe.Pointer(&pcred)),
	)
	if r == 0 {
		// Win32 ERROR_NOT_FOUND = 1168.
		if errno, ok := callErr.(syscall.Errno); ok && errno == 1168 {
			return "", fmt.Errorf("%w: %s", ErrNotFound, key)
		}
		return "", fmt.Errorf("CredReadW: %v", callErr)
	}
	defer procCredFree.Call(uintptr(unsafe.Pointer(pcred)))
	blob := unsafe.Slice(pcred.CredentialBlob, pcred.CredentialBlobSize)
	return string(blob), nil
}

// Set stores or updates a secret.
func (d *DPAPIStore) Set(key, value string) error {
	target, err := syscall.UTF16PtrFromString(credTargetPrefix + key)
	if err != nil {
		return err
	}
	user, err := syscall.UTF16PtrFromString("ReadSync")
	if err != nil {
		return err
	}
	blob := []byte(value)
	var blobPtr *byte
	if len(blob) > 0 {
		blobPtr = &blob[0]
	}
	cred := credential{
		Type:               credTypeGeneric,
		TargetName:         target,
		CredentialBlobSize: uint32(len(blob)),
		CredentialBlob:     blobPtr,
		Persist:            credPersistLocal,
		UserName:           user,
	}
	r, _, callErr := procCredWriteW.Call(
		uintptr(unsafe.Pointer(&cred)),
		0,
	)
	if r == 0 {
		return fmt.Errorf("CredWriteW: %v", callErr)
	}
	return nil
}

// Delete removes a secret from Windows Credential Manager.
func (d *DPAPIStore) Delete(key string) error {
	target, err := syscall.UTF16PtrFromString(credTargetPrefix + key)
	if err != nil {
		return err
	}
	r, _, callErr := procCredDelete.Call(
		uintptr(unsafe.Pointer(target)),
		uintptr(credTypeGeneric),
		0,
	)
	if r == 0 {
		if errno, ok := callErr.(syscall.Errno); ok && errno == 1168 {
			return fmt.Errorf("%w: %s", ErrNotFound, key)
		}
		return fmt.Errorf("CredDeleteW: %v", callErr)
	}
	return nil
}

// PlatformStore returns the OS-native secret store.
func PlatformStore() Store {
	return NewDPAPIStore()
}

// _ = errors satisfies the import when only used in error wrapping below.
var _ = errors.New
