# installer/smoke.ps1
#
# Installer smoke test (Phase 6). Run from an elevated PowerShell on a
# clean Windows VM:
#
#   PS> .\installer\smoke.ps1 -Installer dist\ReadSync-0.6.0-setup.exe
#
# What it does:
#   1. Runs the installer in /VERYSILENT mode.
#   2. Polls Get-Service ReadSync until Running (max 30 seconds).
#   3. Hits http://127.0.0.1:7201/healthz; expects 200.
#   4. Hits the wizard page; expects status 200 with "Setup Wizard" in body.
#   5. Runs the installer's uninstaller (also silent).
#   6. Confirms Get-Service ReadSync errors (i.e. service is gone).
#
# The script exits non-zero on any step failure so CI can gate releases.

[CmdletBinding()]
param(
    [Parameter(Mandatory = $true)]
    [string]$Installer,

    [string]$AdminUrl = 'http://127.0.0.1:7201',

    [int]$WaitSeconds = 30
)

$ErrorActionPreference = 'Stop'

function Invoke-Step($name, $block) {
    Write-Host "==> $name" -ForegroundColor Cyan
    & $block
    if ($LASTEXITCODE -gt 0) {
        Write-Error "Step failed: $name (exit $LASTEXITCODE)"
    }
}

if (-not (Test-Path $Installer)) {
    Write-Error "Installer not found: $Installer"
}

Invoke-Step 'Run installer (silent)' {
    Start-Process -FilePath $Installer -ArgumentList '/VERYSILENT','/SUPPRESSMSGBOXES','/NORESTART' -Wait -NoNewWindow
}

Invoke-Step 'Wait for service to start' {
    $deadline = (Get-Date).AddSeconds($WaitSeconds)
    do {
        Start-Sleep -Seconds 1
        try {
            $svc = Get-Service -Name 'ReadSync' -ErrorAction Stop
            if ($svc.Status -eq 'Running') {
                Write-Host "Service is Running."
                return
            }
        } catch {
            # service not yet registered; keep polling
        }
    } while ((Get-Date) -lt $deadline)
    throw "ReadSync service did not reach Running within ${WaitSeconds}s"
}

Invoke-Step 'Probe /healthz' {
    $r = Invoke-WebRequest -Uri "$AdminUrl/healthz" -UseBasicParsing
    if ($r.StatusCode -ne 200) {
        throw "/healthz returned $($r.StatusCode)"
    }
}

Invoke-Step 'Probe wizard page' {
    $r = Invoke-WebRequest -Uri "$AdminUrl/ui/wizard" -UseBasicParsing
    if ($r.StatusCode -ne 200) {
        throw "/ui/wizard returned $($r.StatusCode)"
    }
    if ($r.Content -notmatch 'Setup Wizard') {
        throw "wizard body missing 'Setup Wizard' marker"
    }
}

Invoke-Step 'Probe CSRF rejection' {
    try {
        $r = Invoke-WebRequest -Uri "$AdminUrl/api/wizard/complete" `
            -Method Post -UseBasicParsing -ErrorAction Stop
        throw "POST without CSRF unexpectedly succeeded (status $($r.StatusCode))"
    } catch [System.Net.WebException] {
        $code = [int]$_.Exception.Response.StatusCode
        if ($code -ne 403) {
            throw "Expected 403, got $code"
        }
    }
}

Invoke-Step 'Run uninstaller (silent)' {
    $uninst = Join-Path ${env:ProgramFiles} 'ReadSync\unins000.exe'
    if (-not (Test-Path $uninst)) {
        $uninst = Join-Path ${env:ProgramFiles} 'ReadSync\unins001.exe'
    }
    if (-not (Test-Path $uninst)) {
        throw "uninstaller not found in $env:ProgramFiles\ReadSync"
    }
    Start-Process -FilePath $uninst -ArgumentList '/VERYSILENT','/SUPPRESSMSGBOXES' -Wait -NoNewWindow
}

Invoke-Step 'Verify service removed' {
    Start-Sleep -Seconds 2
    try {
        Get-Service -Name 'ReadSync' -ErrorAction Stop | Out-Null
        throw 'service still registered after uninstall'
    } catch [Microsoft.PowerShell.Commands.ServiceCommandException] {
        # Expected: service does not exist.
    }
}

Write-Host "`nSMOKE TEST PASSED" -ForegroundColor Green
