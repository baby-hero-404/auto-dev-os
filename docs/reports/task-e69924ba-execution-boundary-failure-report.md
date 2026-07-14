# Incident Report: Task Execution Failure (task `e69924ba-3dae-496c-8684-b9f294b27ef7`)

**Task:** "zentao auto tool" — GitLab ↔ Zentao personal commit-sync service (Go, SQLite)
**Final status:** `failed` (after 3 retry attempts of the `fix` step)
**Data sources:**
- `server/.data/logs/e69924ba-3dae-496c-8684-b9f294b27ef7.jsonl` (90 workflow log lines)
- `server/.data/workspaces/e69924ba-3dae-496c-8684-b9f294b27ef7/task.json`, `metadata.json`
- `server/.data/workspaces/e69924ba-3dae-496c-8684-b9f294b27ef7/logs/llm/call-001-analyze` … `call-047-fix` (47 LLM call transcripts: `prompt.md`, `response.json`, `parsed.json`, `metadata.json`)

## 1. Summary

The task produced **zero working code**. Across three `code_backend_*` steps and three retries of the `fix` step (33 LLM calls total dedicated to implementation), the only artifact that ever landed in the final merge was a 3-line `go.mod` file:

```
go.mod | 3 +++
1 file changed, 3 insertions(+)
```

None of the GitLab client, Zentao client, SQLite repository, sync engine, or scheduler — the actual deliverables — were ever created. The review step correctly caught this and flagged it `critical`/`requires_fix: true`. All three `fix` retries then failed identically, each exhausting its 8-iteration tool-loop budget without writing a single line of the missing code, and the task ended in a hard failure.

**Root cause:** the `analyze` step produced an `execution_boundaries` list that does not cover the Go-convention entrypoint file it itself listed as a required deliverable, and an `affected_files` list too sparse to give 2 of the 3 planned execution units (`api-clients`, `sync-engine-scheduler`) any file target at all. Every attempt by the agent to do the one thing every prompt explicitly told it to do — create `cmd/zentao-sync/main.go` — was rejected by the (unrelated, older) workspace policy engine as an execution-boundary violation. The agent had no coherent path to make progress, floundered, and burned its iteration budget on discovery loops and no-op edits instead.

## 2. Timeline (from the workflow log)

| Time (UTC+7) | Event |
|---|---|
| 17:02:23 | Job queued, `AI Planner` assigned, `context_load` succeeds |
| 17:02:25 – 17:02:33 | `analyze` runs once (`gemini-3.1-flash-lite`), produces the spec; **pauses for human review** |
| 17:08:38 | Human resumes; `plan` step runs |
| 17:08:38 | ⚠️ `plan` logs: *"intent resolution incomplete"* for **all 3** execution units — see §3.1 |
| 17:08:53 – 17:09:41 | `code_backend_0`, `code_backend_1`, `code_backend_2` each run to their 8-iteration cap and are salvaged as **partial** results (5, 4, 1 "edits applied" respectively) |
| 17:09:41 | `merge` — only `go.mod` (3 lines) actually reaches the merged branch |
| 17:09:43 – 17:09:50 | `review` correctly flags the diff as insufficient, `requires_fix: true`, severity `critical` |
| 17:09:50 – 17:10:02 | `fix` attempt 1/3: 7 LLM calls, **zero edits**, ends `failed` |
| 17:10:06 – 17:10:28 | `fix` attempt 2/3: identical pattern, `failed` |
| 17:10:36 – 17:10:46 | `fix` attempt 3/3: identical pattern, `failed` |
| 17:10:46 | Task ends: `agentic tool loop failed: exceeded max iterations (8)` |

## 3. Root Cause

### 3.1 The `analyze` step's own output is internally contradictory

From `call-001-analyze/parsed.json`, the entire task got exactly **one** execution boundary:

```json
"execution_boundaries": [
  { "module": "core", "root": "internal/", "capabilities": ["modify_existing", "create_helper"] }
]
```

and exactly **three** affected files:

```json
"affected_files": [
  { "file": "cmd/zentao-sync/main.go",        "reason": "Entry point for the application" },
  { "file": "internal/sync/engine.go",        "reason": "Core business logic for synchronization" },
  { "file": "internal/repository/sqlite.go",  "reason": "Persistence layer for sync status" }
]
```

`cmd/zentao-sync/main.go` — the file `affected_files` itself labels "Entry point for the application" — sits outside `internal/`, the only root any execution boundary grants. Standard Go project layout *requires* this (entrypoints live in `cmd/`, packages in `internal/`); the analyze step's own `design_md` field even says so ("Cấu trúc thư mục: Tuân thủ theo tiêu chuẩn project layout của Go" — *"Directory structure: follow Go's standard project layout"*). The spec asks for a layout that its own boundary list forbids.

