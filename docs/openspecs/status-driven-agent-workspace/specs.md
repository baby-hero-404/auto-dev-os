# Expected Behavior: Status-Driven Agent Workspace

---

## Scenario: Backend computes available actions for a task

**When:**
- The frontend fetches a task via `GET /api/v1/tasks/{taskID}`

**Then:**
- The response includes an `available_actions` array derived from `task.status` and `workflow.job.status`.
- Each action object contains `id` (string), `label` (string), `style` (string: `"primary"` | `"warning"` | `"danger"` | `"default"`), `confirmation_required` (bool), `endpoint` (string, currently always `"POST /tasks/{taskID}/actions"`), and an optional `disabled_reason` (string) set when the current caller lacks permission for that action (see the Authorization scenario below) — the button is still listed, just rendered disabled.
- **`spec_review` is the only status whose `available_actions` may contain an approval-style action** (an action whose effect is "let the task proceed to the next status"). No other status — including `pr_ready` and `human_review` — ever returns `approve_*`, `reject_*`, or `request_changes`. This is enforced by `computeAvailableActions` as a single code path, not by convention.
- The action set matches exactly the valid operations for the current state (13 real statuses — `planning_split` was verified to have zero backend occurrences and dropped, see design.md):

| `task.status` | `available_actions` | Nature |
|---------------|---------------------|--------|
| `todo` | `execute` | Kick off |
| `context_loading` | `pause`, `cancel` | Operational |
| `analyzing` | `pause`, `cancel` | Operational |
| `spec_review` | `approve_spec`, `request_changes`, `cancel` | **Approval gate (the only one)** |
| `coding` | `pause`, `cancel` | Operational |
| `reviewing` | `pause`, `cancel` | Operational |
| `fixing` | `pause`, `cancel` | Operational |
| `testing` | `pause`, `cancel` | Operational |
| `pr_ready` | `cancel` | Informational — PR is already open; merge happens outside this app |
| `human_review` | `cancel` | Informational — same as `pr_ready`; app never blocks on a merge decision |
| `blocked` | `retry_blocked`, `cancel` | Recovery (resumes a stuck run, not a forward-progress approval) |
| `merged` | *(empty)* | Terminal |
| `failed` | `retry`, `delete` | Recovery |

- If the task has `spec_status === "pending_review"` and `status === "spec_review"`, the actions include `approve_spec` and `request_changes`.
- If the task is a decomposed parent, the split decision is made by the agent during `analyzing`/`coding` and surfaced via an `agent.plan` timeline event plus a conditional `SplitProposalView` sub-view (rendered whenever `proposed_split` is non-empty) — there is no dedicated `planning_split` status and the API never returns `approve_split`/`reject_split`.
- Once `spec_status` reaches `approved` or `auto_approved`, no subsequent status transition up to and including `pr_ready` requires another `GET .../available_actions` check to contain an approval action — a client can assert this by scanning the full status history of a task and confirming `approve_spec` appears at most once.

---

## Scenario: Agent event is normalized and persisted

**When:**
- The CLI Agent (or API-native agent) produces output during task execution (thought text, tool call, command result, file change, test result).

**Then:**
- The backend Agent Adapter intercepts the raw output.
- The adapter normalizes it into an `AgentEvent` with the following schema:
  ```json
  {
    "id": "evt_<uuid>",
    "task_id": "<task-uuid>",
    "sequence_number": 502,
    "type": "<event-type>",
    "schema_version": 1,
    "timestamp": "<ISO-8601>",
    "artifact_id": null,
    "size_bytes": 213,
    "payload": { ... }
  }
  ```
  `sequence_number` (not `id` or `timestamp`) is the authoritative order — see invariant 5. `artifact_id` is non-null only when the payload was externalized to `workflow_artifacts` (invariant 10).
