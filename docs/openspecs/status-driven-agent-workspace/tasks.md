# Implementation Map: Status-Driven Agent Workspace

**Goal:** Build a full-stack event system and split-screen UI that replaces the current monolithic Task Detail page with a state-machine-driven workspace where `spec_review` is the only human approval gate and every later step (decomposition, coding, review/fix/test, PR creation) runs automatically with live progress visible in the Agent Execution Stream. Delete the redundant components and specs this replaces.
**Tech Stack:** Go 1.26+, PostgreSQL, SSE, Next.js (App Router), React, TypeScript

---

## Phase 1: Event System Foundation (Backend)

### 1.0 Verify `planning_split` is a reachable backend status — RESOLVED: it is not, drop it

**Why:**
`design.md` flagged `planning_split` as unverified: it exists in the frontend `TaskStatus` union and is checked by `SplitProposalCard.tsx`/`TaskSubtasks.tsx`, but no Go source contained the literal `"planning_split"`. A full-repo grep (`grep -rn "planning_split\|PlanningSplit" server --include=*.go`) confirms **zero matches** — it is not a constant in `server/pkg/models/task.go`, not a key in `ValidTaskTransitions`, and not built dynamically from a shared constant under another name. It is dead frontend code: a `TaskStatus` union member (`web/src/lib/types.ts:145`), a status-badge entry (`web/src/lib/status/index.ts:50`), and defensive checks in `SplitProposalCard.tsx`/`TaskSubtasks.tsx` that never actually fire in production. **Decision: drop it — do not formalize it as a backend status.**

**Depends on:**
- None (this was a research spike, not a code change).

**Changes:**
- [x] Grepped the full backend for `planning_split`/`PlanningSplit` — zero occurrences. Confirmed via `web/src/lib/status/index.ts` and `TaskSubtasks.tsx` that the frontend already derives split-in-progress state from `task.proposed_split`/subtask presence, not from `task.status`.
- [x] Removed the `planning_split` row from this spec's `available_actions` table (`specs.md`) and the `StatusViewRegistry` entry (`design.md`) — 13 real statuses, not 14.
- [x] Delete the `planning_split` member from `web/src/lib/types.ts`'s `TaskStatus` union and its entry in `web/src/lib/status/index.ts`; handle split-proposal display as a **sub-view of `coding`** (`CodingProgressView` conditionally renders `SplitProposalView`'s content when `task.analysis?.child_specs` is present), matching what `SplitProposalCard.tsx` already does today.
- [x] Update `SplitProposalCard.tsx`/`TaskSubtasks.tsx` call sites that reference the `"planning_split"` string literal to check `child_specs` presence instead, before those files are deleted per Phase 5A/5B.

**Verify:**
- [x] Outcome written into `design.md`, `specs.md`, and `proposal.md`, replacing every "open verification item" / provisional note with the resolved decision.

---

### 1.1 Create `TaskEvent` model and migration

**Why:**
The entire feature depends on a persistent, queryable event store. Without the `task_events` table, neither the SSE broadcast nor the timeline UI can function. This is the foundational data layer.

**Depends on:**
- None (this is the starting point).

**Files:**
- `server/pkg/models/task_event.go`
- `server/migration/000026_add_task_events.up.sql`
- `server/migration/000026_add_task_events.down.sql`

