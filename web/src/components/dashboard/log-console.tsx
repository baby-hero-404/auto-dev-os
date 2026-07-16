import { useState, useMemo } from "react";
import { TerminalSquare, ChevronRight, ChevronDown, CheckCircle2, XCircle, Pause, Loader2 } from "lucide-react";
import type { RealtimeLog } from "@/lib/store/use-realtime-log-store";
import { Virtuoso } from "react-virtuoso";

export type LogGroupStatus = "running" | "success" | "failed" | "paused";

export type GroupedLogItem =
  | { type: "log"; log: RealtimeLog }
  | { type: "group"; key: string; stepName: string; status: LogGroupStatus; logs: RealtimeLog[] };

export type FlattenedLogItem =
  | { type: "log"; log: RealtimeLog; isGroupChild?: boolean }
  | { type: "group"; key: string; stepName: string; status: LogGroupStatus; logs: RealtimeLog[]; isExpanded: boolean; defaultExpanded: boolean; originalIndex: number };

function groupLogs(logs: RealtimeLog[]): GroupedLogItem[] {
  const result: GroupedLogItem[] = [];
  let currentGroup: { key: string; stepName: string; status: LogGroupStatus; logs: RealtimeLog[] } | null = null;
  let groupCount = 0;

  for (const log of logs) {
    const startMatch = log.message.match(/\[#\d+\] step ([\w-]+) running/);
    if (startMatch) {
      if (currentGroup) {
        result.push({ type: "group", ...currentGroup });
      }
      groupCount++;
      currentGroup = {
        key: `${startMatch[1]}_${groupCount}`,
        stepName: startMatch[1],
        status: "running",
        logs: [log]
      };
      continue;
    }

    if (currentGroup) {
      const endMatch = log.message.match(/\[#\d+\] step ([\w-]+) (success|failed|paused)/);
      currentGroup.logs.push(log);

      if (endMatch && endMatch[1] === currentGroup.stepName) {
        currentGroup.status = endMatch[2] as LogGroupStatus;
        result.push({ type: "group", ...currentGroup });
        currentGroup = null;
      }
    } else {
      result.push({ type: "log", log });
    }
  }

  if (currentGroup) {
    result.push({ type: "group", ...currentGroup });
  }

  return result;
}

function LogMessage({ message }: { message: string }) {
  const [isExpanded, setIsExpanded] = useState(false);
  const lines = message.split("\n");
  const isLong = lines.length > 6 || message.length > 500;

  if (!isLong) {
    return <span className="whitespace-pre-wrap text-slate-800 dark:text-slate-100 break-words">{message}</span>;
  }

  const previewLines = lines.slice(0, 5).join("\n");
  const displayedText = isExpanded ? message : previewLines;

  // Check if it looks like code, JSON, diff, or raw trace
  const isCodeOrJson =
    message.includes("```") ||
    message.trim().startsWith("{") ||
    message.trim().startsWith("[") ||
    lines.some(l => l.startsWith("    ") || l.startsWith("\t") || l.startsWith("+ ") || l.startsWith("- "));

  return (
    <div className="flex flex-col gap-1 w-full">
      <div className={`whitespace-pre-wrap text-slate-800 dark:text-slate-100 break-words font-mono ${isCodeOrJson || isExpanded
          ? "bg-slate-100/50 dark:bg-slate-900/50 border border-stroke/40 rounded p-2 max-h-[300px] overflow-y-auto custom-scrollbar text-[11px] leading-relaxed"
          : ""
        }`}>
        {displayedText}
        {!isExpanded && <span className="text-content-muted">...</span>}
      </div>
      <button
        onClick={() => setIsExpanded(!isExpanded)}
        className="text-[10px] font-bold text-brand-primary hover:text-brand-primary/80 transition-colors self-start mt-0.5 cursor-pointer"
      >
        {isExpanded ? "Show less" : `Show more (${lines.length - 5} lines)`}
      </button>
    </div>
  );
}

export interface MilestoneItem {
  id: string;
  timestamp: string;
  message: string;
  type: "success" | "running" | "failed" | "info" | "paused";
}

export function parseMilestones(logs: RealtimeLog[]): MilestoneItem[] {
  const milestones: MilestoneItem[] = [];
  
  for (const log of logs) {
    const msg = log.message;
    
    const stepMatch = msg.match(/step ([\w-]+) (success|failed|running)/i);
    const checkpointMatch = msg.match(/checkpoint/i);
    const pauseResumeMatch = msg.match(/paused|resumed/i);
    const workflowMatch = msg.match(/workflow (failed|completed)/i);
    const errorLevel = log.level === "error";

    let type: "success" | "running" | "failed" | "info" | "paused" = "info";
    let isMilestone = false;
    let message = msg;

    if (workflowMatch) {
      isMilestone = true;
      const status = workflowMatch[1].toLowerCase();
      type = status === "completed" ? "success" : "failed";
      message = `Workflow ${status === "completed" ? "Completed Successfully" : "Failed"}`;
    } else if (stepMatch) {
      isMilestone = true;
      const stepName = stepMatch[1].replace(/_/g, " ");
      const action = stepMatch[2].toLowerCase();
      type = action === "success" ? "success" : action === "failed" ? "failed" : "running";
      message = `Step "${stepName}" ${action}`;
    } else if (checkpointMatch) {
      isMilestone = true;
      type = "success";
      message = msg.replace(/\[#\d+\]\s*/, "");
    } else if (pauseResumeMatch) {
      isMilestone = true;
      type = msg.toLowerCase().includes("paused") ? "paused" : "running";
      message = msg.replace(/\[#\d+\]\s*/, "");
    } else if (errorLevel) {
      isMilestone = true;
      type = "failed";
    }

    if (isMilestone) {
      milestones.push({
        id: `${log.createdAtEpoch}-${milestones.length}`,
        timestamp: new Date(log.createdAtEpoch).toLocaleTimeString(),
        message,
        type
      });
    }
  }
  
  return milestones;
}

interface LogConsoleProps {
  logs: RealtimeLog[];
  isWorkflowRunning?: boolean;
  /** Controlled collapse state (REQ-006). When omitted, the console self-manages. */
  isExpanded?: boolean;
  onToggle?: () => void;
}

export function LogConsole({ logs, isWorkflowRunning, isExpanded, onToggle }: LogConsoleProps) {
  const [expanded, setExpanded] = useState<Record<string, boolean>>({});
  const [viewMode, setViewMode] = useState<"milestones" | "all">(
    isWorkflowRunning ? "milestones" : "all"
  );

  // Collapse control: default-collapsed. Uncontrolled fallback when no props given.
  const [internalOpen, setInternalOpen] = useState(false);
  const isOpen = isExpanded ?? internalOpen;
  const toggleOpen = onToggle ?? (() => setInternalOpen((v) => !v));

  const milestones = useMemo(() => parseMilestones(logs), [logs]);

  // Latest event for the collapsed summary — recomputed every render so it stays
  // live while the workflow runs even when the console is closed (REQ-006).
  const latestMilestone = milestones.length > 0 ? milestones[milestones.length - 1] : null;
  const latestEventText = latestMilestone
    ? latestMilestone.message
    : logs.length > 0
      ? logs[logs.length - 1].message
      : "No logs yet";
  const latestEventType = latestMilestone?.type ?? "info";
  const latestDotColor =
    latestEventType === "success" ? "bg-emerald-500" :
      latestEventType === "running" ? "bg-sky-500 animate-pulse" :
        latestEventType === "failed" ? "bg-rose-500" :
          latestEventType === "paused" ? "bg-amber-500" : "bg-slate-400";

  const toggleGroup = (key: string, defaultExpanded: boolean) => {
    setExpanded((prev) => ({
      ...prev,
      [key]: prev[key] === undefined ? !defaultExpanded : !prev[key]
    }));
  };

  const flattenedItems = useMemo(() => {
    const items: FlattenedLogItem[] = [];
    const grouped = groupLogs(logs);

    grouped.forEach((item, index) => {
      if (item.type === "log") {
        items.push(item);
      } else {
        const defaultExpanded = item.status === "running" || item.status === "failed";
        const isExpanded = expanded[item.key] ?? defaultExpanded;

        items.push({ ...item, isExpanded, defaultExpanded, originalIndex: index });
        if (isExpanded) {
          items.push(...item.logs.map((log) => ({ type: "log" as const, log, isGroupChild: true })));
        }
      }
    });
    return items;
  }, [logs, expanded]);

  const getStatusIcon = (status: LogGroupStatus) => {
    switch (status) {
      case "running":
        return <Loader2 size={12} className="animate-spin text-blue-500" />;
      case "success":
        return <CheckCircle2 size={12} className="text-emerald-500" />;
      case "failed":
        return <XCircle size={12} className="text-red-500" />;
      case "paused":
        return <Pause size={12} className="text-amber-500" />;
      default:
        return null;
    }
  };

  return (
    <div id="log-console" className={`rounded-lg border border-stroke bg-panel p-5 flex flex-col ${isOpen ? "h-full min-h-[400px]" : ""}`}>
      <div className={`flex flex-wrap items-center justify-between gap-4 ${isOpen ? "mb-4" : ""}`}>
        <button
          type="button"
          onClick={toggleOpen}
          className="flex items-center gap-2 cursor-pointer text-left"
          aria-expanded={isOpen}
        >
          {isOpen ? <ChevronDown size={18} className="text-content-muted" /> : <ChevronRight size={18} className="text-content-muted" />}
          <TerminalSquare size={18} className="text-brand-primary" />
          <h2 className="font-mono text-lg font-semibold text-foreground dark:text-white">Execution Logs</h2>
        </button>

        {isOpen && logs.length > 0 && (
          <div className="flex rounded-lg border border-stroke bg-surface/80 p-0.5 text-[10.5px] font-semibold">
            <button
              onClick={() => setViewMode("milestones")}
              className={`rounded px-3 py-1 cursor-pointer transition-all ${
                viewMode === "milestones"
                  ? "bg-card text-brand-primary shadow-sm border border-stroke/10"
                  : "text-content-muted hover:text-foreground border border-transparent"
              }`}
            >
              Milestones
            </button>
            <button
              onClick={() => setViewMode("all")}
              className={`rounded px-3 py-1 cursor-pointer transition-all ${
                viewMode === "all"
                  ? "bg-card text-brand-primary shadow-sm border border-stroke/10"
                  : "text-content-muted hover:text-foreground border border-transparent"
              }`}
            >
              All Logs
            </button>
          </div>
        )}
      </div>

      {!isOpen && (
        <button
          type="button"
          onClick={toggleOpen}
          className="mt-3 flex w-full items-center gap-2.5 rounded-md border border-stroke/50 bg-surface/30 px-3 py-2.5 text-left transition hover:bg-surface/60 cursor-pointer"
        >
          <span className={`h-2 w-2 shrink-0 rounded-full ${latestDotColor}`} />
          <span className="min-w-0 flex-1 truncate font-mono text-xs text-content-muted">{latestEventText}</span>
          <span className="shrink-0 text-[10px] font-bold uppercase tracking-wider text-brand-primary">View full log</span>
        </button>
      )}

      <div className={`flex-1 overflow-hidden rounded-md bg-slate-50 dark:bg-slate-950 border border-stroke min-h-[520px] ${isOpen ? "" : "hidden"}`}>
        {logs.length === 0 ? (
          <p className="text-content-muted p-4 font-mono text-xs">No logs yet. Execute the workflow to start.</p>
        ) : viewMode === "milestones" ? (
          <div className="h-full w-full overflow-y-auto p-4 custom-scrollbar flex flex-col gap-4">
            {milestones.length === 0 ? (
              <p className="text-content-muted font-mono text-xs text-center py-8">No milestones recorded yet.</p>
            ) : (
              <div className="relative border-l border-stroke/70 pl-6 space-y-5 ml-2 pt-2 text-left">
                {milestones.map((m) => {
                  let dotBg = "bg-slate-400";
                  let borderStyle = "border-stroke";
                  let textColor = "text-foreground";
                  
                  if (m.type === "success") {
                    dotBg = "bg-emerald-500";
                    borderStyle = "border-emerald-500/30";
                    textColor = "text-emerald-700 dark:text-emerald-300 font-semibold";
                  } else if (m.type === "running") {
                    dotBg = "bg-sky-500 animate-pulse";
                    borderStyle = "border-sky-500/30";
                    textColor = "text-sky-700 dark:text-sky-300 font-semibold";
                  } else if (m.type === "failed") {
                    dotBg = "bg-rose-500";
                    borderStyle = "border-rose-500/30";
                    textColor = "text-rose-700 dark:text-rose-300 font-bold";
                  } else if (m.type === "paused") {
                    dotBg = "bg-amber-500";
                    borderStyle = "border-amber-500/30";
                    textColor = "text-amber-700 dark:text-amber-300 font-semibold";
                  }

                  return (
                    <div key={m.id} className="relative flex flex-col gap-1">
                      {/* Dot */}
                      <span className={`absolute -left-[30px] top-1 flex h-4 w-4 items-center justify-center rounded-full bg-panel border-2 ${borderStyle}`}>
                        <span className={`h-1.5 w-1.5 rounded-full ${dotBg}`} />
                      </span>
                      <div className="flex items-center justify-between gap-4">
                        <span className={`text-xs ${textColor}`}>{m.message}</span>
                        <span className="text-[10px] font-mono text-content-muted/80 whitespace-nowrap">{m.timestamp}</span>
                      </div>
                    </div>
                  );
                })}
              </div>
            )}
          </div>
        ) : (
          <Virtuoso
            data={flattenedItems}
            className="h-full w-full font-mono text-xs"
            followOutput="smooth"
            itemContent={(_, item) => {
              if (item.type === "group") {
                const Icon = item.isExpanded ? ChevronDown : ChevronRight;
                const statusColor =
                  item.status === "running" ? "text-blue-500" :
                    item.status === "success" ? "text-emerald-500" :
                      item.status === "failed" ? "text-red-500" : "text-amber-500";

                return (
                  <div
                    id={`log-group-${item.stepName}`}
                    className="flex items-center gap-2 px-4 py-2 border-b border-stroke/50 bg-slate-100 dark:bg-slate-900 cursor-pointer hover:bg-slate-200 dark:hover:bg-slate-800 transition-colors"
                    onClick={() => toggleGroup(item.key, item.defaultExpanded)}
                  >
                    <Icon size={14} className="text-slate-500" />
                    <span className="font-semibold text-slate-800 dark:text-slate-200">Step: {item.stepName}</span>
                    <div className="ml-auto flex items-center gap-1.5">
                      {getStatusIcon(item.status)}
                      <span className={`font-bold uppercase text-[10px] ${statusColor}`}>{item.status}</span>
                    </div>
                  </div>
                );
              }

              const log = item.log as RealtimeLog;
              const levelStyle =
                log.level === "error" ? "bg-red-500/10 text-red-600 dark:text-red-400 border border-red-500/20" :
                  log.level === "warn" ? "bg-amber-500/10 text-amber-600 dark:text-amber-400 border border-amber-500/20" :
                    log.level === "debug" ? "bg-slate-500/10 text-slate-600 dark:text-slate-400 border border-slate-500/20" :
                      "bg-emerald-500/10 text-emerald-600 dark:text-emerald-400 border border-emerald-500/20";

              return (
                <div className={`grid gap-2 border-b border-stroke/20 py-2 px-4 md:grid-cols-[120px_80px_1fr] ${item.isGroupChild ? 'bg-slate-50/50 dark:bg-slate-950/50 border-l-2 border-slate-200 dark:border-slate-800 pl-10' : ''}`}>
                  <span className="text-content-muted select-none whitespace-nowrap">{new Date(log.createdAtEpoch).toLocaleTimeString()}</span>
                  <div className="flex">
                    <span className={`${levelStyle} select-none text-[9px] font-bold uppercase px-1.5 py-0.5 rounded leading-none flex items-center justify-center min-w-[50px]`}>{log.level}</span>
                  </div>
                  <LogMessage message={log.message} />
                </div>
              );
            }}
          />
        )}
      </div>
    </div>
  );
}
