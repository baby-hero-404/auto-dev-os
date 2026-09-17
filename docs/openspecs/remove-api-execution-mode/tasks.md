# Implementation Map: CLI-Only Execution

**Goal:** Remove the API-native execution engine entirely, then simplify the surviving CLI pipeline to run unattended to an MR that carries its own OpenSpec.
**Tech Stack:** Go 1.26 (orchestrator, workflow engine, sandbox), PostgreSQL + golang-migrate, Next.js 15 / React (dashboard), Docker sandbox runtime.

**Order matters:** Phase 0 → 1 → 2 → 3 must run in sequence (deletion proceeds leaves-inward so the compiler proves reachability at each step). Phases 4, 5 and 6 are independent of each other and may be done in any order once Phase 3 compiles. Phase 7 last.

---

## Phase 0: Pre-flight & operator note

### Establish who breaks, before anything is deleted

**Why:**
Deploy is a hard break for API-only orgs and for anyone whose gating came from `supervised` autonomy or high-risk patterns. That list has to exist before the migration runs, not after someone's pipeline stops.

**Files:**
- `docs/implementation/remove-api-execution-mode-notes.md` (new)
- `scripts/preflight-cli-only.sql` (new)

**Changes:**
- [ ] Write the pre-flight query: projects with no enabled `type:"cli"` execution-provider row; orgs with no `cli:*` credential; projects with `review_policy != 'always_review'` that are not on `autonomous` autonomy (these lose gating)
- [ ] Export `token_usage` and `provider_models` to CSV for any org that wants cost history retained
- [ ] Write the operator note: what breaks, what to configure, that `000031` is one-way

**Verify:**
- [ ] Query runs against a production snapshot and returns a concrete project/org list
- [ ] Note states plainly that deleted API credentials, `token_usage` rows and embedding vectors are unrecoverable

---

## Phase 1: Delete the leaves (server-side tool loop & runner)

### Remove `internal/tool` and `orchestrator/llmrunner`

**Why:**
Deepest in the dependency graph — the CLI harness brings its own tool loop, so nothing in the CLI pipeline reaches these. Deleting them first makes every later deletion a compiler-verified consequence rather than a judgment call.

**Depends on:** Phase 0

**Files:**
- `server/internal/tool/**` (delete)
- `server/internal/orchestrator/llmrunner/**` (delete)
- `server/internal/prompts/builder.go`, `tools.go`, `compiler.go`, `assembler.go`, `helpers.go` (reduce)
- `server/internal/orchestrator/worker.go` (drop `llmrunner.UnitForStep` at :290)

**Changes:**
- [ ] Delete `internal/tool/` including `tools/` and `registry.go`, `capability.go`
- [ ] Delete `orchestrator/llmrunner/` (runner, toolloop, statemachineloop, step_routing, stallguard + tests)
- [ ] Strip tool-schema assembly from `internal/prompts`; keep only what builds CLI prompt files and `LoadRolePrompt`
- [ ] Remove the `llmrunner.UnitForStep` branch from `worker.go`
- [ ] Delete `internal/prompts/golden_test.go` cases covering tool-schema output; keep CLI prompt-file goldens

**Verify:**
- [ ] `go build ./...` fails only in known API-native call sites (recorded in the notes file), nowhere in `engine/`, `sandbox/`, `gitops/`, `workflow/`
- [ ] `grep -rn "internal/tool" server --include='*.go'` returns nothing
- [ ] Surviving `internal/prompts` tests pass

---

## Phase 2: Delete API-native steps & workflow shapes

### Remove the API-native DAG in one commit with its step constructors

**Why:**
`workflow.Definition` resolves `StepFunc`s by string ID, so a workflow builder referencing a deleted step compiles fine and yields a nil runner at execution time. This is the only silent-failure surface in the whole change — the builders and the steps must die together.

**Depends on:** Phase 1

**Files:**
- `server/internal/orchestrator/steps/`: `analyze.go`, `analyze_parser.go`, `analyze_tools.go`, `plan.go`, `review.go`, `code_backend.go`, `code_frontend.go`, `fix.go`, `testing.go`, `scan.go`, `decompose.go`, `reduce.go`, `patch_retry_loop.go`, `tool_executor.go`, `boundary_tool_executor.go`, `intent_resolver.go`, `compiler_parser.go` + tests (delete)
- `server/internal/orchestrator/`: `llm_step.go`, `llm_trace.go`, `decomposition.go` + tests (delete)
- `server/internal/workflow/step.go` (remove `EasyWorkflow`, `MediumWorkflow`, `HardWorkflow`, `DynamicDAGWorkflow` and their step-ID constants)
- `server/internal/orchestrator/step_registry.go`, `worker.go` (single workflow family)

