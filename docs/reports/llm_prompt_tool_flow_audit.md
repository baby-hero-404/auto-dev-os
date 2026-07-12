# LLM Call, Prompt Build & Tool-Calling Flow — Audit

> **Date:** 2026-07-12
> **Scope:** `server/pkg/llm/*` (providers, gateway/router), `server/internal/gateway/*` (production AIGateway), `server/internal/prompts/*` (prompt assembly), `server/internal/orchestrator/llmrunner/*` (runner, agentic tool loop), `server/internal/tool/*` (tool registry, capability manager)
> **Method:** Full source-code trace against the current tree, verifying every claim in three prior reports (`llm_call_architecture_report.md`, `prompt_construction_report_v2.md`, `verified_architecture_findings.md`, all dated 2026-07-09/10) against today's code, since recent commits ("introduce agentic tool loop", "centralized tool registry and granular capability-based tool system", "modular pipeline assembler and JIT skill routing") materially changed this layer.
> **Supersedes:** the LLM-call-path findings in the three reports above where they conflict with this one. Those reports remain useful for their "what changed since v1" narrative; this report is the current source of truth for this layer.

---

## Executive Summary

The single biggest claim in the prior reports — **"coding/review steps are single-shot, only `analyze` gets real tool calls"** — is **no longer true**. `code_backend`, `code_frontend`, `fix`, and `review` all now drive the same agentic tool-calling loop as `analyze`, via a shared, generalized `RunToolLoop` (`llmrunner/toolloop.go`). Several other previously-flagged gaps (hardcoded token budget, silent budget pruning, double context injection, redundant re-indexing) are also confirmed fixed.

However, this audit found **three new correctness/safety bugs** that are more severe than anything left open in the prior reports, all introduced or exposed by the same recent work that fixed the single-shot problem:

1. **Harness Independence is dead in production.** The Review step's self-review-bias mitigation (`WithExcludeModelID`) is wired end-to-end through the orchestrator and runner, but the actual production `AIGateway` (`internal/gateway/gateway.go`) never reads it. Reviewers can be — and by default will be — routed to the exact same model that wrote the code under review.
2. **The tool-loop circuit breaker has a loophole that defeats its own purpose.** When every tool call in a turn is blocked by the "2 failures on the same path" breaker, the loop artificially un-consumes that iteration (`i--`). If the model keeps retrying an already-blocked call, `maxIterations` is never reached — a scenario the safety net was built to prevent becomes the one case where it doesn't apply.
3. **`PromptAssembler.collect()` cannot report its own failures.** The 424-line function that builds every prompt section has exactly one `return`. If rule-loading fails, the entire rules block (global/role/project/task) is dropped silently — a task can go to the LLM with zero governance rules and nothing will show it happened.

None of these are hard to fix (see Priority Recommendations). All three are the kind of bug that only shows up in production under specific failure conditions, which is exactly why source-level verification caught them and log-reading wouldn't have.

---

## Part 1 — What Was Fixed (Confirmed Against Prior Reports)

| # | Prior claim | Verdict | Evidence |
|---|---|---|---|
| 1 | "Two incompatible LLM call paths" — coding/review had no tool support | ✅ **FIXED** | `llm_step.go:21-26` (`stepIsAgentic`) whitelists `review`, `code_backend*`, `code_frontend*`, `fix`; all drive `RunToolLoop` via `ChatWithOptions`, not `Chat()`. |
| 2 | Token budget hardcoded at 8192 | ✅ **FIXED** | `assembler.go:40-53` (`resolvePromptBudget`) derives budget from `MaxContextTokens × 0.7`, falling back to 8192 only when no model metadata is available. |
| 3 | No observability when `optimizeBudget()` drops sections | ✅ **FIXED** | `builder.go:862-905` logs initial tokens/limit and every dropped section (name, tokens, priority) to a `BudgetTrace`. |
| 4 | Dual markdown + JSON injection wastes tokens | ✅ **FIXED for coding steps** / 🟡 partial overlap remains for analyze/plan | `isCodingStep()` gate (`assembler.go:164-168`) excludes `ProposalMD`/`SpecsMD`/`DesignMD` markdown for coding steps entirely — only structured JSON survives. Analyze/plan still get both markdown and a JSON extract, which is legitimate (structured subset, not the same prose re-serialized) but has some conceptual overlap. |
| 5 | Repository context re-indexed on every LLM call | ✅ **FIXED** | `IsRetryCtxKey` (`assembler.go:101-114`) gates cache bypass; non-retry calls read `ContextCache` from `ContextLoadStep` instead of re-calling `RetrieveContext`/`GetRepoMap`. Only retries do a live re-fetch (intentional, since retry needs fresh state). |
| 6 | Double context injection — same file as semantic snippet AND full-content dump, no dedup | ✅ **FIXED** | `filterAffectedFileSnippets` (`helpers.go:79-98`) explicitly drops snippets for files already delivered via `AffectedFiles` full-content injection. |

