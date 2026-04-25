// cmd/readsyncctl/commands/status.go
//
// readsyncctl status, adapters, conflicts, outbox, diagnostics commands.

package commands

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

const defaultAPIURL = "http://127.0.0.1:7201"

// Status shows the service status.
func Status(args []string) error {
	apiURL := resolveAPIURL(args)
	resp, err := http.Get(apiURL + "/status")
	if err != nil {
		return fmt.Errorf("cannot reach ReadSync service at %s: %w\n"+
			"Is the service running? Try: readsync-service.exe run", apiURL, err)
	}
	defer resp.Body.Close()
	return printJSON(resp.Body)
}

// Adapters shows adapter health.
func Adapters(args []string) error {
	apiURL := resolveAPIURL(args)
	resp, err := http.Get(apiURL + "/adapters")
	if err != nil {
		return fmt.Errorf("cannot reach service: %w", err)
	}
	defer resp.Body.Close()
	return printJSON(resp.Body)
}

// Conflicts dispatches conflict sub-commands.
func Conflicts(args []string) error {
	if len(args) < 1 {
		return listConflicts(args)
	}
	switch args[0] {
	case "list":
		return listConflicts(args[1:])
	case "show":
		if len(args) < 2 {
			return fmt.Errorf("conflicts show requires <id>")
		}
		return showConflict(args[1], args[2:])
	case "resolve":
		if len(args) < 2 {
			return fmt.Errorf("conflicts resolve requires <id>")
		}
		return resolveConflict(args[1], args[2:])
	default:
		return fmt.Errorf("unknown conflicts sub-command: %q", args[0])
	}
}

func listConflicts(args []string) error {
	apiURL := resolveAPIURL(args)
	resp, err := http.Get(apiURL + "/conflicts")
	if err != nil {
		return fmt.Errorf("cannot reach service: %w", err)
	}
	defer resp.Body.Close()
	return printJSON(resp.Body)
}

func showConflict(id string, args []string) error {
	apiURL := resolveAPIURL(args)
	resp, err := http.Get(apiURL + "/conflicts?id=" + id)
	if err != nil {
		return fmt.Errorf("cannot reach service: %w", err)
	}
	defer resp.Body.Close()
	return printJSON(resp.Body)
}

func resolveConflict(id string, args []string) error {
	return fmt.Errorf("conflict resolve not yet implemented (Phase 2): id=%s args=%v", id, args)
}

// Outbox dispatches outbox sub-commands.
func Outbox(args []string) error {
	if len(args) < 1 {
		return listOutbox(args)
	}
	switch args[0] {
	case "list":
		return listOutbox(args[1:])
	case "retry":
		if len(args) < 2 {
			return fmt.Errorf("outbox retry requires <id>")
		}
		return fmt.Errorf("outbox retry not yet implemented (Phase 2): id=%s", args[1])
	case "drop":
		if len(args) < 2 {
			return fmt.Errorf("outbox drop requires <id>")
		}
		return fmt.Errorf("outbox drop not yet implemented (Phase 2): id=%s", args[1])
	default:
		return fmt.Errorf("unknown outbox sub-command: %q", args[0])
	}
}

func listOutbox(args []string) error {
	apiURL := resolveAPIURL(args)
	resp, err := http.Get(apiURL + "/outbox")
	if err != nil {
		return fmt.Errorf("cannot reach service: %w", err)
	}
	defer resp.Body.Close()
	return printJSON(resp.Body)
}

// Diagnostics dispatches diagnostics sub-commands.
func Diagnostics(args []string) error {
	if len(args) < 1 {
		return fmt.Errorf("diagnostics requires a sub-command: export")
	}
	switch args[0] {
	case "export":
		return diagnosticsExport(args[1:])
	default:
		return fmt.Errorf("unknown diagnostics sub-command: %q", args[0])
	}
}

func diagnosticsExport(args []string) error {
	// For Phase 1, collect from the DB directly rather than via the API.
	dbPath := resolveDBPath(args)
	fmt.Println("# ReadSync Diagnostics Export")
	fmt.Printf("db_path: %s\n", dbPath)
	fmt.Println("phase: 1")
	fmt.Println("status: skeleton")
	return nil
}

func resolveAPIURL(args []string) string {
	for i, a := range args {
		if a == "--api" && i+1 < len(args) {
			return args[i+1]
		}
	}
	return defaultAPIURL
}

func printJSON(r io.Reader) error {
	var v any
	if err := json.NewDecoder(r).Decode(&v); err != nil {
		return fmt.Errorf("decode response: %w", err)
	}
	b, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return err
	}
	fmt.Println(string(b))
	return nil
}
