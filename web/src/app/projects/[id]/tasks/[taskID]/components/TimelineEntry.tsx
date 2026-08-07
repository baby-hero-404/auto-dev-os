"use client";

import { useState } from "react";
import { Loader2 } from "lucide-react";
import { UnknownEventCard } from "./UnknownEventCard";
import { useTaskDetail } from "./TaskDetailContext";
import { tasks as tasksApi } from "@/lib/api/projects";
import type {
  TaskEvent,
  AgentReasoningSummaryPayload,
  ToolStartedPayload,
  ToolFinishedPayload,
  FileChangedPayload,
  CommandStartedPayload,
  CommandFinishedPayload,
  TestResultPayload,
  AgentMessagePayload,
} from "@/lib/types/task-event";

const KNOWN_RENDERERS: Record<string, number> = {
  "agent.reasoning_summary": 1,
  "tool.started": 1,
  "tool.finished": 1,
  "file.changed": 1,
  "command.started": 1,
  "command.finished": 1,
  "test.result": 1,
  "agent.message": 1,
};

function CollapsibleOutput({ text }: { text: string }) {
  const lines = text.split("\n");
  const isLong = lines.length > 3;
  const [expanded, setExpanded] = useState(!isLong);
  const displayed = expanded ? text : lines.slice(0, 3).join("\n");
  return (
    <div className="mt-2">
      <pre className="overflow-x-auto whitespace-pre-wrap rounded-lg bg-surface/50 border border-stroke/20 p-3 text-[12px] text-content-muted shadow-inner">
        {displayed}
        {!expanded && isLong ? "\n…" : ""}
      </pre>
      {isLong && (
        <button
          type="button"
          className="mt-1.5 text-[12px] font-semibold text-brand-primary hover:underline"
          onClick={() => setExpanded((v) => !v)}
        >
          {expanded ? "Collapse" : "Expand"}
        </button>
      )}
    </div>
  );
}

function ArtifactOutput({ artifactID }: { artifactID: string }) {
  const { taskID, token } = useTaskDetail();
  const [content, setContent] = useState<unknown>(null);
  const [loading, setLoading] = useState(false);
  const [loaded, setLoaded] = useState(false);

  const load = async () => {
    setLoading(true);
    try {
      const list = await tasksApi.artifactsByTask(taskID, token);
      const found = list.find((a) => a.id === artifactID);
      setContent(found?.payload ?? "Artifact not found");
      setLoaded(true);
    } catch {
      setContent("Failed to load artifact");
      setLoaded(true);
    } finally {
      setLoading(false);
    }
  };

  if (!loaded) {
    return (
      <button
        type="button"
        onClick={load}
        disabled={loading}
        className="mt-2 inline-flex items-center gap-1.5 text-[12px] font-semibold text-brand-primary hover:underline disabled:opacity-50"
      >
        {loading && <Loader2 className="h-3.5 w-3.5 animate-spin" />}
        View full output
      </button>
    );
  }

  return (
    <pre className="mt-2 overflow-x-auto whitespace-pre-wrap rounded-lg bg-surface/50 border border-stroke/20 p-3 text-[12px] text-content-muted shadow-inner">
      {typeof content === "string" ? content : JSON.stringify(content, null, 2)}
    </pre>
  );
}

// Formats the gap since the previous event in the trace, e.g. "+340ms",
// "+2.1s", "+1m 05s" — the core value of a time *trace* over a plain list
// of timestamps is showing pacing/gaps between agent actions.
export function formatElapsed(ms: number): string {
  if (ms < 1000) return `+${Math.round(ms)}ms`;
  const totalSeconds = ms / 1000;
  if (totalSeconds < 60) return `+${totalSeconds.toFixed(1)}s`;
  const totalMinutes = Math.floor(totalSeconds / 60);
  const seconds = Math.floor(totalSeconds % 60);
  if (totalMinutes < 60) return `+${totalMinutes}m ${String(seconds).padStart(2, "0")}s`;
  const hours = Math.floor(totalMinutes / 60);
  const minutes = totalMinutes % 60;
  return `+${hours}h ${String(minutes).padStart(2, "0")}m`;
}

// Shared timestamp display: short local time visible, full absolute
// date+time on hover (tooltip), plus an elapsed-since-previous-event badge
// when a previous timestamp is available.
export function EntryTimestamp({
  timestamp,
  previousTimestamp,
}: {
  timestamp: string;
  previousTimestamp?: string;
}) {
  const date = new Date(timestamp);
  const elapsedMs = previousTimestamp ? date.getTime() - new Date(previousTimestamp).getTime() : null;
  return (
    <span className="flex shrink-0 items-center gap-1.5 text-[12px] text-content-muted">
      {elapsedMs !== null && elapsedMs >= 0 && (
        <span className="font-mono text-content-muted/80">{formatElapsed(elapsedMs)}</span>
      )}
      <span title={date.toLocaleString()}>{date.toLocaleTimeString()}</span>
    </span>
  );
}

