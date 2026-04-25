// cmd/readsyncctl/main.go
//
// readsyncctl: command-line management tool for ReadSync.
//
// Commands:
//   status
//   adapters
//   conflicts list/show/resolve
//   outbox list/retry/drop
//   db migrate/vacuum
//   diagnostics export

package main

import (
	"fmt"
	"os"

	"github.com/readsync/readsync/cmd/readsyncctl/commands"
)

func main() {
	if len(os.Args) < 2 {
		printUsage()
		os.Exit(1)
	}

	cmd := os.Args[1]
	args := os.Args[2:]

	var err error
	switch cmd {
	case "status":
		err = commands.Status(args)
	case "adapters":
		err = commands.Adapters(args)
	case "conflicts":
		err = commands.Conflicts(args)
	case "outbox":
		err = commands.Outbox(args)
	case "db":
		err = commands.DB(args)
	case "diagnostics":
		err = commands.Diagnostics(args)
	case "help", "--help", "-h":
		printUsage()
	default:
		fmt.Fprintf(os.Stderr, "unknown command: %q\n", cmd)
		printUsage()
		os.Exit(1)
	}

	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}

func printUsage() {
	fmt.Print(`readsyncctl - ReadSync management CLI

Usage:
  readsyncctl <command> [arguments]

Commands:
  status                   Show service status
  adapters                 List adapter health states
  conflicts list           List open conflicts
  conflicts show <id>      Show conflict detail
  conflicts resolve <id>   Resolve conflict (choose a or b)
  outbox list              List pending outbox jobs
  outbox retry <id>        Retry a failed/deadletter job
  outbox drop <id>         Drop an outbox job
  db migrate               Apply pending schema migrations
  db vacuum                Vacuum the SQLite database
  diagnostics export       Export diagnostics report to stdout

Flags:
  --db <path>              Path to readsync.db (default: %APPDATA%\ReadSync\readsync.db)
  --api <url>              Admin API URL (default: http://127.0.0.1:7201)

`)
}
