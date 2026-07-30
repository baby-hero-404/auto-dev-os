# Implementation Map: CLI Subsystem Code Review & Refactor

**Goal:** Fix the broken path-containment check in the CLI credential-test
terminal, and remove duplication between the two WS-ticket terminal handlers
and the two CLI post-run bookkeeping call sites.
**Tech Stack:** Go (server/internal/handler, server/internal/orchestrator)

---

## Phase 1: Fix path-containment bug

### Replace prefix-string check with filepath.Rel in cli_test_handler.go

**Why:**
`strings.HasPrefix(filepath.Clean(fullPath), filepath.Clean(tmpDir))` can be
fooled by a sibling directory that shares tmpDir's string prefix. The correct
pattern (`filepath.Rel` + reject `".."` or absolute-escape) is already used in
`engine/cli.go`'s context-file writer — mirror it here instead of inventing a
new check.

**Depends on:** none

**Files:**
- `server/internal/handler/cli_test_handler.go`

**Changes:**
- [x] Replace the `filepath.Clean`/`HasPrefix` traversal guard with a
      `filepath.Rel(tmpDir, fullPath)` check that rejects `rel == ".."` or
      `strings.HasPrefix(rel, ".."+string(filepath.Separator))`, matching
      `engine/cli.go`'s `RunCodeStep` context-file guard
- [x] Replace `providerName[:4] == "cli:"` with `strings.CutPrefix(providerName, "cli:")`

**Verify:**
- [x] New unit test: a payload key that resolves via prefix-collision outside
      `tmpDir` is skipped and no file is written
- [x] Existing unit test: a normal relative payload key still writes correctly
      inside `tmpDir`
- [x] `go test ./server/internal/handler/...` passes

## Phase 2: Extract shared WS-ticket-terminal helper

### Add ws_terminal.go with the shared mint/consume/upgrade/workspace flow

**Why:**
`CLIAuthHandler` and `CLITestHandler` duplicate the org-check → mint-ticket
HTTP handler and the ticket-consume → WS-upgrade → temp-workspace → cleanup
sequence almost verbatim. Centralizing it means a future fix (e.g. ticket
error message wording) is written once.

**Depends on:** none

**Files:**
- `server/internal/handler/ws_terminal.go` (new)
- `server/internal/handler/cli_auth.go`
- `server/internal/handler/cli_test_handler.go`

**Changes:**
- [x] Add a `wsTerminalHandler` helper in `ws_terminal.go` exposing:
      an org-checked mint-ticket HTTP handler function taking a
      request-body-decode callback, and a terminal-setup function that does
      ticket consume + org check + WS upgrade + `newTerminalWorkspace` +
      returns the connection/taskID/tmpDir/cleanup (or writes the HTTP error
      and returns ok=false)
- [x] Update `CLIAuthHandler.MintWSTicket`/`.Terminal` to call the shared
      helper, keeping only the auth-capture-specific banner + file-walk logic
- [x] Update `CLITestHandler.MintWSTicket`/`.Terminal` to call the shared
      helper, keeping only the credential-priming-specific logic

**Verify:**
- [x] Existing tests for both handlers (mint ticket, org mismatch, invalid
      ticket, missing provider/credential_id) still pass unchanged
- [x] `go test ./server/internal/handler/...` passes
- [x] `go vet ./server/internal/handler/...` clean

## Phase 3: Extract shared CLI post-run bookkeeping

### Add finishCLIRun helper used by both cliEngineRunner and cliStepRunner

**Why:**
`RunLLMStep` (cli_engine_step.go) and `RunCLIStep` (cli_spec_step.go)
duplicate the quota-cooldown write-back, auth-failure write-back, finish log
line, and `cli_output` checkpoint save. One helper removes the risk of the
two drifting (e.g. one gaining a new write-back the other doesn't).

**Depends on:** none

**Files:**
- `server/internal/orchestrator/cli_engine_step.go` (add helper here, or a
  new `cli_run_common.go` if that reads cleaner once written)
- `server/internal/orchestrator/cli_spec_step.go`

**Changes:**
- [x] Add an unexported helper (e.g. `(o *Orchestrator) finishCLIRun(ctx,
      taskID, jobID, stepID, credID string, res *engine.CodeStepResult)`)
      that performs: quota cooldown write-back, auth-failure write-back, the
      `"%s: cli engine finished (...)"` log line, and the `cli_output`
      checkpoint artifact save (with the empty-output fallback message)
- [x] Call it from `cliEngineRunner.RunLLMStep` right after `RunCodeStep`
      returns successfully, replacing the inlined block
- [x] Call it from `cliStepRunner.RunCLIStep` right after `RunCodeStep`
      returns successfully, replacing the inlined block

**Verify:**
- [x] Existing tests in `cli_engine_step_test.go` and `cli_spec_step_test.go`
      (quota cooldown set, auth-needs-reauth set, checkpoint artifact
      content on empty output) still pass unchanged
- [x] `go test ./server/internal/orchestrator/...` passes
- [x] `go build ./...` succeeds
