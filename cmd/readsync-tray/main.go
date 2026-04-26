// cmd/readsync-tray/main.go
//
// ReadSync system tray application.
//
// Polls the service /healthz + /api/adapters every 5s and updates the
// tray icon (Windows) or prints a colour-coded status line (other OSes
// + headless mode). Menu items shell out to local /api/* endpoints
// over the loopback HTTP API; CSRF tokens come from /csrf.

package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"time"
)

const defaultBaseURL = "http://127.0.0.1:7201"

func main() {
	base := flag.String("url", defaultBaseURL, "Service base URL")
	headless := flag.Bool("headless", false,
		"Force the polling-stdout tray (no native UI)")
	once := flag.Bool("once", false,
		"Print one status line and exit (used by smoke tests)")
	flag.Parse()

	client := NewServiceClient(*base)

	if *once {
		printStatus(client)
		return
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// On non-Windows or in headless mode, fall back to the polling tray.
	if *headless || runtime.GOOS != "windows" {
		runHeadlessTray(ctx, client)
		return
	}
	if err := runNativeTray(ctx, client); err != nil {
		log.Printf("native tray failed (%v); falling back to headless", err)
		runHeadlessTray(ctx, client)
	}
}

// runHeadlessTray polls the service every 5 seconds and writes a one-
// line status banner to stdout. Menu actions are exposed as keyboard
// shortcuts via stdin (d=dashboard, s=sync now, c=conflicts, a=activity,
// r=restart, q=quit). Used in tests and on non-Windows hosts.
func runHeadlessTray(ctx context.Context, client *ServiceClient) {
	go func() {
		ticker := time.NewTicker(5 * time.Second)
		defer ticker.Stop()
		for {
			printStatus(client)
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
			}
		}
	}()
	fmt.Println("ReadSync tray (headless mode). Commands: d=dashboard s=sync c=conflicts a=activity r=restart q=quit")
	for {
		var cmd string
		if _, err := fmt.Scanln(&cmd); err != nil {
			return
		}
		switch strings.ToLower(strings.TrimSpace(cmd)) {
		case "d", "dashboard":
			openURL(client.base + "/ui/dashboard")
		case "s", "sync":
			if err := client.SyncNow(); err != nil {
				fmt.Println("sync failed:", err)
			} else {
				fmt.Println("sync triggered")
			}
		case "c", "conflicts":
			openURL(client.base + "/ui/conflicts")
		case "a", "activity":
			openURL(client.base + "/ui/activity")
		case "r", "restart":
			if err := client.RestartService(); err != nil {
				fmt.Println("restart failed:", err)
			} else {
				fmt.Println("restart requested")
			}
		case "q", "quit", "exit":
			return
		default:
			fmt.Println("unknown command:", cmd)
		}
	}
}

func printStatus(client *ServiceClient) {
	if !client.Healthz() {
		fmt.Println("[ReadSync] service: UNREACHABLE")
		return
	}
	adapters, err := client.Adapters()
	if err != nil {
		fmt.Printf("[ReadSync] service: ok, adapters: %v\n", err)
		return
	}
	overall := OverallHealth(adapters)
	parts := []string{}
	for _, a := range adapters {
		parts = append(parts, fmt.Sprintf("%s=%s", a.Source, a.State))
	}
	fmt.Printf("[ReadSync] %s | %s\n", overall, strings.Join(parts, " "))
}

// openURL launches the user's default browser for url. Best-effort; on
// platforms without a known launcher the call is a no-op.
func openURL(url string) {
	switch runtime.GOOS {
	case "windows":
		_ = exec.Command("cmd", "/c", "start", "", url).Start()
	case "darwin":
		_ = exec.Command("open", url).Start()
	case "linux":
		_ = exec.Command("xdg-open", url).Start()
	default:
		fmt.Fprintln(os.Stderr, "open URL:", url)
	}
}