Every `code_backend_*` and `fix` prompt in this task carries this identical, single boundary (verified byte-for-byte across `call-002`, `call-010`, `call-018`, and `call-027`'s `prompt.md`). Every attempt to create `main.go` was therefore guaranteed to fail, regardless of which of the three execution units made the attempt, and regardless of which LLM produced the tool call.

Confirmed directly in the transcript — `call-004-code_backend_0` requested:
```json
{"name": "search_replace", "arguments": "{\"path\":\"cmd/zentao-sync/main.go\", ...}"}
```
and the tool result fed back on the next turn (`call-005-code_backend_0/prompt.md`) was:
```
Error: execution boundary violation on "cmd/zentao-sync/main.go": file "cmd/zentao-sync/main.go" is outside of all approved execution boundaries. Choose a file within your assigned module, or explain in your summary why this file must change.
```
(raised by `server/internal/orchestrator/patch/policy_engine.go:204`)

### 3.2 Two of the three execution units never received any file target at all

The plan produced three execution units:

| Unit | Objective | Files in `affected_files` |
|---|---|---|
| `init-core` | "Thiết lập cấu trúc dự án và SQLite" (project setup + SQLite) | `sqlite.go` (blocked: `main.go` also claimed, boundary-rejected) |
| `api-clients` | "Triển khai giao tiếp với GitLab và Zentao API" (GitLab/Zentao clients) | **none** — no GitLab-client or Zentao-client file appears anywhere in `affected_files` |
| `sync-engine-scheduler` | "Hoàn thiện logic đồng bộ và lập lịch chạy" (sync engine + scheduler) | **none** — no sync-engine or scheduler file appears anywhere in `affected_files` |

Since every `code_backend_N` prompt's "Workspace Affected Files" section is populated from the same task-level `affected_files` list (not scoped per unit — see `server/internal/orchestrator/llmrunner/runner.go:57-84`, `BuildInitialMessages`), `code_backend_1` and `code_backend_2` were told about the same 3 files as `code_backend_0`, none of which matched their actual objective. This is directly visible in their tool-call transcripts:

- **`code_backend_1`** (assigned: GitLab/Zentao API clients) never once called `create_file` or `search_replace` on any client file. Its 8 calls were: `list_files`, `read_file(go.mod)`, three `search_replace` calls adding a sqlite dependency to `go.mod` (unrelated to its own objective), a `run_build("go mod tidy")`, another duplicate `go.mod` edit, and a final `run_build("mkdir -p internal/client internal/repository internal/sync cmd/zentao-sync")` — directory scaffolding, no code.
- **`code_backend_2`** (assigned: sync engine + scheduler) spent 5 of its 8 calls on `list_files` across `internal/client`, `internal/repository`, `internal/sync`, `cmd/zentao-sync` (all still empty at that point), then attempted `internal/repository/sqlite.go` — a file that belongs to `init-core`'s objective, not its own — twice, presumably because it was the only concrete Go-shaped path it could infer from the shared, generic `affected_files` list.

### 3.3 Compounding factor: the intent resolver cannot tokenize Vietnamese capability text

Independently of §3.1/§3.2, the `plan` step logged (line 20 of the jsonl, quoted in full since it names all three failures at once):

```
[plan #1] Plan: intent resolution incomplete: intent resolver: node "init-core" capability
"Thiết lập cấu trúc dự án và SQLite": no workspace file matched tokens [thiết, lập, cấu, trúc, dự, án, và, sq, lite]
intent resolver: node "api-clients" capability "Triển khai giao tiếp với GitLab và Zentao API":
no workspace file matched tokens [triển, khai, giao, tiếp, với, git, lab, và, zentao, api]
intent resolver: node "sync-engine-scheduler" capability "Hoàn thiện logic đồng bộ và lập lịch chạy":
no workspace file matched tokens [hoàn, thiện, logic, đồng, bộ, và, lập, lịch, chạy]
```

`intentTokens()` (`server/internal/orchestrator/steps/intent_resolver.go:23-55`) is designed to split identifier-style capability names ("UserRepository", "user_repository") into match tokens, then `pathMatchesTokens()` (same file, line 60) requires **every** token to appear as a substring of a candidate file path. Fed a full Vietnamese sentence instead of an identifier, this tokenizes into syllables ("thiết", "lập", "cấu", "trúc"...) that can never substring-match an English file path — resolution fails **100% of the time** for any non-English/non-identifier capability string, not as an edge case.

This particular run was not gated by this failure — the task ran with `execution.state_machine_enabled=false` (confirmed by the log's `[TELEMETRY-VIOLATION] Shadow state machine: ...` messages, which are the non-blocking shadow-mode telemetry path in `server/internal/orchestrator/llmrunner/runner.go`'s legacy `runAgentic`, not the enforcing `runStateMachine` path), so per `ResolveExecutionIRTargets`'s own doc comment (`server/internal/orchestrator/steps/intent_resolver.go:107-115`) this is currently "logged as warnings rather than pausing the workflow." But the code comment also says hard enforcement is intended to activate once the state machine is fully rolled out — at that point, this same Vietnamese-language task would fail *earlier and harder* (blocked at `PLAN_READY`) instead of quietly producing an empty `resolvedTargets` map.

### 3.4 Consequence: the agent has no error-recovery strategy for a self-contradicting prompt

Given a prompt that says "create `cmd/zentao-sync/main.go`" and a tool that says "you may not touch anything outside `internal/`," the model (`gemini-3.1-flash-lite`, used for every step of this task — analyze, all three `code_backend`, review, and fix) did not recover coherently. From the raw tool-call transcripts:

- `code_backend_0`: `list_files` → `search_replace(go.mod, create)` → `search_replace(main.go)` **[blocked]** → `list_files` (re-orienting) → **four consecutive identical no-op `search_replace` calls on `go.mod`** (`search` field byte-identical to `replace` field) → budget exhausted.
- `fix` (all 3 attempts): 7 calls each, **100% `list_files`**, on paths already listed in a prior call within the same attempt (e.g. attempt 1 lists `.`, then later re-lists `.` again; lists `code/repos/tool_zentao/main` twice). Zero write-tool calls in any of the 21 `fix` LLM calls across all 3 attempts. No edits, no summary, no acknowledgment of the blocked boundary — just repeated, non-progressing discovery until the hard iteration cap.

This also exposed a secondary tool-loop gap: the anti-repetition safeguards in `RunToolLoop` (`server/internal/orchestrator/llmrunner/toolloop.go`) only cover **failed** calls (`failureCounts`, keyed on an `"Error:"`-prefixed result) and **duplicate `read_file` reads** (`readMemo`). A *successful* no-op `search_replace` (search string == replace string) or a repeated *successful* `list_files` on a path already listed this same run trips neither safeguard, so a model that starts thrashing has nothing stopping it from spending its entire budget on motion without progress. The `code_backend_0` log even reports this as apparent success — *"5 edit(s) were already applied; surfacing as a partial result"* — when only one of those five `search_replace` calls (the initial `go.mod` creation) was materially different from the one before it.

## 4. Impact

- Task fully failed after ~8.5 minutes of wall-clock time and **33 LLM calls** dedicated to implementation (`code_backend_0/1/2` + 3× `fix`), none of which produced usable code.
- Of the 3 planned execution units, 2 (`api-clients`, `sync-engine-scheduler` — the majority of the actual scope) never had a chance to succeed: they were never given a file target that matched their objective.
- All 3 `fix` retries were guaranteed to fail identically, since retrying does not change the underlying contradiction (same boundary, same affected-files list, same model) — the 2 extra retries (attempts 2 and 3, ~50 more seconds and 14 more LLM calls) added cost with zero chance of success.

## 5. Recommendations

1. **Validate `execution_boundaries` coverage against `affected_files` in the analyze step** (`server/internal/orchestrator/steps/analyze.go`, near the existing presence-only check at line 280-283). If any `affected_files[].file` falls outside every declared boundary's `root`, either reject the analyze output and re-prompt, or auto-widen/add a boundary to cover it — Go's `cmd/` + `internal/` split should be a first-class case, not an accidental gap.
2. **Give every execution unit its own file targets**, not a single task-wide `affected_files` list shared verbatim across all `code_backend_N` prompts. This is the more impactful fix — `api-clients` and `sync-engine-scheduler` had no way to succeed no matter how boundaries were fixed, because they were never told which files to create. Since `analysis.ExecutionIRTargets` (the intent-resolver's per-node output, see §3.3) is exactly this — a `node_id -> paths` map — wiring `BuildInitialMessages`'s "Workspace Affected Files" section to that per-node map instead of the flat task-level list would fix both this and part of §3.3 at once, provided §3.3's tokenizer is also fixed (below).
3. **Fix the intent-resolver tokenizer for natural-language capability text** (`server/internal/orchestrator/steps/intent_resolver.go:23-55`). At minimum, detect when `Intent.Capability` is a sentence rather than an identifier (contains spaces + multiple words in the source script) and fall back to a different matching strategy (e.g., match against `affected_files[].reason`, or require the analyze step to also emit a short English/identifier-style slug per node alongside the natural-language objective). This is not Vietnamese-specific — any non-identifier capability string fails the same way.
4. **Add a stuck-loop safeguard to `RunToolLoop`** for successful-but-non-progressing calls: a no-op `search_replace` (search == replace) or a repeat of an already-issued read-only call (`list_files` on a path already listed this run) should feed back a corrective nudge ("you already listed this path with no new information; either write to a file now within your boundary or explain in your summary why you cannot") instead of silently consuming iteration budget.
5. **Don't retry `fix` 3 times unconditionally.** When a `fix` attempt makes zero edits (as opposed to an edit that fails validation), the underlying blocker is structural, not transient — retrying with an unmodified prompt against the same boundary is very unlikely to succeed and mainly adds latency/cost. Consider capping retries at 1 for a zero-edit result, or surfacing the specific boundary-violation error back into the *next* retry's prompt so the model (or a human) can actually see and react to the contradiction it hit.
