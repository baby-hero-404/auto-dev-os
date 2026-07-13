# Tasks: LLM Prompt & Tool Flow Hardening

## P0 — Critical

### Task 1.1: Enforce Harness Independence in Production Gateway
> Links to: REQ-M01

**Acceptance Criteria:**
- [x] `internal/gateway/gateway.go`'s route-entry resolution reads `RouteOptions.ExcludeModelID` from context and excludes any entry whose model matches, mirroring the logic already proven in `pkg/llm/router.go:166-169`.
- [x] If exclusion would leave zero eligible entries in the level group, the gateway falls back to using the excluded model anyway and logs a warning via `slog` (not `fmt.Printf`), mirroring `pkg/llm/router.go:190-209`.
- [x] A new test in `internal/gateway/gateway_test.go` covers both the exclusion case and the graceful single-model fallback case, analogous to `pkg/llm/router_test.go:131-155`.
- [x] Manual verification: run a task through Review with `ExcludeModelID` set to the coder's model and confirm (via trace/logs) a different model was actually selected.
- [x] **Enhancement:** new `pkg/llm/route_trace.go` implements `RouteTrace`/`WithRouteTrace`/`RouteTraceFromCtx` (mirroring the existing `BudgetTrace` pattern); `gateway.go`'s graceful-fallback branch sets `RouteTrace.SelfReviewFallback = true` when it fires.
- [x] **Enhancement:** `steps/review.go` reads the trace after the LLM call and sets `out["self_review_fallback"] = true` (+ the reused model ID) when the flag is set.
- [x] **Enhancement:** the `self_review_fallback` flag propagates into checkpoint state and the PR body warning section, following the exact existing pattern used for `review_limit_exceeded`.
- [x] **Enhancement:** a test confirms that when the fallback fires, the Review step's output contains `self_review_fallback: true`, and when it doesn't fire (a different model was actually used), the field is absent/false.

### Task 1.2: Fix Tool-Loop Circuit Breaker Unbounded-Cost Loophole
> Links to: REQ-M02

**Acceptance Criteria:**
- [x] `toolloop.go:98-99` no longer decrements `i` when a round's calls were all blocked by the circuit breaker (`anyExecuted == false`) — that round counts toward `maxIterations` like any other.
- [x] A test simulates the LLM repeatedly issuing the same already-blocked `(tool, path)` call and asserts the loop terminates at `maxIterations` with the existing `"exceeded max iterations"` error, rather than looping past it.
- [x] Path-less tools (`run_tests`, `run_build`, `run_lint`) are now covered by an equivalent repeat-call throttle (extend `extractPath`/circuit-breaker key logic to fall back to the tool name alone when no `path` argument exists).
- [x] Existing `TestRunToolLoop_CircuitBreaker` (`toolloop_test.go:144-193`) still passes; a new test covers the "never resolves" scenario explicitly.

### Task 1.3: Surface Prompt-Assembly Rule-Load Failures
> Links to: REQ-M03

**Acceptance Criteria:**
- [x] `builder.go:468-469`'s `if err == nil { ... }` block around `loadRules` is replaced with explicit error handling: log at `error` level (task ID, project ID, error) when `loadRules` fails.
- [x] `resolveSkills` (`builder.go:457-458`), `RetrieveContext` (`builder.go:769`), and `GetRepoMap` (`builder.go:812`) failures are each logged at `warn` level instead of silently discarded via `_ = err` / ignored return.
- [x] `json.Unmarshal(task.Analysis, &analysis)` call sites (`builder.go:330,407`, `assembler.go:203`) log at `warn` level on unmarshal failure instead of silently proceeding with a zero-value struct.
- [x] A test asserts that when `loadRules` returns an error, the resulting log output contains an identifiable error entry (not just a smaller-than-expected prompt with no trace).

## P1 — High

### Task 2.1: Persist Credential Cooldowns
> Links to: REQ-M04

