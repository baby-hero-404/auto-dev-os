"use client";

import { useEffect, useState, useMemo } from "react";
import { CheckCircle2, Circle, FileCode, Loader2, Timer, ListTodo } from "lucide-react";
import { useTaskDetail, formatStepName, getSemanticStatusColor, getTaskSemanticStatus } from "./TaskDetailContext";
import { parseLiveAction } from "./liveAction";
import { Badge, taskStatusBadge } from "@/components/ui/badge";

interface ExecutionPanelProps {
  onOpenLog: (stepId: string) => void;
}

export function ExecutionPanel({ onOpenLog }: ExecutionPanelProps) {
  const {
    task,
    workflow,
    workflowCompletion,
    workflowStatusCounts,
    workflowSteps,
    analysisData,
    implementationItems,
    currentImplementationItem,
    logs,
  } = useTaskDetail();

  const [elapsedSeconds, setElapsedSeconds] = useState(0);

  const isRunning = workflow?.job?.status === "running";

  useEffect(() => {
    if (!workflow?.checkpoints || workflow.checkpoints.length === 0) {
      setElapsedSeconds(0);
      return;
    }

    const startMs = new Date(workflow.checkpoints[0].created_at).getTime();

    const updateTimer = () => {
      const isRunning = workflow?.job?.status === "running";
      const endMs = isRunning
        ? Date.now()
        : new Date(workflow.checkpoints[workflow.checkpoints.length - 1].created_at).getTime();
      setElapsedSeconds(Math.max(0, Math.round((endMs - startMs) / 1000)));
    };

    updateTimer();
    const interval = setInterval(updateTimer, 1000);
    return () => clearInterval(interval);
  }, [workflow]);

  // Extract last tool call from logs
  const currentAction = useMemo(() => {
    if (!logs || logs.length === 0) return null;
    for (let i = logs.length - 1; i >= 0; i--) {
      const parsed = parseLiveAction(logs[i].message);
      if (parsed) return parsed;
    }
    return null;
  }, [logs]);

  // Format seconds to mm:ss
  const formatTime = (totalSeconds: number) => {
    const mins = Math.floor(totalSeconds / 60);
    const secs = totalSeconds % 60;
    return `${mins.toString().padStart(2, "0")}:${secs.toString().padStart(2, "0")}`;
  };

  if (!workflow || !task) {
    return null;
  }

  const hasTasks = implementationItems && implementationItems.length > 0;
  const totalTasks = hasTasks ? implementationItems.length : 0;
  const completedTasks = hasTasks ? implementationItems.filter(item => item.status === 'done').length : 0;
  const taskProgressPercent = totalTasks > 0 ? Math.round((completedTasks / totalTasks) * 100) : 0;

  const displayProgress = hasTasks ? taskProgressPercent : workflowCompletion;
  const currentStepName = workflow.job?.step
    ? formatStepName(workflow.job.step, analysisData)
    : "None";

  // Minimal state if no checklist items exist (pre-analysis)
  if (!hasTasks) {
    const statusInfo = taskStatusBadge(task.status);
    const semanticColor = getSemanticStatusColor(getTaskSemanticStatus(task.status));

    return (
      <section className="relative overflow-hidden rounded-xl border border-stroke/50 bg-card/60 backdrop-blur-xl p-5 shadow-lg group transition-all duration-200">
        <div className="absolute inset-0 bg-gradient-to-br from-brand-primary/5 via-transparent to-transparent opacity-0 group-hover:opacity-100 transition-opacity duration-500 pointer-events-none" />
        <div className="relative flex flex-col gap-4 z-10">
          <div className="flex flex-wrap items-center justify-between gap-4">
            <div className="flex items-center gap-2">
              <span className={`h-2.5 w-2.5 rounded-full ${semanticColor.dot} shadow-[0_0_8px_currentColor] animate-pulse`} />
              <Badge variant={statusInfo.variant} value={statusInfo.label} />
              <span className="text-xs text-content-muted font-mono ml-2">Current Step:</span>
              <span className="text-xs font-mono font-bold text-foreground">{currentStepName}</span>
            </div>
            <div className="flex items-center gap-1.5 text-xs text-content-muted">
              <Timer size={14} className="text-brand-primary" />
              <span className="font-mono">{formatTime(elapsedSeconds)}</span>
            </div>
          </div>
          <div className="border-t border-stroke/30 pt-3">
            <p className="text-xs text-content-muted italic">Waiting for analysis to generate implementation checklist...</p>
          </div>
        </div>
      </section>
    );
  }

  return (
    <section className="relative overflow-hidden rounded-xl border border-stroke/50 bg-card/60 backdrop-blur-xl p-6 shadow-lg group transition-all duration-200">
      <div className="absolute inset-0 bg-gradient-to-br from-brand-primary/5 via-transparent to-transparent opacity-0 group-hover:opacity-100 transition-opacity duration-500 pointer-events-none" />
      
      <div className="relative z-10 space-y-5">
        {/* Row 1: Inline Progress Strip */}
        <div className="flex flex-wrap items-center justify-between gap-4 border-b border-stroke/30 pb-4">
          <div className="flex items-center gap-3 flex-1 min-w-[200px]">
            <span className="text-sm font-mono font-extrabold text-foreground">{displayProgress}%</span>
            <div className="relative h-2 flex-1 max-w-[240px] overflow-hidden rounded-full bg-surface/50 border border-stroke/30">
              <div
                className="absolute left-0 top-0 h-full rounded-full bg-gradient-to-r from-brand-primary/80 to-brand-primary transition-all duration-700 ease-out shadow-[0_0_10px_rgba(var(--brand-primary),0.5)]"
                style={{ width: `${displayProgress}%` }}
              />
            </div>
            <span className="text-xs font-mono text-content-muted">
              {completedTasks} / {totalTasks} completed
            </span>
          </div>

          <div className="flex items-center gap-4 text-xs text-content-muted">
            <div className="flex items-center gap-1.5">
              <Timer size={14} className="text-brand-primary" />
              <span className="font-mono">{formatTime(elapsedSeconds)} elapsed</span>
            </div>
          </div>
        </div>

        {/* Row 2: Checklist */}
        <div className="space-y-2.5">
          {implementationItems.map((item) => {
            const isCurrent = currentImplementationItem && item.id === currentImplementationItem.id;
            const isDone = item.status === "done";
            
            let icon = <Circle className="text-content-muted shrink-0" size={16} />;
            let rowClass = "border-stroke/30 bg-transparent";
            let textClass = "text-content-muted";

            if (isDone) {
              icon = <CheckCircle2 className="text-emerald-500 shrink-0" size={16} />;
              textClass = "text-content-muted line-through opacity-80";
              rowClass = "border-emerald-500/10 bg-emerald-500/[0.02]";
            } else if (isCurrent) {
              icon = (
                <span className="relative flex h-3.5 w-3.5 shrink-0 mt-0.5">
                  <span className="animate-ping absolute inline-flex h-full w-full rounded-full bg-sky-400 opacity-75"></span>
                  <span className="relative inline-flex rounded-full h-3.5 w-3.5 bg-sky-500"></span>
                </span>
              );
              textClass = "text-foreground font-semibold";
              rowClass = "border-sky-500/40 bg-sky-500/[0.05] border-l-[3px] border-l-sky-500 pl-3";
            }

            const filesCount = item.affectedFiles?.length || 0;

            return (
              <div
                key={item.id}
                onClick={() => onOpenLog(item.stepId)}
                className={`flex items-start justify-between gap-4 p-3.5 rounded-lg border transition-all duration-200 cursor-pointer hover:bg-slate-50 dark:hover:bg-slate-900/60 ${rowClass}`}
              >
                <div className="flex items-start gap-3 min-w-0">
                  <div className="mt-0.5">{icon}</div>
                  <div className="min-w-0">
                    <span className={`text-sm ${textClass}`}>
                      {item.name}
                    </span>
                    {isCurrent && currentAction && (
                      <div className="mt-1 inline-flex items-center gap-1.5 rounded bg-sky-500/10 px-2 py-0.5 border border-sky-500/20">
                        <Loader2 className="animate-spin text-sky-500" size={10} />
                        <span className="text-[10px] text-sky-500 font-medium">
                          {currentAction.action}: <span className="font-mono font-bold text-sky-600 dark:text-sky-400">{currentAction.target}</span>
                        </span>
                      </div>
                    )}
                  </div>
                </div>

                {filesCount > 0 && (
                  <span className="flex items-center gap-1 text-[10px] font-mono px-2 py-0.5 rounded-full bg-slate-100 dark:bg-slate-800 text-content-muted border border-stroke/20 shrink-0">
                    <FileCode size={10} />
                    <span>{filesCount} {filesCount === 1 ? "file" : "files"}</span>
                  </span>
                )}
              </div>
            );
          })}
        </div>
      </div>
    </section>
  );
}
