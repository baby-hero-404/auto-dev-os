# Tasks: Log Output Optimization

## Phase 1: Backend SSE & Hub Implementation
- [x] **1.1** Define `LogHub` (`map[string]map[chan models.TaskLog]struct{}` + `sync.RWMutex`) with `Subscribe`/`Unsubscribe`/`Broadcast` methods in `server/internal/repository/workflow.go`.
- [x] **1.2** Update `CreateLog` to broadcast newly written logs to `LogHub` subscribers via non-blocking sends (both file and DB-fallback paths).
- [x] **1.3** Implement the tail reader: seek backwards from EOF collecting the last 500 lines; add the `LIMIT 500` DB-fallback query for `file_root`-less mode.
- [x] **1.4** Create the SSE handler `StreamLogs` in `server/internal/handler/workflow.go` using the subscribe-first sequence (register channel → buffer live events → stream tail snapshot → flush buffer → live loop → deregister on `ctx.Done()`).
- [x] **1.5** Wire `GET /tasks/{taskID}/logs/stream` into `server/internal/handler/router.go` under the existing authenticated `/api/v1` group.
- [x] **1.6** Backend tests: hub subscribe/broadcast/unsubscribe under concurrency, tail reader correctness (empty file, < N lines, > N lines, partial last line), and a race test proving no log loss when writes overlap connection setup.

## Phase 2: Frontend Data Layer Update
- [x] **2.1** Add a fetch-based SSE client (`streamTaskLogs`) to `web/src/lib/api.ts`: `Authorization: Bearer` header, incremental `text/event-stream` parsing from the `ReadableStream`, `AbortSignal` support.
- [x] **2.2** Implement capped exponential backoff reconnection (1s → 2s → 5s max); each reconnect re-runs the tail snapshot and relies on the store's UUID dedup.
- [x] **2.3** In `use-task-workflow.ts`, replace the 3-second SWR log polling with the stream effect: batch incoming events into a local buffer flushed to Zustand on a 50ms timer; abort on unmount and on terminal workflow status.

## Phase 3: Frontend UI Grouping & Virtualization
- [x] **3.1** Create a pure grouping helper that parses the flat log array into `LogGroup` structures from the `[#<attempt>] step <stepID> <status>` markers; consume it in `LogConsole` via `useMemo` keyed on the logs array.
- [x] **3.2** Refactor `LogConsole` to render mixed content (ungrouped lines and grouped blocks) from the derived structure.
- [x] **3.3** Implement collapsible UI (Radix Accordion or custom) for `LogGroup` items — expanded for `running`/`failed`, collapsed for `success`.
- [x] **3.4** Integrate a virtualized list (`react-virtuoso` preferred for variable row heights) and lift the 200-line render cap.
- [x] **3.5** Verify styling with ANSI/semantic colors against the dark mode aesthetic.
- [x] **3.6** E2E check: long-running task streams live logs, groups collapse correctly, reconnect after server restart shows no duplicates or gaps.

## Docs sync

- [x] Update corresponding `docs/features/` as specified in feature-docs-sync/design.md — N/A: this spec set is not in feature-docs-sync/design.md's 14-set mapping table, no docs/features/ target specified
