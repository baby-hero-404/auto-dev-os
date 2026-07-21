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
        if (currentGroup.status === "running") {
          currentGroup.status = "failed"; // Implicitly failed/interrupted if a new step starts
        }
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
        // Do not close the group if it is paused. Keep it open to receive resume logs.
        if (endMatch[2] !== "paused") {
          result.push({ type: "group", ...currentGroup });
          currentGroup = null;
        }
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
  const isLong = lines.length > 8 || message.length > 1000;

  if (!isLong) {
    return <span className="whitespace-pre-wrap text-[#c9d1d9] break-words font-mono">{message}</span>;
  }

  const previewLines = lines.slice(0, 6).join("\n");
  const displayedText = isExpanded ? message : previewLines;

  return (
    <div className="flex flex-col gap-1 w-full mt-0.5">
      <div className="whitespace-pre-wrap text-[#c9d1d9] break-words font-mono leading-relaxed">
        {displayedText}
        {!isExpanded && <span className="text-[#8b949e]">...</span>}
      </div>
      <button
        onClick={() => setIsExpanded(!isExpanded)}
        className="text-[10px] font-bold text-[#58a6ff] hover:text-[#58a6ff]/80 transition-colors self-start mt-1 cursor-pointer flex items-center gap-1"
      >
        {isExpanded ? "Show less" : `Show more (${lines.length - 6} lines)`}
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
  /** Controlled collapse state (REQ-006). When omitted, the console self-manages. */
  isExpanded?: boolean;
  onToggle?: () => void;
  hideHeader?: boolean;
}

