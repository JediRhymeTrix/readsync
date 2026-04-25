# ReadSync Windows Service Spike

Demonstrates the `kardianos/service` lifecycle for the production ReadSync service.

## Requirements

- Go 1.22+
- Windows (for install/start/stop/uninstall — requires admin)
- Admin PowerShell for service lifecycle commands

## Build

```powershell
# Build Windows binary
go build -o readsync-spike.exe .

# Or cross-compile from Linux/macOS
GOOS=windows GOARCH=amd64 go build -o readsync-spike.exe .
```

## Lifecycle Commands

```powershell
# All commands below require admin (Run as Administrator)

# Install as Windows Service
.\readsync-spike.exe install
# Output: Service ReadSyncSpike: install OK

# Start the service
.\readsync-spike.exe start
# Output: Service ReadSyncSpike: start OK

# Check status (no admin needed)
.\readsync-spike.exe status
# Output: Service ReadSyncSpike status: Running

# View heartbeat log in Event Viewer:
# Windows Logs → Application → Source: ReadSyncSpike

# Stop the service
.\readsync-spike.exe stop

# Uninstall
.\readsync-spike.exe uninstall

# Foreground mode (no admin, for debugging)
.\readsync-spike.exe run
```

## What It Demonstrates

1. `service.New()` — register service with SCM
2. `service.Control(svc, "install")` — register in Windows Service Control Manager
3. `service.Control(svc, "start")` — start via SCM
4. `service.Control(svc, "stop")` — stop via SCM
5. `service.Control(svc, "uninstall")` — remove from SCM
6. `svc.Status()` — query current status
7. Windows Event Log logging via `service.NewLogger()`
8. Graceful shutdown via `stop` channel on `program.Stop()`

## Service in Task Manager / Services.msc

After `install`, the service appears in:
- `services.msc` as "ReadSync Spike (Phase 0)"
- Task Manager → Services tab

## Notes

- The service writes a heartbeat log to Windows Event Log every 5 seconds
- Start type: Manual (not Automatic) — change in production
- The spike uses no CGO — builds with `CGO_ENABLED=0`
