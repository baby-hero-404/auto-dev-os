# Specs: LLM Prompt & Tool Flow Hardening

## Added Requirements

### REQ-001: Execution-Time Tool Capability Enforcement
> ❌ Status: Not Started

**Scenario:**
- WHEN an LLM tool call is received for execution (any tool name, not just `search_replace`/`create_file`)
- THEN `tool.Registry.Execute` (or a wrapping executor) must verify the calling agent's role is authorized for that tool's declared capability before running it
- AND an unauthorized call must return a clean `Error:`-prefixed tool result (consistent with existing tool error conventions) instead of executing

### REQ-002: Partial-Result Salvage on Tool-Loop Exhaustion
> ❌ Status: Not Started

**Scenario:**
- WHEN `RunToolLoop` reaches `maxIterations` without a valid final parsed answer
- AND at least one edit tool call (`search_replace`/`create_file`) succeeded earlier in the same run
- THEN the loop must return a partial-result indicator (not just a hard error) so the caller (`patch_retry_loop.go`) can decide whether to accept the partial edits and run targeted tests, instead of unconditionally discarding completed work

**Scenario:**
- WHEN a partial result is accepted and the caller is about to run targeted tests against the salvaged edits
- THEN a secondary git checkpoint of the current worktree state must be created BEFORE the test command executes
- AND IF the targeted test run hangs, crashes, or corrupts the worktree, THEN the system must be able to restore to that secondary checkpoint without losing the salvaged edits or leaving the worktree in an undefined state

## Modified Requirements

### REQ-M01: Harness Independence Enforced in Production Gateway
> ❌ Status: Not Started

**Scenario:**
- WHEN the Review step sets `RouteOptions.ExcludeModelID` to the model that generated the code under review
- AND `AIGateway.ChatWithOptions` resolves the fallback chain of models for the configured level group
- THEN the resolved chain must exclude the specified model ID
- AND IF excluding it would leave zero eligible models in the level group, THEN the gateway must gracefully fall back to using the excluded model anyway (logged as a warning), matching the behavior already proven in `pkg/llm/router_test.go:131-155`

**Scenario:**
- WHEN the graceful fallback fires (the coder's model is reused for review because no alternative exists)
- THEN the gateway must record this event on a mutable trace object propagated via context (mirroring the existing `BudgetTrace` pattern), not only via a `slog` log line
- AND the Review step must read that trace after the call and set `self_review_fallback: true` (plus the reused model ID) on its output
- AND that flag must propagate the same way `review_limit_exceeded` already does — into checkpoint state and the PR body warning section — so a human reviewer or dashboard can filter for tasks whose review independence was compromised

### REQ-M02: Tool-Loop Circuit Breaker Counts Fully-Blocked Rounds
> ❌ Status: Not Started

**Scenario:**
- WHEN every tool call requested by the LLM in a single loop round is blocked by the per-`(tool, path)` circuit breaker (2+ prior failures on that path)
- THEN that round must still count toward `maxIterations` (no `i--`)
- AND WHEN `maxIterations` is reached under repeated-blocked-call conditions, THEN the loop must terminate with the existing `"exceeded max iterations"` error rather than continuing indefinitely

**Scenario:**
- WHEN the LLM repeatedly calls a path-less tool (`run_tests`, `run_build`, `run_lint`) that produces the same failure
- THEN the circuit breaker must throttle it after a bounded number of repeats, the same way path-bearing tool calls are throttled today

### REQ-M03: Prompt Assembly Surfaces Rule-Load Failures
> ❌ Status: Not Started

**Scenario:**
- WHEN `PromptAssembler.collect()` calls `loadRules(ctx, task.ProjectID)` and it returns an error
- THEN the error must be logged at `error` level with the task/project ID
- AND the resulting prompt must not silently proceed as if zero rules exist without any trace of the failure being recorded

**Scenario:**
- WHEN `resolveSkills`, `RetrieveContext`, `GetRepoMap`, or `json.Unmarshal(task.Analysis, ...)` fail inside `collect()`
- THEN each failure must be logged at `warn` level (distinguishing "intentionally empty" from "failed to load") instead of being discarded via `_ = err`

### REQ-M04: Persistent Credential Cooldowns
> ❌ Status: Not Started

**Scenario:**
- WHEN a `(credential, model)` pair is put into cooldown after a transient failure
- THEN the cooldown state must survive an API server process restart
- AND the cooldown state must be visible to other horizontally-scaled API replicas, not just the instance that observed the failure

**Scenario:**
- WHEN a replica reads credential cooldown state from its in-process cache instead of the persisted store (to avoid a DB round-trip on every credential selection)
- THEN that cache entry must have an explicit, named TTL (default 15s)
- AND after the TTL expires, the next read must re-fetch from the persisted store, bounding how stale a replica's view of another replica's cooldown can be

### REQ-M05: Unified Transient-Error Classification
> ❌ Status: Not Started

**Scenario:**
- WHEN a provider call fails with a network-level error (timeout, connection refused, EOF, dial failure)
- THEN the same classifier used for HTTP-status/rate-limit errors must classify it as transient
- AND the credential/model pair must be cooled down at the point of failure (inner gateway layer), not only retried by the outer `llmrunner` layer without cooldown

### REQ-M06: Tool-Loop Token Growth Bounded
> ❌ Status: Not Started

**Scenario:**
- WHEN a `run_tests`/`run_build`/`run_lint` tool result exceeds a configured size threshold
- THEN the result appended to the loop's message history must be truncated (with a clear "output truncated" marker) instead of appended in full

**Scenario:**
- WHEN the LLM calls `read_file` on a `(path, args)` combination already read earlier in the same tool-loop run
- THEN the loop must return a short "already read at turn N" note instead of re-appending the full file content

### REQ-M07: PromptAssembler.collect() Decomposition
> ❌ Status: Not Started

**Scenario:**
- WHEN `PromptAssembler.collect()` is invoked
- THEN it must delegate to named per-concern helper functions (e.g. base/role prompt loading, layered rules, reviewer-vs-general context routing, semantic context retrieval, repo map) each returning `(PromptSection, error)`
- AND no single function in the prompt-assembly package should exceed ~150 lines

### REQ-M08: AnalyzeStep Migrated to Shared Tool Loop
> ❌ Status: Not Started

**Scenario:**
- WHEN the Analyze step runs its agentic tool-calling loop
- THEN it must invoke the shared `llmrunner.RunToolLoop` (the same entrypoint used by `code_backend`/`code_frontend`/`fix`/`review`)
- AND `AnalyzeStep.runAnalyzeLLMLoop` as a separate implementation must be removed

### REQ-M09: Minor Reliability Fixes
> ❌ Status: Not Started

**Scenario:**
- WHEN `NineRouter` issues an HTTP request to its backend
- THEN the request must be bounded by an explicit `http.Client.Timeout`, consistent with the other three providers

**Scenario:**
- WHEN the outer retry loop in `llmrunner/runner.go` waits before retrying
- THEN the wait must be ctx-aware (`select` on `ctx.Done()`), not a plain `time.Sleep`

**Scenario:**
- WHEN `AIGateway.ChatWithOptions` falls back across multiple provider/credential attempts before succeeding or failing
- THEN a usage/cost record must be captured for each attempted provider/credential, not only the final one

## Removed Requirements
- None.
