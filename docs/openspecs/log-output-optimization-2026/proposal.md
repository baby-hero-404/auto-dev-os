# Proposal: Log Output Optimization

## Why
Task execution logs are appended to `<file_root>/<task_id>.jsonl` (default `.data/logs`, configured via `logging.file_root` / `LOG_FILE_ROOT`; `WorkflowRepo` falls back to the `task_logs` DB table when unset). The frontend (`useTaskWorkflow`) polls `GET /api/v1/tasks/{taskID}/logs` via SWR every 3 seconds until the workflow reaches a terminal state.

On every poll, `WorkflowRepo.ListLogs` opens the file, scans and JSON-unmarshals **every** line, and the handler serializes the full result back to the client. For long-running tasks this file can exceed tens of thousands of lines, causing:

- **Server CPU spikes**: full-file read + parse per request, per connected client, every 3 seconds.
- **Bandwidth waste**: the entire log history is retransmitted on each poll, even though the client dedupes by log ID and discards everything beyond its 500-entry buffer.
- **Lost history & poor UX**: the client store caps at 500 buffered entries and the console renders only the last 200 lines, so earlier output is silently dropped; updates arrive with up to 3 seconds of latency and the console offers no structure for scanning long output.

## What Changes

### Issue 1: Real-time Data Transmission (SSE)
- Replace the 3-second HTTP polling with Server-Sent Events at `GET /api/v1/tasks/{taskID}/logs/stream`.
- The backend pushes new log lines immediately as they are written, via an in-memory `LogHub` pub/sub keyed by task ID: `WorkflowRepo.CreateLog` performs a non-blocking broadcast to subscribed channels after appending to the `.jsonl` file.

### Issue 2: Efficient Initial Load (Tail Reader)
- On SSE connection open, serve only the last N lines (default 500, matching the client buffer cap) instead of the whole file, using a tail reader that seeks backwards from EOF rather than scanning from the start.
- Preserve the DB-fallback path: when `file_root` is unset, the initial snapshot comes from a `LIMIT N` query ordered by `created_at DESC`.

### Issue 3: UI Grouping and Virtualization
- Parse log lines client-side and group them into collapsible sections (accordions) keyed on the step-transition messages emitted by the orchestrator (format: `[#<attempt>] step <stepID> <status>`, e.g. `[#1] step analyze running`).
- Virtualize the log list so the DOM only renders visible rows, allowing the 200-line render cap to be lifted without sacrificing frame rate.

## Capabilities

### New Capabilities
- Real-time log streaming via SSE (`GET /api/v1/tasks/{taskID}/logs/stream`).
- In-memory `LogHub` broadcast system in the backend repository layer.
- Collapsible, step-aware log console with virtual scrolling.

### Modified Capabilities
- `useTaskWorkflow` (frontend hook) subscribes via fetch-based SSE (Bearer-authenticated, since `EventSource` cannot send headers) instead of SWR polling for logs; SWR polling remains for workflow status only.
- `useRealtimeLogStore` buffer semantics updated to support the virtualized console (cap raised or made stream-aware).
- `WorkflowRepo` (backend) manages `LogHub` subscriptions alongside file writing and exposes the tail-read method.

### Removed Capabilities
- 3-second SWR polling of `GET /api/v1/tasks/{taskID}/logs`.
- Full-file JSONL read and parse on every log request.

## Impact

| Area | Files Affected |
|------|----------------|
| Backend Repo | `server/internal/repository/workflow.go` |
| Backend Handler | `server/internal/handler/workflow.go` |
| Backend Router | `server/internal/handler/router.go` |
| Frontend Store | `web/src/lib/store/use-realtime-log-store.ts` |
| Frontend Hooks | `web/src/lib/hooks/use-task-workflow.ts` |
| Frontend UI | `web/src/components/dashboard/log-console.tsx` |