export function LogConsole({ logs, isExpanded, onToggle, hideHeader = false }: LogConsoleProps) {
  const [expanded, setExpanded] = useState<Record<string, boolean>>({});
  const [viewMode, setViewMode] = useState<"milestones" | "all">("all");
  const [isFullscreen, setIsFullscreen] = useState(false);

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
    latestEventType === "success" ? "bg-[#3fb950]" :
      latestEventType === "running" ? "bg-[#58a6ff] animate-pulse" :
        latestEventType === "failed" ? "bg-[#f85149]" :
          latestEventType === "paused" ? "bg-[#d29922]" : "bg-[#8b949e]";

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
        const isGroupExpanded = expanded[item.key] ?? defaultExpanded;

        items.push({ ...item, isExpanded: isGroupExpanded, defaultExpanded, originalIndex: index });
        if (isGroupExpanded) {
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

  const fullscreenClasses = isFullscreen 
    ? "fixed inset-4 z-50 shadow-2xl rounded-xl border border-[#30363d] bg-[#0d1117] flex flex-col"
    : `${hideHeader ? "" : "rounded-lg border border-[#30363d]"} bg-[#0d1117] flex flex-col ${isOpen ? "h-full min-h-[400px]" : ""}`;

  return (
    <>
      {isFullscreen && <div className="fixed inset-0 bg-black/60 z-40 backdrop-blur-sm" onClick={() => setIsFullscreen(false)} />}
      <div id="log-console" className={fullscreenClasses}>
        {(!hideHeader || isFullscreen) && (
          <>
            <div className={`flex flex-wrap items-center justify-between gap-4 p-4 border-b border-[#30363d] bg-[#010409] rounded-t-lg ${isOpen ? "mb-0" : ""}`}>
            <div className="flex items-center gap-2 text-left">
              {!isFullscreen && (
                <button type="button" onClick={toggleOpen} className="cursor-pointer" aria-expanded={isOpen}>
                  {isOpen ? <ChevronDown size={16} className="text-[#8b949e]" /> : <ChevronRight size={16} className="text-[#8b949e]" />}
                </button>
              )}
              <TerminalSquare size={16} className="text-[#58a6ff]" />
              <h2 className="font-mono text-[13px] font-semibold text-[#e6edf3]">Execution Logs</h2>
            </div>

            {isOpen && logs.length > 0 && (
              <div className="flex items-center gap-3">
                <div className="flex rounded-md border border-[#30363d] bg-[#161b22] p-0.5 text-[10px] font-semibold font-mono">
                  <button
                    onClick={() => setViewMode("all")}
                    className={`rounded px-2.5 py-1 cursor-pointer transition-all ${
                      viewMode === "all"
                        ? "bg-[#21262d] text-[#58a6ff] border border-[#30363d]"
                        : "text-[#8b949e] hover:text-[#c9d1d9] border border-transparent"
                    }`}
                  >
                    Terminal
                  </button>
                  <button
                    onClick={() => setViewMode("milestones")}
                    className={`rounded px-2.5 py-1 cursor-pointer transition-all ${
                      viewMode === "milestones"
                        ? "bg-[#21262d] text-[#58a6ff] border border-[#30363d]"
                        : "text-[#8b949e] hover:text-[#c9d1d9] border border-transparent"
                    }`}
                  >
                    Milestones
                  </button>
                </div>
                
                <button
                  onClick={() => setIsFullscreen(!isFullscreen)}
                  className="text-[10px] font-mono font-semibold px-2 py-1.5 rounded-md border border-[#30363d] bg-[#21262d] text-[#c9d1d9] hover:bg-[#30363d] hover:text-white transition-colors flex items-center gap-1.5 cursor-pointer"
                >
                  {isFullscreen ? (
                    <>
                      <svg width="12" height="12" viewBox="0 0 16 16" fill="currentColor"><path d="M5.22 9.78a.75.75 0 0 1 0 1.06l-2.5 2.5h2.03a.75.75 0 0 1 0 1.5H1.25a.75.75 0 0 1-.75-.75v-3.5a.75.75 0 0 1 1.5 0v2.03l2.5-2.5a.75.75 0 0 1 1.06 0Zm5.56 0a.75.75 0 0 1 1.06 0l2.5 2.5v-2.03a.75.75 0 0 1 1.5 0v3.5a.75.75 0 0 1-.75.75h-3.5a.75.75 0 0 1 0-1.5h2.03l-2.5-2.5a.75.75 0 0 1 0-1.06ZM5.22 6.22a.75.75 0 0 1-1.06 0l-2.5-2.5v2.03a.75.75 0 0 1-1.5 0v-3.5a.75.75 0 0 1 .75-.75h3.5a.75.75 0 0 1 0 1.5H2.38l2.5 2.5a.75.75 0 0 1 0 1.06Zm5.56 0a.75.75 0 0 1 0-1.06l2.5-2.5h-2.03a.75.75 0 0 1 0-1.5h3.5a.75.75 0 0 1 .75.75v3.5a.75.75 0 0 1-1.5 0V2.38l-2.5 2.5a.75.75 0 0 1-1.06 0Z"/></svg>
                      Minimize
                    </>
                  ) : (
                    <>
                      <svg width="12" height="12" viewBox="0 0 16 16" fill="currentColor"><path d="M3.72 3.72a.75.75 0 0 1 1.06 1.06L2.56 7h2.19a.75.75 0 0 1 0 1.5H1.25a.75.75 0 0 1-.75-.75V4.25a.75.75 0 0 1 1.5 0v2.19l2.22-2.22Zm8.56 0a.75.75 0 0 1 0 1.06l-2.22 2.22h2.19a.75.75 0 0 1 0 1.5h-3.5a.75.75 0 0 1-.75-.75V4.25a.75.75 0 0 1 1.5 0v2.19l2.22-2.22ZM3.72 12.28a.75.75 0 0 1 0-1.06l2.22-2.22H3.75a.75.75 0 0 1 0-1.5h3.5a.75.75 0 0 1 .75.75v3.5a.75.75 0 0 1-1.5 0v-2.19l-2.22 2.22a.75.75 0 0 1-1.06 0Zm8.56 0a.75.75 0 0 1-1.06 0l-2.22-2.22v2.19a.75.75 0 0 1-1.5 0v-3.5a.75.75 0 0 1 .75-.75h3.5a.75.75 0 0 1 0 1.5h-2.19l2.22 2.22a.75.75 0 0 1 0 1.06Z"/></svg>
                      Expand
                    </>
                  )}
                </button>
              </div>
            )}
          </div>

          {!isOpen && (
            <button
              type="button"
              onClick={toggleOpen}
              className="m-3 flex items-center gap-2.5 rounded-md border border-[#30363d] bg-[#161b22] px-3 py-2 text-left transition hover:bg-[#21262d] cursor-pointer"
            >
              <span className={`h-2 w-2 shrink-0 rounded-full ${latestDotColor}`} />
              <span className="min-w-0 flex-1 truncate font-mono text-[11px] text-[#8b949e]">{latestEventText}</span>
              <span className="shrink-0 text-[10px] font-bold uppercase tracking-wider text-[#58a6ff]">View full log</span>
            </button>
          )}
        </>
      )}

      <div className={`relative flex-1 overflow-hidden rounded-b-lg bg-[#0d1117] min-h-[520px] ${isOpen ? "" : "hidden"}`}>
        {logs.length === 0 ? (
          <p className="text-[#8b949e] p-4 font-mono text-[11px]">No logs yet. Execute the workflow to start.</p>
        ) : viewMode === "milestones" ? (
          <div className="absolute inset-0 overflow-y-auto p-5 custom-scrollbar flex flex-col gap-4 font-mono">
            {milestones.length === 0 ? (
              <p className="text-[#8b949e] text-[11px] text-center py-8">No milestones recorded yet.</p>
            ) : (
              <div className="relative border-l border-[#30363d] pl-6 space-y-5 ml-2 pt-2 text-left">
                {milestones.map((m) => {
                  let dotBg = "bg-[#8b949e]";
                  let borderStyle = "border-[#30363d]";
                  let textColor = "text-[#c9d1d9]";
                  
                  if (m.type === "success") {
                    dotBg = "bg-[#238636]";
                    borderStyle = "border-[#238636]/30";
                    textColor = "text-[#3fb950] font-semibold";
                  } else if (m.type === "running") {
                    dotBg = "bg-[#58a6ff] animate-pulse";
                    borderStyle = "border-[#58a6ff]/30";
                    textColor = "text-[#58a6ff] font-semibold";
                  } else if (m.type === "failed") {
                    dotBg = "bg-[#f85149]";
                    borderStyle = "border-[#f85149]/30";
                    textColor = "text-[#f85149] font-bold";
                  } else if (m.type === "paused") {
                    dotBg = "bg-[#d29922]";
                    borderStyle = "border-[#d29922]/30";
                    textColor = "text-[#d29922] font-semibold";
                  }

                  return (
                    <div key={m.id} className="relative flex flex-col gap-1.5">
                      {/* Dot */}
                      <span className={`absolute -left-[31px] top-0.5 flex h-[18px] w-[18px] items-center justify-center rounded-full bg-[#0d1117] border-[1.5px] ${borderStyle}`}>
                        <span className={`h-2 w-2 rounded-full ${dotBg}`} />
                      </span>
                      <div className="flex items-start justify-between gap-4">
                        <span className={`text-[12px] leading-relaxed ${textColor}`}>{m.message}</span>
                        <span className="text-[10px] text-[#8b949e] whitespace-nowrap mt-0.5">{m.timestamp}</span>
                      </div>
                    </div>
                  );
                })}
              </div>
            )}
          </div>
        ) : (
          <div className="absolute inset-0">
            <Virtuoso
              data={flattenedItems}
              className="h-full w-full font-mono text-[11px]"
              followOutput="smooth"
              itemContent={(_, item) => {
                if (item.type === "group") {
                  const Icon = item.isExpanded ? ChevronDown : ChevronRight;
                  const statusColor =
                    item.status === "running" ? "text-[#58a6ff]" :
                      item.status === "success" ? "text-[#3fb950]" :
                        item.status === "failed" ? "text-[#f85149]" : "text-[#d29922]";

                  return (
                    <div
                      id={`log-group-${item.stepName}`}
                      className="flex items-center gap-2 px-4 py-2 border-y border-[#30363d] bg-[#161b22] cursor-pointer hover:bg-[#21262d] transition-colors -mt-px first:mt-0"
                      onClick={() => toggleGroup(item.key, item.defaultExpanded)}
                    >
                      <Icon size={14} className="text-[#8b949e]" />
                      <span className="font-semibold text-[#e6edf3] uppercase tracking-wider text-[10px]">
                        {(() => {
                          if (item.stepName.startsWith("code_backend_")) {
                            const parsedIdx = Number(item.stepName.substring("code_backend_".length));
                            return `Backend Execution ${isNaN(parsedIdx) ? 1 : parsedIdx + 1}`;
                          }
                          if (item.stepName.startsWith("code_frontend_")) {
                            const parsedIdx = Number(item.stepName.substring("code_frontend_".length));
                            return `Frontend Execution ${isNaN(parsedIdx) ? 1 : parsedIdx + 1}`;
                          }
                          return item.stepName;
                        })()}
                      </span>
                      <div className="ml-auto flex items-center gap-1.5">
                        {getStatusIcon(item.status)}
                        <span className={`font-bold uppercase text-[9px] ${statusColor}`}>{item.status}</span>
                      </div>
                    </div>
                  );
                }

                const log = item.log as RealtimeLog;
                const levelStyle =
                  log.level === "error" ? "text-[#f85149]" :
                    log.level === "warn" ? "text-[#d29922]" :
                      log.level === "debug" ? "text-[#8b949e]" :
                        "text-[#3fb950]";

                return (
                  <div className={`flex gap-3 border-b border-[#30363d]/50 py-1.5 px-4 hover:bg-[#161b22] transition-colors ${item.isGroupChild ? 'bg-[#0d1117] border-l-2 border-[#30363d] pl-8' : ''}`}>
                    <span className="text-[#8b949e] select-none whitespace-nowrap shrink-0 mt-0.5">{new Date(log.createdAtEpoch).toLocaleTimeString()}</span>
                    <div className="flex shrink-0 mt-0.5">
                      <span className={`${levelStyle} select-none text-[10px] font-bold uppercase min-w-[40px]`}>{log.level}</span>
                    </div>
                    <div className="flex-1 min-w-0">
                      <LogMessage message={log.message} />
                    </div>
                  </div>
                );
              }}
            />
          </div>
        )}
      </div>
    </div>
    </>
  );
}