---

## Part 2 — Critical Findings (New)

### Finding 1 — Harness Independence does nothing in production

**Severity: Critical (correctness + product-quality bug, silent)**

The design is real and half-built:
- `pkg/llm/provider.go:106-117` defines `WithExcludeModelID`/`ExcludeModelIDFromContext`.
- `steps/review.go:244-251` correctly looks up the coder's model from the last `code_backend`/`code_frontend`/`fix` checkpoint and calls `WithExcludeModelID` before the review LLM call.
- `llmrunner/runner.go:101` correctly copies it into `RouteOptions.ExcludeModelID` on every call.
- **But `internal/gateway/gateway.go` — the actual `AIGateway` used in production (wired in `cmd/api/main.go:341-351` whenever a credential pool exists, which is the normal case) — never reads `opts.ExcludeModelID` anywhere.** `grep -n "ExcludeModelID" internal/gateway/gateway.go` returns nothing.

The exclusion logic *is* correctly implemented — in `pkg/llm/router.go`'s `Gateway` type, including the graceful-fallback edge case (excluding the only model in a level group falls back to using it anyway, with a warning) and a passing test (`router_test.go:131-155`). But that `Gateway` is dead code in the deployed system — it's only reachable via a static/no-DB-credential-pool config path that production doesn't use.

**Impact:** every Review step in production can silently be run by the same model that wrote the code, defeating the entire point of harness independence (avoiding "consensus bias" / self-review blind spots) documented as a design goal in `docs/features/engineering/02-context-pruning-and-harness-independence.md`.

**Fix:** port the exclusion filter from `pkg/llm/router.go:166-209` into `internal/gateway/gateway.go`'s `routeEntries`/entry-selection logic (around `gateway.go:194-266`), including the graceful single-model fallback. Add a test analogous to `router_test.go:131-155` against the real `AIGateway`.

---

### Finding 2 — Tool-loop circuit breaker has a loophole that makes it unbounded

**Severity: Critical (cost/availability bug)**

`toolloop.go`'s per-`(tool, path)` circuit breaker blocks a tool call after 2 failures on the same path (`toolloop.go:73-79`) — good. But when *every* call in a turn gets blocked this way, `anyExecuted` stays `false`, and the loop does `i--` to avoid consuming an iteration (`toolloop.go:98-100`), reasoning that "nothing actually happened this round." The bug: a blocked call is never re-executed, so `failureCounts[key]` never increments past its threshold and never resets. If the model keeps re-issuing the same already-blocked call, every round still costs a full LLM API call, but `i` never advances — `maxIterations` (8 for coding/review steps, `runner.go:217`) is **never reached**. The one scenario this breaker exists to catch (a model stuck repeating a failing action) is the exact scenario where the iteration cap stops applying.

This is untested: `toolloop_test.go:144-193` (`TestRunToolLoop_CircuitBreaker`) only exercises 3 blocked calls before a mock forces success on the 4th — it never simulates the model never giving up.

