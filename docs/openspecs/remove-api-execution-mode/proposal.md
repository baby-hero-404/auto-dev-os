# Proposal: CLI-Only Execution — Remove API-Native Mode & Simplify the CLI Pipeline

## Problem

Auto Code OS carries two complete, parallel execution engines for the same job — "make an AI agent write code":

1. **API-native**: the server owns the agentic loop. `pkg/llm` (Anthropic/OpenAI/Gemini/9Router providers, router, fallback, pricing, budget) → `internal/gateway` → `orchestrator/llmrunner` (tool loop, state machine, stall guard) → `internal/tool` (24 server-implemented tools: read_file, search_replace, run_tests, git_diff…) → `internal/prompts` (tool-schema assembly, compiler). Workflow shapes: `EasyWorkflow` / `MediumWorkflow` / `HardWorkflow` / `DynamicDAGWorkflow`.
2. **CLI-native**: a black-box coding CLI (`claude`, `codex`, `agy`) owns its own tool loop, context loading and planning; the server is reduced to spec-first gating. Workflow shapes: `CLISpecFirstWorkflow` / `CLISpecFirstParallelWorkflow` (`cli_analyze → cli_spec → cli_implement → [cross_review] → cli_mr`).

Every orchestration concern — credential pooling, cooldown, retries, telemetry, tracing, step status, checkpointing — is implemented twice, once per engine. `ResolveExecutionProvider` exists purely to arbitrate between them, and `shouldUseCLISpecFirstWorkflow` forks the entire DAG on its answer. ~21k lines of Go sit on the API side, imported by 39 files.

The operational reality has settled on the CLI side: the CLI harnesses are driven by flat-rate subscription auth (`claude auth status`, `codex login status`), not metered per-token API keys, and they ship tool loops that outpace anything reimplemented server-side. The API path is now cost, not capability — it doubles the surface area of every change and every bug.

Compounding the duplication, the surviving CLI pipeline is itself over-gated. Whether a task pauses for human spec approval is decided by `ShouldAutoApproveSpec`, a six-input matrix (`projectReviewPolicy` x `agentAutonomy` x `projectAutonomy` x high-risk-domain pattern match x complexity x clarifications). Nobody can predict from a project's settings whether a given task will stop and wait. Meanwhile CLI steps are capped at 30 minutes (`CLIProfiles[*].TimeoutMinutes`, which overrides the engine's own 3h default *downward*), so genuinely long agent runs are killed mid-work and reported as failures.

## Goal

One execution path, one gate, one predictable default. A task runs by invoking a CLI coding agent inside a sandbox and proceeds automatically all the way to an opened Merge Request; the server gates only when explicitly told to, observes, and merges. `pkg/llm` and everything that exists only to serve it is gone.

## Success

- A task submitted through the dashboard runs end to end with no LLM API key configured anywhere in the environment or credential pool.
- `grep -r "pkg/llm" server --include='*.go'` returns nothing.
- There is exactly one workflow family; no code branches on `type == "api"`.
- A task created with default settings reaches an open MR with zero human interaction, and that MR contains the OpenSpec the agent worked from, so the reviewer decides from the spec rather than from the diff alone.
- Flipping one switch — on the project or on the single task — is the only way a run stops for spec review, and it always stops there.
- A CLI step running for four hours completes instead of being killed.

## Assumptions

Taken for granted, not re-validated here:
- The CLI spec-first pipeline is functionally complete for the task shapes in use — `cli_analyze`/`cli_spec`/`cli_implement`/`cli_mr` already ship and are exercised by tests.
- At least one CLI harness (`claude`, `codex`, or `agy`) is installed and authenticated in the sandbox image; without one the platform has no way to execute, by design.
- The sandbox runtime, gitops, PR/merge and checkpoint layers are engine-agnostic and need no change.
- The HTTP API server and Next.js dashboard stay — they are the product's user interface, not "API mode". See Decision D2.

## Decisions

**D1 — Delete `pkg/llm` outright rather than shrink it.**
A trimmed `pkg/llm` retaining only "one small helper call" would keep the credential-pool/model-resolver/pricing/budget graph alive and re-grow. Deletion is the only version of this change that actually collapses the surface area.

**D2 — "API" means the LLM-provider execution path, not the product's HTTP interface.**
`server/cmd/api`, `internal/handler`, `internal/middleware` and `web/` are how humans drive the platform; they have no relationship to how an agent talks to a model. They are untouched except where they expose API-provider *concepts* (credential forms, execution-provider rows).

**D3 — `cross_review` is re-hosted on the CLI engine, not deleted.**
Cross-harness review is a CLI-pipeline feature (`review_harness_policy != "same"`), but `steps/cross_review.go` currently calls `RunLLMStep` on the API runner and uses `llm.WithExcludeProviderID` to force a different provider. The intent — *a different harness reviews the work* — survives; the mechanism must move to a second CLI profile chosen by exclusion from the coder's profile. This is the one piece of net-new implementation in an otherwise subtractive change.

