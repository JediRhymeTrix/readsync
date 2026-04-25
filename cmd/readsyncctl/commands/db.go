// cmd/readsyncctl/commands/db.go
//
// readsyncctl db commands: migrate, vacuum.

package commands

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/readsync/readsync/internal/db"
	"github.com/readsync/readsync/internal/repair"
)

// DB dispatches db sub-commands.
func DB(args []string) error {
	if len(args) < 1 {
		return fmt.Errorf("db requires a sub-command: migrate, vacuum")
	}
	switch args[0] {
	case "migrate":
		return dbMigrate(args[1:])
	case "vacuum":
		return dbVacuum(args[1:])
	default:
		return fmt.Errorf("unknown db sub-command: %q (migrate, vacuum)", args[0])
	}
}

func dbMigrate(args []string) error {
	dbPath := resolveDBPath(args)
	// Ensure the directory exists.
	if err := os.MkdirAll(filepath.Dir(dbPath), 0755); err != nil {
		return fmt.Errorf("mkdir: %w", err)
	}
	database, err := db.Open(dbPath)
	if err != nil {
		return fmt.Errorf("open db: %w", err)
	}
	defer database.Close()

	if err := database.Migrate(); err != nil {
		return fmt.Errorf("migrate: %w", err)
	}
	fmt.Printf("✓ Migrations applied to %s\n", dbPath)
	return nil
}

func dbVacuum(args []string) error {
	dbPath := resolveDBPath(args)
	database, err := db.Open(dbPath)
	if err != nil {
		return fmt.Errorf("open db: %w", err)
	}
	defer database.Close()

	if err := repair.Vacuum(context.Background(), database.SQL()); err != nil {
		return fmt.Errorf("vacuum: %w", err)
	}
	fmt.Printf("✓ VACUUM complete on %s\n", dbPath)
	return nil
}

func resolveDBPath(args []string) string {
	for i, a := range args {
		if a == "--db" && i+1 < len(args) {
			return args[i+1]
		}
	}
	// Default path: %APPDATA%\ReadSync\readsync.db
	dir, err := os.UserConfigDir()
	if err != nil {
		dir = "."
	}
	return filepath.Join(dir, "ReadSync", "readsync.db")
}
