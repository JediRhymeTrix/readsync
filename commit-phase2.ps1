# commit-phase2.ps1  –  Stage, commit, cherry-pick Phase 2 onto master.
# Usage: pwsh -File commit-phase2.ps1
Set-StrictMode -Version Latest
$ErrorActionPreference = 'Stop'
$WT = "C:\Users\Vedant\.cline\worktrees\474d9\ReadSync"
$MR = "C:\Users\Vedant\OneDrive\Dev\CLINE\Projects\ReadSync"
function gWT { & git -C $WT @args 2>&1; if($LASTEXITCODE){throw "git WT $args exit $LASTEXITCODE"} }
function gMR { & git -C $MR @args 2>&1; if($LASTEXITCODE){throw "git MR $args exit $LASTEXITCODE"} }

Write-Host "=== 1. Stage all changes in task worktree ===" -ForegroundColor Cyan
gWT add --all
Write-Host (gWT status --short | Out-String)

Write-Host "=== 2. Commit in task worktree ===" -ForegroundColor Cyan
$msg = "feat(calibre): implement Phase 2 Calibre adapter`n`nDiscovery, custom columns, read/write paths, health, tests.`nSee phase2-manifest.md for full details."
gWT commit -m $msg
$taskHash = (gWT rev-parse HEAD).Trim()
Write-Host "Task commit: $taskHash" -ForegroundColor Green

Write-Host "=== 3. Verify master branch ===" -ForegroundColor Cyan
$branch = (gMR rev-parse --abbrev-ref HEAD).Trim()
if ($branch -ne "master") { throw "Main repo not on master (got: $branch)" }

Write-Host "=== 4. Stash if needed ===" -ForegroundColor Cyan
$dirty = gMR status --porcelain
$stashRef = $null
if ($dirty) {
    gMR stash push -u -m "kanban-pre-cherry-pick"
    $stashRef = ((gMR stash list --format="%gd %s") | Select-String "kanban-pre-cherry-pick" | Select-Object -First 1).ToString().Split(' ')[0]
    Write-Host "Stashed: $stashRef"
}

Write-Host "=== 5. Cherry-pick ===" -ForegroundColor Cyan
$lock = Join-Path $MR ".git\index.lock"
if (Test-Path $lock) {
    Start-Sleep 3
    if ((Test-Path $lock) -and -not (Get-Process git -EA 0)) { Remove-Item $lock -Force }
}
gMR cherry-pick $taskHash
$finalHash = (gMR rev-parse HEAD).Trim()
Write-Host "Cherry-pick OK. Master HEAD: $finalHash" -ForegroundColor Green

Write-Host "=== 6. Restore stash ===" -ForegroundColor Cyan
if ($stashRef) { gMR stash pop $stashRef; Write-Host "Stash restored." }

Write-Host "`n=== COMPLETE ===" -ForegroundColor Green
Write-Host "Final commit hash : $finalHash"
Write-Host "Commit message    : feat(calibre): implement Phase 2 Calibre adapter"
Write-Host "Stash used        : $(if($stashRef){$stashRef}else{'No'})"
