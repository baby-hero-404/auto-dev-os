"use client";

import { useEffect, useRef, useState, useCallback } from "react";
import { TimelineEntry, formatElapsed } from "./TimelineEntry";
import { CheckpointsPanel } from "./CheckpointsPanel";
import { FilesChangedPanel } from "./FilesChangedPanel";
import { useTaskDetail } from "./TaskDetailContext";
import { tasks as tasksApi } from "@/lib/api/projects";
import { isActiveStatus } from "@/lib/status";
import type { TaskEvent, TestResultPayload } from "@/lib/types/task-event";

// Dedup/order by sequence_number (the ordering key per design.md), not
// event.id or insertion order — events can arrive out of order across a
// history-fetch + stream-connect boundary. Called on every single incoming
// SSE event, so the common case (each new event's sequence_number is higher
// than everything seen so far) fast-paths to a plain append instead of
// rebuilding a Map + sorting the whole array — otherwise this is O(n) per
// event, O(n^2) over a long-running task with hundreds/thousands of events.
function mergeEvents(existing: TaskEvent[], incoming: TaskEvent[]): TaskEvent[] {
  const lastExistingSeq = existing.length > 0 ? existing[existing.length - 1].sequence_number : -Infinity;
  const isInOrderAppend = incoming.every((e, i) => {
    const prevSeq = i === 0 ? lastExistingSeq : incoming[i - 1].sequence_number;
    return e.sequence_number > prevSeq;
  });
  if (isInOrderAppend) {
    return [...existing, ...incoming];
  }

  const bySeq = new Map<number, TaskEvent>();
  for (const e of existing) bySeq.set(e.sequence_number, e);
  for (const e of incoming) bySeq.set(e.sequence_number, e);
  return Array.from(bySeq.values()).sort((a, b) => a.sequence_number - b.sequence_number);
}

// Live-ticking "running for Xm Ys" clock for the header — only ticks while
// the task is actually active, so a finished task's timeline shows a fixed
// total duration rather than a clock that runs forever.
function useRunningClock(active: boolean) {
  const [now, setNow] = useState(() => Date.now());
  useEffect(() => {
    if (!active) return;
    const id = setInterval(() => setNow(Date.now()), 1000);
    return () => clearInterval(id);
  }, [active]);
  return now;
}

function formatDuration(ms: number): string {
  return formatElapsed(ms).slice(1);
}

