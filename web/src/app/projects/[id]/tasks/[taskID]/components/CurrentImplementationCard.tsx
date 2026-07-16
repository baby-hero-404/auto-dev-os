"use client";

import { useEffect, useState, useMemo } from "react";
import { useTaskDetail } from "./TaskDetailContext";
import { Loader2, Timer, FileCode, Play } from "lucide-react";

export function parseLiveAction(message: string): { action: string; target: string } | null {
  const lower = message.toLowerCase();
  
  // 1. Editing files
  if (lower.includes("search_replace") || lower.includes("create_file") || lower.includes("write_to_file") || lower.includes("replace_file_content") || lower.includes("multi_replace")) {
    const fileMatch = message.match(/(?:file|path|TargetFile)[:\s'"]+([a-zA-Z0-9_\-\.\/\\:]+)/i) || 
                      message.match(/on\s+([a-zA-Z0-9_\-\.\/\\:]+)/i) ||
                      message.match(/([a-zA-Z0-9_\-\.\/\\:]+\.[a-zA-Z0-9_]+)/i);
    const file = fileMatch ? fileMatch[1].split("/").pop() || fileMatch[1] : "file";
    return { action: "Editing", target: file };
  }
  
  // 2. Running tools/commands
  if (lower.includes("run_tests") || lower.includes("run_build") || lower.includes("run_lint") || lower.includes("run_command") || lower.includes("execute")) {
    const toolMatch = message.match(/(?:tool|command|CommandLine)[:\s'"]+([a-zA-Z0-9_\-\.\/\\: ]+)/i) ||
                      message.match(/running\s+([a-zA-Z0-9_\-\.\/\\: ]+)/i);
    const tool = toolMatch ? toolMatch[1].trim() : "tests/build";
    return { action: "Running", target: tool };
  }

  // 3. Reading files
  if (lower.includes("read_file") || lower.includes("view_file") || lower.includes("list_files") || lower.includes("read_url")) {
    const fileMatch = message.match(/(?:file|path|AbsolutePath)[:\s'"]+([a-zA-Z0-9_\-\.\/\\:]+)/i) ||
                      message.match(/([a-zA-Z0-9_\-\.\/\\:]+\.[a-zA-Z0-9_]+)/i);
    const file = fileMatch ? fileMatch[1].split("/").pop() || fileMatch[1] : "file";
    return { action: "Reading", target: file };
  }
  
  return null;
}

export function CurrentImplementationCard() {
  const { workflow, logs, currentImplementationItem } = useTaskDetail();
  const [elapsed, setElapsed] = useState(0);

  const isRunning = workflow?.job?.status === "running";
  const currentStep = workflow?.job?.step;

  // Find start time of the current step
  const startTimestamp = useMemo(() => {
    if (!isRunning || !currentStep || !workflow?.checkpoints) return null;
    const currentStepCheckpoint = workflow.checkpoints.find(cp => cp.step === currentStep);
    return currentStepCheckpoint ? new Date(currentStepCheckpoint.created_at).getTime() : null;
  }, [isRunning, currentStep, workflow?.checkpoints]);

  // Elapsed timer effect
  useEffect(() => {
    if (!startTimestamp) {
      setElapsed(0);
      return;
    }
    setElapsed(Math.max(0, Math.floor((Date.now() - startTimestamp) / 1000)));

    const interval = setInterval(() => {
      setElapsed(Math.max(0, Math.floor((Date.now() - startTimestamp) / 1000)));
    }, 1000);

    return () => clearInterval(interval);
  }, [startTimestamp]);

  // Extract last tool call from logs
  const currentAction = useMemo(() => {
    if (!logs || logs.length === 0) return { action: "Thinking", target: "..." };
    for (let i = logs.length - 1; i >= 0; i--) {
      const parsed = parseLiveAction(logs[i].message);
      if (parsed) return parsed;
    }
    return { action: "Thinking", target: "..." };
  }, [logs]);

  // Format elapsed seconds to mm:ss
  const formatTime = (totalSeconds: number) => {
    const mins = Math.floor(totalSeconds / 60);
    const secs = totalSeconds % 60;
    return `${mins.toString().padStart(2, "0")}:${secs.toString().padStart(2, "0")}`;
  };

  if (!isRunning || !currentImplementationItem) {
    return null;
  }

  return (
    <div className="rounded-xl border border-sky-500/20 bg-sky-500/5 p-5 shadow-sm backdrop-blur-sm transition-all duration-200">
      <div className="flex items-center justify-between gap-4">
        <div className="flex items-center gap-3">
          <div className="grid size-9 place-items-center rounded-lg bg-sky-500/10 text-sky-500 border border-sky-500/20">
            <Loader2 className="animate-spin text-sky-500" size={18} />
          </div>
          <div>
            <div className="flex items-center gap-2">
              <span className="font-mono text-[10px] font-bold uppercase tracking-wider text-sky-500">
                Current Implementation
              </span>
              <span className="inline-flex items-center gap-1 rounded-full bg-sky-500/15 px-2 py-0.5 text-[9px] font-bold text-sky-500">
                <span className="relative flex h-1.5 w-1.5">
                  <span className="animate-ping absolute inline-flex h-full w-full rounded-full bg-sky-400 opacity-75"></span>
                  <span className="relative inline-flex rounded-full h-1.5 w-1.5 bg-sky-500"></span>
                </span>
                In Progress
              </span>
            </div>
            <h3 className="font-sans font-bold text-sm text-foreground mt-0.5">
              {currentImplementationItem.name}
            </h3>
          </div>
        </div>

        <div className="flex items-center gap-6">
          {/* Real-time elapsed timer */}
          <div className="flex items-center gap-1.5 text-content-muted">
            <Timer size={14} className="text-sky-500" />
            <span className="font-mono text-xs font-bold tabular-nums">
              {formatTime(elapsed)}
            </span>
          </div>

          {/* Current file / operation */}
          <div className="flex items-center gap-1.5 rounded-lg bg-slate-100 dark:bg-slate-900 px-3 py-1.5 border border-stroke/20">
            <FileCode size={13} className="text-content-muted" />
            <span className="text-xs text-foreground font-medium">
              <span className="text-content-muted mr-1">{currentAction.action}:</span>
              <span className="font-mono font-bold text-[11px] text-brand-primary">
                {currentAction.target}
              </span>
            </span>
          </div>
        </div>
      </div>
    </div>
  );
}
