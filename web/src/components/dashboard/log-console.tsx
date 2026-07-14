import { useState, useMemo } from "react";
import { TerminalSquare, ChevronRight, ChevronDown, CheckCircle2, XCircle, Pause, Loader2 } from "lucide-react";
import type { RealtimeLog } from "@/lib/store/use-realtime-log-store";
import { Virtuoso } from "react-virtuoso";

export type LogGroupStatus = "running" | "success" | "failed" | "paused";

export type GroupedLogItem =
  | { type: "log"; log: RealtimeLog }
  | { type: "group"; key: string; stepName: string; status: LogGroupStatus; logs: RealtimeLog[] };

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
      <div className={`whitespace-pre-wrap text-slate-800 dark:text-slate-100 break-words font-mono ${
        isCodeOrJson || isExpanded 
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

interface LogConsoleProps {
  logs: RealtimeLog[];
}

export function LogConsole({ logs }: LogConsoleProps) {
  const [expanded, setExpanded] = useState<Record<string, boolean>>({});

  const toggleGroup = (key: string, defaultExpanded: boolean) => {
    setExpanded((prev) => ({
      ...prev,
      [key]: prev[key] === undefined ? !defaultExpanded : !prev[key]
    }));
  };

  const flattenedItems = useMemo(() => {
    const items: any[] = [];
    const grouped = groupLogs(logs);
    
    grouped.forEach((item, index) => {
      if (item.type === "log") {
        items.push(item);
      } else {
        const defaultExpanded = item.status === "running" || item.status === "failed";
        const isExpanded = expanded[item.key] ?? defaultExpanded;
        
        items.push({ ...item, isExpanded, defaultExpanded, originalIndex: index });
        if (isExpanded) {
          items.push(...item.logs.map((log) => ({ type: "log", log, isGroupChild: true })));
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
    <div className="rounded-lg border border-stroke bg-panel p-5 h-full flex flex-col min-h-[400px]">
      <div className="mb-4 flex items-center gap-2">
        <TerminalSquare size={18} className="text-brand-primary" />
        <h2 className="font-mono text-lg font-semibold text-foreground dark:text-white">Execution Logs</h2>
      </div>
      <div className="flex-1 overflow-hidden rounded-md bg-slate-50 dark:bg-slate-950 border border-stroke min-h-[520px]">
        {logs.length === 0 ? (
          <p className="text-content-muted p-4 font-mono text-xs">No logs yet. Execute the workflow to start.</p>
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
