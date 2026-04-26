// tests/fakeserver/main.go
//
// Fake ReadSync admin server used by the Playwright wizard E2E tests.
// Boots the real internal/api package against an in-memory wizard,
// no database, no adapters. Lets the wizard render every page and
// exposes the HTML routes Playwright walks. Listens on 127.0.0.1:7201
// (or the port from $READSYNC_PORT).

package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/readsync/readsync/internal/api"
	"github.com/readsync/readsync/internal/setup"
)

func main() {
	port := flag.Int("port", 7201, "Port to bind on 127.0.0.1")
	flag.Parse()

	if v := os.Getenv("READSYNC_PORT"); v != "" {
		_, _ = fmt.Sscanf(v, "%d", port)
	}

	wz := setup.New()

	srv, err := api.New(api.Deps{
		Wizard:   wz,
		Version:  "fake-e2e",
		Port:     *port,
		BindAddr: "127.0.0.1",
	})
	if err != nil {
		log.Fatalf("api.New: %v", err)
	}

	// Wire a no-op sync trigger so the wizard's test-sync step succeeds.
	api.SetSyncTrigger(noopTrigger{})

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	if err := srv.Start(ctx); err != nil {
		log.Fatalf("server.Start: %v", err)
	}
	fmt.Printf("Fake ReadSync admin UI listening on %s (CSRF=%s)\n",
		srv.Addr(), srv.CSRFToken())

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)
	<-stop
	cancel()
	_ = srv.Stop()
}

type noopTrigger struct{}

func (noopTrigger) TriggerSync() error { return nil }