Related, smaller gap: the breaker keys on an `args.path` field (`extractPath`, `toolloop.go:143-151`), so it doesn't apply at all to tools without a path argument (`run_tests`, `run_build`, `run_lint`) — repeated unproductive calls to those are unthrottled by any mechanism.

**Fix:** don't decrement `i` when the round was entirely blocked calls — let it count toward `maxIterations` like any other unproductive round. If avoiding token waste on "wasted" rounds is the actual goal, cap total *blocked* rounds separately (e.g. abort after 2 all-blocked rounds) rather than exempting them from the main iteration budget.

---

### Finding 3 — `PromptAssembler.collect()` cannot surface its own errors

**Severity: Critical (silent degraded-prompt risk, safety-relevant)**

`collect()` (`builder.go:401-824`, 424 lines) has exactly one `return` statement (`:823`, `return sections, nil`) despite declaring `([]PromptSection, error)`. Every internal failure inside it is structurally unable to reach the caller. The concrete instances found:

- **Most severe:** `globalRules, projectRules, err := a.loadRules(...)` then `if err == nil { ...build all rule sections... }` (`builder.go:468-469`). If `loadRules` fails (DB error), the entire block that builds Global Rules, Role Constraints, Project Rules (strict + advisory), and Task Rules (`:470-550`) is skipped with **no log, no error, no fallback**. A task can be sent to the LLM with zero governance/security rules and there is no signal anywhere that this happened.
- `_ = json.Unmarshal(task.Analysis, &analysis)` (`builder.go:330`, `:407`, `assembler.go:203`) — a corrupt/truncated `Analysis` blob silently produces a zero-value `TaskAnalysis`.
- `resolveSkills` error ignored (`builder.go:457-458`) — JIT skills silently omitted.
- `RetrieveContext`/`GetRepoMap` errors ignored (`builder.go:769`, `:812`) — semantic context / repo map silently empty.

**Fix:** thread real errors out of `collect()` for at least the rules-loading path (the one with actual security/governance content) — either return the error, or explicitly log at `error` level and continue with a documented degraded-mode marker so it's visible in traces. The JSON-unmarshal and context-retrieval swallows are lower priority but should at minimum get a `warn`-level log each, matching the pattern already used elsewhere in this file (e.g. `builder.go`'s own `BudgetTrace` logging).

---

## Part 3 — High-Severity Findings

### Finding 4 — Credential cooldowns are in-memory only

`CredentialPoolService.modelCooldowns` (`credential_pool.go:52`) is a plain Go map, written on every real failure (`gateway.go:255` always calls `SetCooldown` with a non-empty model). Lost on process restart; not shared across horizontally-scaled replicas — each instance can independently hammer a credential the others have already backed off from. A separate DB-persisted cooldown path exists (`repository/provider_credential.go:108`) but is only reachable when `model == ""`, which no real failure path ever passes — it's effectively dead code today.

**Fix:** persist per-(credential, model) cooldowns (even a simple `UPDATE ... SET cooldown_until` keyed by credential+model would do), or accept the current design but document the single-replica assumption explicitly so it isn't rediscovered the hard way during a scale-out.

### Finding 5 — Two disagreeing transient-error classifiers

`internal/gateway/gateway.go:353-365`'s `isTransientError` only matches HTTP-status-like substrings and rate-limit/quota phrases — not generic network errors (timeout, connection refused, EOF, dial failure). `llmrunner/runner.go:402-419`'s classifier is broader and does match those. Net effect: a plain network timeout isn't retried or cooled-down by the inner (correct, credential-aware) layer — it breaks immediately to the next model entry without cooling the credential — and only gets retried at all because the *outer* runner-level retry reissues the entire `AIGateway.ChatWithOptions` call from scratch. A persistently-unreachable credential can be reselected on every request instead of being backed off.

**Fix:** unify the two classifiers into one shared function; make the inner (gateway-level) retry the canonical place network errors get handled, since it's the layer that actually knows which credential to cool down.

### Finding 6 — Tool registry has no execution-time capability check