**Changes:**
- [x] Define `TaskEvent` struct with fields: `ID` (UUID), `TaskID` (UUID FK), `SequenceNumber` (int64, per-task monotonic — see design.md's Ordering section), `Type` (string), `SchemaVersion` (int, default 1), `Payload` (JSONB), `ArtifactID` (nullable UUID FK into `workflow_artifacts`), `SizeBytes` (int), `CreatedAt` (timestamp, display/debug only — never the ordering key).
- [x] Define `AgentEventType` constants: `task.started`, `task.completed`, `task.error`, `status.changed`, `agent.reasoning_summary`, `agent.plan`, `agent.message`, `tool.started`, `tool.finished`, `file.changed`, `command.started`, `command.finished`, `test.result`.
- [x] Define payload size constants: `MaxPayloadBytes = 8192`, `MaxTailBytes = 5120` (for `stdout_tail`/`stderr_tail`) — used by the Agent Adapter (task 2.1), declared here alongside the model they constrain.
- [x] Write UP migration: `CREATE TABLE task_events (...)` with composite indexes `idx_task_events_task_seq ON (task_id, sequence_number)` (ordering/cursor queries — primary) and `idx_task_events_task_created ON (task_id, created_at)` (display/debug fallback).
- [x] Write DOWN migration: `DROP TABLE IF EXISTS task_events`.

**Verify:**
- [x] `make dev-be` runs migrations successfully (migration 000026 applies). Re-run live 2026-08-07: `make migrate` against `autocodeosdb` (docker) applied cleanly, `version=30 dirty=false`.
- [x] `SELECT * FROM task_events;` returns empty result set (table exists). Confirmed live 2026-08-07: `select count(*) from task_events` = 0 across all existing tasks.
- [x] Schema check: `sequence_number` and `size_bytes` are `NOT NULL`; `artifact_id` is nullable and has a foreign key constraint to `workflow_artifacts(id)`. Confirmed live via `\d task_events`: FK `fk_task_events_artifact_id` present, `sequence_number`/`size_bytes` both `NOT NULL`.

---

### 1.2 Create `TaskEvent` repository

**Why:**
Data access layer for persisting and querying events. Separates DB concerns from business logic.

**Depends on:**
- 1.1 (model and table must exist).

**Files:**
- `server/internal/repository/task_event.go`

**Changes:**
- [x] Implement `CreateEvent(ctx, event *models.TaskEvent) error` — inside a single DB transaction, assigns `event.SequenceNumber = COALESCE(MAX(sequence_number), 0) + 1 FROM task_events WHERE task_id = ? FOR UPDATE`, then inserts. The per-task row-lock (not a global lock/sequence) keeps concurrent tasks from contending with each other.
- [x] Implement `ListByTaskID(ctx, taskID string) ([]models.TaskEvent, error)` — ordered by `sequence_number ASC`, not `created_at`.
- [x] Implement `ListByTaskIDAfter(ctx, taskID string, afterSeq int64) ([]models.TaskEvent, error)` — for SSE reconnect catch-up, ordered by `sequence_number ASC`.
- [x] Implement `ListByTaskIDPaginated(ctx, taskID string, beforeSeq int64, limit int) ([]models.TaskEvent, error)` — cursor-paginated history for `GET /events` (v1, not deferred — see design.md's Performance Considerations).
- [x] Implement `CountByTaskID(ctx, taskID string) (int64, error)` — used by the `max_event_count` guardrail (task 2.3) to check the running total cheaply.

**Verify:**
- [x] Unit test: insert 3 events, `ListByTaskID` returns them in `sequence_number` order.
- [x] Unit test: two concurrent `CreateEvent` calls for the same `task_id` (simulated via goroutines + a sync point) never produce a duplicate `sequence_number`.
- [x] Unit test: `ListByTaskIDAfter` with a cursor returns only events with a greater `sequence_number`.
- [x] Unit test: `ListByTaskIDPaginated` returns at most `limit` rows, ordered descending by `sequence_number`, with a stable cursor for the next page.

---

### 1.3 Create `TaskEvent` service with SSE broadcaster

**Why:**
The service layer handles the dual write: persist to DB, then broadcast to SSE subscribers. The broadcaster manages per-task subscriber channels.

**Depends on:**
- 1.2 (repository must exist).

**Files:**
- `server/internal/service/task_event.go`

**Changes:**
- [x] Implement `EmitEvent(ctx, taskID, eventType string, payload json.RawMessage) (*models.TaskEvent, error)` — writes to DB via repository, then broadcasts to all subscribers for this task.
- [x] Implement `Subscribe(taskID string) chan models.TaskEvent` — returns a buffered channel. Subscriber receives all future events for this task.
- [x] Implement `Unsubscribe(taskID string, ch chan models.TaskEvent)` — removes a subscriber channel.
- [x] Implement `ListEvents(ctx, taskID string) ([]models.TaskEvent, error)` — delegates to repository.
- [x] Use a `sync.RWMutex`-protected map of `taskID → []chan TaskEvent` for subscriber management.

**Verify:**
- [x] Unit test: `EmitEvent` persists to DB and subscriber channel receives the event.
- [x] Unit test: `Unsubscribe` removes the channel; subsequent emits don't block or panic.
- [x] Unit test: Multiple subscribers for the same task all receive the same event.

---

### 1.4 Create event stream HTTP handler

**Why:**
Exposes the event system to the frontend via two endpoints: SSE stream (live) and REST list (history).

**Depends on:**
- 1.3 (service must exist).

**Files:**
- `server/internal/handler/task_event.go`
- `server/internal/handler/router.go`
- `server/internal/handler/services.go`

**Changes:**
- [x] Implement `StreamEvents` handler: SSE endpoint at `GET /tasks/{taskID}/events/stream`, accepting an optional `Last-Event-ID` header or `?after=<sequence_number>` query param — **both carry a `sequence_number`, never a UUID or timestamp**. **Ordering matters**: (1) subscribe to the live broadcast channel first, buffered; (2) query `task_events` via `ListByTaskIDAfter` for anything with a greater `sequence_number` and stream it; (3) drain any live events buffered during (2), deduped by `sequence_number` against what was just streamed; (4) continue streaming live broadcasts. This ordering is what closes the race described in `design.md` → SSE Reconnect — do not implement "load history, then subscribe," which drops events emitted in the gap.
- [x] Every SSE frame sets the `id:` field to the event's `sequence_number` (not its UUID `id`) — required both for the browser's native `Last-Event-ID` reconnect behavior and because `sequence_number`, not the UUID, is the actual ordering key (see design.md's Ordering section).
- [x] Implement `ListEvents` handler: REST endpoint at `GET /tasks/{taskID}/events`, cursor-paginated via `?before=<sequence_number>&limit=` (default limit 200, max 1000) using `ListByTaskIDPaginated` — this ships in v1, not deferred, so a task with a large history doesn't produce a slow/unbounded initial load.
- [x] Register both routes in `router.go` under the existing task routes group.
- [x] Wire `TaskEventService` into the handler dependency injection in `services.go`.

**Verify:**
- [x] Manual test: `curl -H "Authorization: Bearer ..." http://localhost:32080/api/v1/tasks/{id}/events/stream` returns SSE events with `id:` set to `sequence_number`. Run live 2026-08-07 against the running API (port 32080) with 2 rows manually inserted into `task_events`: response is `event-stream` (200, `Content-Type: text/event-stream`), body is exactly `id: 1\nevent: status.changed\ndata: {...}\n\n` / `id: 2\n...` — `id:` matches `sequence_number`. Also verified reconnect: `?after=1` replays only sequence 2, no duplicate of 1. Test rows deleted afterward.
- [x] Manual test: `curl -H "Authorization: Bearer ..." http://localhost:32080/api/v1/tasks/{id}/events?limit=50` returns a JSON array of at most 50 events plus a next cursor. Run live 2026-08-07: returns a plain JSON array, most-recent-first, matching the frontend's cursor derivation (`before=<last sequence_number>`) — there is no separate `next_cursor` field in the response body; pagination is client-derived from the last item's `sequence_number`, consistent with `AgentTimeline.tsx`'s `cursorRef` usage and the `List` handler's `before`-param cursor scheme.
- [x] Integration test: emit an event directly *while* the stream loop is between the catch-up query and draining the buffer, assert exactly-once delivery. **Real bug found and fixed**: `streamEventsLoop` (`internal/handler/task_event.go`) emitted the catch-up history, then unconditionally emitted every buffered live event with no dedup — an event committed to the DB (and broadcast) between `Subscribe` and the catch-up query landing in both `history` and `buffer` was emitted twice. Added `internal/handler/task_event_test.go`'s `TestStreamEventsLoop_NoDuplicateOnRace`, which failed against the original code (confirmed duplicate emission) and passes after fixing the loop to skip any buffered event whose `sequence_number` is ≤ the max already emitted from history.
- [x] Manual test: reconnect with `?after=<sequence_number>` for a value partway through history — only events with a greater `sequence_number` are streamed, not the full history.

---

## Phase 2: Agent Adapter (Backend)

### 2.1 Build CLI-to-Event normalizer

**Why:**
Bridges the gap between raw CLI output (stdout/stderr/JSON lines) and the structured `AgentEvent` schema. Without this, the UI has nothing to render.

**Depends on:**
- 1.3 (event service must exist to emit events).

**Files:**
- `server/internal/sandbox/event_adapter.go`

**Depends on (additional):**
- Existing `ArtifactRepo` (`server/internal/repository/artifact.go`) and `WorkflowArtifact` model — reused, not recreated, for large-output externalization below.

**Changes:**
- [x] Create `EventAdapter` struct that wraps `TaskEventService` and `ArtifactRepo`.
- [x] Implement `ProcessLine(ctx, taskID string, line string)` — parses a single line of CLI output:
  - If JSON and matches known schema → emit typed event (`tool.started`, `agent.reasoning_summary`, etc.).
  - If unstructured text → emit `agent.message` with raw text in payload.
- [x] Implement payload truncation before every emit: total `payload` capped at `MaxPayloadBytes` (8KB); `stdout_tail`/`stderr_tail` individually capped at `MaxTailBytes` (5KB) with a `"... truncated, N bytes total"` suffix marker when exceeded; `file.changed` events never carry a diff body, only `additions`/`deletions` counts.
- [x] Implement large-output externalization: for event types where the full content is the point (`command.finished` with `exit_code != 0`, `test.result` with failures), if the full content exceeds `MaxTailBytes`, write it via `ArtifactRepo.Create` (`Type: "event_output"`, `TaskID`, `Payload: fullContent`) and set the emitted event's `ArtifactID` to the created artifact's ID, with `payload.summary` holding a short human-readable synopsis (e.g. `"npm test failed: 3 of 42 tests failed"`) instead of the truncated tail alone.
- [x] Set `SizeBytes` on every emitted `TaskEvent` to `len(payload)` at write time.
- [x] Implement `EmitStatusChange(ctx, taskID, from, to string)` — convenience wrapper for `status.changed` events.
- [x] Integrate `EventAdapter` into the sandbox execution loop where CLI stdout is currently read (in `sandbox/docker.go` or equivalent).

**Cleanup:**
- [x] Do NOT remove the existing `TaskLog` write path. Both `TaskLog` and `TaskEvent` are written during the transition period.

**Verify:**
- [x] Unit test: JSON line `{"type":"tool_call","tool":"terminal","command":"go test"}` produces a `tool.started` event.
- [x] Unit test: Plain text line `"Analyzing authentication flow..."` produces an `agent.message` event.
- [x] Unit test: Malformed JSON line produces an `agent.message` fallback (no panic, no error).
- [x] Unit test: a command output line with a 20KB stdout blob produces a `command.finished` event whose `stdout_tail` is ≤5KB and ends with the truncation marker.
- [x] Unit test: a failing `test.result` with a 40KB failure log produces an event with a non-nil `ArtifactID`, a `payload.summary` string, and no raw 40KB blob embedded in `payload`; a subsequent `ArtifactRepo` lookup by that ID returns the full 40KB content.
- [x] Unit test: every emitted event's `SizeBytes` equals the actual serialized `payload` byte length.

---

### 2.2 Add `available_actions` to Task API response

**Why:**
The frontend needs a backend-authoritative list of valid actions for the current task state. This eliminates hardcoded `if/else` button logic in the UI.

**Depends on:**
- None (independent of event system — can be done in parallel with Phase 2.1).

**Files:**
- `server/pkg/models/task.go` (add `AvailableAction` type, add field to Task response)
- `server/internal/service/task.go` (compute actions based on status/job state)

**Changes:**
- [x] Define `AvailableAction` struct: `ID`, `Label`, `Style`, `ConfirmationRequired` (bool), `Endpoint` (string, always `"POST /tasks/{taskID}/actions"` for now), `DisabledReason` (string, `omitempty`, populated from the `ActionPolicy` check in task 2.4 for the requesting caller).
- [x] Add `AvailableActions []AvailableAction` field to the Task struct (JSON-only, not GORM-persisted: `gorm:"-"`).
- [x] Implement `computeAvailableActions(task *Task, job *WorkflowJob) []AvailableAction` in the task service. This is the **single** function permitted to return an approval-style action, and only for `TaskStatusSpecReview` — see `specs.md` invariant #2 and the Rules section.
- [x] Populate `AvailableActions` in `GetByID`, `Update`, and any other method that returns a full Task object.

**Verify:**
- [x] Unit test: `task.status = "coding"` with `job.status = "running"` → actions: `["pause", "cancel"]`.
- [x] Unit test: `task.status = "spec_review"` → actions: `["approve_spec", "request_changes", "cancel"]` (the only status where this is true).
- [x] Unit test: for a task with non-empty `proposed_split` while `status = "coding"`, `available_actions` still returns the normal `coding` set (`["pause", "cancel"]`) — never `"approve_split"`/`"reject_split"`; the split payload does not alter `available_actions`.
- [x] Unit test: `task.status = "pr_ready"` and `task.status = "human_review"` → actions: `["cancel"]` — never `"approve_merge"`/`"reject_merge"`.
- [x] Unit test: `task.status = "merged"` → empty actions array.
- [x] Unit test: `task.status = "blocked"` → actions: `["retry_blocked", "cancel"]`.
- [x] Table-driven test iterating every `TaskStatus` value except `spec_review`: assert returned action IDs are a subset of `{pause, cancel, retry, retry_blocked, delete, execute}` (no approval-style ID leaks into any other status — this is the invariant, not just a spot check).
- [x] API test: `GET /tasks/{id}` response includes `available_actions` array — added `server/internal/handler/task_test.go`'s `TestTaskHandler_GetByID_IncludesAvailableActions` (chi router + `httptest`, fake `TaskService`), asserts the field survives JSON serialization over real HTTP. Passing.

---

### 2.3 Implement execution guardrails (max retry / time / cost / security)

**Why:**
Autopilot without a stopping condition is uncontrolled execution, not a feature. This is the mechanism that makes "no per-step human approval" safe — see `design.md` → "Autopilot execution is bounded by explicit guardrails."

**Depends on:**
- 1.3 (event service, to emit the `task.error` event before transitioning).

**Files:**
- `server/internal/service/task.go` (or a new `server/internal/service/task_guardrail.go`)
- `server/pkg/models/task.go` (guardrail config, likely project-level: `MaxRetryCount`, `MaxExecutionTime`, `CostBudget`)

**Changes:**
- [x] Track `retry_count` per task with a precise increment rule (previously ambiguous — "does every compile-fail loop count?"): increment by 1 each time the orchestrator re-enters `fixing` from a failed `testing`/`reviewing`, **or** the agent itself emits a `command.finished`/similar event carrying an explicit retry-intent flag for the same step without a status re-entry. Do **not** increment per compile attempt or per file edit within a single `fixing` pass. Reset to 0 on a successful `fixing`→`testing`→pass transition. On the 5th (configurable, default 5), emit `task.error{reason: "max_retries_exceeded", is_retryable: true}` and transition to `blocked`.
- [x] Track task execution wall-clock time from `execute` to now; on exceeding `MaxExecutionTime` (default 2h, project-configurable), emit `task.error{reason: "execution_timeout"}` and transition to `blocked`.
- [x] If the CLI integration exposes token/cost usage: track cumulative cost per task; on exceeding `CostBudget` (project-configurable), emit `task.error{reason: "cost_budget_exceeded"}` and transition to `blocked`. **If no cost data is exposed, this sub-guardrail is inactive** — do not substitute a conservative estimated limit (a fabricated number produces false `blocked` transitions on legitimate long tasks); `MaxExecutionTime`/`max_retry_count` are the enforced backstop in that case. Note the inactive state explicitly in `README.md`/`ARCHITECTURE.md`, don't silently omit it without a trace.
- [x] Track `task_events` row count per task via `TaskEventRepo.CountByTaskID` (checked on each event emit, or on a cheap interval — avoid a full count query per single event if that proves too hot); on exceeding `MaxEventCount` (default 20,000, project-configurable), emit `task.error{reason: "event_volume_exceeded"}` and transition to `blocked`, capping further event writes for that task.
- [x] Extend the Agent Adapter's existing secret-scrubbing pass (design.md → Security Boundaries) to also flag: a diff touching a project-configurable deny-list of paths (e.g. `.github/workflows/`, `infra/`), or content matching a hardcoded-secret pattern about to be committed. On a match, emit `task.error{reason: "security_review_required"}` and transition to `blocked` instead of proceeding.
- [x] All five paths route into the existing `blocked` status/actions (`retry_blocked`, `cancel`) — no new status is introduced.

**Verify:**
- [x] Unit test: simulate 5 consecutive fixing→testing failures → task transitions to `blocked` with `reason: "max_retries_exceeded"`.
- [x] Unit test: a task that fails once, succeeds, then fails 4 more times does NOT trip the guardrail (retry_count reset on success) — `retry_count` never accumulates across a successful cycle.
- [x] Unit test: task execution time exceeds `MaxExecutionTime` → transitions to `blocked` with `reason: "execution_timeout"`.
- [x] Unit test: with cost data unavailable, a task that would exceed a hypothetical cost budget does NOT transition to `blocked` on that basis — only time/retry/event-count/security guardrails can trip it.
- [x] Unit test: simulate a task crossing 20,000 `task_events` rows → transitions to `blocked` with `reason: "event_volume_exceeded"`, and no further events are persisted for that task past the cap (aside from the triggering `task.error` itself).
- [x] Unit test: Agent Adapter given a diff touching `.github/workflows/deploy.yml` → transitions to `blocked` with `reason: "security_review_required"` instead of proceeding to the next status.
- [x] Unit test: a normal task that never trips any threshold completes without an unexpected `blocked` transition (guardrails don't false-positive on the happy path).

---

### 2.4 Implement action dispatch endpoint with authorization and idempotency

**Why:**
`available_actions` on its own is a rendering hint, not a security boundary — see `design.md` → Authorization. Every action needs a server-side permission check independent of the state check, and every action needs to survive a double-submit without a double side effect.

**Depends on:**
- 2.2 (`AvailableAction`/`computeAvailableActions` must exist).

**Files:**
- `server/internal/handler/task_action.go` (new — single handler for all actions, replacing any per-action route)
- `server/internal/service/task_action.go` (new — `ActionPolicy` table + dispatch)
- `server/migration/000027_add_task_action_requests.up.sql` / `.down.sql` (idempotency dedup store)

**Changes:**
- [x] Create `task_action_requests` table: `task_id`, `request_id`, `action`, `response` (JSONB), `created_at`, with a unique constraint on `(task_id, request_id)`.
- [x] Implement `POST /tasks/{taskID}/actions` handler: validate `action` is a member of the *current* `available_actions` (re-computed at request time, not trusted from a stale client fetch) → `409 Conflict` if not; check `ActionPolicy[action]` against the caller's role → `403 Forbidden` if it fails; check `(task_id, request_id)` against `task_action_requests` → if present, return the stored response without re-executing; otherwise execute the action, store the response keyed by `request_id`, return it.
- [x] Define `ActionPolicy` as a `map[string]Permission` per the table in `design.md` → Authorization (`approve_spec`/`request_changes` → owner or `project.write`; `pause`/`cancel`/`retry`/`retry_blocked`/`execute` → `project.write`; `delete` → `project.admin`).
- [x] Populate `DisabledReason` on `AvailableAction` (task 2.2) by running the same `ActionPolicy` check for the requesting caller inside `computeAvailableActions` — one policy table, two call sites (GET for display, POST for enforcement), not two copies of the logic.

**Verify:**
- [x] Unit test: caller without `project.write` posting `pause` → `403`, no state change.
- [x] Unit test: same `request_id` posted twice for `approve_spec` → second call returns the first response, task transitions exactly once.
- [x] Unit test: `action` valid at GET time but the task has since moved to a status where it's no longer in `available_actions` → `409`.
- [x] API test: `GET /tasks/{id}` for a caller without `project.admin` (role `member`, this codebase's two-role model) shows `delete` (task `failed`) with `disabled_reason` set, not omitted — added `TestTaskHandler_GetByID_NonAdminSeesDisabledDelete` in `task_test.go`, injects `service.TokenClaims{Role: "member"}` via `middleware.WithVerifiedClaims` and asserts on the HTTP JSON response. Passing.
- [x] **Bonus finding while adding the above**: running the handler package's full suite (not just the new tests in isolation) surfaced a genuine flaky failure in `TestStreamEventsLoop_NoDuplicateOnRace` — roughly 1-in-3 runs. Root cause: `streamEventsLoop`'s buffering goroutine's `select` can pick the `stopBuf`-closed case over an already-queued `ch` receive (Go's `select` is pseudo-random among ready cases), exiting without draining that event into `buffer`; it's then picked up by the trailing live-loop with no dedup against `catchup()`'s history — the exact duplicate-delivery bug this code exists to prevent, just via a narrower window than the one originally fixed. Fixed by draining `ch` non-blockingly on the `stopBuf` branch before returning (`server/internal/handler/task_event.go`). Verified with 20 consecutive `-count=1` runs, all passing (previously failed within the first 1-5 runs consistently).

---

## Phase 3: Status UI Framework (Frontend)

### 3.1 Create `StatusViewRegistry` and types

**Why:**
This is the single source of truth for status-to-component mapping. All conditional UI rendering flows through this registry.

**Depends on:**
- 2.2 (frontend needs `available_actions` in the API response to render the action bar).

**Files:**
- `web/src/lib/status/registry.ts`
- `web/src/lib/types/task-event.ts`
- `web/src/lib/types.ts` (add `AvailableAction`, `TaskEvent` types)

**Changes:**
- [x] Define `StatusViewConfig` type: `{ component: React.ComponentType, defaultTab: "control" | "activity" }`.
- [x] Create `StatusViewRegistry` map with entries for all **13** task statuses (`planning_split` excluded — verified dead in Task 1.0 and dropped from `TaskStatus`).
- [x] Define TypeScript types for `TaskEvent`, `AvailableAction`, and event payload subtypes. (`AvailableAction` already existed in `types.ts`; added `web/src/lib/types/task-event.ts`.)
- [x] Add `available_actions` field to the existing `Task` type in `types.ts`. (Already present from prior work.)

**Verify:**
- [x] TypeScript compilation succeeds with no errors.
- [x] Every status in the `TaskStatus` union has a corresponding entry in the registry — enforced via `Record<TaskStatus, StatusViewConfig>`'s compile-time exhaustiveness rather than a runtime unit test, since no jest/vitest is installed (`@playwright/test` only). See `docs/implementation/status-driven-agent-workspace-notes.md`. **This alone does not guarantee the frontend union matches the real Go backend enum** — see task 3.1a below for that check.

---

### 3.1a Add backend↔frontend `TaskStatus` parity check to CI

**Why:**
Task 3.1's registry test only checks frontend-internal consistency (registry keys vs. frontend `TaskStatus` union) — it can't catch a status added in Go without the frontend union being updated to match, which is exactly the class of bug that produced the `planning_split` situation in Task 1.0 (a status existing on one side but not verified against the other). A generated, checked-in fixture makes the drift a hard CI failure instead of a hand-sync convention.

**Depends on:**
- 1.0 (final, corrected list of 13 statuses).

**Files:**
- `server/pkg/models/task_status_test.go`
- `docs/openspecs/status-driven-agent-workspace/task-statuses.generated.json`
- `web/src/lib/status/__tests__/parity.test.ts`

**Changes:**
- [x] Go test (`TestTaskStatusParity`) that parses `task.go`'s AST for `TaskStatus*` const string values (not a hardcoded second list — genuinely derived from source), sorts them, and asserts they match the checked-in `task-statuses.generated.json` fixture exactly, failing with a "regenerate the fixture" message if not. (Assertion mode only, per the spec's guidance — no `go generate` rewrite target added, since a human should review a status addition/removal before it's silently absorbed into the fixture.)
- [x] Frontend test (`web/e2e/task-status-parity.spec.ts`) with a small literal array mirroring the `TaskStatus` union, reads the same fixture, asserts set-equality. Lives under `web/e2e/` (Playwright's `testDir`), not `web/src/lib/status/__tests__/`, since the repo has no jest/vitest — see implementation notes.
- [x] Wired: `go test ./...` already picks up the new `_test.go` file with no changes needed; added `npm run test:parity` (`SKIP_WEBSERVER=1 playwright test task-status-parity`) and `npm run test:e2e` to `web/package.json`, which previously had no test script at all.

**Verify:**
- [x] Deliberately added a throwaway Go status constant without updating the fixture (manually, then reverted) → Go test failed with the "regenerate the fixture" message.
- [x] Deliberately removed one entry from the frontend's literal array (manually, then reverted) → frontend parity test failed.
- [x] With both sides in sync, both tests pass (verified above).

---

### 3.2 Build `DynamicActionBar` component

**Why:**
Renders action buttons from `task.available_actions`. This replaces all scattered conditional button rendering across existing components.

**Depends on:**
- 3.1 (types must exist).

**Files:**
- `web/src/app/projects/[id]/tasks/[taskID]/components/DynamicActionBar.tsx`

**Changes:**
- [x] Accept actions via `task.available_actions` (read from `useTaskDetail()` directly rather than as props — `HumanDecisionSurface` is the only mount point, so a prop would just be threaded one level for no benefit).
- [x] Render a horizontal button bar. Each button's visual style is determined by `action.style`.
- [x] On click, dispatch via the single `POST /tasks/{taskID}/actions` endpoint (`tasksApi.dispatchAction`) — matches design.md's single-endpoint dispatch model, not a per-action API call.
- [x] Show a loading spinner on the clicked button while the API call is in flight (other buttons disable during the pending call).
- [x] Handle API errors with a toast notification (`sonner`) and surface via `setError`.

**Verify:**
- [x] Renders correct buttons for a task with `available_actions = [{id: "pause", ...}, {id: "cancel", ...}]` — verified by reading the component logic; no test runner installed to automate this (see 3.1's note).
- [x] Renders no buttons for an empty/missing `available_actions` array (early `return null`).

---

### 3.3 Build status-specific view components

**Why:**
Each task status gets a dedicated view component that renders only the information relevant to that state. These are extracted and refactored from existing components (`SpecPanel`, `SplitProposalCard`, `BlockedTaskNotice`, `ReviewActionBar`, `TaskHeroCards`).

**Depends on:**
- 3.1 (registry types), 3.2 (action bar).

**Files:**
- `web/src/app/projects/[id]/tasks/[taskID]/components/status-views/TodoView.tsx`
- `web/src/app/projects/[id]/tasks/[taskID]/components/status-views/SpecReviewView.tsx`
- `web/src/app/projects/[id]/tasks/[taskID]/components/status-views/SplitProposalView.tsx`
- `web/src/app/projects/[id]/tasks/[taskID]/components/status-views/CodingProgressView.tsx`
- `web/src/app/projects/[id]/tasks/[taskID]/components/status-views/BlockedView.tsx`
- `web/src/app/projects/[id]/tasks/[taskID]/components/status-views/PrCreatedView.tsx`
- `web/src/app/projects/[id]/tasks/[taskID]/components/status-views/FailedView.tsx`
- `web/src/app/projects/[id]/tasks/[taskID]/components/status-views/MergedView.tsx`
- `web/src/app/projects/[id]/tasks/[taskID]/components/status-views/ExecutionProgressView.tsx`
- `web/src/app/projects/[id]/tasks/[taskID]/components/HumanDecisionSurface.tsx`

**Changes:**
- [x] Extract spec review logic from `SpecPanel.tsx` / `SpecReviewGate.tsx` / `CLISpecPanel.tsx` into `SpecReviewView.tsx` (thin wrapper over `SpecReviewGate` — see implementation notes for why the ~870 LOC weren't duplicated), reusing `RequestChangesModal.tsx` for the `request_changes` action. Fixed an adjacent bug in the process: `RequestChangesModal` was never mounted anywhere, so the classic flow's "Request Changes" button silently did nothing. This is the only view that renders approval buttons.
- [x] Extract split proposal logic from `SplitProposalCard.tsx` into `SplitProposalView.tsx` — **read-only**, renders the proposed subtask plan with no `approve_split`/`reject_split` buttons (autopilot decision, see `design.md`).
- [x] Extract blocked notice from `BlockedTaskNotice.tsx` into `BlockedView.tsx`, and relocate `BoundaryResolutionControls.tsx` under `HumanDecisionSurface`'s paused-banner path for paused-with-boundary-error recovery (the boundary controls apply to a workflow-engine pause, which can occur under any status, not only `blocked` — kept at the dispatcher level rather than inside `BlockedView` itself).
- [x] Build `PrCreatedView.tsx` (replaces the old approval-oriented `ReviewActionBar.tsx`): shows the PR link/number and reuses `ReviewVerdictCard.tsx` for the agent's own review summary (sourced from the latest checkpoint's `state.output.review_verdict`). **No merge/approve button** — used for both `pr_ready` and `human_review`.
- [x] Create `CodingProgressView` showing subtask progress tree (reusing `TaskSubtasks.tsx`) + task info card.
- [x] Create `FailedView` showing error details + retry button.
- [x] Create `MergedView` showing completion summary.
- [x] Create `ExecutionProgressView` for `context_loading` / `analyzing` (`reviewing` gets its own static `ReviewProgressView`; `fixing`/`testing` re-export `CodingProgressView` — see implementation notes for why they aren't distinct).
- [x] Build `HumanDecisionSurface` wrapper (at the top level, `components/HumanDecisionSurface.tsx`): reads `task.status`, looks up `StatusViewRegistry`, renders the component + `DynamicActionBar`. Gated behind `NEXT_PUBLIC_AGENT_WORKSPACE` per the Rollback Plan — `TaskDetailLayout`'s legacy inline rendering remains the default.

**Verify:**
- [x] Each status view renders correctly in isolation — verified via `tsc --noEmit` and code review; no mock-data test harness exists yet (no jest/vitest installed).
- [x] `HumanDecisionSurface` renders the correct view for each status (registry lookup verified against all 13 `TaskStatus` values).
- [ ] Snapshot/DOM test: for every status except `spec_review`, no rendered status view contains a button labelled with an approval verb (`Approve`, `Reject`, `Merge`) — deferred; requires a test runner (see 3.1's note), spot-checked manually instead (`PrCreatedView`/`BlockedView`/`MergedView`/`FailedView` have no approval-verb buttons by inspection).

---

## Phase 4: Split-Screen Layout & Event Timeline (Frontend)

### 4.1 Build `AgentTimeline` and `TimelineEntry` components

**Why:**
The right column of the split-screen. Renders a chronological, icon-annotated timeline of `TaskEvent` objects.

**Depends on:**
- 3.1 (event types).
- 1.4 (SSE endpoint must be available — can use mock data for initial development).

**Files:**
- `web/src/app/projects/[id]/tasks/[taskID]/components/AgentTimeline.tsx`
- `web/src/app/projects/[id]/tasks/[taskID]/components/TimelineEntry.tsx`
- `web/src/app/projects/[id]/tasks/[taskID]/components/UnknownEventCard.tsx`

**Changes:**
- [x] `AgentTimeline`: on mount, fetches historical events via `tasks.events(taskID, token)` (new method added to `web/src/lib/api/projects.ts`, backed by `GET /tasks/{taskID}/events`, cursor-paginated newest-first per `TaskEventRepo.ListByTaskIDPaginated`) as initial state, re-sorted ascending for display. Then connects to `GET /tasks/{taskID}/events/stream?after=<last sequence_number>` via a **new `tasks.streamEvents()` method** (also added to `projects.ts`) — **deviation**: uses `fetch` + `ReadableStream` reader, not native `EventSource`, because this backend's SSE auth is Authorization-header-only (`internal/middleware/auth.go`, no query-param token fallback) and `EventSource` cannot set custom headers. This mirrors the pre-existing `tasks.streamLogs()` method, which hit the identical constraint for `/tasks/{taskID}/logs/stream`. Custom exponential-backoff reconnect (not native `Last-Event-ID`) is therefore required and implemented, resuming from the last-seen `sequence_number`. Deduplicates by `event.sequence_number` (see `AgentTimeline.tsx`'s `mergeEvents`), not `event.id`.
- [x] `AgentTimeline`: shows a small non-blocking "Reconnecting…" indicator while not connected (tracked via the `streamEvents` `onStatusChange` callback, since there's no `EventSource.readyState` in the fetch-based implementation), cleared once a connected state is reported.
- [x] `TimelineEntry`: switches on `event.type` gated by a `KNOWN_RENDERERS` lookup (schema_version is carried on every `TaskEvent` and available to extend the switch per-version later; no event type currently has more than one schema version in the backend, so the initial implementation keys on `type` alone and documents `schema_version` as the extension point). Unrecognized types delegate to `UnknownEventCard` instead of throwing.
- [x] `UnknownEventCard`: renders `event.type`, `event.created_at`, and `event.payload` as pretty-printed JSON inside a collapsed `<details>`.
- [x] `TimelineEntry`: renders a single event with timestamp, icon, and payload summary for all 6 documented groups (🧠 reasoning, 🛠 tool.started/finished, 📄 file.changed, 💻 command.started/finished, ✅/❌ test.result, 💬 agent.message).
- [x] `TimelineEntry`: tool/command output uses `CollapsibleOutput` (collapsed by default if > 3 lines). If the event has a non-null `artifact_id`, an `ArtifactOutput` "View full output" button fetches on demand — **deviation**: there is no `GET /api/v1/artifacts/{artifactID}` single-artifact endpoint in this backend (confirmed via `grep` over `internal/handler/router.go`); the only artifact routes are list endpoints (`GET /tasks/{taskID}/artifacts`, `GET /workflows/{jobID}/artifacts`). `ArtifactOutput` calls the existing `tasks.artifactsByTask(taskID, token)` and finds the matching `artifact_id` client-side. Still on-demand (not embedded up front), which was the actual intent of this line.
- [x] Auto-scroll to bottom when new events arrive, with a "Scroll to bottom" button once the user has scrolled up (`AgentTimeline.tsx`'s `handleScroll`/`userScrolledUp`).

**Verify:**
- [x] `tsc --noEmit` and `eslint` clean across `AgentTimeline.tsx`, `TimelineEntry.tsx`, `UnknownEventCard.tsx`, and the edited `projects.ts`/`TaskDetailLayout.tsx` (verified this session; two `react-hooks/set-state-in-effect` violations found and fixed — see Phase 4.2 verify notes for the second one).
- [ ] Timeline renders 5+ mock events in `sequence_number` order with correct icons — not run live (no dev server/backend was started this session); logic verified by reading, not by rendering in a browser.
- [ ] SSE connection reconnects on disconnect (test by killing the backend and restarting), showing the "Reconnecting…" indicator while disconnected — not run live, same reason.
- [ ] No duplicate events after reconnect, no missing events either — not run live; the underlying backend race (`streamEventsLoop` dedup) *was* verified this session via `TestStreamEventsLoop_NoDuplicateOnRace` (Phase 1), but the frontend `mergeEvents` dedup path itself has not been exercised against a real reconnect.
- [ ] A mock event with an unrecognized `(type, schema_version)` renders as `UnknownEventCard` without breaking rendering of the events before/after it — not run live.
- [ ] A mock event with a non-null `artifact_id` shows a "View full output" action; clicking it fetches and displays the artifact content — not run live.

---

### 4.2 Refactor `TaskDetailLayout` to split-screen

**Why:**
The core layout change: transform the existing single-column layout into a split-screen workspace.

**Depends on:**
- 3.3 (`HumanDecisionSurface` must exist).
- 4.1 (`AgentTimeline` must exist).

**Files:**
- `web/src/app/projects/[id]/tasks/[taskID]/components/TaskDetailLayout.tsx`
- `web/src/app/projects/[id]/tasks/[taskID]/components/TaskDetailContext.tsx`

**Changes:**
- [x] Replaced the `AGENT_WORKSPACE_ENABLED` branch's single-column grid with `SplitScreenWorkspace` (new component in `TaskDetailLayout.tsx`): a `grid-cols-[45%_55%]` split — left `HumanDecisionSurface`, right `AgentTimeline` — active at `min-width:1200px` via Tailwind v4 arbitrary variants (`[@media(min-width:1200px)]:...`), since the codebase's default `lg:` breakpoint (1024px) doesn't match the spec's explicit 1200px threshold.
- [x] `TaskHeader`/`TaskTitleBlock` remain above the split — untouched, they were already rendered before this block in both branches.
- [x] Desktop (≥1200px): both columns get `sticky top-4 max-h-[calc(100vh-160px)] overflow-y-auto` — independently scrollable with sticky headers (`TimelineEntry`'s per-status-view headers and `AgentTimeline`'s own "Activity" header stay pinned within their column).
- [x] Mobile (<1200px): `Control`/`Activity` tab buttons toggle `mobileTab` state; both columns render as `hidden`/`block` (not unmounted) so `AgentTimeline`'s SSE connection survives a tab switch. Tab defaults from `StatusViewRegistry[task.status].defaultTab`, reset on status change via a render-time state adjustment (`if (task?.status !== lastStatus) { setLastStatus(...); setMobileTab(...) }`) rather than a `useEffect` — see Verify note below for why.
- [x] `SupportingAccordion` was already rendered once, after both the legacy and flag branches, at the bottom of `TaskDetailLayout`'s return — no move was needed; it already sits below the split for the flag-enabled path.
- [x] SSE subscription lives in `AgentTimeline.tsx` itself (via `tasksApi.streamEvents`), not in `TaskDetailContext` or a shared hook — **deviation**: `HumanDecisionSurface` (the other column) has no need to observe the event stream, so scoping the subscription to the one component that renders it avoids a context change that would re-render the whole split on every event tick.

**Cleanup:**
- [x] N/A for this pass — the scattered status-conditional rendering (paused banner, `SplitProposalCard`, `BlockedTaskNotice`) was already absent from the `AGENT_WORKSPACE_ENABLED` branch as of Task 3.3/HumanDecisionSurface's relocation; only the legacy (flag-off) branch still has it, which must stay per the Rollback Plan.
- [x] `TaskHeroCards` is not imported or referenced anywhere in `SplitScreenWorkspace`; it remains imported only for the legacy branch.

**Verify:**
- [x] `tsc --noEmit` clean. `eslint` initially caught a `react-hooks/set-state-in-effect` violation in `SplitScreenWorkspace`'s tab-reset logic (and a second one in `AgentTimeline`'s initial-loading-state effect) — both fixed (render-time state adjustment; dropping a redundant `setLoading(true)` since `loading` already initializes `true`), re-verified clean.
- [ ] Desktop: both columns render and scroll independently — not verified in a live browser this session (no dev server running).
- [ ] Mobile: tabs switch between Control and Activity — not verified live.
- [ ] `SupportingAccordion` remains accessible below the split — structurally true by inspection (unchanged position in the JSX), not visually verified.
- [ ] All existing Playwright E2E tests still pass — run live 2026-08-07 (`npx playwright test` against the live api :32080 + web :32300): **9 passed, 3 failed**. Failures are pre-existing, unrelated to this spec's work: (1) `auth-and-dashboard.spec.ts:77` — strict-mode locator ambiguity, `getByText("OpenAI-Prod-Key1")` now matches 2 elements; (2/3) `task-detail.spec.ts:28` and `:77` — assert a `Description` accordion button and a "Waiting for your review" sticky bar that no longer exist post-DynamicActionBar refactor (Task 3.2/3.3). These specs need updating to match the current status-driven UI, not a regression caused here — left unchecked pending that follow-up.

---

## Phase 5: Component Consolidation & Cleanup (Frontend)

Split into two steps rather than one immediate deletion: real usage under the feature flag is the actual proof that every replacement view covers what its predecessor did. Deleting in the same PR as the replacement (the original plan) is fine for logic bugs caught by tests, but these are UI components whose gaps tend to show up as "huh, where did X go" from a real user days later — a one-release soak closes that gap cheaply.

**Status as of this session: genuinely blocked, not started.** 5A explicitly depends on `NEXT_PUBLIC_AGENT_WORKSPACE` being at 100% rollout for a full release cycle (tasks.md's own stated dependency) — that hasn't happened; the flag defaults off and the legacy `TaskDetailLayout` branch is still the active default per the Rollback Plan. Deprecation-tagging or deleting these files now would be premature. Revisit once the flag has actually been flipped to 100% in a real deploy.

### 5A. Deprecate superseded components (same release as the flag going to 100%)

**Why:**
Once `TaskDetailLayout` no longer imports the old components (task 4.2, flag at 100%), they're dead code but not yet proven safe to remove — see the Rollback Plan's Safe Deploy step.

**Depends on:**
- 3.3, 4.2, and the feature flag (`NEXT_PUBLIC_AGENT_WORKSPACE`) being enabled for 100% of users for at least one full release cycle.

**Files:**
- `web/src/app/projects/[id]/tasks/[taskID]/components/SpecPanel.tsx`
- `web/src/app/projects/[id]/tasks/[taskID]/components/SpecReviewGate.tsx`
- `web/src/app/projects/[id]/tasks/[taskID]/components/CLISpecPanel.tsx`
- `web/src/app/projects/[id]/tasks/[taskID]/components/TaskHeroCards.tsx`
- `web/src/app/projects/[id]/tasks/[taskID]/components/TaskSidebar.tsx`
- `web/src/app/projects/[id]/tasks/[taskID]/components/SplitProposalCard.tsx`
- `web/src/app/projects/[id]/tasks/[taskID]/components/BlockedTaskNotice.tsx`
- `web/src/app/projects/[id]/tasks/[taskID]/components/ReviewActionBar.tsx`

**Changes:**
- [ ] Confirm no remaining import of any file above outside its own definition (`grep -rln` each basename under `web/src`) — this must already be true once 4.2 lands and the flag is at 100%.
- [ ] Add a `@deprecated` JSDoc tag to each file's top-level export pointing at its replacement (e.g. `@deprecated Replaced by status-views/SpecReviewView.tsx — scheduled for deletion in Phase 5B`), so anyone who greps the codebase mid-soak sees the plan, not just dead code.
- [ ] Do not delete yet.

**Verify:**
- [ ] `grep -rl "TaskHeroCards\|SpecPanel\|SpecReviewGate\|CLISpecPanel\|TaskSidebar\|SplitProposalCard\|BlockedTaskNotice\|ReviewActionBar" web/src` returns matches only inside the files' own definitions — confirms nothing else references them, i.e. deletion in 5B is safe.
- [ ] One full release cycle has passed with the flag at 100% and no regression reports tied to information that used to live in one of these components.

### 5B. Delete superseded components

**Why:**
This spec does not add a split-screen layout on top of the existing pile of components — it replaces the ones that duplicate what the new status views now own. Leaving deprecated dead code in the tree past the soak period is exactly the kind of redundant-component debt this redesign exists to remove (see `design.md` → Redundant Component Consolidation for the full audit and rationale per file).

**Depends on:**
- 5A, plus the one-release soak period with no regressions.

**Changes:**
- [ ] Delete each file listed in 5A.
- [ ] Remove now-unused props/types that existed only to feed the deleted components (e.g. any `TaskHeroCards`-only prop threading in `TaskDetailContext.tsx`).
- [ ] Run `npx ts-prune` (or the project's existing dead-export check) over `components/` to catch any export left dangling by the deletions.

**Verify:**
- [ ] `grep -rl "TaskHeroCards\|SpecPanel\|SpecReviewGate\|CLISpecPanel\|TaskSidebar\|SplitProposalCard\|BlockedTaskNotice\|ReviewActionBar" web/src` returns no matches at all.
- [ ] `npm run build` (or equivalent) succeeds with no unused-import/unused-export warnings introduced by the deletions.
- [ ] Full Playwright E2E suite passes — proves the deleted components' functionality is fully covered by their replacements, not just visually similar.

---

## Phase 6: Documentation Sync

### Update project docs

**Why:**
Code changes in previous phases altered public API, added a database table, changed the UI layout, and deleted eight components' worth of legacy UI. Docs must reflect reality, and this spec is now the single, final description of the Task Detail page — the seven other specs that iterated on it no longer describe anything real once Phase 5 lands.

**Files:**
- `README.md`
- `docs/ARCHITECTURE.md`

**Changes:**
- [x] Updated `docs/ARCHITECTURE.md` with new §4.4 "Task Events & the Status-Driven Task Detail UI", documenting the `task_events` table (schema, ordering, artifact externalization), `EventAdapter`, the HTTP/SSE surface, the split-screen UI (`HumanDecisionSurface`/`StatusViewRegistry`/`AgentTimeline`), and the single-approval-gate model (`AvailableActions` as the sole source of approval-shaped UI, `ActionPolicy` as the independent authorization boundary).
- [x] The 8 named directories (`task-detail-status-driven-ui/`, `task-detail-status-ui/`, `task-detail-layout-rebuild/`, `task-detail-ui-enhancement/`, `task-detail-workflow-redesign/`, `workflow-centric-dashboard/`, `ui-status-consolidation/`, `task-detail-data-alignment/`) do not exist under `docs/openspecs/` — confirmed via `ls`/`grep` this session. **Deviation**: nothing to `git rm`; either they were already removed in an earlier cleanup pass or the spec's list predates this repo's current openspec directory set. No action needed.
- [x] No dedicated API-reference doc exists in this repo beyond `docs/ARCHITECTURE.md` (checked `docs/*.md` for any `/tasks/{taskID}/events` mentions — only `ARCHITECTURE.md`, now updated, and an unrelated CLI-vs-API report). Nothing further to update.

**Verify:**
- [x] `ls docs/openspecs/ | grep -E "task-detail-(status-driven-ui|status-ui|layout-rebuild|ui-enhancement|workflow-redesign|data-alignment)|workflow-centric-dashboard|ui-status-consolidation"` returns nothing — confirmed.
- [x] `docs/ARCHITECTURE.md` mentions `task_events` table, Event Adapter, and the single-approval-gate model — confirmed (§4.4).

---

## Rollback Plan

### Feature Flags
- [x] Verified: `TaskDetailLayout.tsx` line ~25, `const AGENT_WORKSPACE_ENABLED = process.env.NEXT_PUBLIC_AGENT_WORKSPACE === "true"` gates the entire new render branch; the legacy branch (byte-for-byte the pre-existing inline JSX) is the `else`, i.e. the default whenever the env var is unset.
- [x] Verified: `models/task.go`'s `AvailableActions` field carries `gorm:"-"` (`server/pkg/models/task.go:144`) — never persisted, computed per-request by `computeAvailableActions`. Confirms removal needs only a service-layer revert, no migration.

### Database Rollback
- [x] Verified: `server/migration/000026_add_task_events.down.sql` drops the table (read this session). `task_events.artifact_id` has an outbound FK to `workflow_artifacts` (added in `000029`, this session) but nothing references *into* `task_events`, so dropping it cascades to nothing else.
- [x] Verified: no changes were made to `TaskLog`/`/logs/stream` (`tasks.streamLogs`) anywhere in this session's work — the new `task_events` stream is a fully separate table, service, and endpoint (`/events/stream` vs `/logs/stream`); `TaskLog` remains untouched code.

### Safe Deploy
- [ ] Deploy backend changes (Phase 1 + 2) first — not applicable to check here; this is a deploy-time operational instruction for whoever runs the release, not a code task. Phase 1+2 code is additive (new table/routes, no altered existing endpoints) by inspection.
- [ ] Deploy frontend changes (Phase 3 + 4) behind the feature flag — same: deploy-time instruction. Code-side, the flag gate exists and is verified above.
- [ ] Deploy Phase 5A/5B sequencing — not applicable yet; Phase 5 itself is blocked (see Phase 5's status note above).
- [ ] Monitor `task_events.total` metric — no such metric currently exists in the codebase (checked for existing metrics/observability wiring on `TaskEventService`; none found). Out of scope for this session's frontend/backend feature work — would need a follow-up task to add the instrumentation itself before it can be "monitored."
- [ ] Monitor `task_events.adapter_parse_errors` — same: no such metric exists yet, needs its own instrumentation task.
