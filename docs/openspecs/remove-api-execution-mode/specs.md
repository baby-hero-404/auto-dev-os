# Expected Behavior: CLI-Only Execution

## Scenario: Default task runs unattended to an open MR

**When:**
- A task is created on a project with an enabled CLI execution-provider row, an authenticated `cli:*` credential, and `require_spec_review` unset on both task and project.

**Then:**
- `cli_analyze → cli_spec → cli_implement → [cross_review] → cli_mr` runs with no human interaction.
- Task status never enters `spec_review`; `cli_spec` writes the spec, sets spec status `auto_approved`, and the workflow engine proceeds to `cli_implement` in the same job run.
- The task ends at `pr_ready` with an MR URL recorded.
- `cross_review` is inserted only when the project's `review_harness_policy != "same"`, exactly as today.

## Scenario: Spec review requested on the task

**When:**
- A task is created with `require_spec_review: true`, on a project whose default is `false`.

**Then:**
- After `cli_spec` completes, task status becomes `spec_review` and the job halts before `cli_implement`.
- `computeAvailableActions` returns `approve_spec` and `request_changes`.
- `approve_spec` transitions to `coding` and resumes at `cli_implement` from the existing checkpoint — the spec is not re-authored.
- `request_changes` transitions back to `analyzing` and re-runs `cli_spec` with the reviewer's comment threaded into the prompt.

## Scenario: Spec review defaulted on the project

**When:**
- A project has `require_spec_review: true` and a task is created without specifying it (`null`).

**Then:**
- The task gates at `spec_review`. The task-level value is an override, not a merge: `task.RequireSpecReview` decides whenever it is non-null, otherwise the project's value decides.
- A task explicitly created with `require_spec_review: false` on such a project runs straight through — an explicit `false` overrides a project `true`.

## Scenario: MR carries the OpenSpec

**When:**
- `cli_mr` opens a merge request for a task that produced a spec at `docs/openspecs/<slug>/`.

**Then:**
- The spec directory is committed into the MR branch under `docs/openspecs/<slug>/`.
- The MR body contains, above the change summary, a section rendering the spec's goal, key decisions, and verify steps, plus any clarification the agent raised together with the assumption it proceeded on.
- This happens for every MR — there is no toggle, and `IncludeSpecInMR` no longer exists in the analysis payload.

## Scenario: Long-running CLI step is allowed to finish

**When:**
- A `cli_implement` step runs for 4 hours while continuously emitting stdout.

**Then:**
- The step is not killed. The wall-clock cap is 5 hours (`CLIProfiles[*].TimeoutMinutes = 300`, engine fallback `defaultCLITimeout = 5h`).
- The 15-minute idle timeout is not triggered, because activity resets it.

## Scenario: Role is resolved on every CLI step

**When:**
- Any step in the CLI pipeline runs.

**Then:**
- The step assigns a role-matched agent and prepends that agent's role prompt via `LoadRolePrompt(agent, task, stepID)` before invoking the CLI.
- `cli_implement_backend` / `cli_implement_frontend` continue to run in role-suffixed worktrees on role branches, reconciled by `StepMerge`.
- Steps with no natural role (`cli_analyze`, `cli_spec`, `cli_mr`) resolve the project's default/lead agent rather than skipping role resolution — every step goes through one code path.

---

## Failure Scenario: No CLI provider configured

**When:**
- A task starts on a project with no enabled CLI execution-provider row.

**Then:**
- The task fails immediately at resolution with `no CLI execution provider configured for this project` and a pointer to the Execution Providers screen.
- It does **not** fall back to any API provider — no such path exists.
- Status is `failed`; no sandbox is started and no CLI binary is invoked.

## Failure Scenario: API execution provider submitted

**When:**
- A client `PUT`s an `execution_providers` list containing `{"type":"api","ref":"anthropic"}`.

**Then:**
- 400 with `execution_providers[i].type must be "cli"`, naming the valid refs (`claude_code`, `openai_codex`, `antigravity`, `custom`).
- Nothing is persisted — the whole list is rejected, not silently filtered.

## Failure Scenario: Non-CLI credential submitted

