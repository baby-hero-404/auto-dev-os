# Tech Debt & SSE Log Streaming Fixes Implementation Plan

> **For agentic workers:** Use subagent-driven-development or executing-plans
> to implement this plan task-by-task. Steps use checkbox syntax for tracking.

**Goal:** Close the lost-log race in the new SSE log-streaming feature, restore LLM observability in the state-machine execution path, remove a retry amplification bug, and fix two frontend reliability issues in the streaming client — with a regression test for each backend fix.

**Architecture:** No architectural change. Every task is a targeted fix inside the existing SSE/tool-loop/gateway code introduced by the log-streaming feature and its surrounding runner. Phase 1 and 2 are fully specified below with exact code and tests. Phase 3 separates a handful of small, mechanical cleanups (specified below) from four large architectural refactors that are **not** specified here — see the note at the end of Phase 3 for why, and what to do before starting them.

**Tech Stack:** Go (server), TypeScript/React (web), stdlib `testing` (no testify, no frontend unit-test runner — see Phase 2 note).

---

## Phase 1: Critical Correctness Fixes (🔴 High Priority)

### Task 1: Fix the lost-log race in `StreamLogs`

**Problem:** In `StreamLogs` (`server/internal/handler/workflow.go:63-131`), a background goroutine buffers live logs while the tail-read snapshot is in flight. `close(stopBuf)` signals that goroutine to stop, but does **not** guarantee it has actually stopped before the handler starts reading `ch` directly in its own loop. In the window between `close(stopBuf)` and the goroutine's next `select` iteration, both the dying goroutine and the handler's new loop are live receivers on the same channel `ch`. Go delivers each value to whichever receiver the runtime happens to pick — if the dying goroutine wins, the log is appended to `buffer`, which was already flushed and is never read again. **The log is lost**, silently, with no error.

This is the same failure mode `design.md`'s subscribe-first ordering was written to eliminate — the implementation reintroduced it in a narrower window.

**Files:**
- Modify: `server/internal/handler/workflow.go:63-131`
- Test: `server/internal/handler/workflow_test.go`

- [x] **Step 1: Write the failing regression test**

Add to `server/internal/handler/workflow_test.go` (package `handler`, same package as the existing tests in that file):

```go
func TestStreamLogsLoop_NoLostLogsDuringTailRace(t *testing.T) {
	ch := make(chan models.TaskLog, 10)
	proceedTail := make(chan struct{})

	tail := func() ([]models.TaskLog, error) {
		<-proceedTail // block until the test says the tail read may finish
		return []models.TaskLog{{ID: "hist-1", Message: "history"}}, nil
	}

	var mu sync.Mutex
	var emitted []models.TaskLog
	emit := func(log models.TaskLog) {
		mu.Lock()
		emitted = append(emitted, log)
		mu.Unlock()
	}

	ctx, cancel := context.WithCancel(context.Background())
	loopDone := make(chan struct{})
	go func() {
		defer close(loopDone)
		_ = streamLogsLoop(ctx, ch, tail, emit)
	}()

	// Simulate a log broadcast arriving WHILE the tail read is still in flight — this is
	// exactly the window where the pre-fix code raced two consumers on ch.
	ch <- models.TaskLog{ID: "live-1", Message: "during tail"}
	time.Sleep(20 * time.Millisecond) // let the background buffering goroutine consume it
	close(proceedTail)                // let tail() return

	time.Sleep(20 * time.Millisecond)
	ch <- models.TaskLog{ID: "live-2", Message: "after tail"}
	time.Sleep(20 * time.Millisecond)

	cancel()
	<-loopDone

	mu.Lock()
	defer mu.Unlock()
	ids := make([]string, len(emitted))
	for i, e := range emitted {
		ids[i] = e.ID
	}
	want := []string{"hist-1", "live-1", "live-2"}
	if !reflect.DeepEqual(ids, want) {
		t.Fatalf("lost or misordered logs: got %v, want %v", ids, want)
	}
}
```

Add `"reflect"`, `"sync"`, and `"time"` to the import block of `workflow_test.go` if not already present.

- [x] **Step 2: Run test to verify it fails to compile**

