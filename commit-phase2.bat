@echo off
REM commit-phase2.bat
REM Stages, commits Phase 2 changes in task worktree, cherry-picks onto master.
REM Run this from any directory.

setlocal
set WT=C:\Users\Vedant\.cline\worktrees\474d9\ReadSync
set MR=C:\Users\Vedant\OneDrive\Dev\CLINE\Projects\ReadSync

echo === Step 1: Stage all changes in task worktree ===
git -C "%WT%" add --all
if errorlevel 1 goto :fail

echo === Step 2: Commit in task worktree ===
git -C "%WT%" commit -m "feat(calibre): implement Phase 2 Calibre adapter" -m "Discovery, custom columns, read/write paths, health, tests. See phase2-manifest.md."
if errorlevel 1 goto :fail

for /f %%h in ('git -C "%WT%" rev-parse HEAD') do set TASK_HASH=%%h
echo Task commit: %TASK_HASH%

echo === Step 3: Verify master branch ===
for /f %%b in ('git -C "%MR%" rev-parse --abbrev-ref HEAD') do set BRANCH=%%b
if not "%BRANCH%"=="master" (
    echo ERROR: Main repo not on master, got: %BRANCH%
    goto :fail
)

echo === Step 4: Stash if dirty ===
git -C "%MR%" diff --quiet --exit-code
if errorlevel 1 (
    git -C "%MR%" stash push -u -m "kanban-pre-cherry-pick"
    set STASHED=1
) else (
    git -C "%MR%" diff --cached --quiet --exit-code
    if errorlevel 1 (
        git -C "%MR%" stash push -u -m "kanban-pre-cherry-pick"
        set STASHED=1
    ) else (
        set STASHED=0
    )
)

echo === Step 5: Cherry-pick ===
git -C "%MR%" cherry-pick %TASK_HASH%
if errorlevel 1 (
    echo Cherry-pick failed. Resolve conflicts in %MR% then:
    echo   git -C "%MR%" cherry-pick --continue
    goto :fail
)

for /f %%h in ('git -C "%MR%" rev-parse HEAD') do set FINAL_HASH=%%h
echo Cherry-pick OK. Master HEAD: %FINAL_HASH%

echo === Step 6: Restore stash ===
if "%STASHED%"=="1" (
    git -C "%MR%" stash pop
    if errorlevel 1 echo WARNING: stash pop had issues - resolve manually
)

echo.
echo === COMPLETE ===
echo Final commit hash: %FINAL_HASH%
echo Commit message   : feat(calibre): implement Phase 2 Calibre adapter
echo Stash used       : %STASHED%
goto :eof

:fail
echo FAILED - check errors above
exit /b 1