**When:**
- A client `POST`s a provider credential with `provider: "anthropic"` (or any non-`cli:` value).

**Then:**
- 400 with `provider must be a cli:* provider`. `IsAllowedProvider` returns false for everything except `cli:*`.

## Failure Scenario: Every CLI credential is unusable

**When:**
- All CLI credentials for the resolved profiles are in cooldown, or all are `needs_reauth`.

**Then:**
- Cooldown: the job sleeps until the earliest `cooldown_until` (`ErrAllCredentialsInCooldown`, existing behavior) and retries.
- `needs_reauth` with no usable alternative: the task fails with a message naming the credential label and asking for re-authentication.
- Neither case degrades to an API provider.

## Failure Scenario: CLI step exceeds 5 hours

**When:**
- A CLI step emits output steadily but has not exited after 5 hours.

**Then:**
- The step is killed with kill reason `timeout` (not `idle_timeout`), the task fails, and the log states the 5-hour wall-clock cap was reached.
- Partial work in the worktree is preserved for inspection, as with any failed CLI step today.

## Failure Scenario: CLI step goes silent

**When:**
- A CLI step produces no stdout/stderr for 15 minutes.

**Then:**
- Killed with `KillReasonIdleTimeout` and the existing stall message, regardless of how much of the 5-hour budget remains. Raising the wall-clock cap does not relax stall detection.

## Failure Scenario: Spec missing at MR time

**When:**
- `cli_mr` runs but `docs/openspecs/<slug>/` does not exist in the workspace.

**Then:**
- The MR is still opened — a missing spec must not block delivery of working code.
- The MR body carries an explicit `⚠️ No OpenSpec was produced for this task` notice in place of the spec section, and a warning is logged against the job.

## Failure Scenario: Memory search with no vector stream

**When:**
- `MemoryService.Search` is called.

**Then:**
- RRF merges the BM25 and knowledge-graph streams only. No embedding call is attempted and no error surfaces to the caller.
- Results remain ranked and capped by the same `limit`; only recall quality changes.

---

## Invariants

- No code path reachable from a task run makes an outbound LLM API call. The only outbound model traffic originates inside the CLI harness process in the sandbox, under its own subscription auth.
- `require_spec_review` is the only input to spec gating. Complexity, risk domains, autonomy level and review policy have no effect on whether a task pauses.
- A merge request always contains the spec the work was based on, or an explicit notice that none exists.
- Nothing merges without a human — removing the spec gate never removes the MR gate.
- Execution provider resolution is single-valued: `ResolvedExecutionProvider` is always CLI, resolved once at task start (existing REQ-005) and never re-resolved mid-task to switch harnesses.
- A credential is either `cli:*` or it does not exist in the pool.

## Rules

- `require_spec_review` resolution: `task.RequireSpecReview != nil ? *task.RequireSpecReview : project.RequireSpecReview`. Default at both levels is `false`.
- Clarification still gates independently of the flag: when `cli_spec` returns unanswered clarifying questions, the task enters `spec_review` even in auto mode — an agent that asked a question must not have it silently answered by proceeding. `MaxClarificationRounds = 2` continues to bound this so a task cannot deadlock on an unresponsive author.
- Timeout precedence is unchanged, only its values: explicit `req.Timeout` > `cfg.TimeoutMinutes` (300) > `defaultCLITimeout` (5h). Idle timeout is resolved independently and is never scaled from the wall-clock value.
- Deletion is complete, not conditional: no `if apiEnabled` flags, no dead `pkg/llm` shim, no commented-out provider constructors.

## Constraints

- The `.down.sql` for `000031` restores schema shape only. Deleted API credentials, `token_usage` rows and embedding vectors are unrecoverable — this migration is one-way in practice and must be announced as such.
- `cross_review` requires at least two distinct CLI profiles to be authenticated to run as a genuine cross-harness review. With only one profile available it is skipped and the job logs `cross_review skipped: only one CLI harness available`, rather than reviewing with the same harness that wrote the code.
- Existing `spec_review` UI, actions and transitions are reused unchanged; this work adds no new task status.