- The event is first assigned the next `sequence_number` for its task and written to the `task_events` table in PostgreSQL. Before writing, the adapter enforces the payload size limit: `payload` is capped at 8KB total, with `stdout_tail`/`stderr_tail` individually capped at 5KB and truncated with a `"... truncated, N bytes total"` marker if exceeded. `file.changed` never carries a diff body — only `additions`/`deletions` counts. If the content that would be truncated is itself the point of the event (e.g. a full test failure log), the adapter instead writes the full content to `workflow_artifacts` and stores a summary + `artifact_id` in the event's `payload`, per invariant 10.
- The event is then broadcast to all active SSE subscribers for that task.
- Valid event types are: `task.started`, `task.completed`, `task.error`, `agent.reasoning_summary`, `agent.message`, `agent.plan`, `tool.started`, `tool.finished`, `file.changed`, `command.started`, `command.finished`, `test.result`, `status.changed`. (`agent.reasoning_summary` carries the agent's own public-facing explanation of what it's doing — never raw chain-of-thought — the name is deliberately explicit about that, not just `agent.thought`.)

---

## Scenario: Frontend connects to event stream

**When:**
- The Task Detail page is opened for a task that is in an active status (`context_loading`, `analyzing`, `coding`, `testing`, `reviewing`, `fixing`).

**Then:**
- The frontend first fetches `GET /api/v1/tasks/{taskID}/events` for full history, ordered by `sequence_number ASC` (cursor-paginated via `?before=<sequence_number>&limit=` if the history is large), and renders it.
- The frontend then opens an SSE connection to `GET /api/v1/tasks/{taskID}/events/stream?after=<id of last event from history>`.
- The backend subscribes this connection to the live broadcast channel **before** querying for any catch-up events (see the SSE Reconnect design in `design.md` — this ordering is what prevents the connect-then-load-then-subscribe race), streams anything created after the cursor, then continues with live broadcasts.
- Each event is rendered as a `TimelineEntry` in the Agent Execution Stream (right column), deduplicated by `event.id`.

---

## Scenario: Decomposition proceeds without human approval

**When:**
- The agent, while `analyzing` or `coding`, decides to split a task into subtasks.

**Then:**
- The task's `status` stays `analyzing`/`coding` throughout — there is no `planning_split` status (verified absent from the backend; see design.md). `GET /api/v1/tasks/{taskID}` continues to return that status's normal `available_actions: [{"id":"pause",...}, {"id":"cancel",...}]` — never `approve_split`/`reject_split`.
- The task gains a non-empty `proposed_split` payload, which causes `SplitProposalView` to render as a read-only sub-view within `CodingProgressView`, showing the proposed subtask breakdown.
- The right column (Agent Execution Stream) shows an `agent.plan` event describing the split.
- The orchestrator dispatches the subtasks and proceeds directly into `coding` without waiting for any API call from the frontend beyond the standard `pause`/`cancel` operational controls.

---

## Scenario: PR is opened automatically; merge is out of scope

**When:**
- All tests pass for a task in `testing` status.

**Then:**
- The task transitions to `pr_ready`. The backend/agent has already opened the pull request on the git host.
- `GET /api/v1/tasks/{taskID}` for a `pr_ready` or `human_review` task returns `available_actions: [{"id":"cancel",...}]` — never `approve_merge`/`reject_merge`.
- The `PrCreatedView` in the left column shows the PR link/number and a read-only summary of the agent's own review verdict (`ReviewVerdictCard`). There is no "Approve" or "Merge" button anywhere in this app.
- Whether and when the PR is merged is determined entirely by the repository's own CI checks / branch protection / a human reviewing on the git host — a system outside this app's scope. This app's task record transitions to `merged` only when that external merge is detected (e.g. via a webhook or poll), not via a button click here.

---

## Scenario: Frontend renders the Human Decision Surface

**When:**
- The Task Detail page loads with any `task.status`.

