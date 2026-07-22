# Implementation Notes: CLI Spec-First Flow (backend core pass)

Spec: `docs/openspecs/cli-spec-first-flow/`. Scope of this pass, per user decision: sections 1-3 + 6.1 of `tasks.md` (workflow definition, prompt templates, the 4 CLI steps, checkbox parser, unit + integration tests). REQ-004 (approval gate), REQ-006/REQ-007 API+UI equivalents (spec-read API, frontend spec panel) are explicitly deferred to a follow-up pass.

## Deviations from the spec doc

1. **`CaptureFiles` engine addition (not in the original spec).** The existing `cliEngine.RunCodeStep` (`server/internal/orchestrator/engine/cli.go`) deletes the whole `.autocode/` directory via `rm -rf` inside the same blocking shell invocation the CLI subprocess runs in — there's no way for the server to read `.autocode/analysis.md` afterward through the existing `Runtime` interface (single blocking `Run` call, no live streaming). Added `CaptureFiles []string` to `engine.CodeStepRequest` and `Files map[string]string` to `engine.CodeStepResult`: the shell script base64-encodes requested files to stdout (wrapped in null-byte-prefixed sentinel markers) before the cleanup `rm -rf` runs; `extractCapturedFiles` parses them out of the combined output afterward. Only `cli_analyze` uses this — `cli_spec`/`cli_implement` read their outputs (`docs/openspecs/...`) straight off the host worktree instead, since those files are normal repo content that survives the run.

2. **Lightweight prompt loading instead of full Assembler wiring (tasks.md 2.4).** The spec's task 2.4 called for wiring into the existing `PromptAssembler`/`Assemble`/`AssembleForAgent` machinery. That system builds incremental multi-turn tool-loop prompts for the API-native path; the CLI steps instead build one full standalone instruction string per spawn. Added `PromptBuilder.LoadStepPrompt(stepID string) (string, error)` — a thin wrapper reusing the existing `steps/*.md` file-loading infra (`paths.PromptPaths.StepPrompt` + `paths.FileSystem`) — rather than forcing the CLI steps through the heavier multi-section assembler.

3. **`cli_mr` via struct embedding.** `CLIMRStep{ *PRStep }` (`server/internal/orchestrator/steps/cli_mr.go`) overrides only `ID()`, reusing `PRStep`'s push/merge-request logic verbatim. Known cosmetic trade-off: checkpoint records may be attributed to whichever step ID variant a given code path captures inside `PRStep`'s internals. Accepted as fine for a backend-core pass; revisit if checkpoint attribution needs to be exact per-step.

4. **`cliAnalysisPayload` is distinct from `models.TaskAnalysis`.** The API-native flow's `TaskAnalysis` is a structured shape assembled from tool-loop turns. A black-box CLI agent's `.autocode/analysis.md` is unstructured markdown; `cli_analyze` best-effort-parses `## Tech Stack` / `## Affected Files` / `## Risks` sections into `cliAnalysisPayload` (raw markdown + tech stack string + file/risk lists) and stores that JSON into `task.Analysis` instead. Downstream code that expects `models.TaskAnalysis`'s richer shape (e.g. `ExecutionUnits`, `SpecHash`) will not find those fields for CLI-flow tasks — by design, since `worker.go` checks `cliengine.ResolveEngine(...)` before falling through to the `ExecutionUnits`-based dynamic DAG logic, so CLI-flow tasks never reach that code path.

## Key files

- `server/internal/workflow/step.go` — step IDs + `CLISpecFirstWorkflow(runners)` definition.
- `server/internal/workflow/parser.go` — `ParseCheckboxes` (tolerant regex, strips fenced code blocks first).
- `server/internal/prompts/steps/cli_{analyze,spec,implement}.md` — prompt templates; loaded via `PromptBuilder.LoadStepPrompt`.
- `server/internal/orchestrator/steps/cli_{analyze,spec,implement,mr}.go` — the 4 steps.
- `server/internal/orchestrator/cli_spec_step.go` — `cliStepRunner`, dispatches through the CLI engine directly (no patch-retry loop, no "zero changes = failure" assumption — each step validates its own file-based contract).
- `server/internal/orchestrator/step_registry.go` — wires the 4 steps into `stepRunners()`.
- `server/internal/orchestrator/worker.go` — selects `workflow.CLISpecFirstWorkflow(runners)` when `cliengine.ResolveEngine(task.ExecutionEngine, projectEngine) == models.ExecutionEngineCLI`, before falling through to the existing complexity/dynamic-DAG selection.

