// main.go — root-package development runner.
// Use cmd/readsync-service for the production Windows Service binary.
//
// Usage:
//   go run . [--port 7200] [--db readsync.db]

package main

import (
	"context"
	"flag"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/readsync/readsync/internal/adapters/koreader"
	"github.com/readsync/readsync/internal/core"
	"github.com/readsync/readsync/internal/db"
	"github.com/readsync/readsync/internal/logging"
)

func main() {
	dbPath := flag.String("db", "readsync.db", "SQLite database path")
	flag.Parse()

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	logger := logging.New(os.Stdout, os.Stderr, logging.LevelInfo)
	logger.Info("ReadSync dev runner starting")

	database, err := db.Open(*dbPath)
	if err != nil {
		log.Fatalf("db.Open: %v", err)
	}
	defer database.Close()

	if err := database.Migrate(); err != nil {
		log.Fatalf("Migrate: %v", err)
	}

	pipeline := core.NewPipeline(database.SQL(), logger)
	go pipeline.Run(ctx)

	cfg := koreader.DefaultConfig()
	cfg.RegistrationOpen = true // open for easy dev/testing
	adapter := koreader.New(cfg, database.SQL(), logger)
	adapter.SetPipeline(pipeline)
	if err := adapter.Start(ctx); err != nil {
		log.Fatalf("koreader adapter: %v", err)
	}

	logger.Info("ReadSync ready — KOSync on :7200, Ctrl-C to stop")
	<-ctx.Done()
	logger.Info("shutdown")
}