**Acceptance Criteria:**
- [x] New `CredentialCooldown` model + migration (additive only) as described in `design.md`.
- [x] `CredentialPoolService.SelectCredential`/`SetCooldown` (`credential_router.go`) read/write through the persisted store, with an in-process cache to avoid a synchronous DB read on every credential selection.
- [x] A cooldown set by one process instance is observable by a second instance reading from the same database (integration test or documented manual verification).
- [x] Restarting the process does not clear an active cooldown (verified by test: set cooldown, simulate restart of the in-memory cache, confirm cooldown still active from persisted store).
- [x] **Enhancement:** the in-process read cache uses a named constant `cooldownCacheTTL = 15 * time.Second` (not an unbounded/write-only-invalidated cache) — a cache entry older than the TTL is re-fetched from the persisted store on next read.
- [x] **Enhancement:** a test confirms that a cache entry older than `cooldownCacheTTL` triggers a fresh DB read rather than returning stale data, and that a fresh entry within the TTL window does not.

### Task 2.2: Unify Transient-Error Classification
> Links to: REQ-M05

**Acceptance Criteria:**
- [x] Single `isTransientError` function replaces the two separate implementations at `internal/gateway/gateway.go:353-365` and `internal/orchestrator/llmrunner/runner.go:402-419`; it recognizes both HTTP-status/rate-limit substrings AND generic network errors (timeout, connection refused, EOF, dial failure).
- [x] The unified classifier lives in a shared location (e.g. `pkg/llm` or `internal/gateway`) importable by both the gateway and the runner.
- [x] A network-timeout-simulating test confirms the credential/model pair is cooled down at the gateway layer (not just retried blindly by the outer runner layer).

### Task 2.3: Partial-Result Salvage & Retry Context Carry-Forward
> Links to: REQ-002

**Acceptance Criteria:**
- [x] `RunToolLoop` tracks which `search_replace`/`create_file` calls succeeded during the run.
- [x] On `maxIterations` exhaustion, if at least one edit call succeeded, the loop returns a partial-result signal (per `design.md`'s `ToolLoopResult`) instead of only a hard error.
- [x] `patch_retry_loop.go` (`:87-90`) checks for a partial result and, when present, runs targeted tests against the applied edits before deciding whether to retry or accept, instead of immediately `return nil, false, err`.
- [x] When an outer retry does start a fresh attempt, the retry instruction includes a short list of file paths already read in the prior attempt (best-effort context carry-forward), sourced from the discarded conversation's tool-call history.
- [x] **Enhancement:** before running targeted tests against a salvaged partial result, `patch_retry_loop.go` calls `CreateGitCheckpoint(stepID+"_salvage")` to snapshot the worktree state.
- [x] **Enhancement:** if the targeted test run fails in a way that leaves the worktree corrupted/hung, the system restores to that salvage checkpoint (`RestoreGitCheckpoint`) rather than the last full-step checkpoint (which would lose the salvaged edits) or nothing at all (which risks a corrupted worktree persisting into the next retry).
- [x] **Enhancement:** a checkpoint-creation failure at this point is logged at `error` level and does NOT block using the partial result — it degrades to "no safety net for this test run," not "discard the salvaged edits."
- [x] **Enhancement:** a test simulates a targeted-test failure after a salvage checkpoint and confirms the worktree is restorable to the salvaged (not pre-edit) state.it) state.

## P2 — Medium

### Task 3.1: Execution-Time Tool Capability Enforcement
> Links to: REQ-001

