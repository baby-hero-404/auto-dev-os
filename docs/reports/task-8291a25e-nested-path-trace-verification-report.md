# Verified Trace Report — Nested Path Bug & Fix-Loop Failure

**Task:** `8291a25e-017b-4c99-9e8e-bb07896e2beb` (zentao auto)
**Sources:** `server/.data/logs/8291a25e-*.jsonl` (372 entries), `server/.data/workspaces/8291a25e-*/logs/llm/` (154 LLM call traces with full prompt/request/response/parsed), workspace filesystem state, and server source code (commit `8b05fc1` + uncommitted working-tree hotfixes).
**Purpose:** Verify or refute the speculative findings in the v4.0 log-analysis report (Part 1) and the v5.0 architectural report (Part 2) with concrete log and code evidence.

---

## Executive Summary

The v4.0 report's headline finding (nested path bug) is **real but its root-cause narrative is wrong in an important way**. The nested directory tree was not caused by generic "ambiguous execution context" — it is the deterministic result of **three concrete, locatable defects** interacting:

1. **Diff generation and tool execution use different path bases.** The workspace diff shown to the reviewer/fixer uses *workspace-relative* paths (deliberately injected via `git diff --src-prefix`), while the tool executor resolves LLM-supplied paths against the *repository root*. Any path copied from the diff into a tool call lands nested.
2. **The fix step is tool-starved and role-misconfigured.** In the logged run, the fix step never advertised any edit tool (`create_file`/`search_replace`) to the LLM at all, while its instruction text explicitly commanded it to use them. When the model hallucinated those calls anyway, the capability layer rejected them (`role "reviewer" is not authorized to use tool "create_file"`).
3. **No path canonicalization or duplicate-prefix detection exists anywhere in the pipeline** (prompt builder, boundary policy, or tool layer), so once bugs 1+2 aligned, nothing stopped the nested writes.

Critically: **the actual workflow failure ("no workspace progress", exceeded max iterations) was caused by defect #2, not by the nested-path bug.** The nesting only became physically possible *after* a mid-session hotfix (uncommitted `llm_step.go` change remapping executor role `reviewer → backend`) let the previously-rejected `create_file` calls through — at which point defect #1 steered them to the wrong location.

The run ended in `structural failure (no workspace progress)` after 6 review→fix cycles and 154 LLM calls (~103 of them spent in fix loops).

---

## Timeline of the Run (from logs)

| Time | Event |
|---|---|
| 07-14 18:34 | Job queued, agent "AI Planner" assigned; `context_load`, `analyze` succeed |
| 07-14 18:35–18:40 | `code_backend_0/1/2` — LLM uses **correct repo-relative paths** (`go.mod`, `cmd/sync/main.go`, `internal/...`); each loop exhausts its 8-iteration budget but edits are salvaged ("3/4/1 edit(s) were applied... salvage as a partial result") |
| 07-14 18:40 | `review` (call-051) — receives diff with `code/repos/tool_zentao/main/...` prefixed paths, emits findings keyed by those paths |
| 07-14 18:41–18:49 | `fix` cycle 1 (calls 052–107, **Planner persona**, read-only toolset) — burns all iterations on `list_files` navigation; zero edits → `exceeded max iterations (8)` → `ErrNoProgress` |
| 07-15 09:28 | Run resumed; `fix` cycle 2 (calls 108+, **Reviewer persona**). Model calls `create_file` with **correct repo-relative paths** (`internal/repository/sqlite.go`, call-108) → rejected: `role "reviewer" is not authorized to use tool "create_file"` |
| 07-15 09:34–09:43 | ~9-minute gap between call-127 and call-131 — consistent with the server being rebuilt with the uncommitted `llm_step.go` reviewer→backend remap |
| 07-15 09:43 | call-131: model (after being told nothing about why earlier attempts failed differently) uses the **diff/finding paths** `code/repos/tool_zentao/main/internal/...` → `create_file` now executes → **nested tree created** at `<repo>/code/repos/tool_zentao/main/...` |
| 07-15 09:50 | Final failure: `step fix failed` → `structural failure (no workspace progress); skipping remaining retries` |

