// cmd/readsync-service/main_service.go
//
// ReadSync Windows Service entry point.
// This file contains the actual implementation; main.go is left for reference.

package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"

	"github.com/kardianos/service"
	"github.com/readsync/readsync/internal/adapters/koreader"
	"github.com/readsync/readsync/internal/adapters/moon"
	"github.com/readsync/readsync/internal/api"
	"github.com/readsync/readsync/internal/core"
	"github.com/readsync/readsync/internal/db"
	"github.com/readsync/readsync/internal/logging"
	"github.com/readsync/readsync/internal/secrets"
)

const (
	serviceName2        = "ReadSync"
	serviceDisplayName2 = "ReadSync Book Sync Service"
	serviceDescription2 = "Keeps reading progress in sync across Calibre, KOReader, Moon+, and Goodreads."
	version2            = "0.1.0-phase3"
)

type program2 struct {
	svc    service.Service
	cancel context.CancelFunc
	done   chan struct{}
}

func (p *program2) Start(s service.Service) error {
	p.svc = s
	ctx, cancel := context.WithCancel(context.Background())
	p.cancel = cancel
	p.done = make(chan struct{})
	go p.runService2(ctx)
	return nil
}

func (p *program2) runService2(ctx context.Context) {
	defer close(p.done)

	logger := logging.New(os.Stdout, os.Stderr, logging.LevelInfo)
	logger.Info("ReadSync service starting", logging.F("version", version2))

	dbPath := defaultDBPath2()
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

	pipeline := core.NewPipeline(database.SQL(), logger)
	go pipeline.Run(ctx)

	apiServer, err := api.New(api.Deps{Port: 7201})
	if err != nil {
		logger.Error("api init failed", logging.F("error", err))
		return
	}
	if err := apiServer.Start(ctx); err != nil {
		logger.Error("api start failed", logging.F("error", err))
		return
	}

	// Start KOReader KOSync adapter on port 7200.
	koreaderAdapter := koreader.New(koreader.DefaultConfig(), database.SQL(), logger)
	koreaderAdapter.SetPipeline(pipeline)
	if err := koreaderAdapter.Start(ctx); err != nil {
		// Non-fatal: log and continue â€” KOReader sync unavailable but service runs.
		logger.Error("koreader adapter start failed", logging.F("error", err))
	}
	// Start Moon+ Reader Pro adapter (embedded WebDAV server, Phase 4).
	moonCfg := moon.Defaults()
	moonCfg.WebDAV.DataDir = filepath.Join(filepath.Dir(dbPath), "moon")
	moonCfg.WebDAV.BindAddr = "0.0.0.0:8765"
	moonAdapter, mErr := moon.New(moonCfg, database.SQL(), logger, &secrets.EnvStore{})
	if mErr != nil {
		logger.Error("moon adapter init failed", logging.F("error", mErr))
	} else {
		moonAdapter.SetPipeline(pipeline)
		if err := moonAdapter.Start(ctx); err != nil {
			logger.Error("moon adapter start failed", logging.F("error", err))
		}
	}

	logger.Info("ReadSync service ready")
	<-ctx.Done()
	logger.Info("ReadSync service stopping")
}

func (p *program2) Stop(_ service.Service) error {
	if p.cancel != nil {
		p.cancel()
	}
	if p.done != nil {
		<-p.done
	}
	return nil
}

func defaultDBPath2() string {
	dir, err := os.UserConfigDir()
	if err != nil {
		dir = "."
	}
	return filepath.Join(dir, "ReadSync", "readsync.db")
}

func main() {
	runService2()
}

func runService2() {
	svcConfig := &service.Config{
		Name:        serviceName2,
		DisplayName: serviceDisplayName2,
		Description: serviceDescription2,
	}
	prg := &program2{}
	svc, err := service.New(prg, svcConfig)
	if err != nil {
		log.Fatalf("service.New: %v", err)
	}
	if len(os.Args) < 2 {
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
		fmt.Printf("Service %s: %s OK\n", serviceName2, os.Args[1])
	case "run":
		fmt.Printf("Running %s in foreground (Ctrl+C to stop)\n", serviceName2)
		if err := svc.Run(); err != nil {
			log.Fatalf("run: %v", err)
		}
	default:
		fmt.Fprintf(os.Stderr, "Unknown action: %q\n", os.Args[1])
		os.Exit(1)
	}
}