export function AgentTimeline() {
  const { taskID, token, task, workflow } = useTaskDetail();
  const [events, setEvents] = useState<TaskEvent[]>([]);
  const [loading, setLoading] = useState(true);
  const [connected, setConnected] = useState(false);
  const [userScrolledUp, setUserScrolledUp] = useState(false);
  const scrollRef = useRef<HTMLDivElement | null>(null);
  const cursorRef = useRef(0);
  const [hasConnectedOnce, setHasConnectedOnce] = useState(false);

  useEffect(() => {
    let cancelled = false;
    tasksApi
      .events(taskID, token)
      .then((history) => {
        if (cancelled) return;
        // history arrives most-recent-first (before/limit cursor pagination);
        // display wants ascending sequence order.
        const ascending = [...history].sort((a, b) => a.sequence_number - b.sequence_number);
        setEvents(ascending);
        cursorRef.current = ascending.length > 0 ? ascending[ascending.length - 1].sequence_number : 0;
      })
      .catch(() => {})
      .finally(() => {
        if (!cancelled) setLoading(false);
      });
    return () => {
      cancelled = true;
    };
  }, [taskID, token]);

  useEffect(() => {
    if (loading) return;
    const controller = new AbortController();
    tasksApi.streamEvents(
      taskID,
      token,
      cursorRef.current,
      controller.signal,
      (event) => {
        setEvents((prev) => mergeEvents(prev, [event]));
      },
      undefined,
      (isConnected) => {
        if (isConnected) setHasConnectedOnce(true);
        setConnected(isConnected);
      },
    );
    return () => controller.abort();
  }, [taskID, token, loading]);

  const handleScroll = useCallback(() => {
    const el = scrollRef.current;
    if (!el) return;
    const atBottom = el.scrollHeight - el.scrollTop - el.clientHeight < 40;
    setUserScrolledUp(!atBottom);
  }, []);

  useEffect(() => {
    if (userScrolledUp) return;
    const el = scrollRef.current;
    if (el) el.scrollTop = el.scrollHeight;
  }, [events, userScrolledUp]);

  const scrollToBottom = () => {
    const el = scrollRef.current;
    if (el) el.scrollTop = el.scrollHeight;
    setUserScrolledUp(false);
  };

  const isActive = task ? isActiveStatus(task.status) : false;
  const now = useRunningClock(isActive);
  const firstEvent = events[0];
  const lastEvent = events[events.length - 1];
  const totalDurationMs = firstEvent
    ? (isActive ? now : new Date(lastEvent.created_at).getTime()) - new Date(firstEvent.created_at).getTime()
    : null;
  const latestTestResult = [...events].reverse().find((e) => e.type === "test.result");
  const testStats = latestTestResult ? (latestTestResult.payload as unknown as TestResultPayload) : null;

  return (
    <div className="relative flex h-full flex-col">
      <div className="flex items-center justify-between border-b border-stroke px-3 py-2">
        <h3 className="text-sm font-medium text-content">Activity</h3>
        <div className="flex items-center gap-2">
          {totalDurationMs !== null && totalDurationMs >= 0 && (
            <span className="text-[11px] text-content-secondary" title={`Started ${new Date(firstEvent.created_at).toLocaleString()}`}>
              {isActive ? "running " : ""}
              {formatDuration(totalDurationMs)}
            </span>
          )}
          {testStats && (
            <span
              className={`rounded-full px-2 py-0.5 text-[11px] font-medium ${
                testStats.failed > 0 ? "bg-red-500/10 text-red-500" : "bg-emerald-500/10 text-emerald-500"
              }`}
              title={`${testStats.passed} passed, ${testStats.failed} failed, ${testStats.skipped} skipped (latest run)`}
            >
              {testStats.failed > 0 ? "✗" : "✓"} {testStats.passed}/{testStats.passed + testStats.failed + testStats.skipped}
            </span>
          )}
          {!connected && !loading && hasConnectedOnce && (
            <span className="text-[11px] text-content-secondary">Reconnecting…</span>
          )}
        </div>
      </div>
      <div
        ref={scrollRef}
        onScroll={handleScroll}
        className="flex-1 space-y-2 overflow-y-auto p-3"
      >
        {loading && <p className="text-xs text-content-secondary">Loading activity…</p>}
        {!loading && events.length === 0 && (workflow?.checkpoints?.length ?? 0) > 0 && (
          <CheckpointsPanel />
        )}
        {!loading && events.length === 0 && (workflow?.checkpoints?.length ?? 0) === 0 && (
          <p className="text-xs text-content-secondary">No activity yet.</p>
        )}
        {events.length > 0 && <FilesChangedPanel events={events} />}
        {events.map((event, i) => {
          const previous = i > 0 ? events[i - 1] : undefined;
          const isNewDay =
            previous && new Date(previous.created_at).toDateString() !== new Date(event.created_at).toDateString();
          return (
            <div key={event.sequence_number}>
              {isNewDay && (
                <div className="my-2 flex items-center gap-2 text-[10px] uppercase tracking-wide text-content-secondary/70">
                  <span className="h-px flex-1 bg-stroke" />
                  {new Date(event.created_at).toLocaleDateString(undefined, { month: "short", day: "numeric", year: "numeric" })}
                  <span className="h-px flex-1 bg-stroke" />
                </div>
              )}
              <TimelineEntry event={event} previousTimestamp={previous?.created_at} />
            </div>
          );
        })}
      </div>
      {userScrolledUp && (
        <button
          type="button"
          onClick={scrollToBottom}
          className="absolute bottom-4 right-4 rounded-full bg-brand-primary px-3 py-1 text-xs text-background shadow"
        >
          Scroll to bottom
        </button>
      )}
    </div>
  );
}