Physical evidence of nesting (workspace filesystem):

```
workspaces/8291a25e-.../code/repos/tool_zentao/main/
└── code/repos/tool_zentao/main/
    ├── cmd/sync/main.go
    ├── go.mod, config.json
    └── internal/{config,model,core,gitlab,zentao,repository,scheduler}/...
```

---

## Root-Cause Chain (fully traced, code-level)

### Step 1 — Coding steps operate repo-root-relative (correct)

`server/internal/orchestrator/llm_step.go:154` (`resolveAgenticWorkspace`) sets the tool executor's workspace to the **repo main checkout** (`.../code/repos/tool_zentao/main`) for agentic steps. The coding prompt explicitly states (call-003 prompt, line 494):

> `IMPORTANT: Your workspace root IS the repository root. All file paths MUST be relative (e.g., internal/model/commit.go).`

Result: all 50 coding-phase calls used clean repo-relative paths. **The LLM behaved correctly.**

### Step 2 — Diff generation injects workspace prefixes (defect)

`server/internal/orchestrator/gitops/client.go:195` (`GetWorkspaceDiff`, embedded Python):

```python
res = subprocess.run(["git", "-C", full_path, "diff",
    f"--src-prefix=a/{rel_path}/", f"--dst-prefix=b/{rel_path}/"], ...)
```