**Changes:**
- [x] Delete the API-native step files and their tests
- [x] Delete `DefaultWorkflow` / `EasyWorkflow` / `MediumWorkflow` / `HardWorkflow` / `DynamicDAGWorkflow` constructors (`internal/workflow/step.go`) and their now-orphaned tests
- [x] Reduce `stepRunners` to the CLI steps; delete `llmRunnerAdapter`, `codeStepLLM` and the `resolveCLIEngineRunner` swap (there is nothing left to swap from)
- [x] `worker.go`: delete the `shouldUseCLISpecFirstWorkflow` branch — select `CLISpecFirstWorkflow` / `CLISpecFirstParallelWorkflow` on the parallel-implement condition alone
- [ ] Add a test asserting every `StepDefinition.ID` across all surviving workflow definitions maps to a non-nil `StepFunc`
- [x] `checkpoint/recovery.go`: `WithCheckpointRecovery` simplified — dropped the dead analyze-skip/review-fix-rerun/patch-reapply special-casing (unreachable now that no runner produces `StepAnalyze`/`StepReview`/`StepFix`/`StepCodeBackend`/`StepCodeFrontend` events; CLI steps commit to git worktrees directly, so there is no saved patch to reapply on resume), keeping only the generic checkpoint-skip + status-restore behavior. Dropped the now-unused `agent`/`applyPatch` params; updated the call site in `step_registry.go` and both tests in `recovery_test.go`.
- [ ] **Residual debt (not yet done):** the legacy step-ID constants (`StepAnalyze`, `StepPlan`, `StepCodeBackend`, `StepCodeFrontend`, `StepFix`, `StepTest`, `StepPR`; note `StepMerge`/`StepReview` are exceptions still actively used — `StepMerge` by the CLI parallel-implement workflow's merge step, worth double-checking `StepReview` before touching) are still defined in `step.go` and still referenced in dead disjuncts — always-false `strings.HasPrefix`/`==` branches ORed alongside live `cli_*` checks — by `worker.go` (~10 sites, mostly around event-status-to-task-status mapping at lines ~226-434) and `internal/prompts/builder.go` (~4 sites). Each disjunct is individually provably dead (no runner ever emits those step IDs) but interleaved with live CLI-step logic in the same conditionals, so removing them safely needs a careful line-by-line pass, not a bulk deletion.

**Verify:**
- [ ] Workflow-totality test passes for both CLI workflow shapes, with and without `cross_review`
- [x] `go test ./internal/workflow/... ./internal/orchestrator/...` passes
- [ ] `grep -rn "StepAnalyze\|StepPlan\|StepCodeBackend\|StepReview\b" server --include='*.go'` returns only `cli_*` equivalents or nothing (blocked on the residual debt above)

---

## Phase 3: Delete `pkg/llm`, gateway & wiring

### Cut the provider layer and everything that constructs it

**Why:**
With no callers left, this is mechanical — and it is the deletion the whole change exists for.

**Depends on:** Phase 2

**Files:**
- `server/pkg/llm/**` (delete, 17 files)
- `server/internal/gateway/**` (delete)
- `server/internal/service/`: `credential_connection.go`, `provider_model.go`, `embedder_breaker.go` (delete/reduce)
- `server/internal/service/memory_search.go` (drop vector stream)
- `server/internal/repository/analytics.go` (drop `RecordLLMUsage`)
- `server/cmd/api/main.go` (drop `buildLLMProvider`, gateway wiring)
- `server/pkg/config/config.go`, `.env.example`

**Changes:**
- [x] Delete `pkg/llm/` and `internal/gateway/`
- [x] Delete `provider_model.go` service + repository + handler + routes (model choice now lives inside the harness)
- [x] Reduce `MemoryService.Search` to RRF over BM25 + knowledge-graph; delete the embedding call and `embedder_breaker.go`
- [x] Delete `RecordLLMUsage` and repoint analytics cost/token fields at workflow-job telemetry
- [x] Drop `OPENAI_API_KEY`, `ANTHROPIC_API_KEY`, `GEMINI_API_KEY`, `LLM_*`, embedding keys from config and `.env.example`
- [x] Remove LLM provider/gateway construction from `cmd/api/main.go`

**Verify:**
- [ ] `grep -rn "pkg/llm" server --include='*.go'` returns nothing
- [ ] `go build ./... && go test ./...` green
- [ ] Server boots with no LLM API key set in the environment
- [ ] Memory search returns ranked results with the embedding column absent

---

## Phase 4: Lock the contract to CLI-only

### Reject API providers and credentials at the boundary

**Why:**
Deleting the code is not the same as closing the door. Until validation rejects them, stale clients and stored jsonb keep injecting `type:"api"` rows that now resolve to nothing.

**Depends on:** Phase 3

**Files:**
- `server/pkg/models/project.go`, `provider_credential.go`
- `server/internal/orchestrator/execution_router.go`
- `server/migration/000031_remove_api_execution_mode.{up,down}.sql` (new)

**Changes:**
- [x] `validExecutionProviderTypes` → `{"cli"}`; `ValidateExecutionProviders` rejects `"api"` with the message from specs.md
- [x] Drop `"api"` rows from `DefaultExecutionProviderRows` (7 rows → 4) and the api arms of `executionProviderRowForCredential`
- [x] `IsAllowedProvider` → `cli:*` only; delete `ProviderOpenAI`/`ProviderAnthropic`/`ProviderGemini`/`Provider9Router`/`ProviderGateway`
- [x] `ResolvedExecutionProvider`: drop `Type`; `ResolveExecutionProvider` returns an error when no CLI candidate is available
- [x] Delete `legacyResolveExecutionProvider`, `legacyCLIEngineConfig`, `ExecutionEngine*` constants
- [x] Write migration `000031`: delete non-`cli:` credentials, strip `type:"api"` from `projects.execution_providers` and `organizations.default_execution_providers`, drop `projects.execution_engine` / `cli_engine_config`, drop `token_usage` / `provider_models`, drop both `embedding` columns and the hnsw index

**Verify:**
- [ ] `PUT` with an api row → 400 naming the valid CLI refs; nothing persisted
- [ ] `POST` credential `provider: "anthropic"` → 400
- [ ] Task on a project with no enabled CLI row fails with the configured-provider message and starts no sandbox
- [ ] `migrate up` then `migrate down` on a seeded DB leaves a schema matching `000030` in shape

---

## Phase 5: Simplify the gate (auto by default)

### One flag replaces the auto-approval matrix

**Why:**
D7. The six-input matrix is unpredictable and its risk detection is substring matching; the MR is the boundary that actually holds.

**Depends on:** Phase 3 (independent of 4 and 6)

**Files:**
- `server/internal/policy/review_policy.go`
- `server/pkg/models/task.go`, `project.go`
- `server/internal/orchestrator/steps/cli_spec.go`
- `server/internal/service/task.go`, `project.go`
- `server/migration/000032_add_require_spec_review.{up,down}.sql` (new)
- `web/src/components/projects/create-task-panel.tsx`, project settings screen

**Changes:**
- [x] Replace `ShouldAutoApproveSpec` with `RequiresSpecReview` + `ResolveSpecGate` (signatures in design.md)
- [x] Delete `HighRiskPatterns` and `HasHighRiskDomains`; keep `MaxClarificationRounds` and `IsDefinitionOfReadyBypassed`
- [x] Add `Task.RequireSpecReview *bool` and `Project.RequireSpecReview bool`
- [x] Migration `000032`: add both columns; backfill `require_spec_review = true` where `review_policy = 'always_review'`; drop `projects.review_policy`, `projects.autonomy_level`, `agents.autonomy_level`
- [x] `cli_spec` gates on `ResolveSpecGate` alone
- [x] Accept `require_spec_review` on `POST /tasks` and `PATCH /projects/{id}`
- [x] Add the toggle to the create-task panel and project settings, labeled "Review spec before implementing"

**Verify:**
- [ ] Default task (both flags unset) runs `cli_analyze → cli_mr` with no `spec_review` status recorded
- [ ] `require_spec_review: true` on the task halts at `spec_review`; `approve_spec` resumes at `cli_implement` without re-authoring the spec; `request_changes` re-runs `cli_spec`
- [ ] Project `true` + task `false` runs through — explicit false overrides
- [x] Unanswered clarifications gate even with both flags false, and bypass after `MaxClarificationRounds` (see `TestCLISpecStep_ClarificationBypassAfterMaxRounds`/`_ClarificationBypassStillPausesWithoutSpecFiles`)
- [ ] A task touching `auth/` no longer self-gates

---

## Phase 6: MR carries the spec; cross-review on a second harness

### Make the MR the decision surface

**Why:**
D8. With nobody watching the run, the MR is the first place a human sees what the agent decided — the spec has to be there, not behind a toggle.

**Depends on:** Phase 3 (independent of 4 and 5)

**Files:**
- `server/internal/orchestrator/steps/pr.go`
- `server/internal/orchestrator/gitops/pr_generator.go`
- `server/internal/orchestrator/steps/cross_review.go`
- `server/internal/orchestrator/cli_spec_step.go`
- `server/pkg/models/` (drop `IncludeSpecInMR` from the analysis payload)

**Changes:**
- [ ] Make the `includeSpec` branch in `pr.go` unconditional; delete `IncludeSpecInMR` and its review-UI toggle
- [ ] Add `SpecSection` to `PRSummary` and render goal / key decisions / verify steps / clarifications above the change summary
- [ ] Render the `⚠️ No OpenSpec was produced` notice and log a warning when the spec dir is absent — never block the MR
- [ ] Apply `engine/redact.go` to the rendered section
- [ ] Re-host `cross_review` on `CLIStepRunner`; add `resolveReviewProfile` (excludes the coder's ref); delete the `llm.WithExcludeProviderID` / `RunLLMStep` path
- [ ] Skip `cross_review` with `cross_review skipped: only one CLI harness available` when no distinct profile is authenticated

**Verify:**
- [ ] MR body contains the spec section; branch contains `docs/openspecs/<slug>/`
- [ ] Missing spec dir → MR still opens, notice present, warning logged
- [ ] Two authenticated profiles → `cross_review` runs on the non-coder profile
- [ ] One profile → skipped with the logged reason, and the job still reaches `cli_mr`
- [ ] A secret planted in spec text is redacted in the rendered body

---

## Phase 7: Timeouts, roles, docs

### Let long runs finish; standardize role resolution

**Why:**
D9 and D10. The 30-minute profile cap silently overrides the engine's own default downward and kills healthy long runs; role handling is currently applied on some steps and skipped on others.

**Depends on:** Phases 4, 5, 6

**Files:**
- `server/pkg/models/cli_profiles.go`
- `server/internal/orchestrator/engine/cli.go`
- `server/internal/orchestrator/step_registry.go`, `steps/cli_*.go`
- `README.md`, `ARCHITECTURE.md`, `docs/implementation/remove-api-execution-mode-notes.md`

**Changes:**
- [ ] `TimeoutMinutes: 30` → `300` on `claude_code`, `openai_codex`, `antigravity`
- [ ] `defaultCLITimeout` `3h` → `5h`; leave `sandbox.DefaultIdleTimeout` at 15m and comment why
- [ ] Route `cli_analyze`, `cli_spec`, `cli_mr` through the same role-agent + `LoadRolePrompt` path the implement tracks use, resolving the project's lead agent where no role applies
- [ ] Add the metrics from design.md (`task.spec_gate.required`, `cli.step.duration_seconds`, `cli.step.killed`, `cross_review.skipped`, `mr.spec_missing`)
- [ ] Update `README.md` (drop API-key setup, document CLI auth + the review toggle) and `ARCHITECTURE.md` File Dependencies
- [ ] Record deviations and decisions in the implementation notes

**Verify:**
- [ ] A CLI step emitting output for >30min is not killed; effective timeout resolves to 5h
- [ ] A step silent for 15min is still killed with `idle_timeout`, not `timeout`
- [ ] Every CLI step logs its resolved agent and role
- [ ] `make test` and the project lint suite pass
- [ ] `grep -rn "ANTHROPIC_API_KEY\|OPENAI_API_KEY\|GEMINI_API_KEY" . --include='*.md' --include='*.go' --include='*.yml'` returns nothing outside archived docs

---

## Rollback Plan

### Feature Flags
- [ ] None. D1 rejects a flagged API path on purpose — rollback is `git revert` of the deletion commits plus the migration reversal below, not a runtime switch.

### Database Rollback
- [ ] `migrate down` `000032` restores `review_policy` / `autonomy_level` columns as empty and drops both `require_spec_review` columns — **gate settings are lost and must be re-entered**
- [ ] `migrate down` `000031` restores schema shape only — **deleted API credentials, `token_usage` rows and embedding vectors do not come back**; Phase 0's CSV export is the only recovery path for usage history
- [ ] Take a full DB snapshot immediately before `000031`; it is the real rollback mechanism

### Safe Deploy
- [ ] Ship Phases 1-3 (pure deletion, no schema change) first and let them run a full day — behavior for CLI-configured orgs is unchanged, so any breakage is a deletion mistake, caught cheaply
- [ ] Circulate Phase 0's affected-project list and give owners a window to enable a CLI row before `000031`
- [ ] Run `000031` / `000032` in a maintenance window with no in-flight jobs (`workflow_jobs` with a non-terminal status drained first — an in-flight job holding a deleted api provider resolves to nothing mid-run)
- [ ] Post-deploy, watch `cli.step.killed{reason="timeout"}` and `task.spec_gate.required` for 48h: a `timeout` spike means 5h masks a stall the idle detector misses; a `spec_gate.required` spike means the backfill over-gated
