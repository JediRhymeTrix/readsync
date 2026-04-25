// tools/winsvc-spike/main.go
//
// ReadSync Windows Service Spike
//
// A minimal hello-world Windows service using kardianos/service.
// Demonstrates: install, start, stop, uninstall via readsyncctl-style subcommands.
//
// Usage (requires admin for install/start/stop/uninstall):
//   go build -o readsync-spike.exe .
//   .\readsync-spike.exe install
//   .\readsync-spike.exe start
//   .\readsync-spike.exe status
//   .\readsync-spike.exe stop
//   .\readsync-spike.exe uninstall
//   .\readsync-spike.exe run      (foreground mode, no admin needed)

package main

import (
	"fmt"
	"log"
	"os"
	"time"

	"github.com/kardianos/service"
)

const (
	serviceName        = "ReadSyncSpike"
	serviceDisplayName = "ReadSync Spike (Phase 0)"
	serviceDescription = "ReadSync Windows service spike — demonstrates kardianos/service lifecycle."
)

// program implements service.Interface.
type program struct {
	svc  service.Service
	stop chan struct{}
}

func (p *program) Start(s service.Service) error {
	p.svc = s
	p.stop = make(chan struct{})
	go p.run()
	return nil
}

func (p *program) run() {
	logger, err := p.svc.Logger(nil)
	if err != nil {
		log.Printf("logger init: %v", err)
		return
	}

	_ = logger.Info("ReadSync spike running -- heartbeat every 5s")
	tick := time.NewTicker(5 * time.Second)
	defer tick.Stop()

	for {
		select {
		case <-tick.C:
			ts := time.Now().UTC().Format(time.RFC3339)
			_ = logger.Infof("ReadSync heartbeat at %s", ts)
		case <-p.stop:
			_ = logger.Info("ReadSync spike stopping")
			return
		}
	}
}

func (p *program) Stop(s service.Service) error {
	close(p.stop)
	return nil
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

	// When SCM launches the binary to run the service it passes no arguments.
	// Call svc.Run() directly in that case — kardianos/service connects to
	// the SCM dispatcher and calls program.Start().
	if len(os.Args) < 2 {
		if err := svc.Run(); err != nil {
			log.Fatalf("run: %v", err)
		}
		return
	}

	action := os.Args[1]
	switch action {
	case "install", "uninstall", "start", "stop":
		if err := service.Control(svc, action); err != nil {
			log.Fatalf("%s: %v", action, err)
		}
		fmt.Printf("Service %s: %s OK\n", serviceName, action)

	case "status":
		status, err := svc.Status()
		if err != nil {
			log.Fatalf("status: %v", err)
		}
		fmt.Printf("Service %s status: %s\n", serviceName, statusString(status))

	case "run":
		// Run in foreground — useful for debugging without admin.
		fmt.Printf("Running %s in foreground (Ctrl+C to stop)\n", serviceName)
		if err := svc.Run(); err != nil {
			log.Fatalf("run: %v", err)
		}

	default:
		fmt.Fprintf(os.Stderr, "Unknown action: %q\n", action)
		fmt.Fprintf(os.Stderr, "Valid actions: install uninstall start stop status run\n")
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
