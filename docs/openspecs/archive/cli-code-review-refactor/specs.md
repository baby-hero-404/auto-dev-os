# Expected Behavior: CLI Subsystem Code Review & Refactor

## Scenario: Credential payload path stays within the temp workspace
**When:**
- `CLITestHandler.Terminal` writes a decrypted credential's `payloadMap`
  entries into the per-session temp dir, and one entry's relative path is
  crafted to escape via a sibling-prefix collision (e.g. temp dir is
  `/tmp/auto-code-os-cli-test/<uuid>`, and a payload key resolves to a
  path outside it that happens to share the literal string prefix)

**Then:**
- The write is refused (skipped), identically to how a `../`-containing
  relative path is already skipped today — no file is ever written outside
  `tmpDir`
- A relative path that resolves inside `tmpDir` (the common case) is written
  exactly as before

## Scenario: WS-ticket-gated terminal handlers share one implementation
**When:**
- `CLIAuthHandler.MintWSTicket`/`.Terminal` and `CLITestHandler.MintWSTicket`/
  `.Terminal` run their mint → consume → upgrade → workspace → cleanup flow

**Then:**
- Both handlers delegate the org-check, ticket mint, ticket consume +
  WS upgrade, temp-workspace creation, and cleanup steps to the same shared
  code path
- Each handler still performs its own distinct pre-step (auth-capture banner
  vs. credential-file priming) and post-step (raw file walk vs. plain exit)
- Existing endpoint behavior (status codes, response JSON shapes, WS message
  types) is unchanged for both handlers

## Scenario: CLI subprocess post-run bookkeeping runs once, from one place
**When:**
- A CLI subprocess run (either `code_backend`/`code_frontend`/`fix` via
  `cliEngineRunner.RunLLMStep`, or `cli_analyze`/`cli_spec`/`cli_implement`
  via `cliStepRunner.RunCLIStep`) completes and returns a `*CodeStepResult`

**Then:**
- Quota-cooldown write-back (`QuotaExceeded` → `SetCooldown`), auth-failure
  write-back (`AuthFailed` → `MarkNeedsReauth`), the finish log line, and the
  `cli_output` checkpoint artifact save all happen through one shared helper
  invoked by both call sites
- The caller-specific behavior (empty-output message construction using
  `res.ExitCode`/`res.Command`, and what happens after — noop-check for
  `RunLLMStep`, `ChangedFiles` lookup for `RunCLIStep`) is untouched

## Rules
- No change to which conditions are treated as quota-exceeded or
  auth-failed — only where the resulting write-back call happens from.
- No change to WS wire format (message `type`s, ticket query param, JSON
  field names) — this is an internal refactor, not an API change.
- The path-containment fix must reject exactly the same "resolves outside
  tmpDir" cases the current `..`-based check already implicitly covers, plus
  the sibling-prefix case it currently misses.

## Constraints
- `server/internal/orchestrator/engine` must not import
  `server/internal/handler` or vice versa — the two refactors (post-run
  helper, WS-terminal helper) stay in their existing packages.
- The shared post-run helper takes concrete types already available to both
  callers (`*Orchestrator`, task/job IDs, credential ID, `*engine.CodeStepResult`)
  — no new interface abstraction introduced solely for this.
