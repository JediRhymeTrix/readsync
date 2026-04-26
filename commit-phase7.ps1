Set-StrictMode -Version Latest
$ErrorActionPreference = 'Stop'
$Worktree = 'C:\Users\Vedant\.cline\worktrees\89846\ReadSync'
$MainRepo = 'C:\Users\Vedant\OneDrive\Dev\CLINE\Projects\ReadSync'

$nl = [Environment]::NewLine
$Msg = "Phase 7: QA and hardening pass against spec section 20 acceptance criteria" + $nl
$Msg += $nl + "Unit tests" + $nl
$Msg += "- resolver/resolver_bands_test.go: all 5 confidence bands, all 10 signals" + $nl
$Msg += "- conflicts/conflict_scenario_test.go: spec sec-6 three-way scenario" + $nl
$Msg += "  (KOReader 72%/Calibre 70%/Goodreads 38%), all 5 suspicious-jump detectors" + $nl
$Msg += "- outbox/statemachine_test.go: full state machine + backoff tests" + $nl
$Msg += "- logging/redact_corpus_test.go: corpus sweep (6 secrets), boundary words" + $nl
$Msg += "- calibre/calibre_unit_test.go: ISBN-13/10, ASIN, raw-locator, columns" + $nl
$Msg += "- moon/parser_extended_test.go: boundaries, malformed, case, annotations" + $nl
$Msg += "- koreader/translate_test.go: locatorType, toAdapterEvent, canonicalToPull" + $nl
$Msg += $nl + "Integration tests with fakes (CGO required)" + $nl
$Msg += "- tests/integration/e2e_test.go: fake KOReader push -> canonical -> outbox" + $nl
$Msg += "- tests/integration/conflict_scenario_test.go: spec sec-6 pipeline scenario" + $nl
$Msg += "  (single ISBN; Goodreads finished-from-<85% blocked; backward-jump test)" + $nl
$Msg += $nl + "Security tests" + $nl
$Msg += "- tests/security/admin_ui_test.go: loopback-only, CSRF on 11 endpoints," + $nl
$Msg += "  GET bypass, valid token, secrets absent from JSONL output" + $nl
$Msg += $nl + "Release prep" + $nl
$Msg += "- CHANGELOG.md, README.md setup walkthrough, docs/diagnostic-bundle.md" + $nl
$Msg += "- docs/qa/acceptance-checklist.md: all 14 spec sec-20 ACs mapped to tests" + $nl
$Msg += "- Makefile: test-phase7-unit/test-security/test-integration targets" + $nl
$Msg += "- .github/workflows/ci.yml: phase7-unit/security/integration CI jobs" + $nl
$Msg += "- .phase7-manifest.md: test-count table (+86 tests) and AC matrix"

function Invoke-Git {
    param([string]$Repo, [string[]]$GitArgs)
    Write-Host ("  git " + ($GitArgs -join " ")) -ForegroundColor DarkGray
    # Merge stderr into stdout so warnings don't become PowerShell ErrorRecords,
    # then separate them back out by type for clean error reporting.
    $combined = & git -C $Repo @GitArgs 2>&1
    $exitCode = $LASTEXITCODE
    $stdout = $combined | Where-Object { $_ -isnot [System.Management.Automation.ErrorRecord] }
    if ($exitCode -ne 0) {
        $errText = ($combined | Where-Object { $_ -is [System.Management.Automation.ErrorRecord] }) -join "`n"
        throw ("git " + ($GitArgs -join " ") + " failed (exit $exitCode):`n$errText")
    }
    return $stdout
}

Write-Host ""
Write-Host "==> Verifying paths..." -ForegroundColor Cyan
foreach ($p in @($Worktree, $MainRepo)) {
    if (-not (Test-Path $p)) {
        Write-Error ("Path not found: " + $p)
    }
}
Write-Host ("  Worktree HEAD : " + (Invoke-Git $Worktree @('rev-parse','HEAD')))
Write-Host ("  master HEAD   : " + (Invoke-Git $MainRepo @('rev-parse','refs/heads/master')))

Write-Host ""
Write-Host "==> Checking main repo for uncommitted changes..." -ForegroundColor Cyan
$stashRef = $null
$dirty = Invoke-Git $MainRepo @('status','--porcelain')
if ($dirty) {
    Write-Host "  Stashing..." -ForegroundColor Yellow
    Invoke-Git $MainRepo @('stash','push','-u','-m','kanban-pre-cherry-pick')
    $stashRef = Invoke-Git $MainRepo @('stash','list','--format=%gd','-1')
    Write-Host ("  Stash ref: " + $stashRef)
}

Write-Host ""
Write-Host "==> Staging Phase 7 files..." -ForegroundColor Cyan
Invoke-Git $Worktree @('add','-A')
$staged = Invoke-Git $Worktree @('diff','--cached','--name-only')

$taskCommit = $null
if ($staged) {
    Write-Host ("  " + $staged.Count + " file(s) staged.")
    Write-Host ""
    Write-Host "==> Committing in worktree..." -ForegroundColor Cyan
    $tmp = [IO.Path]::GetTempFileName()
    [IO.File]::WriteAllText($tmp, $Msg, [Text.Encoding]::UTF8)
    Invoke-Git $Worktree @('commit','-F',$tmp)
    Remove-Item $tmp -Force
    $taskCommit = Invoke-Git $Worktree @('rev-parse','HEAD')
} else {
    Write-Host "  Nothing new to stage - re-using current HEAD." -ForegroundColor Yellow
    $taskCommit = Invoke-Git $Worktree @('rev-parse','HEAD')
}
Write-Host ("  Task commit: " + $taskCommit) -ForegroundColor Green

Write-Host ""
Write-Host "==> Cherry-picking onto master..." -ForegroundColor Cyan
Invoke-Git $MainRepo @('cherry-pick',$taskCommit)
$finalHash = Invoke-Git $MainRepo @('rev-parse','HEAD')
Write-Host ("  master is now: " + $finalHash) -ForegroundColor Green

if ($stashRef) {
    Write-Host ""
    Write-Host ("==> Restoring stash " + $stashRef + "...") -ForegroundColor Cyan
    Invoke-Git $MainRepo @('stash','pop',$stashRef)
}

Write-Host ""
Write-Host "===== Phase 7 commit complete =====" -ForegroundColor Green
Write-Host ("  Final commit hash : " + $finalHash)
Write-Host ("  Stash used        : " + $(if ($stashRef) { $stashRef } else { "no" }))
Write-Host "  Conflicts         : none"
