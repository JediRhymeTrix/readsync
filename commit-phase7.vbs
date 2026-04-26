' deleted
Set objEnv = objShell.Environment("PROCESS")

Dim worktree : worktree = "C:\Users\Vedant\.cline\worktrees\89846\ReadSync"
Dim mainRepo : mainRepo = "C:\Users\Vedant\OneDrive\Dev\CLINE\Projects\ReadSync"

Dim commitMsg
commitMsg = "Phase 7: QA and hardening pass against spec section 20 acceptance criteria" & Chr(10) & Chr(10) & _
"- resolver/resolver_bands_test.go: all 5 confidence bands + all 10 signals" & Chr(10) & _
"- conflicts/conflict_scenario_test.go: spec §6 three-way scenario + all 5 detectors" & Chr(10) & _
"- outbox/statemachine_test.go: full state machine + backoff tests" & Chr(10) & _
"- logging/redact_corpus_test.go: corpus sweep + boundary tests" & Chr(10) & _
"- calibre/calibre_unit_test.go: ISBN-13/10, ASIN, raw-locator, columns" & Chr(10) & _
"- moon/parser_extended_test.go: boundaries, malformed, case, annotations" & Chr(10) & _
"- koreader/translate_test.go: locatorType, toAdapterEvent, canonicalToPull" & Chr(10) & _
"- tests/integration/e2e_test.go: fake push -> canonical -> outbox" & Chr(10) & _
"- tests/integration/conflict_scenario_test.go: spec §6 pipeline + backward jump" & Chr(10) & _
"- tests/security/admin_ui_test.go: loopback, CSRF, secrets" & Chr(10) & _
"- CHANGELOG.md, README.md, docs/diagnostic-bundle.md" & Chr(10) & _
"- docs/qa/acceptance-checklist.md: all 14 spec §20 ACs mapped to tests" & Chr(10) & _
"- .phase7-manifest.md, Makefile, .github/workflows/ci.yml"

Dim gitExe : gitExe = Chr(34) & "C:\Program Files\Git\cmd\git.exe" & Chr(34)

' Stage all changes in the worktree
Dim addCmd : addCmd = gitExe & " -C " & Chr(34) & worktree & Chr(34) & " add -A"
Dim ret : ret = objShell.Run(addCmd, 0, True)
If ret <> 0 Then WScript.Echo "git add failed: " & ret : WScript.Quit 1

' Write commit message to a temp file
Dim fso : Set fso = CreateObject("Scripting.FileSystemObject")
Dim tmpFile : tmpFile = objEnv("TEMP") & "\phase7-commit-msg.txt"
Dim f : Set f = fso.CreateTextFile(tmpFile, True, False)
f.Write commitMsg
f.Close

' Commit in the worktree
Dim commitCmd : commitCmd = gitExe & " -C " & Chr(34) & worktree & Chr(34) & " commit -F " & Chr(34) & tmpFile & Chr(34)
ret = objShell.Run(commitCmd, 0, True)
If ret <> 0 Then WScript.Echo "git commit failed: " & ret : WScript.Quit 1

' Get the new commit hash
Dim revParseCmd : revParseCmd = gitExe & " -C " & Chr(34) & worktree & Chr(34) & " rev-parse HEAD"
Dim objExec : Set objExec = objShell.Exec(revParseCmd)
Dim taskCommit : taskCommit = Trim(objExec.StdOut.ReadAll())
WScript.Echo "Task commit: " & taskCommit

' Cherry-pick onto master in main repo
Dim cpCmd : cpCmd = gitExe & " -C " & Chr(34) & mainRepo & Chr(34) & " cherry-pick " & taskCommit
ret = objShell.Run(cpCmd, 0, True)
If ret <> 0 Then WScript.Echo "cherry-pick failed: " & ret : WScript.Quit 1

' Get final master hash
Set objExec = objShell.Exec(gitExe & " -C " & Chr(34) & mainRepo & Chr(34) & " rev-parse HEAD")
Dim masterCommit : masterCommit = Trim(objExec.StdOut.ReadAll())
WScript.Echo "Master now at: " & masterCommit
WScript.Echo "Phase 7 commit complete."