**Then:**
- The `StatusViewRegistry` is consulted to determine which React component to render in the left column.
- Exactly one status view component is rendered — never multiple overlapping panels.
- The `DynamicActionBar` renders buttons from `task.available_actions`. Button `style` maps to visual variants (primary = brand color, warning = amber, danger = red).
- Clicking an action button dispatches `POST /tasks/{taskID}/actions` with `{ "action": "<action.id>", "request_id": "<client-generated uuid>" }` — a single endpoint for every action, not one route per action. The `request_id` is generated once per click and reused on any automatic retry of that same click (e.g. a network error triggering a client-side retry), so a duplicate isn't dispatched as a duplicate side effect.
- If the button's `disabled_reason` is set, it renders disabled with that reason as a tooltip and dispatches nothing on click.

---

## Scenario: Page refresh loads historical events

**When:**
- A user refreshes the browser while viewing a task in `coding` status.

**Then:**
- The frontend re-fetches full history via `GET /tasks/{taskID}/events` (a page refresh has no client-side cursor to resume from — it's a cold load, same as the "Frontend connects to event stream" scenario above) and reconnects to the SSE stream from that point forward.
- The timeline reconstructs to the same state as before the refresh.
- No events are lost.

---

## Scenario: Desktop split-screen layout (≥1200px)

**When:**
- The viewport width is 1200px or wider.

**Then:**
- The layout renders as a two-column split:
  - Left column (~45%): Human Decision Surface (status view + action bar).
  - Right column (~55%): Agent Execution Stream (timeline).
- Both columns scroll independently.

---

## Scenario: Mobile/tablet layout (<1200px)

**When:**
- The viewport width is below 1200px.

**Then:**
- The split-screen collapses into a tabbed layout with two tabs: `Control` and `Activity`.
- `Control` tab renders the Human Decision Surface.
- `Activity` tab renders the Agent Execution Stream.
- The tab corresponding to the current status category is auto-selected:
  - Human Gate statuses (`spec_review`, `human_review`, `pr_ready`, `blocked`) → `Control` tab.
  - Agent Execution statuses (`coding`, `testing`, `fixing`, `reviewing`, `analyzing`) → `Activity` tab.

---

## Scenario: Autopilot guardrail auto-blocks a runaway task

**When:**
- A task in the automatic (post-`spec_review`) portion of its lifecycle hits one of: 5 consecutive `fixing`→`testing` failures (see design.md for the precise definition of what counts as a retry), 2 hours of wall-clock execution, its project's cost budget (only enforced when the CLI adapter exposes cost data — otherwise this trigger is inactive, not defaulted to a guess), 20,000 `task_events` rows written for the task, or the Agent Adapter's security scrubber flags a hardcoded secret or a diff touching a deny-listed path.

**Then:**
- The orchestrator emits a `task.error` event with `payload.reason` set to one of `max_retries_exceeded` / `execution_timeout` / `cost_budget_exceeded` / `event_volume_exceeded` / `security_review_required`, and `is_retryable: true` (all five are recoverable via `retry_blocked`).
- The task transitions to `blocked`.
- `available_actions` for the task becomes `["retry_blocked", "cancel"]`, same as any other `blocked` task — the guardrail routes into the existing recovery status rather than a new one.
- `BlockedView` surfaces the specific `reason` from the triggering `task.error` event so the human knows *why* it stopped, not just that it stopped.
- This is the mechanism that makes "no per-step approval" safe: the system self-halts on concrete evidence of a problem instead of relying on a human noticing a stuck task.

---

## Scenario: Action request is rejected for lacking permission

**When:**
- A caller without the required role (see `design.md` → `ActionPolicy`) sends `POST /tasks/{taskID}/actions` with an action ID that is present in the task's `available_actions`.

**Then:**
- The server returns `403 Forbidden` with a machine-readable `reason` (e.g. `"requires project.write permission"`).
- No state change occurs.
- Independently, `GET /tasks/{taskID}` for that same caller returns the action with `disabled_reason` set, so the frontend never renders it as clickable in the first place — the 403 is a defense-in-depth backstop, not the primary UX signal.

---

## Scenario: Duplicate action request is idempotent

**When:**
- The same `request_id` is submitted twice for `POST /tasks/{taskID}/actions` (e.g. a double-click, or a client-side retry after a timeout where the first request actually succeeded).

**Then:**
- The second request performs no additional side effect (task status does not transition twice, no duplicate `task_events` row is written for the action).
- The second request returns the same response the first one did.
- This applies to every action, but matters most for `approve_spec` (shouldn't double-fire spec approval), `cancel`, and `delete` (a duplicate delete must not error or corrupt state — it's a no-op, not a second deletion).

---

## Failure Scenario: SSE connection drops

**When:**
- The SSE connection to `/events/stream` is interrupted (network error, server restart).

**Then:**
- The UI shows a small non-blocking "Reconnecting…" indicator in the timeline header; no full-page error state.
- The frontend automatically reconnects with exponential backoff (1s, 2s, 4s, max 30s). The browser's native `EventSource` sends the last-received event's `sequence_number` via the `Last-Event-ID` header automatically on reconnect (the SSE `id:` field carries `sequence_number`, not the event's UUID — see design.md's Wire Format).
- The backend resumes from that cursor — subscribing to live broadcasts first, then streaming only events with a greater `sequence_number`, then continuing live (see SSE Reconnect in `design.md`) — not by replaying the entire history from the beginning, and not using `created_at` as the resume point.
- No duplicate events are shown — the frontend deduplicates by `sequence_number` defensively, though the backend's subscribe-then-catchup ordering means duplicates from a normal reconnect shouldn't occur in the first place.
- No events are silently dropped in the gap between disconnect and resubscribe — this was a known race in an earlier draft of this spec and is fixed by the subscribe-before-catchup-query ordering, not by client-side backoff alone.
- If a resumed event carries a `(type, schema_version)` the frontend build doesn't recognize (e.g. backend deployed a new event type after this tab was opened), it renders as `UnknownEventCard` rather than breaking the reconnect flow.

---

## Failure Scenario: Agent adapter receives unparseable CLI output

**When:**
- The CLI agent produces output that cannot be parsed into any known event type (malformed JSON, unexpected format).

**Then:**
- The adapter emits a fallback `agent.message` event with the raw text in `payload.text`.
- The raw text is still visible in the timeline as a generic message block.
- No data is silently dropped.

---

## Failure Scenario: No events exist for a task

**When:**
- The task is in `todo` status and has never been executed.

**Then:**
- The right column (Agent Execution Stream) shows an empty state: "No agent activity yet. Execute the task to see the AI timeline."
- The left column (Human Decision Surface) renders the `TodoView` with an `Execute` action button.

---

## Invariants

1. **Single view per status.** Exactly one status view component is rendered in the left column at any time. There is never a state where two status-specific panels compete for attention.
2. **`spec_review` is the only approval gate.** Across the entire task lifecycle, `approve_spec`/`request_changes` are the only actions that unblock forward progress on a human decision. Every status added to `StatusViewRegistry` and `computeAvailableActions` in the future must default to operational-only actions (`pause`/`cancel`/`retry`/`delete`) unless a deliberate, documented product decision reopens this invariant — it is not opt-out per status.
3. **Actions are backend-authoritative.** The frontend never computes which buttons to show. It renders exactly what `available_actions` contains. If the backend returns an empty array, no action buttons are shown.
4. **Events are immutable.** Once written to `task_events`, an event is never updated or deleted (except by the 90-day terminal-task retention job, which deletes whole rows, never edits them). The table is append-only.
5. **Event ordering is by `sequence_number`, never `created_at`.** Events are ordered by a per-task monotonic `sequence_number` (bigint, assigned at write time). `created_at` is a display/debugging field only — it is never used as a sort key or reconnect cursor, because concurrent writers (e.g. separate stdout/stderr readers) can produce equal or out-of-order timestamps. The frontend renders in `sequence_number` order and deduplicates SSE deliveries by `sequence_number`, not by timestamp.
6. **The existing `TaskLog` SSE endpoint (`/logs/stream`) remains operational.** It is not removed during this feature's implementation.
7. **Every persisted event carries a `schema_version`.** A consumer switches on `(type, schema_version)`, never `type` alone. A payload shape change without a version bump is a spec violation. An event whose `(type, schema_version)` a client doesn't recognize renders as `UnknownEventCard`, never a crash.
8. **Action requests are idempotent by `request_id`.** Replaying the same `(task_id, request_id)` never produces a second side effect.
9. **Autopilot always has a stopping condition.** No task runs in the automatic portion of its lifecycle (`spec_review` approval → `pr_ready`, including the in-band decomposition decision) without an active `max_retry_count`/`max_execution_time`/`cost_budget`/`max_event_count` guardrail backing it. A status that can loop indefinitely, or an event stream that can grow indefinitely, with none of these wired up is an incomplete implementation of this spec, not an acceptable v1 gap.
10. **No event payload embeds unbounded content.** Any field whose natural size could exceed the 8KB payload cap is either truncated with an explicit marker (`stdout_tail`/`stderr_tail`, capped at 5KB) or externalized to `workflow_artifacts` with only a summary + `artifact_id` left in the event. A `task_events.payload` column is never the home for a raw multi-hundred-KB blob.
11. **Backend and frontend status enums are checked for parity in CI, not hand-synced.** `TaskStatus` in `server/pkg/models/task.go` and its frontend counterpart in `web/src/lib/types.ts` are compared against a shared generated fixture on every CI run; divergence is a build failure, not a runtime surprise.

## Rules

- The `StatusViewRegistry` is the single source of truth for status-to-component mapping. No status-conditional rendering (`if task.status === "X"`) is allowed outside the registry.
- The `AgentEvent` schema is the contract between backend and frontend. The frontend does not parse raw CLI stdout/stderr text.
- The `task_events` table is write-heavy but read-light (reads happen on page load and reconnect). The index on `(task_id, sequence_number)` covers ordering/cursor queries; `(task_id, created_at)` remains as a secondary index for display/debugging queries. No full-text search index is required in v1.
- `computeAvailableActions` is the single function that may return an approval-style action, and it may only do so for `task.status === "spec_review"`. This is enforced by a unit test that asserts, for every other status in `TaskStatus`, the returned action IDs are a subset of `{pause, cancel, retry, retry_blocked, delete, execute}`.
- Every action dispatch goes through `POST /tasks/{taskID}/actions` and is checked against `ActionPolicy` server-side (`design.md` → Authorization) — `available_actions` on the GET response is advisory for rendering, never the security boundary.

## Constraints

- The `task_events` table stores events per task, not per job. If a task is retried (new `WorkflowJob`), events from both the old and new job appear in the same timeline, distinguished by their timestamps.
- Each event's `payload` is capped at 8KB (5KB for individual `stdout_tail`/`stderr_tail` fields), enforced by the Agent Adapter before the DB write, with oversized content externalized to `workflow_artifacts` rather than dropped — see `design.md` → Performance Considerations. This bounds per-row size. Per-task event *count* is separately bounded in v1 by the `max_event_count` guardrail (20,000/task, invariant 9) and `GET /events` is cursor-paginated in v1 (`?before=<sequence_number>&limit=`), so a large history no longer implies a slow initial load.
- Events for tasks in a terminal status are retained for 90 days after the task's last activity, then eligible for cleanup; events for active tasks are retained indefinitely. See `design.md` → Performance Considerations.
- The event stream does not include raw Chain-of-Thought text. It includes structured agent updates (`agent.reasoning_summary`), tool calls, and results.
- This app never merges a pull request itself. Detecting that an externally-merged PR should transition the task to `merged` is a separate concern (webhook/poll) and is out of scope for the actions/registry work described here — it is called out so implementers don't accidentally wire a "merge" button.
- `planning_split` was verified (Task 1.0) to be unreachable dead frontend code — zero occurrences in `server/`. It has been removed from `TaskStatus`, the action table, and `StatusViewRegistry`; the split decision is now surfaced via `proposed_split` payload data plus an `agent.plan` event instead.
