$w = 'C:\Users\Vedant\.cline\worktrees\89846\ReadSync'
$m = 'C:\Users\Vedant\OneDrive\Dev\CLINE\Projects\ReadSync'

Set-Location $w
git add -A
$c = (git diff --cached --name-only 2>&1)
if ($c) {
    $tmp = [IO.Path]::GetTempFileName()
    Set-Content $tmp 'Phase 7: QA and hardening pass against spec section 20 acceptance criteria'
    git commit -F $tmp
    Remove-Item $tmp
}
$sha = (git rev-parse HEAD 2>&1)

Set-Location $m
$dirty = (git status --porcelain 2>&1)
$stash = $null
if ($dirty) {
    git stash push -u -m kanban-pre-cherry-pick
    $stash = (git stash list --format=%gd -1 2>&1)
}
git cherry-pick $sha
$final = (git rev-parse HEAD 2>&1)
if ($stash) { git stash pop $stash }

Write-Host "Done. master is now: $final"