`CapabilityManager.ToolsForRole()` filters which tool *definitions* get advertised to the LLM per role (front-door only). `Registry.Execute()` (`tool/registry.go:36-44`) does a bare name lookup with no check that the calling agent's role is actually allowed to invoke that tool. `boundary_tool_executor.go` layers a real policy check on top, but **only for `search_replace`/`create_file`** (`boundary_tool_executor.go:19-22`) — every other tool (including `read_file`, `run_tests`, `grep_search`) passes straight through unfiltered. In practice this is defense-in-depth against a hallucinated/off-list tool call or a future prompt-injection scenario, not an active exploit today (mainstream function-calling APIs generally do restrict calls to the advertised tool list) — but there's no server-side backstop if that assumption ever breaks.

**Fix:** add a role/capability check inside `Registry.Execute` (or a wrapping executor) for every tool call, not just edit-capability ones.

### Finding 7 — No partial-result salvage when the tool loop exhausts its budget

If `RunToolLoop` hits `maxIterations` without a valid final answer, it returns a hard error (`toolloop.go:140`) that propagates all the way to the step failing — even though the tool calls already made real edits to the workspace via `search_replace`/`create_file`. That work isn't surfaced as a partial/best-effort result; the whole step is a hard failure with no salvage path, and (per the outer-retry finding below) the next attempt starts from scratch, discarding what was already learned/done.

**Fix:** on iteration exhaustion, check whether any edit tool calls succeeded; if so, capture a partial summary and let the caller decide whether to accept partial progress + targeted-test it, rather than always hard-failing.

### Finding 8 — Outer retry discards the inner tool loop's conversation

Each outer retry (`patch_retry_loop.go`, up to 3 attempts) calls `RunLLMStep` fresh; `Runner.initialMessages()` always rebuilds from `AssemblePrompt(..., history=nil)` (`runner.go:348-352`). All `read_file`/tool results from the prior attempt are discarded — only a short `retryErrorMsg` string carries forward. The LLM has to re-read files it already read in the previous attempt. Not a correctness bug (each attempt is self-consistent), but a real efficiency loss layered on top of Finding 9's per-loop cost.

**Fix:** consider carrying forward a compact summary of files already read (paths + hashes) so the model can skip redundant reads, or at minimum note in the retry instruction which files were already inspected.

---

## Part 4 — Medium-Severity Findings