**D4 — Memory search loses its vector stream, not the whole feature.**
`MemoryService.Search` merges three streams via RRF: vector (needs `pkg/llm` embeddings), knowledge-graph, and BM25 (`tsvector`, pure Postgres). Only the vector stream depends on an API. Deleting all three would destroy working functionality that costs nothing to keep, so the vector stream and its `embedding vector(1536)` columns go and RRF merges the remaining two. *This deviates from the literal instruction to disable memory search; flagged deliberately — say the word and it becomes a full removal.*

**D5 — Clean break, no backward compatibility.**
Migrations drop the columns and rows rather than leaving tombstones, and `ValidateExecutionProviders` rejects `type:"api"` instead of quietly ignoring it. A project configured API-only fails loudly at validation with an actionable message rather than silently changing behavior.

**D6 — Cost analytics re-sources from CLI telemetry.**
`token_usage` is written only by the LLM gateway (`RecordLLMUsage`), so it dies with it. The CLI engine already parses `total_cost_usd` / `duration_ms` / `usage` from `claude --output-format json` into workflow-job telemetry (migration `000023`); the analytics dashboard reads that instead.

**D7 — One boolean replaces the auto-approval matrix; the default is fully automatic.**
`ShouldAutoApproveSpec`'s six inputs collapse to a single `require_spec_review` flag, resolved as `task.RequireSpecReview ?? project.RequireSpecReview ?? false`. `false` (the default) means the run never pauses: `cli_analyze → cli_spec → cli_implement → [cross_review] → cli_mr` executes straight through. `true` means it always pauses at `spec_review` after `cli_spec`, no exceptions and no heuristics.

Predictability is the whole point. The old matrix was *safer on paper* — it auto-gated anything touching `auth/`, `payment`, `migrations/` — but a rule nobody can predict is a rule nobody relies on, and the high-risk patterns were substring matches over file paths (`role` matched `role_templates.go`; `api/` matched every handler), so the gate fired on ordinary work and got ignored. The real safety boundary is the MR: nothing merges without a human, and now the spec travels with it (D8). Explicitly accepted: a high-risk task no longer self-gates, and an org that wants that back sets `require_spec_review` on the project.

**D8 — The MR always carries the OpenSpec; it stops being a toggle.**
`analysis.IncludeSpecInMR` becomes unconditional. With the pipeline running unattended, the MR is the first and only place a human sees what was decided, so the spec is not optional there. Two channels, because they answer different questions: `docs/openspecs/<slug>/` is committed into the branch (reviewable diff, permanent record) *and* the spec's goal + key decisions + verify steps are rendered into the MR body above the change summary (decidable at a glance, no file navigation). Clarifications the agent raised and the answers it assumed are rendered too — under auto mode those were never surfaced to a human before.

**D9 — CLI step timeout raised to 5 hours, and the idle timeout is the real stall detector.**
`CLIProfiles[*].TimeoutMinutes` goes 30 → 300 and `defaultCLITimeout` 3h → 5h. Raising the wall-clock cap alone would be cosmetic: `sandbox.DefaultIdleTimeout` kills any step with no stdout/stderr for 15 minutes, which is what actually fires first on a long run. The wall-clock timeout becomes a genuine backstop and the 15-minute idle timeout stays the stall detector — a working agent prints; a hung one does not. A CLI step is therefore killed for silence, not for duration.

**D10 — The role dimension is preserved exactly as it is.**
Backend/frontend role branches, role-matched agent assignment, and `LoadRolePrompt` per step are unchanged by this work. Simplifying the gate must not be read as flattening the pipeline into one generic agent: `CLISpecFirstParallelWorkflow`'s `cli_implement_backend` / `cli_implement_frontend` fan-out, its `StepMerge` reconciliation, and the role-suffixed worktrees stay. What is standardized is that *every* step — including `cli_analyze`, `cli_spec` and `cross_review` — resolves its agent and prompt through the same role path, instead of some steps carrying a role and others running role-less.

## Trade-offs

**Gain:** one engine, one credential model, one telemetry path; ~21k lines and 4 workflow shapes removed; no per-token billing surface; new features are written once. The gate becomes one switch whose behavior is stated in a sentence, and long-running agent work stops being reported as spurious timeout failures.

