# Specs: Log Output Optimization

## Added Requirements

### REQ-001: SSE Streaming Endpoint
> ❌ Status: Not Started

**Scenario:**
- WHEN an authenticated client requests `GET /api/v1/tasks/{taskID}/logs/stream` with a valid Bearer token
- THEN the backend establishes a `text/event-stream` connection
- AND streams the tail snapshot followed by real-time log events as they are written.

### REQ-002: In-Memory Log Pub/Sub Hub
> ❌ Status: Not Started

**Scenario:**
- WHEN a log is written via `WorkflowRepo.CreateLog`
- THEN the log is persisted (`.jsonl` file, or DB row in fallback mode)
- AND broadcast non-blockingly to every channel subscribed to that `task_id`
- AND a slow subscriber never blocks or fails the write path.

### REQ-003: Race-Free Connection Handshake
> ❌ Status: Not Started

**Scenario:**
- WHEN a client connects while the orchestrator is actively writing logs
- THEN the handler registers its hub channel *before* running the tail reader, buffering live events during the snapshot
- AND no log line is lost between the snapshot and the live stream
- AND duplicate deliveries are tolerated because the client deduplicates by log UUID.

### REQ-004: Tail Reading for Cold Starts
> ❌ Status: Not Started

**Scenario:**
- WHEN a user opens a task page whose log file has 50,000 lines
- THEN the tail reader seeks backwards from EOF and serves only the last 500 lines without reading the whole file
- AND when `logging.file_root` is unset, the snapshot comes from a `LIMIT 500` DB query instead.

### REQ-005: Collapsible Step UI Parsing
> ❌ Status: Not Started

**Scenario:**
- WHEN the frontend receives a log matching `[#<attempt>] step <name> running`
- THEN it derives (via memoized parsing, not render-time recomputation) a collapsible group in the `LogConsole`
- AND subsequent logs nest inside the group until a `success`, `failed`, or `paused` marker for that step
- AND `running`/`failed` groups default to expanded while `success` groups default to collapsed.

### REQ-006: Batched Ingestion & Virtualized Rendering
> ❌ Status: Not Started

**Scenario:**
- WHEN a burst of 50 log events arrives within 50 milliseconds
- THEN the client coalesces them into a single store update (≤ ~20 flushes/second)
- AND the log list renders only visible rows via virtualization, keeping interaction at 60 FPS regardless of log count.

## Modified Requirements

### REQ-M01: Client-Side Log Streaming
> ❌ Status: Not Started

**Scenario:**
- WHEN the `useTaskWorkflow` hook mounts for a non-terminal task
- THEN it opens a fetch-based SSE stream with the `Authorization: Bearer` header instead of a 3000ms SWR polling interval
- AND on stream failure it reconnects with capped exponential backoff, re-running the tail snapshot and relying on UUID dedup
- AND the connection is aborted on unmount or when the workflow reaches a terminal state.

## Removed Requirements
- REQ-R01: Remove 3-second SWR polling of `GET /api/v1/tasks/{taskID}/logs` to eliminate per-poll full-file reads and retransmission.
