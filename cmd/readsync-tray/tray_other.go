//go:build !windows
// +build !windows

// cmd/readsync-tray/tray_other.go
//
// Non-Windows stub: native tray is unavailable on Linux/macOS, so we
// always fall back to the headless poller. Keeps the build green for
// CI on Linux runners.

package main

import (
	"context"
	"errors"
)

func runNativeTray(ctx context.Context, client *ServiceClient) error {
	return errors.New("native tray is only supported on Windows")
}
