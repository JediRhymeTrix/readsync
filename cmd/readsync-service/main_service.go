// cmd/readsync-service/main_service.go
//
// ReadSync Windows Service entry point.

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
	"github.com/readsync/readsync/internal/diagnostics"
	"github.com/readsync/readsync/internal/logging"
	"github.com/readsync/readsync/internal/repair"
	"github.com/readsync/readsync/internal/secrets"
	"github.com/readsync/readsync/internal/setup"
)

const (
	serviceName2        = "ReadSync"
	serviceDisplayName2 = "ReadSync Book Sync Service"
	serviceDescription2 = "Keeps reading progress in sync across Calibre, KOReader, Moon+, and Goodreads."
	version2            = "0.6.0-phase6"
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

// diagnosticsAdapter wraps the diagnostics.Collector so it satisfies the
// api.DiagnosticsCollector interface (which uses `any` for the report).
type diagnosticsAdapter struct{ inner *diagnostics.Collector }

func (d diagnosticsAdapter) Collect(ctx context.Context) (any, error) {
	return d.inner.Collect(ctx)
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

	// Setup wizard (phase 6) - file-backed, persists between runs.
	wizardPath := filepath.Join(filepath.Dir(dbPath), "wizard.json")
	wizard, werr := setup.NewWithPath(wizardPath)
	if werr != nil {
		logger.Error("wizard init failed", logging.F("error", werr))
	}

	// Diagnostics collector.
	diag := diagnostics.New(database.SQL(), version2)

	// Secrets store: Windows DPAPI in production, EnvStore in dev.
	secretsStore := secrets.NewChainStore(secrets.PlatformStore(), &secrets.EnvStore{})

	// Wire repair-action callbacks the API exposes via /api/sync_now etc.
	api.SetRestartHook(func() error {
		res := repair.RestartService()
		if !res.OK {
			return fmt.Errorf("%s", res.Message)
		}
		return nil
	})
	api.SetSyncTrigger(syncTriggerStub{})
	api.SetSecretsStore(secretsStore)

	apiServer, err := api.New(api.Deps{
		DB:          database.SQL(),
		Wizard:      wizard,
		Diagnostics: diagnosticsAdapter{inner: diag},
		Version:     version2,
		Port:        7201,
	})
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
		logger.Error("koreader adapter start failed", logging.F("error", err))
	}

	// Start Moon+ Reader Pro adapter (Phase 4).
	moonCfg := moon.Defaults()
	moonCfg.WebDAV.DataDir = filepath.Join(filepath.Dir(dbPath), "moon")
	moonCfg.WebDAV.BindAddr = "0.0.0.0:8765"
	moonAdapter, mErr := moon.New(moonCfg, database.SQL(), logger, secretsStore)
	if mErr != nil {
		logger.Error("moon adapter init failed", logging.F("error", mErr))
	} else {
		moonAdapter.SetPipeline(pipeline)
		if err := moonAdapter.Start(ctx); err != nil {
			logger.Error("moon adapter start failed", logging.F("error", err))
		}
	}

	logger.Info("ReadSync service ready",
		logging.F("admin_url", "http://127.0.0.1:7201"))
	<-ctx.Done()
	logger.Info("ReadSync service stopping")
}

// syncTriggerStub is a no-op SyncTrigger used until adapters expose a
// real one. Returning nil keeps /api/sync_now happy.
type syncTriggerStub struct{}

func (syncTriggerStub) TriggerSync() error { return nil }

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