| # | Finding | Evidence | Fix |
|---|---|---|---|
| 9 | **Unbounded token growth within one tool-loop run.** No truncation inside `RunToolLoop`; `run_tests`/`run_build` results append full stdout+stderr with no cap; `TruncateHistory` exists but only applies to cross-step history (`history=nil` for every fresh loop, so it never fires here). A run hitting 8 iterations could plausibly reach 20K-80K+ tokens by the final call, since every prior tool result is resent every turn. | `toolloop.go:48-141`, `tools/run_tests.go:125`, `runner.go:348-352` | Cap tool-result size (esp. test/build output) before appending to messages; consider summarizing older tool results once the conversation crosses a size threshold. |
| 10 | **No read-dedup.** Nothing stops the LLM from calling `read_file` on the identical path 5+ times within one loop — each costs a full read + full appended result. | `tools/read_file.go:56-151` | Cache `(path, args)` → result within a single loop run; on repeat, return a short "already read, see turn N" note instead of the full content. |
| 11 | **`collect()` is a 424-line, single-return monolith** — worse than the 300+ lines the prior report flagged, since JIT skills/tools/execution-manifest/frozen-context logic has been added since without decomposition. | `builder.go:401-824` | Split into named per-concern helpers (base/role prompts, rules, reviewer-vs-general routing, semantic context, repo map) that each return `(PromptSection, error)`, composed by a thin `collect()`. |
| 12 | **Scattered magic numbers**, inconsistent with the one constant that *was* named (`defaultPromptBudget`). 22+ inline `Priority`/`RenderOrder` literal pairs, JIT skill limit (`5`), scoring weights (`15/5/3/2`), snippet caps (`8`/`4`), repo-map token clamps (`2048`/`256`), dedup overlap threshold (`0.5`) — none are named constants. | `builder.go:355-372,387,437,445,492,564,701,739-741,756-758,798,806-809`; `helpers.go:67` | Promote to named constants in one place (mirroring `assembler.go:28,32`), so tuning them doesn't require hunting through a 424-line function. |
| 13 | **`AnalyzeStep` still has its own hand-rolled tool loop** (`runAnalyzeLLMLoop`, `analyze.go:242-440`) instead of the new shared `RunToolLoop` — the doc comment in `toolloop.go:44-47` explicitly says it generalizes analyze's pattern "so review/coding steps can reuse it," but analyze itself was never migrated. Its loop also has no circuit breaker at all (arguably safer than Finding 2's buggy one, but inconsistent). | `analyze.go:242-440` vs `toolloop.go` | Migrate `analyze` onto `RunToolLoop` for one code path instead of two; decide deliberately whether it needs the circuit breaker or not, rather than diverging by omission. |
| 14 | `NineRouter` provider has no `http.Client.Timeout` (all other providers set 5 min); relies entirely on caller context having a deadline. | `nine_router.go:31` | Add an explicit timeout matching the other providers. |
| 15 | Outer retry backoff sleeps (`llmrunner/runner.go:125,228`) use plain `time.Sleep` instead of ctx-aware waiting (the gateway-level backoff one layer down already does this correctly) — cancellation can be delayed up to ~4s. | `runner.go:125,228` vs `gateway.go:241-247` | Swap to a `select` on `ctx.Done()` / timer, matching the gateway's pattern. |
| 16 | Usage/cost telemetry only records the **last** attempted provider/credential per call (`gateway.go:125-160`) — if a call succeeded after failing over several models, earlier failed attempts aren't visible in cost/usage records (only in the free-text error string on total failure). | `gateway.go:112-187` | Record one usage row per attempt (the unused `pkg/llm.Gateway` already does this, `router.go:174-206`) for real fallback-chain observability. |
| 17 | Stray `fmt.Printf` debug/warning output bypassing the codebase's structured `slog` logging. | `gemini.go:169`, `router.go:191` | Replace with `slog` calls (or delete if genuinely just debug scaffolding). |

---

## Part 5 — What's Working Well

Confirmed correct on inspection, worth calling out so future changes don't accidentally regress them:

- **Bounded fallback loop.** The outer `AIGateway.ChatWithOptions` full-cycle retry (`gateway.go:206-296`) is capped at exactly 2 passes via a `retried` flag — it cannot spin indefinitely even if every credential is cooling down simultaneously.
- **Credential pool concurrency.** `CredentialPoolService.mu sync.Mutex` correctly guards every access to the cooldown/round-robin maps; `recoveryCounter` uses `atomic`. No missing-lock bug found.
- **Context propagation & cancellation** for the actual HTTP calls: every provider uses `http.NewRequestWithContext`, and a task-level cancel (`orchestrator/queue.go:50-51`) genuinely propagates down to interrupt an in-flight LLM call.
- **Tool error messages are clean and actionable** — `read_file`/`search_replace` failures return structured, human-readable diagnostics (not raw Go errors), consistently prefixed so the loop's own logic can parse them.
- **Sequential tool execution** within a turn avoids any concurrent-mutation risk on the message history — a deliberate, correct design choice (at some latency cost).
- **Boundary policy enforcement** for edit tools (`search_replace`/`create_file`) is real: `SeverityCritical` correctly pauses the task for human review rather than silently proceeding.
- **Cost/error surfacing to the task record**: gateway failures produce a detailed per-provider/per-credential failure string that survives intact through `llmrunner` → `worker.go` → the task's `last_error` field. Nothing is silently swallowed at that layer (contrast with Finding 3's prompt-assembly swallows).

---

## Priority Recommendations