**Acceptance Criteria:**
- [x] `tool.Registry.Execute` (or a wrapping executor invoked by every call site, not just the boundary-checked one) receives the calling agent's role and rejects execution for tools not in that role's `DefaultRoleProfiles()` capability set.
- [x] Rejection returns a clean `Error:`-prefixed message consistent with existing tool error conventions (e.g. `tool_executor.go`'s formatting), not a raw Go error.
- [x] A test confirms a reviewer-role tool call to `search_replace` (an edit tool the reviewer profile doesn't include) is rejected before `patch.EvaluatePolicy` or any filesystem mutation runs.
- [x] Existing legitimate role/tool combinations (backend → `search_replace`, reviewer → `read_file`/`grep_search`/`git_diff`) continue to work unchanged.

### Task 3.2: Bound Tool-Loop Token Growth
> Links to: REQ-M06

**Acceptance Criteria:**
- [x] Tool results exceeding `maxToolResultChars` (per `design.md`) are truncated before being appended to the loop's message history, with a visible "truncated" marker.
- [x] `run_tests`/`run_build`/`run_lint` outputs specifically are covered (currently `tools/run_tests.go:125` appends unbounded stdout+stderr).
- [x] Within a single `RunToolLoop` invocation, a repeated `read_file` call on an already-read `(path, line-range)` returns a short "already read at turn N" note instead of the full content again.
- [x] A test with a run producing large tool output (>10K chars) confirms the appended message is truncated and the loop still completes successfully.

### Task 3.3: Decompose PromptAssembler.collect()
> Links to: REQ-M07

**Acceptance Criteria:**
- [x] Before refactoring: capture golden-file prompt snapshots for at least 3 representative (task, agent, step) combinations (one coding step, one review step, one analyze step).
- [x] `collect()` (`builder.go:401-824`) is split into named helper functions — at minimum: base/role prompt loading, layered rules assembly, reviewer-vs-general context routing, semantic context retrieval, repo map construction — each independently testable and each returning `(PromptSection, error)`.
- [x] No single function in `prompts/builder.go` exceeds ~150 lines after the split.
- [x] Golden-file snapshots from the "before" state match the "after" state byte-for-byte (or documented, intentional diffs only — e.g. newly-added error logging shouldn't change prompt *content*).

## P3 — Low

### Task 4.1: Name Magic Number Constants in Prompt Builder
> Links to: REQ-M07

**Acceptance Criteria:**
- [x] `Priority`/`RenderOrder` integer literals passed to `NewPromptSection(...)` throughout `collect()` (22+ call sites) are replaced with named constants (e.g. `PriorityBasePrompt`, `PriorityJITSkills`).
- [x] JIT skill limit (`builder.go:387`, currently `5`), scoring weights (`builder.go:355,364,367,372`), semantic snippet caps (`builder.go:739-741,756-758`, currently `8`/`4`), repo-map token clamps (`builder.go:806-809`, `2048`/`256`), and snippet dedup overlap threshold (`helpers.go:67`, `0.5`) are all named constants, following the existing pattern of `assembler.go:28,32` (`defaultPromptBudget`, `promptBudgetReserveRatio`).

### Task 4.2: Migrate AnalyzeStep to Shared Tool Loop
> Links to: REQ-M08

**Acceptance Criteria:**
- [x] `AnalyzeStep` calls `llmrunner.RunToolLoop` instead of its own `runAnalyzeLLMLoop` (`analyze.go:242-440`).
- [x] `runAnalyzeLLMLoop` is deleted once migration is confirmed working (no dead code left behind).
- [x] Existing `analyze_test.go` suite passes against the migrated implementation; any analyze-specific behavior (contract field validation) is preserved via the shared loop's `Validate` callback mechanism.

### Task 4.3: Provider & Retry Reliability Cleanups
> Links to: REQ-M09

**Acceptance Criteria:**
- [x] `NineRouter`'s `http.Client` (`nine_router.go:31`) has an explicit `Timeout` matching the other three providers (5 minutes).
- [x] Outer retry backoff sleeps in `llmrunner/runner.go:125,228` use a ctx-aware wait (`select` on `ctx.Done()` / timer) instead of plain `time.Sleep`.
- [x] `AIGateway.ChatWithOptions` records one usage/cost row per attempted provider/credential (not only the last), matching the granularity already present in the unused `pkg/llm.Gateway` (`router.go:174-206`).
- [x] `fmt.Printf` calls in `gemini.go:169` and `router.go:191` are replaced with structured `slog` calls at the appropriate level.