**Lose / risk:**
- No fallback when every CLI harness is unauthenticated or quota-exhausted — previously the router could drop to an API provider. Failure becomes total rather than degraded. Accepted: the CLI credential pool already supports multiple profiles and cooldown rotation, which is the intended redundancy.
- Semantic memory recall weakens to lexical + graph. Acceptable given BM25 is already one of three ranked streams.
- The server loses fine-grained per-tool-call observability; CLI runs are observable only at the granularity the harness reports.
- `codex` telemetry is already known-zero (`parseCLITelemetry` returns `ok=false`, see `cli_profiles.go`), so cost analytics degrade for Codex-backed orgs until its JSON flag is confirmed.
- **Auto-by-default means unreviewed specs reach implementation.** A misread requirement is now discovered at MR time, after the agent has written the code, not before. Mitigated by D8 (the spec is in the MR) and bounded by the fact that nothing merges unattended — but the wasted-work window grows from one step to the whole pipeline.
- A 5-hour cap means a genuinely stuck-but-chatty agent can burn 5 hours of sandbox and subscription quota before the wall-clock kill. The idle timeout and `loop_detector.go` are what bound that in practice; neither is changed here.

## Non-goals

- Not improving the CLI pipeline's capability or prompt quality — behavior after this change should match today's CLI-mode behavior exactly, minus cross_review's mechanism.
- Not re-tuning memory ranking weights after the vector stream is dropped.
- Not redesigning the spec-review UI — the existing `spec_review` status, `approve_spec` / `request_changes` actions and their screens are reused as-is for the opt-in path.
- Not making the 5-hour timeout configurable per project; one constant, changed in one place.
- Not touching sandbox, gitops, PR/merge, checkpoint, or auth layers.

## Out of Scope

- Confirming `codex exec`'s structured-output flag to restore its telemetry (pre-existing TODO in `cli_profiles.go`).
- Adding new CLI profiles beyond the existing `claude_code` / `openai_codex` / `antigravity` / `custom`.
- Replacing vector memory with a local embedding model.
- Any change to `server/cmd/mcp-context`, `minttoken`, or `rollout-gate`.
- Per-step timeout overrides, or a UI for them.
- Reworking `loop_detector.go` / idle-timeout heuristics.
- Adding new roles beyond backend/frontend.

## Impact

### Components

| Component | Fate |
|---|---|
| `server/pkg/llm/` | **Deleted** (17 files) |
| `server/internal/gateway/` | **Deleted** |
| `server/internal/orchestrator/llmrunner/` | **Deleted** |
| `server/internal/tool/` | **Deleted** (server-side tool loop; the CLI harness brings its own) |
| `server/internal/prompts/` | **Reduced** to CLI prompt-file assembly; tool-schema/compiler paths deleted |
| `server/internal/orchestrator/steps/` | API-native steps deleted; `cli_*` steps kept |
| `server/internal/workflow/` | Easy/Medium/Hard/DynamicDAG deleted; CLI workflows kept |
| `server/internal/service/` | Credential pool reduced to `cli:*`; embedder/memory-vector deleted |
| `server/internal/policy/review_policy.go` | **Reduced** to a single `require_spec_review` resolution (D7) |
| `server/internal/orchestrator/engine/cli.go`, `pkg/models/cli_profiles.go` | Timeouts 30m/3h → 5h (D9) |
| `server/internal/orchestrator/steps/pr.go`, `gitops/pr_generator.go` | Spec always committed + rendered into MR body (D8) |
| `web/src/app/ai-providers/` | API-credential UI deleted; CLI auth UI kept |
| HTTP server, handlers, middleware, sandbox, gitops, workflow engine | **Unchanged** |

### Files

*Deleted (primary):*
- `server/pkg/llm/**` — all 17 files
- `server/internal/gateway/gateway.go`, `gateway_test.go`
- `server/internal/orchestrator/llmrunner/**`
- `server/internal/tool/**`
- `server/internal/orchestrator/llm_step.go`, `llm_step_test.go`, `llm_trace.go`, `llm_trace_test.go`
- `server/internal/orchestrator/steps/`: `analyze.go`, `analyze_parser.go`, `analyze_tools.go`, `plan.go`, `review.go`, `code_backend.go`, `code_frontend.go`, `fix.go`, `testing.go`, `scan.go`, `decompose.go`, `reduce.go`, `patch_retry_loop.go`, `tool_executor.go`, `boundary_tool_executor.go`, `intent_resolver.go`, `compiler_parser.go` + their tests
- `server/internal/service/embedder_breaker.go`, vector half of `memory_search.go`
- `web/src/app/ai-providers/components/AddApiCredentialModal.tsx`

