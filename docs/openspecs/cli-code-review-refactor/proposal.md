# Proposal: CLI Subsystem Code Review & Refactor

## Problem
A full review of the CLI-related code (subprocess execution engine, spec-first
workflow steps, and the WS terminal/auth handlers used for CLI credential
capture and testing) surfaced one real correctness/security bug and several
duplication hot spots that increase the odds of the two copies drifting:

1. `CLITestHandler.Terminal` (`server/internal/handler/cli_test_handler.go`)
   guards against path traversal when writing decrypted credential files with
   `strings.HasPrefix(filepath.Clean(fullPath), filepath.Clean(tmpDir))`. This
   is a broken containment check: a sibling directory that merely shares the
   prefix (e.g. `tmpDir="/tmp/x/1"`, `fullPath="/tmp/x/12/secret"`) passes it.
   The rest of the CLI code (`engine/cli.go`'s `ResolveCredentialFiles`/context
   writer) already uses the correct `filepath.Rel` + `".."`-prefix check for
   the exact same class of untrusted-relative-path problem.
2. `CLIAuthHandler` and `CLITestHandler` (both in `server/internal/handler/`)
   hand-roll near-identical WS-ticket mint/consume + upgrade + temp-workspace
   boilerplate. Any future fix to that flow (e.g. ticket error handling) has
   to be applied twice by hand.
3. `cliEngineRunner.RunLLMStep` (`cli_engine_step.go`) and
   `cliStepRunner.RunCLIStep` (`cli_spec_step.go`) duplicate ~20 lines of
   post-run bookkeeping: quota-cooldown write-back, auth-failure write-back,
   result logging, and saving the `cli_output` checkpoint artifact.
4. `CLITestHandler.Terminal` strips the `"cli:"` provider prefix with manual
   index slicing (`providerName[:4] == "cli:"`) instead of `strings.CutPrefix`,
   which every other prefix-strip in this subsystem already uses.

## Goal
Fix the broken path-containment check and remove the two duplication hot
spots, without changing any externally observable behavior of the CLI
execution engine, the spec-first workflow, or the terminal/auth endpoints.

## Success
- The credential-write path in `cli_test_handler.go` can no longer be tricked
  by a crafted relative path into writing outside its intended temp dir.
- Adding a new WS-ticket-gated terminal handler in the future requires
  writing only the parts specific to that handler (ticket payload shape,
  workspace priming), not re-implementing mint/consume/upgrade/cleanup.
- The quota/auth write-back + logging + checkpoint sequence that runs after
  every CLI subprocess invocation exists in exactly one place.

## Decisions
- Fix the traversal check in place using the same `filepath.Rel`-based
  pattern already established in `engine/cli.go`, rather than introducing a
  new helper — keeps the fix minimal and consistent with existing code.
- Extract the shared WS-ticket-handler boilerplate (org check, mint, ticket
  consume + upgrade, workspace priming, cleanup) into a small shared type in
  `server/internal/handler/ws_terminal.go` that `CLIAuthHandler` and
  `CLITestHandler` both compose, rather than a shared base "class" with
  inheritance-style overrides — Go favors composition, and each handler still
  has a distinct pre/post step (auth capture vs. credential injection).
- Extract the shared CLI post-run bookkeeping (quota/auth write-back, log,
  checkpoint save) into a single unexported helper function used by both
  `cliEngineRunner.RunLLMStep` and `cliStepRunner.RunCLIStep`, taking the
  `*Orchestrator`, task/job IDs, step ID, credential ID, and `*CodeStepResult`
  — no interface needed since both callers already hold an `*Orchestrator`.
- Replace the manual `providerName[:4] == "cli:"` slice with
  `strings.CutPrefix(provider, "cli:")` for consistency with the rest of the
  package.

## Trade-offs
- The shared WS-terminal helper adds one more indirection layer for a reader
  tracing through `CLIAuthHandler.Terminal`; accepted because the two
  handlers were already ~80% identical and the duplication is the harder
  cost to maintain long-term.
- No behavior changes are validated by new integration tests beyond the
  existing unit tests plus one new regression test for the traversal fix —
  the goal is a safe refactor, not new feature coverage.

## Out of Scope
- No changes to the CLI execution engine's quota/auth-failure *detection*
  rules (`cli_quota.go`, `cli_auth_failure.go`) — only where the write-back
  after detection is invoked from.
- No changes to `CLIProfiles` (`cli_profiles.go`), the frontend CLI picker
  (`web/src/lib/cliProfiles.ts`), or the spec-first step prompts/logic
  (`cli_analyze.go`, `cli_spec.go`, `cli_implement.go`, `cli_mr.go`) — these
  were reviewed and found free of the issues above.
- No move to a distributed ticket store (`ws_ticket_store.go` already
  documents the single-replica limitation as an accepted, out-of-scope risk).

## Impact
- `server/internal/handler/cli_test_handler.go` (bug fix + prefix-strip fix)
- `server/internal/handler/cli_auth.go`, `cli_terminal.go` (extract shared type)
- `server/internal/handler/ws_terminal.go` (new, shared WS-ticket-handler helper)
- `server/internal/orchestrator/cli_engine_step.go`, `cli_spec_step.go` (extract shared post-run helper)
- Corresponding `_test.go` files for the above
