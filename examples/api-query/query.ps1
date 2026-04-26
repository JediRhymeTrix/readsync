#!/usr/bin/env pwsh
# examples/api-query/query.ps1
#
# Query the ReadSync admin API: service status, adapter health, open conflicts.
#
# Prerequisites:
#   - ReadSync service running on http://127.0.0.1:7201/
#   - OR: go run ./tests/fakeserver -port 7201
#
# Usage: .\query.ps1 [-BaseUrl http://127.0.0.1:7201]

param(
    [string]$BaseUrl = "http://127.0.0.1:7201"
)

Set-StrictMode -Version Latest
$ErrorActionPreference = "Stop"

function Invoke-Api {
    param([string]$Path)
    try {
        Invoke-RestMethod -Uri "$BaseUrl$Path" -Method GET
    } catch {
        Write-Warning "GET $Path failed: $_"
        $null
    }
}

Write-Host "==> ReadSync Admin API Query" -ForegroundColor Cyan
Write-Host "    Base URL: $BaseUrl"
Write-Host ""

# 1. Get CSRF token (needed for write endpoints)
Write-Host "--- CSRF Token ---" -ForegroundColor Yellow
$csrf = Invoke-Api "/csrf"
if ($csrf) {
    Write-Host "Token: $($csrf.csrf)"
    $csrfToken = $csrf.csrf
} else {
    Write-Warning "Could not fetch CSRF token — write examples will fail."
    $csrfToken = ""
}
Write-Host ""

# 2. Service status
Write-Host "--- Service Status ---" -ForegroundColor Yellow
$status = Invoke-Api "/api/status"
if ($status) {
    $status | Format-List
}

# 3. Adapter health
Write-Host "--- Adapter Health ---" -ForegroundColor Yellow
$adapters = Invoke-Api "/api/adapters"
if ($adapters) {
    $adapters | Format-Table -AutoSize
}

# 4. Open conflicts (example — POST to resolve requires CSRF)
Write-Host "--- Open Conflicts ---" -ForegroundColor Yellow
$conflicts = Invoke-Api "/api/conflicts"
if ($conflicts -and $conflicts.conflicts) {
    $conflicts.conflicts | Format-Table -Property id, book_id, reason, status -AutoSize
} else {
    Write-Host "(no open conflicts)"
}
Write-Host ""

# 5. Trigger sync-now (write endpoint — requires CSRF)
if ($csrfToken) {
    Write-Host "--- Trigger Sync Now ---" -ForegroundColor Yellow
    try {
        $result = Invoke-RestMethod `
            -Uri "$BaseUrl/api/sync/now" `
            -Method POST `
            -Headers @{ "X-ReadSync-CSRF" = $csrfToken }
        Write-Host "Sync triggered: $($result | ConvertTo-Json -Compress)"
    } catch {
        Write-Warning "Sync trigger failed: $_"
    }
}

Write-Host ""
Write-Host "Done." -ForegroundColor Green
