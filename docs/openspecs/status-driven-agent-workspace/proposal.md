# Proposal: Status-Driven Agent Workspace

## Problem

The current Task Detail page has two fundamental limitations:

1. **No live agent visibility.** When a CLI Agent (or API-native agent) is executing a task, the user cannot see what the agent is thinking, planning, or doing in real time. The only feedback is a raw log dump inside a collapsed accordion (`SupportingAccordion.tsx`), which is not structured, not filterable, and not human-friendly.

2. **Static, component-centric UI.** The page renders the same set of panels regardless of the task's lifecycle state. Action buttons (Approve, Retry, Pause) are conditionally sprinkled via `if/else` blocks across `TaskHeroCards`, `TaskHeader`, `TaskDetailLayout`, `SplitProposalCard`, and `BlockedTaskNotice`. Adding a new status (e.g. `security_review`, `deploy_review`) requires touching multiple files — a violation of the Open/Closed principle.

The result is a page that feels like a **dashboard showing data** rather than an **agentic workspace where a human collaborates with an AI**.

## Goal

Transform the Task Detail page into a **Split-Screen Agent Workspace** with two distinct planes, and reduce the human-in-the-loop surface to a single gate:

- **Left: Human Decision Surface** — a state-machine-driven control panel that renders exactly one view per `task.status`, with a dynamic Action Bar whose buttons are declared by the backend (not hardcoded in the frontend). **`spec_review` is the only status with approval-style actions** (`approve_spec` / `request_changes`). Every other status offers only operational controls (`pause`, `cancel`, `retry`, `delete`) — never a "please approve to continue" gate.
- **Right: Agent Execution Stream** — a structured, time-stamped event timeline (not a raw log dump) that streams normalized `AgentEvent` objects from the backend via SSE, persisted in a `task_events` table for replay and post-mortem. This is how the user watches decomposition, coding, review, fixing, testing, and PR creation happen — without being asked to click anything.

## Product decision: one human gate, then full autopilot

Once a task's spec is approved (`approve_spec`), the task must run to `pr_ready` (PR opened) without ever pausing to ask a human to approve a step:

- **Decomposition is agent-decided, not human-gated.** The agent may split a task into subtasks or keep it as one; either way execution proceeds automatically. The split plan is shown as an `agent.plan` timeline event and a conditional sub-view of `CodingProgressView` (keyed off `proposed_split` data, not a dedicated status — `planning_split` was verified dead and dropped, see design.md), not an approval prompt.
- **Review, fixing, testing loops are agent-decided.** The agent runs `reviewing` → `fixing` → `testing` cycles on its own; the human only observes progress in the Agent Execution Stream.
- **PR creation is automatic; merge is out of scope.** The agent opens a PR once tests pass (`pr_ready`). The agent does **not** merge it — merging is left to the repository's own CI / branch protection / a human reviewing on GitHub, entirely outside this app. `pr_ready` and `human_review` are therefore informational progress states in this app, not action-gated states.
- **Recovery actions remain available but are not "approvals."** `pause`/`cancel` are always-available operational controls, and `blocked`/`failed` still expose `retry`/`delete` — these resume or abandon a stuck run, they don't gate forward progress on a human decision.
- **Autopilot is bounded, not open-ended.** Removing per-step approval only works if the system has its own judgment for when to stop. Four guardrails (`max_retry_count`, `max_execution_time`, `cost_budget`, `security_block`) auto-transition a task to `blocked` on concrete evidence of a problem — a human is pulled in when something is actually wrong, not on a fixed schedule of checkpoints. See `design.md` for specifics.
- **Actions are authorized and idempotent.** `available_actions` says what's valid for the task's *state*; a separate server-side `ActionPolicy` check says what's valid for the *caller* (owner/`project.write`/`project.admin`, action-dependent). Every action dispatch carries a client-generated `request_id` so a double-click or retried request never produces a duplicate side effect.

## Success

- A user approves a spec once (`approve_spec`); from that point the task runs to `pr_ready` unattended, and the user can watch every step (decomposition decision, coding, review/fix/test loops, PR creation) live in the Agent Execution Stream.
- No status other than `spec_review` ever shows an approval-style button (`approve_*` / `reject_*` / `request_changes`) in the Action Bar.
- A user watching a task in `coding` status sees a live IDE-like timeline (timestamps, icons, tool calls, file diffs, test results) in the right column — without switching tabs or opening accordions.
- A user arriving at a `spec_review` task sees only the Spec Review panel and Approve/Request Changes buttons — nothing else competes for attention.
- A developer adding a new status (e.g. `migration_review`) only touches a single registry file to map it to a view component and action set — no layout/header/hero file changes needed.
- This document is the **single, final spec** for the Task Detail page. The eight prior, fully-implemented specs that iterated on this same page (`task-detail-status-driven-ui`, `task-detail-status-ui`, `task-detail-layout-rebuild`, `task-detail-ui-enhancement`, `task-detail-workflow-redesign`, `workflow-centric-dashboard`, `ui-status-consolidation`, `task-detail-data-alignment`) are retired; their outcomes are folded into this one.