*Modified (primary):*
- `server/pkg/models/project.go` — drop `"api"` from `validExecutionProviderTypes`, `DefaultExecutionProviderRows`, `executionProviderRowForCredential`; delete `ExecutionEngine*` legacy constants
- `server/pkg/models/provider_credential.go` — `IsAllowedProvider` accepts only `cli:*`
- `server/internal/orchestrator/execution_router.go` — `ResolvedExecutionProvider.Type` collapses to CLI-only; delete `legacyResolveExecutionProvider`, `shouldUseCLISpecFirstWorkflow`
- `server/internal/orchestrator/step_registry.go`, `worker.go` — single workflow family
- `server/internal/orchestrator/steps/cross_review.go` — re-host on `CLIStepRunner` (D3)
- `server/cmd/api/main.go` — drop LLM provider/gateway construction and wiring
- `server/pkg/config/config.go`, `.env.example` — drop `OPENAI_API_KEY`, `ANTHROPIC_API_KEY`, `GEMINI_API_KEY`, `LLM_*`, embedding config
- `web/src/app/ai-providers/page.tsx`, `AddCredentialModal.tsx`, `ProviderSummary.tsx`, `ProviderCredentialsTable.tsx`, `web/src/lib/model-options.ts`, `web/src/lib/types.ts`, `web/src/components/projects/execution-providers-list.tsx`
- `server/internal/policy/review_policy.go` — `ShouldAutoApproveSpec` → `RequiresSpecReview(task, project) bool`; delete `HighRiskPatterns`, `HasHighRiskDomains`
- `server/pkg/models/task.go`, `project.go` — add `RequireSpecReview *bool` (task override) and `bool` (project default)
- `server/internal/orchestrator/steps/cli_spec.go` — gate on the new flag
- `server/pkg/models/cli_profiles.go` — `TimeoutMinutes: 30` → `300` on all three profiles
- `server/internal/orchestrator/engine/cli.go` — `defaultCLITimeout` `3h` → `5h`
- `server/internal/orchestrator/steps/pr.go` — `includeSpec` unconditional
- `server/internal/orchestrator/gitops/pr_generator.go` — render spec section into MR body
- `server/internal/orchestrator/step_registry.go` — role resolution on every CLI step (D10)
- `web/src/components/projects/create-task-panel.tsx` — "Require spec review" task toggle
- `web/src/app/projects/[id]/**` — project-level default toggle
- `README.md`, `ARCHITECTURE.md`

### Public API

Breaking, by D5:
- `POST/PUT /projects/{id}/execution-providers` — rejects `type:"api"` with 400 and a message naming the CLI profiles.
- `POST /provider-credentials` — rejects non-`cli:*` providers with 400.
- `GET /provider-models` and the provider-model resolver endpoints — removed; model selection now lives inside the CLI harness.
- `GET /analytics/*` — token/cost fields re-source from workflow-job telemetry; `token_usage`-derived fields are removed or zero for Codex.
- Task/project payloads drop `execution_engine` and `cli_engine_config` legacy fields in favor of `execution_providers`.
- `POST /tasks` accepts `require_spec_review` (nullable bool, default null → inherit project).
- `PATCH /projects/{id}` accepts `require_spec_review` (bool, default false).
- `projects.review_policy`, `projects.autonomy_level` and `agents.autonomy_level` no longer affect spec gating; the fields are dropped (D5, D7).

### Migration

New migration `000031_remove_api_execution_mode`:
- `DELETE` `provider_credentials` rows where `provider NOT LIKE 'cli:%'`
- Rewrite `projects.execution_providers` jsonb, stripping `type:"api"` entries
- `DROP COLUMN projects.execution_engine`, `projects.cli_engine_config` (legacy single-engine path, D5)
- `DROP COLUMN organizations.default_execution_providers` API entries (same jsonb strip)
- `DROP TABLE token_usage`, `provider_models`
- `DROP COLUMN memories.embedding`, `episodic_memories.embedding`; `DROP INDEX idx_episodic_memories_embedding`
- `ALTER TABLE projects ADD COLUMN require_spec_review boolean NOT NULL DEFAULT false`
- `ALTER TABLE tasks ADD COLUMN require_spec_review boolean` (nullable — null inherits project)
- Backfill: projects whose old `review_policy = 'always_review'` get `require_spec_review = true`, so orgs that had explicitly demanded review keep it. Every other combination becomes `false` (auto).
- `DROP COLUMN projects.review_policy`, `projects.autonomy_level`, `agents.autonomy_level`

The `.down.sql` restores schema shape only, not deleted data — documented as one-way.

### Backward Compatibility

None, by D5. Any org whose projects run API-only stops executing tasks at the moment of deploy and must enable a CLI row in Execution Providers plus authenticate a CLI credential. Operator pre-flight query and the operator note are in `tasks.md` Phase 0.

Gating behavior changes for existing projects even where it is preserved: only `review_policy = 'always_review'` survives as `require_spec_review = true`. Projects that relied on `supervised` autonomy or the high-risk-domain patterns to gate medium/hard tasks will start running those tasks straight through to MR. The Phase 0 operator note lists affected projects so owners can opt back in before deploy.
