# Design: Log Output Optimization

## 1. Architecture Overview
The system will transition from a Pull-based (HTTP Polling) architecture to a Push-based (SSE + Pub/Sub) architecture.

### 1.1 Backend Component: LogHub
A central struct `LogHub` will be introduced to manage SSE client subscriptions safely.
```go
type LogHub struct {
    mu          sync.RWMutex
    subscribers map[string]map[chan models.TaskLog]struct{} // taskID -> set of subscriber channels
}
```
A set-of-channels map (instead of a slice) makes deregistration trivially correct — no index bookkeeping when the same task has multiple concurrent viewers.

Integration with `WorkflowRepo`:
- Inside `CreateLog()`, after appending to the `.jsonl` file (or DB row in fallback mode), the repo looks up the `taskID` in the `LogHub` and performs a **non-blocking** send to every subscribed channel. Slow subscribers drop messages rather than block the write path; the reconnect flow (see 1.4) recovers any gap.

### 1.2 Backend Component: Tail Reader
To serve the initial snapshot without loading the entire file into memory:
- Seek to EOF and read backwards until N lines (default 500, matching the client buffer cap) are collected, then reverse the slice before streaming.
- **DB fallback**: when `logging.file_root` is unset, the snapshot comes from a `WHERE task_id = ? ORDER BY created_at DESC LIMIT N` query, reversed.

### 1.3 SSE Endpoint
`GET /api/v1/tasks/{taskID}/logs/stream` — handler in `server/internal/handler/workflow.go`, wired in `server/internal/handler/router.go` alongside the existing `/tasks/{taskID}/logs` route.

- Sets headers: `Content-Type: text/event-stream`, `Cache-Control: no-cache`, and flushes after every event.
- Connection sequence (**subscribe-first** — ordering matters to avoid lost logs):
  1. Register a channel with `LogHub` **before** touching the file.
  2. Start buffering anything that arrives on the channel into a local slice (goroutine or select loop).
  3. Run the Tail Reader and stream the historical snapshot.
  4. Flush the buffered live logs, then enter the steady-state loop: read from the channel, write JSON-encoded SSE events.
  5. Deregister the channel and close on client disconnect (`ctx.Done()`).

A log written while the tail read is in flight may appear in both the snapshot and the live buffer; that is acceptable because every log has a UUID assigned in `CreateLog` and the client store already deduplicates by ID (`appendUniqueLogs`).

> **Rejected ordering**: tail-read first, subscribe second. Any log written between the tail read and channel registration would be neither in the snapshot nor broadcast — silently lost until reconnect.

### 1.4 Authentication & Reconnection
All API routes require a `Authorization: Bearer <JWT>` header (`middleware/auth.go`); there is no cookie or query-param auth. Native `EventSource` cannot send headers, so the client uses **fetch-based SSE**:

- `fetch(url, { headers: { Authorization: ... }, signal })` and incrementally parse the `text/event-stream` body from the `ReadableStream`.
- Reconnection: on stream error/close while the workflow is non-terminal, retry with capped exponential backoff (e.g. 1s → 2s → 5s max). Each reconnect simply re-runs the full sequence (tail snapshot + live); ID-dedup in the store makes the overlap harmless, so no `Last-Event-ID` resume machinery is needed on the server.
- The `AbortController` signal is tied to the effect cleanup so navigation away closes the connection.

## 2. Frontend Design

### 2.1 Stream Integration & Ingestion Batching
In `use-task-workflow.ts`, replace the SWR log hook with an effect that owns the stream connection. Each SSE message is a separate task, so React's automatic batching will not coalesce bursts — a fast step can emit dozens of logs in milliseconds, which must not become dozens of store updates and render passes.

```typescript
useEffect(() => {
  const controller = new AbortController();
  let pending: RealtimeLog[] = [];
  let flushTimer: number | null = null;

  const flush = () => {
    if (pending.length) appendLogs(pending);
    pending = [];
    flushTimer = null;
  };

  streamTaskLogs(taskID, token, controller.signal, (log) => {
    pending.push(toRealtimeLog(taskID, log));
    flushTimer ??= window.setTimeout(flush, 50); // batch bursts into one store update
  });

  return () => { controller.abort(); flush(); };
}, [taskID, token, appendLogs]);
```

The Zustand store stays synchronous and presentation-agnostic; batching lives entirely in the effect that owns the connection.

### 2.2 Collapsible Log Grouping (memoized derivation)
`LogConsole` derives a hierarchical structure from the flat log array with `useMemo` keyed on the logs array. Groups therefore recompute only when logs change (at most ~20×/s with 50ms batching) — never on scroll or unrelated renders, which matters once the list is virtualized.

- **Regex Detectors** (matches the orchestrator format `[#<attempt>] step <stepID> <status>` from `tracker.go` + `worker.go`):
  - Start Marker: `/\[#\d+\] step (\w+) running/`
  - End Marker: `/\[#\d+\] step (\w+) (success|failed|paused)/`
- **Data Structure**:
  ```typescript
  type LogGroup = {
     type: 'group';
     stepName: string;
     status: 'running' | 'success' | 'failed' | 'paused';
     logs: RealtimeLog[];
  }
  ```
- **Rendering**: Radix UI Accordion or a custom collapsible div. Groups with `status: 'running'` or `'failed'` default to expanded; `'success'` groups default to collapsed.
- Grouping stays **out of the Zustand store**: baking step regexes and accordion structure into the data layer would complicate `clearLogs`/dedup for no measurable gain at the 500-entry buffer cap. Revisit (incremental grouping at ingestion) only if the buffer cap is raised into the tens of thousands.

### 2.3 Virtualization
Virtualize the visible log rows (e.g. `react-virtuoso`, which handles variable row heights and grouped sticky headers) so the 200-line render cap can be lifted without frame drops.

## 3. Tradeoffs & Decisions
- **Why SSE over WebSockets?** Log streaming is strictly unidirectional (server → client). SSE is plain HTTP — no protocol upgrade, works through the existing chi middleware stack.
- **Why fetch-based SSE over native `EventSource`?** The app authenticates every request with a Bearer header held in JS; `EventSource` cannot send headers, and adding query-param tokens would leak JWTs into access logs and browser history. Hand-rolled reconnection is ~15 lines and the reconnect story (fresh tail + ID-dedup) is simpler than `Last-Event-ID` resume.
- **Why subscribe-before-tail?** Eliminates the lost-log race window at the cost of a small in-handler buffer and possible duplicates, which the client already dedupes by UUID.
- **Why client-side regex grouping?** Avoids migrating the DB schema or changing orchestrator logging contracts. Provides immediate UX value with low backend risk.
- **Why batch in the effect, not the store?** Keeps store actions synchronous and unit-testable; the connection owner is the natural place to own flush timing.