| Priority | Finding | Fix | Impact |
|:--:|---|---|---|
| 🔴 P0 | #1 Harness Independence dead in prod | Port exclusion filter from `pkg/llm/router.go` into `internal/gateway/gateway.go` | Restores the self-review-bias mitigation that's currently a no-op |
| 🔴 P0 | #2 Circuit breaker loophole | Don't `i--` on all-blocked rounds; let them count toward `maxIterations` | Closes an unbounded-cost loop |
| 🔴 P0 | #3 `collect()` swallows rule-load errors | Surface/log the `loadRules` failure path at minimum | Prevents tasks running with zero governance rules undetected |
| 🟠 P1 | #4 In-memory-only cooldowns | Persist per-credential-model cooldowns, or explicitly document single-replica assumption | Avoids repeated hammering of a bad credential across restarts/replicas |
| 🟠 P1 | #5 Disagreeing transient-error classifiers | Unify into one shared classifier, canonical at the gateway layer | Correct retry targeting + credential cooldown for network errors |
| 🟠 P1 | #7 No partial-result salvage on loop exhaustion | Surface partial edits instead of always hard-failing | Avoids throwing away real completed work |
| 🟡 P2 | #6 No execution-time tool capability check | Add role check inside `Registry.Execute` for all tools, not just edit tools | Defense-in-depth against off-list tool calls |
| 🟡 P2 | #9/#10 Unbounded tool-loop token growth, no read-dedup | Cap tool-result size; memoize reads within a loop run | Real cost/latency savings on long-running tasks |
| 🟡 P2 | #11 `collect()` monolith | Split into named per-concern helpers | Maintainability, testability, makes #3 easier to fix properly |
| ⚪ P3 | #12 Magic numbers | Name the constants | Tuning without spelunking a 424-line function |
| ⚪ P3 | #13 Analyze's own loop | Migrate to shared `RunToolLoop` | One code path instead of two for the same pattern |
| ⚪ P3 | #14-17 | Timeout on NineRouter, ctx-aware outer backoff, per-attempt usage records, replace stray `fmt.Printf` | Small reliability/observability wins |

---

## Appendix — Current Verified Call Flow

```
Orchestrator.runLLMStep (llm_step.go:40-76)
  │  stepIsAgentic(stepID)? → wire Tools + BoundaryCheckedToolExecutor
  ▼
llmrunner.Runner.Run (runner.go:104-106)
  │  isAgentic := Tools != nil && ToolExecutor != nil
  ├─ false → r.Provider.Chat(ctx, messages)               [effectively unreachable in prod]
  └─ true  → runAgentic() → RunToolLoop (toolloop.go)
              │  loop (max 8 for coding/review/fix, 6 default):
              │    Provider.ChatWithOptions(ctx, messages, tools)
              │    ├─ tool calls → execute sequentially via BoundaryCheckedToolExecutor
              │    │                 → Registry.Execute → real tool impl (internal/tool/tools/*)
              │    │                 → append role:"tool" result, continue loop
              │    └─ final text  → ParseJSONMarkdown → validate → return
              ▼
r.Provider = *gateway.AIGateway (internal/gateway/gateway.go)
  │  entries := ResolveModels(orgID, levelGroup)   [priority-ordered fallback chain]
  │  for each entry:
  │    cred := CredentialPool.SelectCredential(...)
  │    provider := providerFromCredential(cred, entry.Model)  [OpenAI/Anthropic/Gemini/NineRouter]
  │    for up to 4 attempts (transient-error retry, exp backoff):
  │      provider.ChatWithOptions(ctx, messages, opts) → real HTTP call
  │    on failure → SetCooldown(cred, model) [in-memory only] → next entry
  │  (bounded to 2 full passes over entries)
  ▼
Concrete provider (llm.Anthropic / llm.OpenAI / llm.Gemini / llm.NineRouter)
  → http.NewRequestWithContext → POST to provider API
```

**Note:** `RouteOptions.ExcludeModelID` is stamped into `ctx` at the top of this chain (`runner.go:94-102`) but is never read anywhere inside `AIGateway.ChatWithOptions` — see Finding 1.