function EntryShell({
  icon,
  title,
  timestamp,
  previousTimestamp,
  children,
  semanticStyle = "neutral",
}: {
  icon: React.ReactNode;
  title: React.ReactNode;
  timestamp: string;
  previousTimestamp?: string;
  children?: React.ReactNode;
  semanticStyle?: "error" | "warning" | "success" | "info" | "neutral" | "agent";
}) {
  const styles = {
    error: "border-red-500/20 bg-red-500/5",
    warning: "border-amber-500/20 bg-amber-500/5",
    success: "border-emerald-500/20 bg-emerald-500/5",
    info: "border-blue-500/20 bg-blue-500/5",
    agent: "border-brand-primary/20 bg-brand-primary/5 shadow-sm",
    neutral: "border-stroke/30 bg-card/40 hover:bg-card/60 transition-colors",
  };

  return (
    <div className={`rounded-xl border ${styles[semanticStyle]} p-3.5`}>
      <div className="flex items-start justify-between gap-3">
        <div className="flex items-start gap-2.5">
          <span aria-hidden className="mt-0.5">{icon}</span>
          <span className="text-[14px] font-medium text-foreground leading-snug">{title}</span>
        </div>
        <EntryTimestamp timestamp={timestamp} previousTimestamp={previousTimestamp} />
      </div>
      {children}
    </div>
  );
}

export function TimelineEntry({ event, previousTimestamp }: { event: TaskEvent; previousTimestamp?: string }) {
  if (!KNOWN_RENDERERS[event.type]) {
    return <UnknownEventCard event={event} previousTimestamp={previousTimestamp} />;
  }

  switch (event.type) {
    case "agent.reasoning_summary": {
      const p = event.payload as unknown as AgentReasoningSummaryPayload;
      return <EntryShell semanticStyle="agent" icon="🧠" title={p.summary} timestamp={event.created_at} previousTimestamp={previousTimestamp} />;
    }
    case "agent.message": {
      const p = event.payload as unknown as AgentMessagePayload;
      return <EntryShell semanticStyle="agent" icon="💬" title={p.text} timestamp={event.created_at} previousTimestamp={previousTimestamp} />;
    }
    case "tool.started": {
      const p = event.payload as unknown as ToolStartedPayload;
      return (
        <EntryShell semanticStyle="neutral" icon="🛠" title={<span className="font-mono text-xs">{p.tool}</span>} timestamp={event.created_at} previousTimestamp={previousTimestamp}>
          <CollapsibleOutput text={p.input} />
          {event.artifact_id && <ArtifactOutput artifactID={event.artifact_id} />}
        </EntryShell>
      );
    }
    case "tool.finished": {
      const p = event.payload as unknown as ToolFinishedPayload;
      const successStyle = p.success ? "success" : "error";
      return (
        <EntryShell semanticStyle={successStyle} icon="🛠" title={<><span className="font-mono text-xs">{p.tool}</span> <span className={p.success ? "text-emerald-500" : "text-red-500"}>{p.success ? "✓" : "✗"}</span> ({p.duration_ms}ms)</>} timestamp={event.created_at} previousTimestamp={previousTimestamp}>
          <CollapsibleOutput text={p.output} />
          {event.artifact_id && <ArtifactOutput artifactID={event.artifact_id} />}
        </EntryShell>
      );
    }
    case "file.changed": {
      const p = event.payload as unknown as FileChangedPayload;
      return (
        <EntryShell
          semanticStyle="info"
          icon="📄"
          title={<><span className="font-mono text-xs">{p.path}</span> <span className="text-emerald-500 text-xs">(+{p.additions})</span> <span className="text-red-500 text-xs">(-{p.deletions})</span></>}
          timestamp={event.created_at}
          previousTimestamp={previousTimestamp}
        />
      );
    }
    case "command.started": {
      const p = event.payload as unknown as CommandStartedPayload;
      return <EntryShell semanticStyle="neutral" icon="💻" title={<span className="font-mono text-xs bg-surface/50 px-1 py-0.5 rounded">{p.command}</span>} timestamp={event.created_at} previousTimestamp={previousTimestamp} />;
    }
    case "command.finished": {
      const p = event.payload as unknown as CommandFinishedPayload;
      const successStyle = p.exit_code === 0 ? "success" : "error";
      return (
        <EntryShell semanticStyle={successStyle} icon="💻" title={<><span className="font-mono text-xs bg-surface/50 px-1 py-0.5 rounded">{p.command}</span> (exit {p.exit_code})</>} timestamp={event.created_at} previousTimestamp={previousTimestamp}>
          <CollapsibleOutput text={`${p.stdout_tail}${p.stderr_tail ? `\n${p.stderr_tail}` : ""}`} />
          {event.artifact_id && <ArtifactOutput artifactID={event.artifact_id} />}
        </EntryShell>
      );
    }
    case "test.result": {
      const p = event.payload as unknown as TestResultPayload;
      const icon = p.failed > 0 ? "❌" : "✅";
      const style = p.failed > 0 ? "error" : "success";
      return (
        <EntryShell semanticStyle={style} icon={icon} title={`${p.passed} passed, ${p.failed} failed, ${p.skipped} skipped`} timestamp={event.created_at} previousTimestamp={previousTimestamp}>
          {p.details && <CollapsibleOutput text={p.details} />}
        </EntryShell>
      );
    }
    default:
      return <UnknownEventCard event={event} previousTimestamp={previousTimestamp} />;
  }
}