## Assumptions

- The existing `GET /tasks/{taskID}/logs/stream` SSE endpoint in `WorkflowHandler.StreamLogs` will be superseded by a new event-based endpoint, but kept operational during the migration behind the same route.
- The existing `TaskLog` model (unstructured `message` string) will coexist with the new `TaskEvent` model; `TaskLog` is not deleted, only deprecated for new UI consumption.
- The CLI sandbox container's stdout/stderr remains the raw data source; normalization into `AgentEvent` happens inside a new Go adapter layer in the backend, not in the frontend.
- The existing `ValidTaskTransitions` state machine in `server/pkg/models/task.go` is the source of truth for which statuses exist.

## Decisions

1. **Backend-driven Action Bar.** The backend will compute and return `available_actions` on the Task response object, derived from `task.status`, `workflow.job.status`, and (future) user permissions/roles. The frontend renders buttons from this list — zero hardcoded `if(status === "coding") show Pause`. This makes future permission/enterprise gates trivial.

2. **Event Normalization in Backend, not Frontend.** Raw CLI output (stdout, stderr, JSON stream) is adapted into a typed `AgentEvent` schema by a Go adapter before being persisted and broadcast. The frontend never parses raw CLI text. This decouples the UI from any specific CLI tool (Claude CLI, Codex CLI, Gemini CLI, API-native).

3. **Structured Timeline, not Terminal.** The right column renders events as an IDE-like vertical timeline with icons (🧠 Planning, 🛠 Terminal, 📄 File Changed, ✅ Result), not as a raw terminal dump. Users ask "what happened?", not "show me stdout."

4. **Events persisted to DB before broadcast.** `Agent → task_events table → SSE broadcast → UI`. Page refresh loads history from DB. This enables replay, debugging, analytics, and future training data extraction.

5. **Status Registry pattern.** A single TypeScript registry object (`StatusViewRegistry`) maps each `task.status` string to `{ component: React.FC, actions: ActionID[] }`. All status-dependent rendering flows through this registry. Adding a status is a one-line addition.

6. **Execution guardrails, not open-ended autopilot.** A guardrail check runs before/around each agent step and auto-transitions the task to the existing `blocked` status (no new status) when `max_retry_count` (5), `max_execution_time` (2h), `cost_budget` (only if the CLI adapter exposes cost/token data — not fabricated), or `security_block` (secret/destructive-command pattern match) trips. `blocked` already has `retry`/`delete` actions, so this reuses the existing recovery path instead of inventing a new one.

7. **Single action-dispatch endpoint, not one route per action.** `POST /api/v1/tasks/{taskID}/actions` with `{ "action": "<id>", "request_id": "<uuid>" }` replaces the old pattern of a bespoke endpoint per action. The server validates the action against `available_actions` (state) and `ActionPolicy` (caller permission), returning 409 for a stale/invalid action and 403 for a permission failure.

8. **`request_id` idempotency.** Each dispatched action carries a client-generated `request_id`, stored in a new `task_action_requests` table under a unique `(task_id, request_id)` constraint. A retried or double-clicked request with the same `request_id` returns the original result instead of re-executing the action.

9. **Event schema versioning.** `TaskEvent` carries a `schema_version` field, versioned per event `type` (not globally), so the payload shape for `agent.tool_call` can evolve independently of `agent.reasoning_summary` without breaking older stored events or in-flight frontend clients.

10. **`sequence_number` is the ordering/cursor key, `created_at` is not.** A per-task monotonic `sequence_number` (bigint) replaces timestamps for replay order, the SSE `id:` field, and `Last-Event-ID`/`?after=` cursors — two events can share (or nearly share) a `created_at` when written by concurrent goroutines, but never a `sequence_number`.

11. **Large event output is externalized, not truncated-and-lost; event volume is capped in v1.** Content that would need to exceed the 8KB payload cap to be useful in full (full test logs, full diffs) is written to the existing `workflow_artifacts` table with a summary + `artifact_id` left in the event, rather than truncated with no way to see the rest. Separately, a `max_event_count` guardrail (20,000/task) auto-blocks a task whose *event count* (not just payload size) runs away — both this and cursor-based pagination on `GET /events` ship in v1, not deferred to Phase 5.

