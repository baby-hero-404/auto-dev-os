# Tasks: Execution Boundary & Target Resolution Hardening

## Phase 1: Analyze Contract (Issues 1 + 2 — source of truth) 🟢
- [x] **1.1** Add `TargetFiles []string \`json:"target_files"\`` to `models.ExecutionUnit` in `server/pkg/models/task.go`; document the field in the analyze step's output-schema prompt text.
- [x] **1.2** Implement `validateBoundaryCoverage(analysis)` (path-prefix coverage with trailing-`/` guard; empty root covers all) and wire it into the analyze `Validate` hook in `server/internal/orchestrator/steps/analyze.go`, next to the existing presence check (`analyze.go:280-283`).
- [x] **1.3** Extend the same hook to reject any execution unit with a missing/empty `target_files` list, and run each entry through `validateBoundaryCoverage`.
- [x] **1.4** On tool-loop budget exhaustion with coverage still violated, fail `AnalyzeStep` with an error listing every uncovered file (REQ-001 scenario 2).
- [x] **1.5** Tests (`analyze` step): uncovered `affected_files` entry → corrective validation error naming file + roots; covered output passes; empty `target_files` → rejected; budget exhaustion → hard failure, no coding step scheduled. Include a regression fixture mirroring the e69924ba shape (`root: internal/` + `cmd/.../main.go`).

## Phase 2: Downstream Consumption (Issues 2 + 3) 🟢
- [x] **2.1** Implement `unitForStep(analysis, stepID)` (subtask-index → same-role unit, matching `prompts.extractSubtaskIndex` semantics) in `server/internal/orchestrator/llmrunner/runner.go`.
- [x] **2.2** In `Runner.BuildInitialMessages` (`runner.go:72`), for `code_backend_N`/`code_frontend_N` steps, build "Workspace Affected Files" from the unit's `TargetFiles`; fall back to task-wide `AffectedFiles` when the unit declares none (legacy analyses). `fix` step keeps the task-wide view.
- [x] **2.3** In `ResolveIntent` (`server/internal/orchestrator/steps/intent_resolver.go:80`), resolve from the owning unit's `TargetFiles` first; keep token matching as fallback; extend `IntentResolutionError.Reason` to name both attempted strategies.
- [x] **2.4** Add natural-language detection to `intentTokens` (≥3 words after separator normalization, or any non-ASCII letter → skip token matching).
- [x] **2.5** Tests: `code_backend_1` prompt contains exactly its unit's files and none of `init-core`'s; Vietnamese-sentence capability with `target_files` resolves to them; identifier capability without `target_files` keeps current behavior; double-failure error names both strategies.

## Phase 3: Tool-Loop Stall Safeguard (Issue 4) 🟢
- [x] **3.1** Create `server/internal/orchestrator/llmrunner/stallguard.go` with `stallGuard` (`Check`/`RecordSuccess` per design §5.1), including the two corrective-feedback texts.
- [x] **3.2** Wire `stallGuard` into `RunToolLoop` (`toolloop.go`) before `ExecuteTool`, alongside the existing `failureCounts`/`readMemo` checks; intercepted calls still consume the iteration.
- [x] **3.3** Wire the same guard at the same position in `runStateMachine` (`statemachineloop.go`).
- [x] **3.4** Tests: no-op `search_replace` intercepted, not executed, absent from `EditsApplied`, and a budget-exhausted run with only no-op edits reports `Partial == false` (REQ-M01); repeated identical `list_files` intercepted with the nudge text; both loops covered.

## Phase 4: Structural-Failure Retry Policy (Issue 5) 🟢
- [x] **4.1** Add `workflow.ErrNoProgress` sentinel next to `ErrPaused`/`ErrReviewFixLoop`.
- [x] **4.2** In the coding/fix failure path (`server/internal/orchestrator/steps/patch_retry_loop.go`), wrap the terminal error with `ErrNoProgress` when the loop applied zero edits.
- [x] **4.3** In the worker retry loop (`server/internal/orchestrator/worker.go:430-435`), break out of remaining retries when `errors.Is(err, workflow.ErrNoProgress)` and at least one re-attempt has run.
- [x] **4.4** Carry the failed attempt's terminal error into the retry: pass it via the engine run input map (`worker.go:406-408`) and append the "PREVIOUS ATTEMPT FAILED …" block to the step instruction in `BuildInitialMessages`.
- [x] **4.5** Tests: zero-edit failure → exactly one re-attempt (not `maxRetries`); retry prompt contains the prior boundary-violation text; a failure *with* edits keeps today's full retry behavior.

## Phase 5: End-to-End Regression 🟡