## Test coverage added

- `server/internal/orchestrator/steps/cli_analyze_test.go`, `cli_spec_test.go`, `cli_implement_test.go` — happy path, missing files, runner error, prompt-load error, docs-only bypass (label and frontmatter variants) per step.
- `server/internal/orchestrator/steps/cli_spec_first_integration_test.go` — drives all 4 steps through the real `workflow.Engine` DAG with a fake CLI engine runner that writes real files into a temp worktree (tasks.md 6.1).
- `server/internal/workflow/parser_test.go`, `state_machine_test.go` — `ParseCheckboxes` variants, `CLISpecFirstWorkflow` DAG shape.
- `server/internal/orchestrator/engine/cli_test.go` — `CaptureFiles` script wiring, `extractCapturedFiles`.

## Follow-up pass: sections 4-5 (approval gate + spec read API/UI)

5. **Approval gate reuses the existing spec-review state machine, not a new `awaiting_spec_approval` status.** `CLISpecStep` (`server/internal/orchestrator/steps/cli_spec.go`) now takes `TaskUpdater`/`ProjectReader` deps. After validating the 4 spec files, it checks `project.DefaultAutonomy`: `autonomous` → auto-sets `SpecStatus=auto_approved` and continues; anything else → persists `SpecStatus=pending_review`/`Status=spec_review` and returns `workflow.PauseError{Reason: "workflow paused for human spec review"}` — the exact same reason string `analyze.go` already uses, so the existing paused-banner classification in `CheckpointsPanel.tsx` needed no changes. A resume-guard at the top of `Execute` short-circuits re-spawning the CLI when `SpecStatus == approved` (post-approval `engine.Resume` re-invocation), and appends a `## Reviewer feedback` section built from `task.Description` when `SpecStatus == changes_requested` (populated by `RequestAnalysisChanges` via the new endpoint).

6. **`POST /tasks/{id}/spec-review` reuses `TaskService.ApproveAnalysis`/`RequestAnalysisChanges` directly** (`server/internal/handler/task.go: SpecReview`) rather than adding near-duplicate service methods, since both actions need the identical `SpecReview`/`Coding` transitions. `request_changes` additionally calls two new `Orchestrator` methods: `CheckSpecReviewLoopLimit` (counts `WorkflowCheckpoint{Step: "cli_spec_review_cycle"}` rows against `project.MaxReviewFixCycles`, mirroring `CheckReviewLoopLimit`'s `pr_rejection` pattern) and `SaveSpecReviewCycle` (records one such checkpoint per cycle).

7. **`GET /tasks/{id}/spec` (`Orchestrator.GetTaskSpec`) reads live off the host worktree**, not from any DB column — it resolves the repo host path via `repoutil.Manager`, derives the slug via `steps.TaskSpecSlug`, and reads the same 4 files `cli_spec` validates, returning `models.TaskSpec{Proposal, Specs, Design, Tasks, Progress}` with checkbox progress from `workflow.ParseCheckboxes`. 404s if `cli_spec` hasn't run yet (dir absent).

8. **Frontend: a separate `CLISpecPanel.tsx`/`CLISpecReviewControls.tsx` pair, not a modification of `SpecPanel.tsx`/`BoundaryResolutionControls.tsx`.** `SpecPanel` renders `task.analysis` (API-native flow's structured JSON); the CLI flow has no such JSON, so `CLISpecPanel` fetches `GET /tasks/{id}/spec` via `useAuthedSWR` and renders the 4 raw markdown files + a checkbox progress bar, gated on `task.execution_engine === "cli"`. `TaskDetailLayout.tsx` now branches on the pause reason: if `last_error === "workflow paused for human spec review"` and `execution_engine === "cli"`, it renders `CLISpecReviewControls` (explicit `action: "approve"|"request_changes"` calls) instead of `BoundaryResolutionControls` (which is boundary-violation-regex-specific and does not apply here). `CheckpointsPanel.tsx` gained a `STEP_LABELS` map for the 4 new step IDs so the checkpoint timeline reads "Author Spec (CLI)" etc. instead of the raw step ID.

## Not in this pass

- None remaining for this spec — sections 1-6 of `tasks.md` are complete.