## Trade-offs

| Gain | Cost |
|------|------|
| Clean separation: human-gate vs agent-stream | High initial effort (~4 engineering phases) |
| Backend-driven actions enable future RBAC/enterprise | Adds a new field to the Task API response |
| Persisted events enable replay/analytics | New `task_events` table, new migration, increased DB writes |
| CLI-agnostic event format | Must build and maintain an Agent Adapter per CLI provider |
| Timeline UI is richer than raw logs | More complex frontend rendering components |
| Autopilot doesn't stall on missing human input | Requires guardrail logic (retry/time/cost/security checks) to be correct — a bug here means either a false `blocked` or a runaway task |
| Actions can't be replayed/duplicated by accident | New `task_action_requests` table, one more write per action dispatch |

## Non-goals

- **Mobile-first design.** The agent workspace is designed for 1200px+ screens. Mobile falls back to a tabbed layout (Control | Activity), not a split screen.
- **Full terminal emulator.** The right column is a structured timeline. A raw log tab is available as a fallback, but the primary UX is event-based.
- **Real-time collaborative editing.** Only one user interacts with the Decision Surface at a time. Multi-user presence is out of scope.

## Out of Scope

- Cost/token visualization per event (Phase 5 future work). Note: the `cost_budget` guardrail (Decision 6) only *enforces* a cost ceiling when the CLI adapter already exposes cost data — it does not add cost telemetry or a cost UI.
- Event search, filtering, and collapse UI (Phase 5 future work).
- Historical event replay with playback controls (Phase 5 future work).
- Changes to the project dashboard or task list page.
- Automated retry/resume logic in the backend orchestrator.
- Merging the PR — the agent opens it and stops; merge is entirely external to this app (see Product decision above).

## Impact

### Components

| Layer | Component | Change Type |
|-------|-----------|-------------|
| Backend (Models) | `TaskEvent` model | **New** |
| Backend (Models) | `Task` response — `available_actions` field | **Modified** |
| Backend (Repository) | `task_events` repository | **New** |
| Backend (Service) | Event persistence + SSE broadcast service | **New** |
| Backend (Adapter) | CLI-to-AgentEvent normalizer | **New** |
| Backend (Handler) | `EventStreamHandler` (new SSE endpoint) | **New** |
| Backend (Handler) | `WorkflowHandler.StreamLogs` | **Deprecated** (kept for backward compat) |
| Backend (Service) | Execution guardrail checker (retry/time/cost/security) | **New** |
| Backend (Service) | `ActionPolicy` authorization + action dispatch service | **New** |
| Backend (Handler) | `TaskActionHandler` (`POST .../actions`) | **New** |
| Backend (Repository) | `task_action_requests` repository (idempotency) | **New** |
| Frontend (Registry) | `StatusViewRegistry` | **New** |
| Frontend (Layout) | `TaskDetailLayout` → Split-Screen | **Modified** |
| Frontend (Components) | `AgentTimeline`, `TimelineEntry` | **New** |
| Frontend (Components) | `HumanDecisionSurface`, `DynamicActionBar` | **New** |
| Frontend (Components) | Status-specific views (`SpecReviewView`, `CodingProgressView`, etc.) | **New** (refactored from existing) |

### Files

**New Files:**
- `server/pkg/models/task_event.go`
- `server/internal/repository/task_event.go`
- `server/internal/service/task_event.go`
- `server/internal/handler/task_event.go`
- `server/internal/sandbox/event_adapter.go`
- `server/internal/service/guardrail.go`
- `server/internal/service/task_action.go`
- `server/internal/handler/task_action.go`
- `server/internal/repository/task_action_request.go`
- `server/migration/000026_add_task_events.up.sql`
- `server/migration/000026_add_task_events.down.sql`
- `server/migration/000027_add_task_action_requests.up.sql`
- `server/migration/000027_add_task_action_requests.down.sql`
- `web/src/lib/status/registry.ts`
- `web/src/app/projects/[id]/tasks/[taskID]/components/AgentTimeline.tsx`
- `web/src/app/projects/[id]/tasks/[taskID]/components/TimelineEntry.tsx`
- `web/src/app/projects/[id]/tasks/[taskID]/components/HumanDecisionSurface.tsx`
- `web/src/app/projects/[id]/tasks/[taskID]/components/DynamicActionBar.tsx`
- `web/src/app/projects/[id]/tasks/[taskID]/components/status-views/SpecReviewView.tsx`
- `web/src/app/projects/[id]/tasks/[taskID]/components/status-views/CodingProgressView.tsx`
- `web/src/app/projects/[id]/tasks/[taskID]/components/status-views/BlockedView.tsx`
- `web/src/app/projects/[id]/tasks/[taskID]/components/status-views/SplitProposalView.tsx` (conditional sub-view of `CodingProgressView`, not a top-level registry entry — `planning_split` was verified dead and dropped)
- `web/src/app/projects/[id]/tasks/[taskID]/components/status-views/PrCreatedView.tsx` (covers `pr_ready` and `human_review`; renamed from the earlier `HumanReviewView` concept since neither status requires human action)
- `web/src/lib/types/task-event.ts`
- `server/pkg/models/task_status_test.go` (generates the backend↔frontend status parity fixture)
- `docs/openspecs/status-driven-agent-workspace/task-statuses.generated.json` (checked-in parity fixture)
- `web/src/lib/status/__tests__/parity.test.ts` (asserts frontend `TaskStatus` matches the fixture)
- `web/src/app/projects/[id]/tasks/[taskID]/components/UnknownEventCard.tsx` (fallback renderer for an unrecognized `schema_version`)