Run: `go test ./server/internal/handler/... -run TestStreamLogsLoop_NoLostLogsDuringTailRace -v`
Expected: FAIL — `undefined: streamLogsLoop` (the function doesn't exist yet).

- [x] **Step 3: Extract the race-prone logic into a testable, HTTP-agnostic function and add the synchronous handshake**

Replace the body of `StreamLogs` in `server/internal/handler/workflow.go` with:

```go
func (h *WorkflowHandler) StreamLogs(w http.ResponseWriter, r *http.Request) {
	taskID := chi.URLParam(r, "taskID")
	ctx := r.Context()

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "Streaming unsupported", http.StatusInternalServerError)
		return
	}
	flusher.Flush()

	ch := h.orch.SubscribeLogs(taskID)
	defer h.orch.UnsubscribeLogs(taskID, ch)

	emit := func(log models.TaskLog) {
		data, _ := json.Marshal(log)
		fmt.Fprintf(w, "event: log\ndata: %s\n\n", string(data))
		flusher.Flush()
	}

	if err := streamLogsLoop(ctx, ch, func() ([]models.TaskLog, error) {
		return h.orch.TailLogs(ctx, taskID, 500)
	}, emit); err != nil {
		fmt.Fprintf(w, "event: error\ndata: %s\n\n", err.Error())
		flusher.Flush()
	}
}

// streamLogsLoop drains the tail snapshot then live logs from ch onto emit. It is deliberately
// decoupled from http.ResponseWriter/chi so the subscribe-first race can be exercised directly
// in a unit test without wiring up a full Orchestrator.
//
// Ordering is subscribe-first: the caller has already subscribed to ch before calling this
// function, so nothing broadcast after subscription is missed. A background goroutine buffers
// anything that arrives on ch while tail() is in flight; once tail() returns and the historical
// snapshot has been emitted, we must be certain that goroutine has fully detached from ch before
// this function starts reading ch directly — otherwise the two would race as concurrent
// receivers on the same channel and a value could be delivered to whichever one the Go runtime
// happens to pick, silently dropping it if that's the goroutine (its buffer is never read again
// after the flush below). Closing stopBuf alone does not guarantee that detachment happens
// before this function proceeds; <-done does.
func streamLogsLoop(ctx context.Context, ch chan models.TaskLog, tail func() ([]models.TaskLog, error), emit func(models.TaskLog)) error {
	var buffer []models.TaskLog
	var bufMu sync.Mutex
	stopBuf := make(chan struct{})
	done := make(chan struct{})

	go func() {
		defer close(done)
		for {
			select {
			case <-stopBuf:
				return
			case log, ok := <-ch:
				if !ok {
					return
				}
				bufMu.Lock()
				buffer = append(buffer, log)
				bufMu.Unlock()
			}
		}
	}()

	history, err := tail()
	if err != nil {
		close(stopBuf)
		<-done
		return err
	}

	for _, log := range history {
		emit(log)
	}

	close(stopBuf)
	<-done // guarantees the goroutine above has fully detached from ch before we read it directly

	bufMu.Lock()
	for _, log := range buffer {
		emit(log)
	}
	bufMu.Unlock()

	for {
		select {
		case <-ctx.Done():
			return nil
		case log, ok := <-ch:
			if !ok {
				return nil
			}
			emit(log)
		}
	}
}
```

- [x] **Step 4: Run test to verify it passes**

Run: `go test ./server/internal/handler/... -run TestStreamLogsLoop_NoLostLogsDuringTailRace -v`
Expected: PASS. (Against the pre-fix two-consumer version this test is flaky-to-failing, since it depends on Go's scheduler; against the `<-done` handshake it passes deterministically because the handshake removes the race outright rather than making it statistically less likely.)

- [x] **Step 5: Run the full handler package test suite to confirm no regressions**

Run: `go test ./server/internal/handler/... -v`
Expected: PASS (all tests, including existing `TestStreamLogsLoop_NoLostLogsDuringTailRace` and pre-existing handler tests).

- [x] **Step 6: Commit**

```bash
git add server/internal/handler/workflow.go server/internal/handler/workflow_test.go
git commit -m "fix: close subscribe/tail race in SSE log streaming"
```

---

### Task 2: Restore LLM trace in the state-machine execution path

**Problem:** `runStateMachine` (`server/internal/orchestrator/llmrunner/statemachineloop.go:68-358`) never calls `r.WriteTrace`. The legacy path (`runAgentic`'s `RunToolLoop` via its `OnCall` hook, `runner.go:307-313`) writes a trace after every LLM call. Turning on `EXECUTION_STATE_MACHINE_ENABLED` therefore silently drops all per-iteration prompt/response observability — exactly the debug data needed while this newer code path is still being hardened.

**Files:**
- Modify: `server/internal/orchestrator/llmrunner/statemachineloop.go:68-156`

- [x] **Step 1: Write the failing test**

Add to `server/internal/orchestrator/llmrunner/statemachine_test.go` (or extend the existing `TestRunner_Run_StateMachineMode` in `runner_test.go` if a suitable fixture already exists there — check first with `grep -n "WriteTrace" server/internal/orchestrator/llmrunner/runner_test.go`). If no fixture wires `WriteTrace` yet, add:

```go
func TestRunner_Run_StateMachineMode_WritesTrace(t *testing.T) {
	var traceCalls int
	r := Runner{
		Provider: &mockAgenticProvider{ /* return a single Done-state JSON response, no tool calls */ },
		Tools:    []llm.ToolDefinition{},
		ToolExecutor: func(ctx context.Context, name, args string) (string, error) {
			return "", nil
		},
		WriteTrace: func(ctx context.Context, task *models.Task, agent *models.Agent, stepID string, msgs []llm.Message, resp *llm.Response, parsed map[string]any, iteration int, latency time.Duration) {
			traceCalls++
		},
	}
	ctx := context.WithValue(context.Background(), models.StateMachineEnabledCtxKey, true)
	task := &models.Task{ID: "t1"}
	agent := &models.Agent{ID: "a1"}

	_, err := r.Run(ctx, task, agent, "job1", "code_backend", "do the thing")
	if err != nil {
		t.Fatalf("Run failed: %v", err)
	}
	if traceCalls == 0 {
		t.Fatal("expected WriteTrace to be called at least once in state machine mode, got 0 calls")
	}
}
```

Adapt `mockAgenticProvider`'s canned response to whatever shape makes `runStateMachine` reach `StateDone` in one turn — follow the existing pattern in `TestRunner_Run_StateMachineMode` (`runner_test.go:230`) for the exact fixture, since it already drives this path successfully.

- [x] **Step 2: Run test to verify it fails**

Run: `go test ./server/internal/orchestrator/llmrunner/... -run TestRunner_Run_StateMachineMode_WritesTrace -v`
Expected: FAIL — `traceCalls == 0`.

- [x] **Step 3: Add the trace call**

In `server/internal/orchestrator/llmrunner/statemachineloop.go`, inside the main `for` loop of `runStateMachine`, immediately after the LLM call:

```go
		// LLM call for the turn
		resp, err := r.Provider.ChatWithOptions(ctx, msgs, llm.ChatOptions{Tools: allowedTools, ToolChoice: "auto"})
		if err != nil {
			return nil, fmt.Errorf("llm state machine loop call failed in state %s: %w", currentState, err)
		}
		lastResp = resp
```

replace with:

```go
		// LLM call for the turn
		callStart := time.Now()
		resp, err := r.Provider.ChatWithOptions(ctx, msgs, llm.ChatOptions{Tools: allowedTools, ToolChoice: "auto"})
		latency := time.Since(callStart)
		if err != nil {
			return nil, fmt.Errorf("llm state machine loop call failed in state %s: %w", currentState, err)
		}
		lastResp = resp

		if r.WriteTrace != nil {
			var tracedParsed map[string]any
			if len(resp.ToolCalls) > 0 {
				tracedParsed = map[string]any{"tool_calls": resp.ToolCalls}
			} else {
				tracedParsed = map[string]any{"raw_content": resp.Content}
			}
			r.WriteTrace(ctx, task, agent, stepID, msgs, resp, tracedParsed, sm.used[currentState]+1, latency)
		}
```

`time` is already imported in this file (used by `saveExecutionSnapshot`); no new imports needed.

- [x] **Step 4: Run test to verify it passes**

Run: `go test ./server/internal/orchestrator/llmrunner/... -run TestRunner_Run_StateMachineMode_WritesTrace -v`
Expected: PASS.

- [x] **Step 5: Run the full llmrunner package test suite**

Run: `go test ./server/internal/orchestrator/llmrunner/... -v`
Expected: PASS.

- [x] **Step 6: Commit**

```bash
git add server/internal/orchestrator/llmrunner/statemachineloop.go server/internal/orchestrator/llmrunner/statemachine_test.go
git commit -m "fix: write LLM call trace in state machine execution loop"
```

---

### Task 3: Remove retry amplification in `runner.go`

**Problem:** A transient network error currently gets retried at three overlapping layers: `runner.go`'s inner `chatAttempt` loop (3 tries, exponential backoff up to ~6s total) **inside** `runner.go`'s outer `attempt` loop (3 tries) for two separate call sites (`Run()`'s single-shot path and `runAgentic`'s `Chat` closure passed into `RunToolLoop`) — on top of the gateway's own per-credential retry (4 attempts) and full credential/model cycling with cooldowns (`server/internal/gateway/gateway.go`). Since `isTransientError` now classifies identically at both layers (per the comment at `runner.go:497`), the runner-level retry only adds latency, it never succeeds where the gateway wouldn't have.

**Do not** touch the **outer** `attempt` loop in `Run()` (`runner.go:137-218`) — that loop retries on *malformed LLM output* (JSON parse/schema/business validation failure) by re-prompting with corrective feedback, which is a completely different concern from network transient-error retry. Only the **inner** `chatAttempt` loops (one in `Run()`, one in `runAgentic`'s `Chat` closure) are being removed.

**Files:**
- Modify: `server/internal/orchestrator/llmrunner/runner.go:137-155` (inner loop inside `Run()`)
- Modify: `server/internal/orchestrator/llmrunner/runner.go:254-279` (inner loop inside `runAgentic`'s `Chat` closure)
- Modify: `server/internal/orchestrator/llmrunner/runner.go:497-519` (delete now-unused `isTransientError`/`ctxSleep`)
- Modify: `server/internal/orchestrator/llmrunner/runner_test.go:17-46` (delete now-unused `ctxSleep` tests)

- [x] **Step 1: Confirm no existing test asserts the inner retry behavior**

Run: `grep -n "chatAttempt\|isTransientError\|ctxSleep" server/internal/orchestrator/llmrunner/runner_test.go`
Expected: only the two `TestCtxSleep_*` tests at lines 19 and 32 reference these symbols — no test exercises the retry-and-backoff behavior of `Run()`/`runAgentic` itself, so removing the inner loops does not break any existing assertion beyond those two tests (deleted in Step 4).

- [x] **Step 2: Remove the inner retry loop in `Run()`**

In `server/internal/orchestrator/llmrunner/runner.go`, replace:

```go
	for attempt := 1; attempt <= 3; attempt++ {
		finalAttempt = attempt
		var chatErr error
		for chatAttempt := 1; chatAttempt <= 3; chatAttempt++ {
			callStart := time.Now()
			resp, chatErr = r.Provider.Chat(ctx, messages)
			callLatency = time.Since(callStart)
			if chatErr == nil {
				break
			}
			if isTransientError(chatErr) && chatAttempt < 3 {
				r.log(ctx, task.ID, nil, "warn", fmt.Sprintf("%s: llm chat call failed (attempt %d/3) with transient error: %v. Retrying in %d seconds...", stepID, chatAttempt, chatErr, chatAttempt*2))
				if sleepErr := ctxSleep(ctx, time.Duration(chatAttempt)*2*time.Second); sleepErr != nil {
					return nil, fmt.Errorf("llm call retry backoff interrupted: %w", sleepErr)
				}
				continue
			}
			break
		}
		if chatErr != nil {
			return nil, fmt.Errorf("llm call failed: %w", chatErr)
		}
```

with:

```go
	for attempt := 1; attempt <= 3; attempt++ {
		finalAttempt = attempt
		callStart := time.Now()
		var chatErr error
		resp, chatErr = r.Provider.Chat(ctx, messages)
		callLatency = time.Since(callStart)
		if chatErr != nil {
			return nil, fmt.Errorf("llm call failed: %w", chatErr)
		}
```

- [x] **Step 3: Remove the inner retry loop in `runAgentic`'s `Chat` closure**

In the same file, replace:

```go
		Chat: func(ctx context.Context, msgs []llm.Message, opts llm.ChatOptions) (*llm.Response, error) {
			var resp *llm.Response
			var chatErr error
			for chatAttempt := 1; chatAttempt <= 3; chatAttempt++ {
				resp, chatErr = r.Provider.ChatWithOptions(ctx, msgs, opts)
				if chatErr == nil {
					break
				}
				if isTransientError(chatErr) && chatAttempt < 3 {
					r.log(ctx, task.ID, nil, "warn", fmt.Sprintf("%s: llm chat call failed (attempt %d/3) with transient error: %v. Retrying in %d seconds...", stepID, chatAttempt, chatErr, chatAttempt*2))
					if sleepErr := ctxSleep(ctx, time.Duration(chatAttempt)*2*time.Second); sleepErr != nil {
						return nil, sleepErr
					}
					continue
				}
				break
			}
			if chatErr == nil {
				lastResp = resp
			}
			return resp, chatErr
		},
```

with:

```go
		Chat: func(ctx context.Context, msgs []llm.Message, opts llm.ChatOptions) (*llm.Response, error) {
			resp, chatErr := r.Provider.ChatWithOptions(ctx, msgs, opts)
			if chatErr == nil {
				lastResp = resp
			}
			return resp, chatErr
		},
```

- [x] **Step 4: Delete now-unused helpers and their tests**

Delete the `isTransientError` and `ctxSleep` function definitions from `server/internal/orchestrator/llmrunner/runner.go` (the block starting at the `// isTransientError delegates...` comment through the end of `ctxSleep`).

Delete `TestCtxSleep_ReturnsWhenDurationElapses` and `TestCtxSleep_ReturnsImmediatelyOnCancellation` from `server/internal/orchestrator/llmrunner/runner_test.go` (lines 17-46). If `"time"` becomes unused in that test file as a result, remove the import too — check with `go build ./server/...` in Step 5.

- [x] **Step 5: Build and run the full package test suite**

Run: `go build ./server/... && go test ./server/internal/orchestrator/llmrunner/... -v`
Expected: builds cleanly (no unused imports/vars) and all tests PASS.

- [x] **Step 6: Commit**

```bash
git add server/internal/orchestrator/llmrunner/runner.go server/internal/orchestrator/llmrunner/runner_test.go
git commit -m "fix: remove runner-level transient retry, let the gateway own it exclusively"
```

---

## Phase 2: Frontend Stream Reliability (🟠 Moderate Priority)

**Note on testing:** `web/` has no unit-test runner configured (only Playwright E2E specs under `web/e2e/`, which require a running backend and are out of scope for these two narrow fixes). Both tasks below are verified manually with exact browser/devtools steps instead of an automated test — do not introduce a new test framework just for this plan.

### Task 4: Stop infinite-retrying on non-recoverable stream errors

**Problem:** `tasks.streamLogs` (`web/src/lib/api/projects.ts:759-819`) catches every non-abort error identically and retries forever with capped exponential backoff. An expired/invalid token (HTTP 401) — or a task/route that will never resolve (403/404) — causes the client to hammer the server every ~5 seconds indefinitely, with the failure never surfaced anywhere (not the console meaningfully, not the UI).

**Files:**
- Modify: `web/src/lib/api/projects.ts:759-819`
- Modify: `web/src/lib/hooks/use-task-workflow.ts:62-67`

- [x] **Step 1: Add a typed fatal-error and stop retrying on it**

In `web/src/lib/api/projects.ts`, above the `tasks` object export, add:

```ts
class StreamFatalError extends Error {
  status: number;
  constructor(status: number) {
    super(`Stream failed: ${status}`);
    this.status = status;
  }
}
```

Replace the `streamLogs` method body:

```ts
  async streamLogs(
    taskID: string,
    token: string,
    signal: AbortSignal,
    onLog: (log: TaskLog) => void
  ) {
    const API_BASE = process.env.NEXT_PUBLIC_API_URL ?? "http://localhost:32080/api/v1";
    let retryDelay = 1000;

    while (!signal.aborted) {
      try {
        const res = await fetch(`${API_BASE}/tasks/${taskID}/logs/stream`, {
          headers: { Authorization: `Bearer ${token}` },
          signal,
        });

        if (!res.ok) {
          throw new Error(`Stream failed: ${res.status}`);
        }

        retryDelay = 1000;
        if (!res.body) throw new Error("No body");
        
        const reader = res.body.getReader();
        const decoder = new TextDecoder();
        let buffer = "";

        while (true) {
          const { done, value } = await reader.read();
          if (done) break;
          buffer += decoder.decode(value, { stream: true });
          
          const lines = buffer.split("\n");
          buffer = lines.pop() ?? "";
          
          let currentEvent = "";
          for (const line of lines) {
            if (line.startsWith("event: ")) {
              currentEvent = line.slice(7).trim();
            } else if (line.startsWith("data: ")) {
              const dataStr = line.slice(6);
              if (currentEvent === "log") {
                try {
                  onLog(JSON.parse(dataStr));
                } catch (e) {}
              }
            }
          }
        }
      } catch (err: any) {
        if (err.name === "AbortError" || signal.aborted) {
          return;
        }
      }

      if (signal.aborted) return;
      
      await new Promise(resolve => setTimeout(resolve, retryDelay));
      retryDelay = Math.min(retryDelay * 2, 5000);
    }
  },
```

with:

```ts
  async streamLogs(
    taskID: string,
    token: string,
    signal: AbortSignal,
    onLog: (log: TaskLog) => void,
    onFatalError?: (err: Error) => void,
  ) {
    const API_BASE = process.env.NEXT_PUBLIC_API_URL ?? "http://localhost:32080/api/v1";
    let retryDelay = 1000;

    while (!signal.aborted) {
      try {
        const res = await fetch(`${API_BASE}/tasks/${taskID}/logs/stream`, {
          headers: { Authorization: `Bearer ${token}` },
          signal,
        });

        if (!res.ok) {
          if (res.status === 401 || res.status === 403 || res.status === 404) {
            throw new StreamFatalError(res.status);
          }
          throw new Error(`Stream failed: ${res.status}`);
        }

        retryDelay = 1000;
        if (!res.body) throw new Error("No body");

        const reader = res.body.getReader();
        const decoder = new TextDecoder();
        let buffer = "";

        while (true) {
          const { done, value } = await reader.read();
          if (done) break;
          buffer += decoder.decode(value, { stream: true });

          const lines = buffer.split("\n");
          buffer = lines.pop() ?? "";

          let currentEvent = "";
          for (const line of lines) {
            if (line.startsWith("event: ")) {
              currentEvent = line.slice(7).trim();
            } else if (line.startsWith("data: ")) {
              const dataStr = line.slice(6);
              if (currentEvent === "log") {
                try {
                  onLog(JSON.parse(dataStr));
                } catch (e) {}
              }
            }
          }
        }
      } catch (err: any) {
        if (err.name === "AbortError" || signal.aborted) {
          return;
        }
        if (err instanceof StreamFatalError) {
          onFatalError?.(err);
          return;
        }
      }

      if (signal.aborted) return;

      await new Promise(resolve => setTimeout(resolve, retryDelay));
      retryDelay = Math.min(retryDelay * 2, 5000);
    }
  },
```

- [x] **Step 2: Wire the fatal-error callback to the hook's existing error state**

In `web/src/lib/hooks/use-task-workflow.ts`, replace:

```ts
    api.streamTaskLogs(taskID, token, controller.signal, (log) => {
      pending.push(toRealtimeLog(taskID, log));
      if (flushTimer === null) {
        flushTimer = window.setTimeout(flush, 50);
      }
    }).catch(console.error);
```

with:

```ts
    api.streamTaskLogs(
      taskID,
      token,
      controller.signal,
      (log) => {
        pending.push(toRealtimeLog(taskID, log));
        if (flushTimer === null) {
          flushTimer = window.setTimeout(flush, 50);
        }
      },
      (err) => setError(`Log stream error: ${err.message}`),
    ).catch(console.error);
```

(`setError` is already destructured from `useState` at the top of the hook — no new state needed.)

- [x] **Step 3: Manually verify**

1. Run `cd web && npm run dev`.
2. Open a task page with an active (non-terminal) workflow.
3. Open DevTools → Application → session storage (or wherever the session token lives per `useSession`) and corrupt the token value to simulate expiry.
4. Trigger a re-render (e.g. navigate away and back to the task, or wait for the next natural reconnect).
5. Confirm in the Network tab: **one** failed request to `/logs/stream` returning 401, and **no further requests** to that endpoint afterward (previously: a new request every ~5s indefinitely).
6. Confirm the task page surfaces the error (via the hook's existing `error` state / wherever `TaskDetailContext` renders it).

- [x] **Step 4: Commit**

```bash
git add web/src/lib/api/projects.ts web/src/lib/hooks/use-task-workflow.ts
git commit -m "fix: stop infinite SSE reconnect loop on 401/403/404"
```

---

### Task 5: Stop reconnecting the stream on every non-terminal status change

**Problem:** The effect that owns the SSE connection (`use-task-workflow.ts:38-73`) lists `workflow?.job?.status` in its dependency array. Every status transition — including ones that stay non-terminal, like `queued`→`running`, or `paused`→`running` on each approval-gate resume — tears down and reopens the SSE connection, which re-runs `TailLogs` server-side and resends up to 500 historical log lines the client already has. The fix must not simply delete the dependency, though: the effect also uses the status to decide whether to open the stream at all or do a one-shot terminal fetch, so it must still react to the *one* transition that matters — non-terminal → terminal.

**Files:**
- Modify: `web/src/lib/hooks/use-task-workflow.ts:38-73`

- [x] **Step 1: Depend on the derived terminal boolean, not the raw status string**

Replace:

```ts
  useEffect(() => {
    if (!taskID || !token) return;
    
    const isTerminal = isWorkflowTerminal(workflow?.job?.status);
    if (isTerminal) {
      api.taskLogs(taskID, token).then(logs => {
        if (logs) appendLogs(logs.map(log => toRealtimeLog(taskID, log)));
      }).catch(console.error);
      return;
    }

    const controller = new AbortController();
    let pending: RealtimeLog[] = [];
    let flushTimer: number | null = null;

    const flush = () => {
      if (pending.length) appendLogs(pending);
      pending = [];
      if (flushTimer !== null) {
        window.clearTimeout(flushTimer);
        flushTimer = null;
      }
    };

    api.streamTaskLogs(
      taskID,
      token,
      controller.signal,
      (log) => {
        pending.push(toRealtimeLog(taskID, log));
        if (flushTimer === null) {
          flushTimer = window.setTimeout(flush, 50);
        }
      },
      (err) => setError(`Log stream error: ${err.message}`),
    ).catch(console.error);

    return () => {
      controller.abort();
      flush();
    };
  }, [taskID, token, workflow?.job?.status, appendLogs]);
```

with:

```ts
  const isTerminal = isWorkflowTerminal(workflow?.job?.status);

  useEffect(() => {
    if (!taskID || !token) return;

    if (isTerminal) {
      api.taskLogs(taskID, token).then(logs => {
        if (logs) appendLogs(logs.map(log => toRealtimeLog(taskID, log)));
      }).catch(console.error);
      return;
    }

    const controller = new AbortController();
    let pending: RealtimeLog[] = [];
    let flushTimer: number | null = null;

    const flush = () => {
      if (pending.length) appendLogs(pending);
      pending = [];
      if (flushTimer !== null) {
        window.clearTimeout(flushTimer);
        flushTimer = null;
      }
    };

    api.streamTaskLogs(
      taskID,
      token,
      controller.signal,
      (log) => {
        pending.push(toRealtimeLog(taskID, log));
        if (flushTimer === null) {
          flushTimer = window.setTimeout(flush, 50);
        }
      },
      (err) => setError(`Log stream error: ${err.message}`),
    ).catch(console.error);

    return () => {
      controller.abort();
      flush();
    };
  }, [taskID, token, isTerminal, appendLogs]);
```

(This depends on Task 4 already being applied, since it carries forward the `onFatalError` callback wiring from that task's Step 2 — apply Task 4 first.)

- [x] **Step 2: Manually verify**

1. Run `cd web && npm run dev`.
2. Open a task that requires an approval gate (pauses mid-execution, e.g. after `analyze`).
3. In DevTools → Network, filter on `logs/stream`.
4. Execute the task, let it pause, then approve/resume it.
5. Confirm **exactly one** `logs/stream` connection opens across the whole queued→running→paused→running→done lifecycle (previously: a new connection — and a fresh 500-line resend — on every status change).
6. Confirm logs still render correctly and in order throughout, including across the pause/resume boundary.

- [x] **Step 3: Commit**

```bash
git add web/src/lib/hooks/use-task-workflow.ts
git commit -m "fix: only reconnect SSE stream on the terminal transition, not every status change"
```

---

## Phase 3A: Small Mechanical Cleanups (🟡 Low Priority, safe to batch)

These three are pure refactors with no behavior change — safe to do in one sitting.

### Task 6: Use `editToolNames` instead of repeated string literals

**Problem:** `"search_replace"`/`"create_file"` are checked via literal `||` comparisons in four places, while `toolloop.go` already has `editToolNames = map[string]bool{"search_replace": true, "create_file": true}` (`server/internal/orchestrator/llmrunner/toolloop.go:42`) in the same package.

**Files:**
- Modify: `server/internal/orchestrator/llmrunner/statemachineloop.go:175, 221, 381`
- Modify: `server/internal/orchestrator/llmrunner/runner.go:285`

- [x] **Step 1:** In `statemachineloop.go:175`, replace `if (call.Name == "search_replace" || call.Name == "create_file") && len(resolvedTargets) > 0 {` with `if editToolNames[call.Name] && len(resolvedTargets) > 0 {`
- [x] **Step 2:** In `statemachineloop.go:221`, replace `if (call.Name == "search_replace" || call.Name == "create_file") && discriminator != "" {` with `if editToolNames[call.Name] && discriminator != "" {`
- [x] **Step 3:** In `statemachineloop.go:381`, replace `if call.Name == "search_replace" || call.Name == "create_file" {` with `if editToolNames[call.Name] {`
- [x] **Step 4:** In `runner.go:285`, replace `if (name == "search_replace" || name == "create_file") && len(resolvedTargets) > 0 {` with `if editToolNames[name] && len(resolvedTargets) > 0 {`
- [x] **Step 5:** Run `go test ./server/internal/orchestrator/llmrunner/... -v` — expect PASS (behavior-preserving).
- [x] **Step 6:** Commit: `git commit -m "refactor: use shared editToolNames instead of repeated literals"`

### Task 7: Fix the validation-pass/fail heuristic to use `HasPrefix`

**Problem:** `statemachineloop.go:259` checks `strings.Contains(msg.Content, "Error:")` to decide whether validation tools failed, while every other check in the same file (lines 217, 235) uses `strings.HasPrefix(result, "Error:")` — the `"Error:"` prefix is the actual tool-result error contract (`toolloop.go`'s `truncateToolResult`/tool executors prefix real errors this way). `Contains` means a passing test whose own output happens to mention the word "Error:" (e.g. a test named `TestErrorHandling` or a log line quoting an error message) gets misclassified as a validation failure.

**Files:**
- Modify: `server/internal/orchestrator/llmrunner/statemachineloop.go:259`

- [x] **Step 1:** Replace `if strings.Contains(msg.Content, "Error:") {` with `if strings.HasPrefix(msg.Content, "Error:") {`
- [x] **Step 2:** Run `go test ./server/internal/orchestrator/llmrunner/... -v` — expect PASS.
- [x] **Step 3:** Commit: `git commit -m "fix: use HasPrefix for validation-tool error detection, matching the rest of the file"`

### Task 8: Make `updateShadowSM` a method on `Runner` with idiomatic parameter order

**Problem:** `updateShadowSM(shadowSM *StateMachine, resp *llm.Response, resolvedTargets []string, r Runner, ctx context.Context, taskID string)` (`statemachineloop.go:369`) takes `ctx` fourth-to-last instead of first, and takes `r Runner` as a plain parameter instead of being a method — inconsistent with every other function in this file (`r.log`, `r.truncateToolResult`, `r.saveExecutionSnapshot` are all methods with `ctx` as the first real parameter).

**Files:**
- Modify: `server/internal/orchestrator/llmrunner/statemachineloop.go:369`
- Modify: `server/internal/orchestrator/llmrunner/runner.go:312`

- [x] **Step 1:** In `statemachineloop.go`, change the signature from:
  ```go
  func updateShadowSM(shadowSM *StateMachine, resp *llm.Response, resolvedTargets []string, r Runner, ctx context.Context, taskID string) {
  ```
  to:
  ```go
  func (r Runner) updateShadowSM(ctx context.Context, shadowSM *StateMachine, resp *llm.Response, resolvedTargets []string, taskID string) {
  ```
- [x] **Step 2:** In `runner.go:312`, change `updateShadowSM(shadowSM, resp, resolvedTargets, r, ctx, task.ID)` to `r.updateShadowSM(ctx, shadowSM, resp, resolvedTargets, task.ID)`
- [x] **Step 3:** Run `go build ./server/... && go test ./server/internal/orchestrator/llmrunner/... -v` — expect clean build and PASS.
- [x] **Step 4:** Commit: `git commit -m "refactor: make updateShadowSM a Runner method with ctx-first signature"`

---

## Phase 3B: Deferred — needs its own plan before starting

The remaining four items from the original report are genuine architectural refactors, not mechanical fixes, and each has open design questions this plan cannot answer without guessing:

- **Gateway complexity / typed rate-limit detection** (`server/internal/gateway/gateway.go` — 185-line `ChatWithOptions`, string-matching on `"429"`/`"quota"`). Extracting the single-credential attempt into a helper is safe; switching rate-limit detection to `HTTPStatusError` needs a decision on which providers' errors actually get wrapped as `HTTPStatusError` today vs. which still only offer a raw error string (verify per-provider before assuming full coverage).
- **Tool-loop duplication** (`toolloop.go` `RunToolLoop` vs. `statemachineloop.go` `runStateMachine`). A shared `ToolCallHandler` needs its interface designed first — the two loops differ in what they do on a blocked tool call (state-machine records `ToolCallRecord` telemetry; the plain tool loop doesn't), so a naive extraction will either lose that telemetry or leak state-machine concepts into the generic loop.
- **`json.go` custom parser → structured outputs.** Moving `requiresStrictJSON` steps to provider-native JSON-schema/forced-tool-call output requires confirming schema support across every configured provider/model combination in `credential_pool.go`'s seed list, and defining the fallback behavior for providers that don't support it. That's a design doc, not a mechanical patch.
- **`prompts/builder.go` split + `optimizeBudget` truncation.** Splitting skill-parsing into a subpackage is mechanical, but making `optimizeBudget` truncate (not just drop) sections requires deciding a truncation strategy per section type (e.g. is truncating the repo map safe, or does it need to stay atomic?) — a real design decision, not a rename.

Recommendation: write a separate `docs/plans/PLAN-*.md` for each (or one plan covering all four if they're small enough) once those questions are answered — don't fold them into this plan's task list as unexamined code.

The model-hardcoding item (`seedDefaultModels` in `credential_pool.go`) is similarly deferred, but for a narrower reason: moving it out of Go code into `config.yaml` (as originally suggested) raises a product question — should every org share one hardcoded default list, or should this be operator-configurable without a redeploy? That's worth a one-line decision from whoever owns `credential_pool.go` before writing the task.

---

## Execution Order

1. Phase 1, Tasks 1–3, in order (each is independent of the others but all are backend/Go — land them before touching the frontend).
2. Phase 2, Task 4 then Task 5 (Task 5 builds on Task 4's callback signature).
3. Phase 3A, Tasks 6–8, any order, can be batched into one PR.
4. Phase 3B: do not start until each item has its own reviewed plan.
