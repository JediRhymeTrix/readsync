// cmd/readsync-service/main.go
//
// ReadSync Windows Service entry point.
// Installs and runs as a proper Windows Service using kardianos/service.
//
// Usage (requires admin for install/start/stop/uninstall):
//   readsync-service.exe install
//   readsync-service.exe start
//   readsync-service.exe stop
//   readsync-service.exe uninstall
//   readsync-service.exe status
//   readsync-service.exe run      (foreground, no admin needed)

package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"

	"github.com/kardianos/service"
	"github.com/readsync/readsync/internal/api"
	"github.com/readsync/readsync/internal/core"
	"github.com/readsync/readsync/internal/db"
	"github.com/readsync/readsync/internal/logging"
)

const (
	serviceName        = "ReadSync"
	serviceDisplayName = "ReadSync Book Sync Service"
	serviceDescription = "Keeps reading progress in sync across Calibre, KOReader, Moon+, and Goodreads."
	version            = "0.1.0-phase1"
)

type program struct {
	svc    service.Service
	cancel context.CancelFunc
	done   chan struct{}
}

func (p *program) Start(s service.Service) error {
	p.svc = s
	ctx, cancel := context.WithCancel(context.Background())
	p.cancel = cancel
	p.done = make(chan struct{})
	go p.runService(ctx)
	return nil
}

func (p *program) runService(ctx context.Context) {
	defer close(p.done)

	logger := logging.New(os.Stdout, os.Stderr, logging.LevelInfo)
	logger.Info("ReadSync service starting", logging.F("version", version))

	// Open database.
	dbPath := defaultDBPath()
	database, err := db.Open(dbPath)
	if err != nil {
		logger.Error("failed to open database", logging.F("path", dbPath), logging.F("error", err))
		return
	}
	defer database.Close()

	if err := database.Migrate(); err != nil {
		logger.Error("migration failed", logging.F("error", err))
		return
	}
	logger.Info("database ready", logging.F("path", dbPath))

	// Start pipeline.
	pipeline := core.NewPipeline(database.SQL(), logger)
	go pipeline.Run(ctx)

	// Start API server.
	apiServer, err := api.New(api.Deps{Port: 7201})
	if err != nil {
		logger.Error("api init failed", logging.F("error", err))
		return
	}
	if err := apiServer.Start(ctx); err != nil {
		logger.Error("api start failed", logging.F("error", err))
		return
	}

	logger.Info("ReadSync service ready")

	// Wait for shutdown.
	<-ctx.Done()
	logger.Info("ReadSync service stopping")
}

func (p *program) Stop(_ service.Service) error {
	if p.cancel != nil {
		p.cancel()
	}
	if p.done != nil {
		<-p.done
	}
	return nil
}

func defaultDBPath() string {
	dir, err := os.UserConfigDir()
	if err != nil {
		dir = "."
	}
	return filepath.Join(dir, "ReadSync", "readsync.db")
}

func main() {
	svcConfig := &service.Config{
		Name:        serviceName,
		DisplayName: serviceDisplayName,
		Description: serviceDescription,
	}

	prg := &program{}
	svc, err := service.New(prg, svcConfig)
	if err != nil {
		log.Fatalf("service.New: %v", err)
	}

	if len(os.Args) < 2 {
		// SCM launches with no args → run the service.
		if err := svc.Run(); err != nil {
			log.Fatalf("run: %v", err)
		}
		return
	}

	switch os.Args[1] {
	case "install", "uninstall", "start", "stop":
		if err := service.Control(svc, os.Args[1]); err != nil {
			log.Fatalf("%s: %v", os.Args[1], err)
		}
		fmt.Printf("Service %s: %s OK\n", serviceName, os.Args[1])

	case "status":
		status, err := svc.Status()
		if err != nil {
			log.Fatalf("status: %v", err)
		}
		fmt.Printf("Service %s: %s\n", serviceName, statusString(status))

	case "run":
		fmt.Printf("Running %s in foreground (Ctrl+C to stop)\n", serviceName)
		if err := svc.Run(); err != nil {
			log.Fatalf("run: %v", err)
		}

	default:
		fmt.Fprintf(os.Stderr, "Unknown action: %q\n", os.Args[1])
		fmt.Fprintf(os.Stderr, "Valid: install uninstall start stop status run\n")
		os.Exit(1)
	}
}

func statusString(s service.Status) string {
	switch s {
	case service.StatusRunning:
		return "Running"
	case service.StatusStopped:
		return "Stopped"
	case service.StatusUnknown:
		return "Unknown"
	default:
		return fmt.Sprintf("status(%d)", s)
	}
}