**Modified Files:**
- `server/pkg/models/task.go` (add `AvailableActions` to Task response)
- `server/internal/handler/router.go` (add event stream route)
- `server/internal/handler/services.go` (wire new service)
- `server/internal/service/task.go` (compute `available_actions`)
- `web/src/app/projects/[id]/tasks/[taskID]/components/TaskDetailLayout.tsx` (split-screen layout)
- `web/src/app/projects/[id]/tasks/[taskID]/components/TaskDetailContext.tsx` (add event stream hook)
- `web/src/lib/types.ts` (add `TaskEvent`, `AvailableAction` types)
- `web/src/lib/api/projects.ts` (add event stream API, `available_actions`)

### Public API

**New Endpoints:**
- `GET /api/v1/tasks/{taskID}/events/stream` — SSE stream of `TaskEvent` objects. Supports `Last-Event-ID` header / `?after=<event_id>` for cursor-based reconnect.
- `GET /api/v1/tasks/{taskID}/events` — Paginated historical events from DB.
- `POST /api/v1/tasks/{taskID}/actions` — Single action-dispatch endpoint. Body: `{ "action": "<id>", "request_id": "<uuid>" }`. Replaces the old per-action-endpoint pattern (e.g. what would have been `POST /tasks/{taskID}/pause`); the server resolves `action` against `available_actions` + `ActionPolicy` instead of routing by URL. Returns 409 if `action` isn't currently valid for the task's status, 403 if the caller lacks permission, and 200 with the original result if `request_id` was already processed (idempotent replay).

**Modified Response:**
- `GET /api/v1/tasks/{taskID}` — Task object gains an `available_actions` array:
  ```json
  {
    "available_actions": [
      {
        "id": "pause",
        "label": "Pause",
        "style": "warning",
        "endpoint": "/api/v1/tasks/{taskID}/actions",
        "confirmation_required": false,
        "disabled_reason": null
      },
      {
        "id": "cancel",
        "label": "Cancel",
        "style": "danger",
        "endpoint": "/api/v1/tasks/{taskID}/actions",
        "confirmation_required": true,
        "disabled_reason": null
      }
    ]
  }
  ```
  `disabled_reason` is set (e.g. `"insufficient_permission"`) when an action is shown greyed-out rather than omitted, for UX clarity; the server still enforces via `ActionPolicy` regardless of what the client sends.

### Migration

- `000026_add_task_events.up.sql`: Creates `task_events` table with columns `id`, `task_id`, `sequence_number` (bigint, per-task monotonic ordering key — see design.md), `type`, `schema_version`, `payload` (JSONB, capped at 8KB), `artifact_id` (nullable FK into the existing `workflow_artifacts` table, set when large output was externalized), `size_bytes`, `created_at`. Indexed on `(task_id, sequence_number)` (ordering/cursor queries) and `(task_id, created_at)` (display-order fallback/debugging).
- `000027_add_task_action_requests.up.sql`: Creates `task_action_requests` table with columns `id`, `task_id`, `request_id`, `action`, `result` (JSONB), `created_at`. Unique constraint on `(task_id, request_id)` for idempotency.

### Backward Compatibility

- The existing `GET /tasks/{taskID}/logs/stream` SSE endpoint remains functional and unchanged. It is not removed, only deprecated for new frontend consumption.
- The existing `TaskLog` model is not modified.
- The new `available_actions` field on the Task response is additive — existing clients that don't read it are unaffected.
- The `SupportingAccordion` log viewer remains available as a fallback tab for users who prefer raw logs.