> **Decisions (aligned 2026-07-14):** tests live in a dedicated file (not appended to the 1000-line `orchestrator_test.go`), same package so `mock_test.go` fixtures are reused as-is; both the hard-failure and the self-repair mock scenarios are covered (they verify the two distinct REQ-001 scenarios); the live run uses the existing `gateway` provider + `make dev` stack, triggered through the Web UI because the workflow pauses for human spec approval.

- [ ] **5.1a** Extend `mockLLMProvider` (`server/internal/orchestrator/mock_test.go:233`) with a call log for step attribution:
  - Add `calls []string` and, in `ChatWithOptions`, append the matched step key (the `responses` map key found in the last message, or `"queued"` when serving from `responseQueue`).
  - No behavior change for existing tests; the log is append-only.
- [ ] **5.1b** Create `server/internal/orchestrator/boundary_regression_test.go` (package `orchestrator`, reusing the `TestOrchestrator_Run_Integration` fixture pattern from `orchestrator_test.go:23-85`) with the **hard-failure** scenario:
  - `TestAnalyze_BoundaryViolation_ExhaustsBudget_FailsBeforeCoding`: load `responseQueue` with N copies (N = the analyze loop's `MaxIterations`) of an analyze JSON mirroring e69924ba — `execution_boundaries: [{root: "internal/"}]`, `affected_files` containing `cmd/zentao-sync/main.go`, one unit whose `target_files` includes the `cmd/` path — so every corrective re-prompt returns the same uncovered output.
  - Assert: `orch.run(...)` leaves the job failed; the failure error names `cmd/zentao-sync/main.go`; and `mockLLMProvider.calls` contains **zero** `code_backend`/`fix` entries (the incident burned 33 such calls).
- [ ] **5.1c** Same file, the **self-repair** scenario:
  - `TestAnalyze_BoundaryViolation_SelfRepairsOnFeedback`: `responseQueue` = [uncovered analyze output, corrected analyze output adding `{root: "cmd/"}` (or widening to repo root)], then keyed `responses` for the downstream steps as in the existing integration test.
  - Assert: analyze succeeds on turn 2; the persisted analysis contains the corrected boundaries; the workflow proceeds into coding steps (call log contains `code_backend`); and the second analyze prompt (capture via a queue-aware message inspection or the mock's last-seen messages) contains the corrective coverage-error text from Task 1.2.
- [ ] **5.1d** Run `go test ./server/internal/orchestrator/... -run 'TestAnalyze_BoundaryViolation' -v` — both PASS; then the full package suite for regressions.
- [ ] **5.2** Live-model verification runbook ("zentao auto tool", original Vietnamese description from `server/.data/workspaces/e69924ba-.../task.json`):
  1. **Env:** repo-root `.env` provides the provider key(s) — `GEMINI_API_KEY` (and/or `OPENAI_API_KEY`/`ANTHROPIC_API_KEY`; bound in `server/pkg/config/config.go:152-154`); config stays `llm.provider: "gateway"` (`server/pkg/config/config.yaml`). Set `LOG_LLM_TRACE_ENABLED=true` so per-call transcripts land in `server/.data/workspaces/<task-id>/logs/llm/`.
  2. **Start:** `make dev` (Postgres in Docker + API on :32080 + Web on :32300). Ensure a Gemini credential exists for the org (Settings → Provider Credentials) since the gateway routes via the credential pool.
  3. **Trigger:** in the Web UI, create a task in a test project with the original title/description verbatim; run Analyze; when the workflow pauses for spec review, approve it; let execution run to terminal state.
  4. **Verify (pass criteria):**
     - `logs/llm/call-001-analyze/parsed.json`: every `execution_units[]` has non-empty `target_files`, all covered by `execution_boundaries` (the `cmd/` entrypoint must be covered — the original run's contradiction);
     - each `code_backend_N` `prompt.md` lists only its own unit's files ("Workspace Affected Files" section);
     - the workflow log's `git_merge_*` line shows at least one non-`go.mod` source file in the merge diff (original run: `go.mod | 3 +++` only);
     - zero `execution boundary violation` errors in any tool result across `logs/llm/call-*/prompt.md`;
     - if any step still fails with zero edits, the log shows at most one re-attempt (Phase 4) instead of the original 3× retry burn.
  5. Attach the resulting task ID + a short pass/fail note per criterion to this spec (update REQ statuses in `specs.md`).

## Docs sync

- [x] Update corresponding `docs/features/` as specified in feature-docs-sync/design.md — N/A: this spec set is not in feature-docs-sync/design.md's 14-set mapping table, no docs/features/ target specified