where `rel_path = code/repos/tool_zentao/main`. This deliberately rewrites repo-relative diff paths into workspace-relative ones. (The uncommitted hotfix to `getDiffPrefixes` at `client.go:99` only fixes `GetDiff`/`GetPRDiff` — **`GetWorkspaceDiff`'s Python script still injects prefixes**, and it is the one used by review/fix. The hotfix is incomplete.)

### Step 3 — Review findings inherit prefixed paths; fix passes them through verbatim (defect)

The reviewer (call-051) consumed the prefixed diff and emitted findings with `"file": "code/repos/tool_zentao/main/cmd/sync/main.go"`. `server/internal/orchestrator/steps/fix.go:160` injects those findings JSON **unchanged** into the fix instruction, together with the prefixed diff, then adds a *wrong* context statement (fix.go:163):

> `All file paths are relative to your workspace root.`

So the fix prompt simultaneously shows workspace-prefixed paths and tells the model they're workspace-root-relative — while the executor resolves them against the **repo root**. The model did the only reasonable thing: reused the paths it was given.

### Step 4 — Tool layer accepts the nested path (defect)

- `tool.SafeWorkspacePath` (`internal/tool/helpers.go:12`) only prevents directory traversal *escaping* the root — it never detects a duplicated `code/repos/<repo>/<branch>` prefix.
- The execution-boundary check (`steps/boundary_tool_executor.go:50`, `patch.EvaluatePolicy`) evaluated the nested path as at most a Warning (auto-expansion into `ExpandedBoundaries`), not a violation.
- `create_file` (`internal/tool/tools/create_file.go:95`) then `MkdirAll`s the entire phantom hierarchy.

Call-131 alone created 13 nested files + ran 2 `search_replace` on nested paths; call-134 recreated the same set.

---

## The Bigger Bug the v4.0 Report Missed: Fix-Step Tool Starvation

This is the **actual cause of the workflow failure** and of most of the 103 wasted fix calls.

Verified from the rendered "Available Tools" section of the prompts:

| Call | Persona | Advertised tools | Edit tools? |
|---|---|---|---|
| call-052-fix | `# Planner Role` ("Do NOT write implementation code") | file_exists, find_symbol, grep_search, list_files, read_affected_files, read_file, read_spec | **none** |
| call-108-fix … call-154-fix | `# Reviewer Role` | same + git_diff | **none** |
| call-003-code_backend | Senior Go Architect | read/search/build/git + **search_replace** | no `create_file` |

Why: tools are advertised from `capManager.ToolsForRole(agent.Role)` (`llm_step.go:48`), and `DefaultRoleProfiles()` (`internal/tool/capability.go`, committed version) gives:

- `planner`: Read/Search/Context/Docs — no edit caps
- `reviewer`: Read/Search/GitDiff/Context — no edit caps
- `backend`: Read/**Edit**/Build/Git/Search/Context — **CapCreate missing** (hotfixed in working tree)

Meanwhile the fix instruction (fix.go:163) says *"Use the available tools (e.g. search_replace, create_file) to fix ONLY the findings"*, and the coding instruction (coding_instruction.tmpl) says *"you MUST use the 'create_file' tool"* for new files. Consequences observed in logs:

1. **Fix cycle 1 (Planner):** model had literally no way to edit; looped on `list_files`/`read_file` until `exceeded max iterations (8)` → `ErrNoProgress` — repeated 3 attempts × multiple job retries.
2. **Fix cycle 2 (Reviewer, before hotfix):** model hallucinated undeclared `create_file` calls **with correct repo-relative paths** (call-108: `internal/repository/sqlite.go` etc.) — rejected by `Registry.Execute` (`internal/tool/registry.go:49`). The error text tells the model about roles, not about what it *should* do; nothing in the loop repairs the situation.
3. **Coding phase:** with `create_file` absent, the model worked around via `search_replace` on non-existent files — this also produced the stray compiled binary `sync` committed into the repo (flagged by review; the uncommitted `ReadLimitedFile` binary-guard hotfix in `pkg/paths/helpers.go` is a downstream symptom fix).
4. **After the mid-session hotfix** (uncommitted `llm_step.go:87` `reviewer→backend` remap): execution-side authorization passed, but **tool advertisement still comes from the unmapped role** — the model was executing tools it was never formally offered. It succeeded (call-131), but with the poisoned paths from Step 3 → nested tree.

> Note the tragic sequence: the model's *first* edit attempts (call-108) used **correct** paths and were rejected for authorization; by the time execution was allowed (call-131), the model had churned through more context and switched to the prefixed paths from the diff/findings. Both defects had to be fixed for the nesting to appear.

---

## Verdicts on the v4.0 Findings

| # | v4.0 Finding | Verdict | Evidence |
|---|---|---|---|
| 1 | Workspace root resolution / nested path bug | **CONFIRMED, mechanism corrected** | Nesting is a *path-base mismatch* between `GetWorkspaceDiff` (workspace-relative) and tool executor (repo-root-relative), not a missing-cwd problem. LLM emitted correct paths for 50+ calls before being fed prefixed ones. |
| 2 | Missing path canonicalization | **CONFIRMED** | No normalization in fix.go findings pass-through, `SafeWorkspacePath`, or `EvaluatePolicy`. Duplicated prefix accepted end-to-end. |
| 3 | Prompt leaks infrastructure paths | **CONFIRMED** | `gitops/client.go:195` injects `a/code/repos/tool_zentao/main/` prefixes; review findings and fix prompt carry them verbatim. |
| 4 | Context ambiguity (workspace vs repo root) | **CONFIRMED for fix step; refuted for coding steps** | Coding prompt has an explicit, correct header ("Your workspace root IS the repository root"). Fix prompt has an explicit **incorrect** one ("All file paths are relative to your workspace root") alongside prefixed paths. |
| 5 | Missing execution context header | **PARTIAL** | Header exists for coding; the fix step's header is present but *wrong*, which is worse than missing. |
| 6 | Prompt builder passes through previous outputs | **CONFIRMED** | `fix.go:146-160`: findings JSON injected unchanged; diff injected unchanged. |
| 7 | Natural language as integration protocol | **PARTIAL** | Findings are structured JSON, not free text — but the `file` field's path *semantics* are never normalized, so structure alone didn't help. |
| 8 | Missing structured execution state | **PARTIAL** | Execution snapshots + PromptHash resume + a phase state machine exist in code (`llm_step.go:95`, `statemachineloop.go`) but are feature-flagged; **0 state-machine log lines in this run**. |
| 9 | Prompt mixes facts and instructions | **CONFIRMED** | Fix prompt interleaves spec, manifest, diff, criteria, boundaries, findings, and instructions in one narrative. |
| 10 | Tool layer lacks path validation | **CONFIRMED** | `create_file` `MkdirAll`s any in-root path; boundary check passed nested path (recorded as ExpandedBoundary at most). |
| 11 | Progress detection inconsistent ("3 edits applied" vs "No workspace progress") | **REFUTED as a contradiction; real issue found** | The messages come from different steps/cycles. `ErrNoProgress` (`patch_retry_loop.go:85`) fires only when zero edits succeeded — which was *true* for the fix loops (all edits rejected by authorization). Progress detection worked; the tooling/authorization did not. |
| 12 | Global iteration budget | **CONFIRMED** | `runner.go:296` `MaxIterations: 8` shared across explore/edit/validate; fix cycle 1 burned all 8 on navigation every attempt. |
| 13 | Prompt not phase-aware | **PARTIAL** | Phase-aware prompts exist only in the inactive state machine. |
| 14 | Tool exposure not phase-specific | **PARTIAL, worse than reported** | Exposure *is* role-scoped — but misconfigured: fix got a read-only toolset while being ordered to edit. |
| 15 | Human review breaks deterministic context | Not evaluated in depth | Snapshot/PromptHash resume mechanism exists (`llm_step.go:95-144`); untested in this run. |
| 16 | Decisions not persisted | **PARTIAL** | FrozenContext (acceptance criteria, boundaries) *is* persisted and re-injected (`fix.go:134-143`); planner narrative is not. |
| 17 | Missing semantic validation before LLM calls | **CONFIRMED** | No pre-call validation of finding paths, duplicated prefixes, or advertised-tools-vs-instruction consistency. |
| 18–20 | Execution memory / graph / prompt-driven runtime | Architectural; partially addressed by in-flight state-machine work, not evidenced either way in this run. |

---

## New Findings (not in the v4.0 report)

| # | Finding | Severity | Evidence |
|---|---|---|---|
| N1 | **Fix step advertises zero edit tools while instructing their use** — the actual cause of the run's failure | P0 | Prompt tool lists of calls 052/108/131; `capability.go` role profiles; rejection errors in call-109 history |
| N2 | **Advertisement/enforcement role mismatch persists after hotfix**: `ToolsForRole(agent.Role)` uses raw role, executor uses remapped role — model executes tools it was never declared | P0 | `llm_step.go:48` vs `llm_step.go:87` (uncommitted) |
| N3 | **Fix/review steps run under wrong personas**: fix cycle 1 under `# Planner Role` ("Do NOT write implementation code"), cycle 2 under `# Reviewer Role` — never a coder persona | P1 | prompt.md line 15 of calls 052 vs 150; assigned agent per metadata.json |
| N4 | **`GetWorkspaceDiff` hotfix incomplete**: uncommitted `getDiffPrefixes` bypass covers `GetDiff`/`GetPRDiff` only; the multi-repo Python path (used by review/fix) still injects prefixes | P0 | `gitops/client.go:195` working tree |
| N5 | **Compiled binary committed and shown to reviewer**: `sync` binary landed in the repo (create-via-search_replace workaround + `run_build`), producing noise findings; `ReadLimitedFile` binary guard added as symptom hotfix | P2 | review finding on `code/repos/tool_zentao/main/sync`; `pkg/paths/helpers.go` uncommitted diff |
| N6 | **Tool-rejection feedback is not actionable**: `role "reviewer" is not authorized...` tells the model nothing it can act on, so it thrashes (path permutations across calls 108→127→131: `internal/...` → `code/repos/tool_zentao/internal/...` → `code/repos/tool_zentao/main/...`) | P1 | parsed.json path sequence across fix calls |
| N7 | **Mid-session code hotfixes changed system behavior inside one logged run** (create_file rejected at 09:28, succeeded at 09:43), making the log a composite of two configurations — worth noting for anyone re-reading these logs | Info | call timestamps; uncommitted git diff |

---

## Corrected Priority Matrix

| Priority | Action | Fix location |
|---|---|---|
| **P0** | Give the fix step an editing toolset: advertise and authorize the same (remapped) role for both `ToolsForRole` and the executor | `llm_step.go:48,87`, `capability.go` |
| **P0** | Stop injecting workspace prefixes into `GetWorkspaceDiff` (or strip them before review/fix consumption) — complete the `getDiffPrefixes` hotfix for the Python multi-repo path | `gitops/client.go:164-198` |
| **P0** | Canonicalize `file` paths in review findings to repo-relative before building the fix instruction | `fix.go:146-160` |
| **P1** | Reject or normalize duplicated repo-prefix paths in `SafeWorkspacePath` / boundary policy (detect `code/repos/<x>/<y>/` inside a repo-rooted workspace) | `internal/tool/helpers.go`, `patch.EvaluatePolicy` |
| **P1** | Fix the fix-step context line: paths in the diff/findings must be declared repo-relative (after P0 #2/#3 make that true) | `fix.go:163` |
| **P1** | Run fix under a coder persona (or a dedicated fixer profile), not Planner/Reviewer | prompt assembly / agent assignment |
| **P1** | Make tool-authorization errors actionable to the LLM, and validate instruction-vs-advertised-tools consistency before calling | `registry.go:49`, prompt builder |
| **P2** | Per-phase iteration budgets (the state-machine loop already models this — activate/finish it) | `runner.go:296`, `statemachineloop.go` |
| **P2** | Keep build artifacts out of the repo (gitignore in scaffold, or block binary writes) | scaffolding / `create_file` |

---

## Part 1 Conclusion

The v4.0 report correctly identified the nested-path symptom and the missing-canonicalization class of defects, but the log+code trace shows the system's failure mode was more specific and more fixable than "ambiguous execution context":

- The LLM was consistently correct whenever the pipeline gave it consistent information.
- The workflow failed because the fix step was **structurally incapable of editing** (no edit tools advertised, edits rejected by role authorization) — every "no workspace progress" failure traces to that.
- The nested tree was created in a single call (call-131) at the exact intersection of two defects: prefixed paths leaked from `GetWorkspaceDiff` into review findings, and the mid-session authorization hotfix that first allowed writes to execute.

Fixing the three P0 items above closes the observed failure end-to-end; the P1/P2 items harden the pipeline against the same class of drift recurring.

---
---

# Part 2 — Verdicts on the v5.0 Architectural Report

The v5.0 report ("Root Cause Analysis Beyond Prompt & Path Issues") argues the failures are symptoms of one architectural weakness: *natural language as the inter-agent protocol instead of a runtime-owned execution contract*. This part verifies each claim against the actual codebase and the run's traces.

## Headline correction: most of the proposed architecture already exists — it was switched off

The v5.0 "Proposed Target Architecture" (`Task → Planner → Execution Graph → Execution IR → Execution State → Prompt Compiler → Prompt → LLM`) is not a future design — it is substantially **implemented in commit `020435f`** ("feat: introduce formal execution state machine and IR-based contracts...") and specified in `docs/openspecs/execution-semantics-2026/`:

| v5.0 proposal | Existing implementation | Status in logged run |
|---|---|---|
| Typed Execution Contract | `models.ExecutionIR` (`pkg/models/ir.go`) — schema-versioned (`1.0`), embedded JSON schema, strict validation (`ValidateExecutionIR`), unknown-field rejection | **inactive** |
| Prompt Compiler (semantics → rendering) | `prompts.PromptCompiler` interface + `DefaultPromptCompiler.Compile(ir, physicalTargets)` (`internal/prompts/compiler.go`) — rejects invalid IR before rendering | **inactive** |
| Runtime-owned execution state | `models.ExecutionSnapshot` (`ir.go:128`) — current state, iteration, workspace diff, tool history, **PromptHash** for replay integrity | **inactive** |
| Node/phase state machine, per-phase budgets, phase-scoped tools | `llmrunner/statemachine.go` + `statemachineloop.go` — Discovery/Implementation/Validation phases, `PhaseBudgets`, per-phase allowed tools, resolved write targets | **inactive** |
| Immutable planner decisions | `models.FrozenContext` (`task.go:213`) — spec hash, execution units, IRs, boundaries, acceptance criteria; re-injected each step (`fix.go:134-143`) | **active** ✅ |
| Deterministic / idempotent prompts | PromptHash byte-identical resume check (`llm_step.go:95-144`) | **inactive** |

Why inactive: `pkg/config/config.yaml:72` ships `execution.state_machine_enabled: false` (wired via `cmd/api/main.go:200` → `WithStateMachineEnabled`), and the run log contains **zero** state-machine lines. The run executed on the legacy `toolloop.go` path with the flat 8-iteration budget.

So the accurate statement is not "the architecture is missing" but: **the new architecture exists, is disabled by default, and — even when enabled — does not yet cover the exact seam the bug flowed through** (see F1 verdict below).

## Verdicts on v5.0 findings

| # | v5.0 Finding | Verdict | Evidence |
|---|---|---|---|
| 1 | Missing typed execution contract | **PARTIAL — and the gap is narrower but sharper than claimed.** The planner→coder direction *is* typed (`TaskAnalysis`, `FrozenContext`, `ExecutionIR` — all Go structs persisted on the task). The genuinely untyped seam is **review→fix**: `getReviewFindings` (`steps/review.go:38`) returns `any` (raw `map[string]any` from LLM JSON), and `fix.go:160` re-marshals it verbatim into the next prompt. The v5.0 suggestion of a `path_type`/`semantic_path` discriminator is correct and **missing everywhere** — `AffectedFile{Repo, File}` and finding `file` fields carry no base-path semantics. |
| 2 | Prompt has become the integration layer | **CONFIRMED for the review→fix and diff transport; overstated elsewhere.** The workspace diff (text) is the *only* carrier of "what changed" into review and fix, and findings ride the prompt. But acceptance criteria, boundaries, and affected files travel as typed FrozenContext, not free prose. |
| 3 | Runtime has no source of truth | **PARTIALLY REFUTED.** `Task.Analysis` is a persisted, typed source of truth; FrozenContext is explicitly the "immutable execution contract for a workflow run"; artifacts/checkpoints persist step outputs. What has *no* owner is path semantics (see F5) and review output as a first-class typed artifact. |
| 4 | Semantic drift through prompt accumulation | **MECHANISM REFUTED, symptom confirmed.** The v5.0 drift example (`internal/config.go` → review adds prefix → fix doubles prefix *in the prompt*) did not happen. Grep of all 154 traces: **no prompt and no LLM output ever contained the doubled path.** The prefix was injected in one deterministic place (`GetWorkspaceDiff`'s `--src-prefix`, `gitops/client.go:195`); the doubling happened at **tool-execution time** when the executor joined the prefixed path onto the repo root. This is not gradual NL drift — it is two components disagreeing about a path base, which a typed contract would prevent but which is also fixable with a one-line canonicalization today. |
| 5 | Missing data ownership | **CONFIRMED — this is the strongest v5.0 finding.** Three components each assume a different path base with no owner: `gitops` emits workspace-relative (deliberately), the tool executor resolves repo-relative (`resolveAgenticWorkspace`), and the fix instruction *declares* workspace-relative (`fix.go:163`). Each was locally reasonable; no component owns the invariant. |
| 6 | Prompt builder behaves like a string builder | **PARTIALLY REFUTED.** `PromptAssembler` (`internal/prompts/builder.go`, ~1000 lines) is a section-based system with priorities, render order, immutable sections, token-budget optimization, and layered rules — not naive appending. However, the *step-level instruction builders* (e.g. `fix.go:128-164`) are exactly the `append → append → append` pattern v5.0 describes: diff text + findings JSON + context sentence concatenated with **no normalize/canonicalize/validate stage**, and the assembler treats the result as opaque text. The missing stage is real; its location is the step layer, not the assembler. |
| 7 | Runtime uses untyped context | **PARTIALLY REFUTED.** Same evidence as F1/F3: rich typed models exist and are used. Untyped: review findings, instruction strings, diff text. |
| 8 | LLM performs runtime responsibilities | **CONFIRMED for path interpretation.** In the fix step the model had to *guess* the path base from contradictory cues (prefixed diff + "relative to workspace root" + repo-rooted executor) — a runtime decision delegated to the model. The inactive state machine moves exactly this class of decision (current goal, phase, allowed targets) back to the runtime. |
| 9 | Prompt is not deterministic | **PARTIAL.** Determinism infrastructure (PromptHash, byte-identical resume) is implemented behind the flag. On the active path, prompts embed accumulated diffs/findings, so the v5.0 claim held for this run. |
| 10 | Missing context versioning | **PARTIAL.** `SpecHash`, `ExecutionSnapshot`, checkpoints and artifact history exist; there is no per-agent context version lineage (v1→v2→v3 as v5.0 sketches). Replay/rollback primitives exist but only under the flag. |
| 11 | Missing semantic boundary Runtime → Contract → Compiler → LLM | **EXISTS BY DESIGN, INACTIVE IN PRACTICE.** The exact desired pipeline (`ExecutionIR → PromptCompiler → LLM`) is in the codebase. The active path in the logged run was `runtime → instruction string → assembler → LLM`. |
| 12 | Natural language used as API | **MIXED.** Findings and manifests are structured JSON (not markdown prose), so the literal claim is overstated. But the run proves the deeper point: JSON structure without **semantic typing of the values inside it** (the untyped `file` path string) provides no protection — the poisoned path traveled through perfectly valid JSON. |

## What the architectural lens gets right — and what it would not have fixed

**Right:** the nested-path bug is a textbook ownership/contract failure (F5), and the proposed `semantic_path + path_type` discriminator directly prevents the entire class. The direction of `execution-semantics-2026` matches v5.0's target architecture almost 1:1 — the work is validated by this incident.

**Would not have fixed the run:** the workflow died from **fix-step tool starvation** (Part 1, N1) — no edit tools advertised, edits rejected by role authorization. That is a capability-wiring bug, orthogonal to the communication protocol: a fix step with a perfect typed contract and zero edit tools fails identically. Any architectural migration must not absorb or defer these mechanical P0s:

1. Fix-step toolset/role alignment (`llm_step.go:48` vs `:87`, `capability.go`)
2. `GetWorkspaceDiff` prefix injection (`gitops/client.go:195` — hotfix currently incomplete)
3. Path canonicalization at the review→fix seam (`fix.go:146-160`)

## Revised recommendations (merging Part 1 + Part 2)

| Priority | Action | Notes |
|---|---|---|
| **P0** | Ship the three mechanical fixes above | Independent of architecture; closes the observed failure |
| **P0** | Type the review→fix seam: a `ReviewFinding` struct with `semantic_path` (repo-relative, canonicalized by the runtime) replacing the `any` pass-through | Smallest possible slice of the v5.0 contract, applied where the bug actually flowed |
| **P1** | Add `path_type` semantics to `AffectedFile` / finding / boundary paths, with one canonicalization owner (runtime) | v5.0 F1/F5, confirmed by evidence |
| **P1** | Enable `execution.state_machine_enabled` in a staging profile and burn down its gaps (it already provides phase budgets, phase-scoped tools, PromptHash determinism) | The architecture v5.0 asks for is idle behind this flag |
| **P1** | Add the missing normalize→validate stage to step-level instruction builders (fix/review), not the PromptAssembler | Correct location per F6 evidence |
| **P2** | Context version lineage across agent hops (analyzer v1 → review v2 → fix v3) on top of existing SpecHash/snapshots | v5.0 F10 |
| **P2** | Persist review output as a first-class typed artifact (not only prompt text) | v5.0 F3 residual gap |

## Final Conclusion (updated)

Both external reports converge on real weaknesses, but the verified evidence assigns causality differently than either:

- **v4.0** was right about the symptom (nested paths) but wrong about the mechanism (it was not LLM confusion — the model was fed poisoned paths by the runtime and blocked from editing by role wiring).
- **v5.0** is right about the disease class (no owner for execution semantics; untyped values crossing agent seams) but wrong that the architecture is absent — it is **built and disabled**, and it currently leaves untyped exactly the seam that failed (review→fix findings). Its drift narrative (gradual NL mutation across iterations) is refuted by the traces: the corruption was a single deterministic injection plus a path-base mismatch.
- The run's terminal failure was mechanical, not architectural: a fix step ordered to edit with a read-only toolset.

The pragmatic path is therefore: fix the three mechanical P0s now; introduce the typed `semantic_path` contract at the review→fix seam as the first real consumer of the ExecutionIR philosophy; then activate and harden the already-built state machine rather than designing a new platform from scratch.
