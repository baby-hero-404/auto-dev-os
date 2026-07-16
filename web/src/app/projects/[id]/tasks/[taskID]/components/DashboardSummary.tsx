"use client";

import { useEffect, useState } from "react";
import { useTaskDetail, formatStepName, getSemanticStatusColor, getTaskSemanticStatus } from "./TaskDetailContext";
import { Badge, taskStatusBadge } from "@/components/ui/badge";
import { Clock, CheckSquare, AlertTriangle, ListTodo, Layers } from "lucide-react";
import { CurrentImplementationCard } from "./CurrentImplementationCard";

export function DashboardSummary() {
  const {
    task,
    workflow,
    workflowCompletion,
    workflowStatusCounts,
    workflowSteps,
    analysisData,
    implementationItems,
    currentImplementationItem,
  } = useTaskDetail();

  const [elapsedTime, setElapsedTime] = useState("0s");

  useEffect(() => {
    if (!workflow?.checkpoints || workflow.checkpoints.length === 0) {
      return;
    }

    const startMs = new Date(workflow.checkpoints[0].created_at).getTime();

    const updateTimer = () => {
      const isRunning = workflow?.job?.status === "running";
      const endMs = isRunning
        ? Date.now()
        : new Date(workflow.checkpoints[workflow.checkpoints.length - 1].created_at).getTime();

      const durationSec = Math.max(0, Math.round((endMs - startMs) / 1000));
      if (durationSec < 60) {
        setElapsedTime(`${durationSec}s`);
      } else {
        const min = Math.floor(durationSec / 60);
        const sec = durationSec % 60;
        setElapsedTime(`${min}m ${sec}s`);
      }
    };

    updateTimer();
    const interval = setInterval(updateTimer, 1000);
    return () => clearInterval(interval);
  }, [workflow]);

  if (!workflow || !task) {
    return null;
  }

  const currentStepName = workflow.job?.step
    ? formatStepName(workflow.job.step, analysisData)
    : "None";

  const statusInfo = taskStatusBadge(task.status);
  const semanticColor = getSemanticStatusColor(getTaskSemanticStatus(task.status));

  const hasTasks = implementationItems && implementationItems.length > 0;
  const totalTasks = hasTasks ? implementationItems.length : 0;
  const completedTasks = hasTasks ? implementationItems.filter(item => item.status === 'done').length : 0;
  const remainingTasks = hasTasks ? implementationItems.filter(item => item.status === 'pending').length : 0;
  const taskProgressPercent = totalTasks > 0 ? Math.round((completedTasks / totalTasks) * 100) : 0;

  const displayProgress = hasTasks ? taskProgressPercent : workflowCompletion;
  const displayCurrent = hasTasks
    ? (currentImplementationItem?.name || currentStepName)
    : currentStepName;

  return (
    <div className="flex flex-col gap-4 w-full">
      <div className="relative overflow-hidden rounded-xl bg-card/60 backdrop-blur-xl p-5 shadow-lg border border-stroke/50 transition-all hover:shadow-xl hover:border-stroke/80 group">
        <div className="absolute inset-0 bg-gradient-to-br from-brand-primary/5 via-transparent to-transparent opacity-0 group-hover:opacity-100 transition-opacity duration-500 pointer-events-none" />
        <div className="relative grid grid-cols-2 md:grid-cols-3 lg:grid-cols-6 gap-4 divide-y md:divide-y-0 lg:divide-x divide-stroke/30 z-10">
          {/* Status */}
          <div className="flex flex-col gap-1.5 justify-center">
            <span className="text-[10px] font-bold uppercase tracking-wider text-content-muted">Status</span>
            <div className="flex items-center gap-2">
              <span className={`h-2.5 w-2.5 rounded-full ${semanticColor.dot} shadow-[0_0_8px_currentColor] animate-pulse`} />
              <Badge variant={statusInfo.variant} value={statusInfo.label} />
            </div>
          </div>

          {/* Current Step / Current Task */}
          <div className="flex flex-col gap-1.5 justify-center pt-3 md:pt-0 md:pl-0 lg:pl-4">
            <span className="text-[10px] font-bold uppercase tracking-wider text-content-muted">
              {hasTasks ? "Current Task" : "Current Step"}
            </span>
            <span className="text-sm font-mono font-bold text-foreground truncate max-w-[150px]" title={displayCurrent}>
              {displayCurrent}
            </span>
          </div>

          {/* Progress */}
          <div className="flex flex-col gap-1.5 justify-center pt-3 md:pt-0 lg:pl-4">
            <span className="text-[10px] font-bold uppercase tracking-wider text-content-muted">Progress</span>
            <div className="flex items-center gap-2">
              <span className="text-sm font-mono font-extrabold text-foreground">{displayProgress}%</span>
              <div className="relative h-2 w-16 overflow-hidden rounded-full bg-surface/50 shrink-0 border border-stroke/30">
                <div
                  className="absolute left-0 top-0 h-full rounded-full bg-gradient-to-r from-brand-primary/80 to-brand-primary transition-all duration-700 ease-out shadow-[0_0_10px_rgba(var(--brand-primary),0.5)]"
                  style={{ width: `${displayProgress}%` }}
                />
              </div>
            </div>
          </div>

          {/* Steps Done / Tasks Done */}
          <div className="flex flex-col gap-1.5 justify-center pt-3 lg:pl-4">
            <span className="text-[10px] font-bold uppercase tracking-wider text-content-muted">
              {hasTasks ? "Tasks Completed" : "Steps Completed"}
            </span>
            <div className="flex items-center gap-1.5 text-sm font-semibold text-foreground">
              <CheckSquare size={14} className="text-emerald-500" />
              <span>
                {hasTasks ? completedTasks : workflowStatusCounts.done} <span className="text-content-muted">/</span> {hasTasks ? totalTasks : workflowSteps.length}
              </span>
            </div>
          </div>

          {/* Error Count / Remaining Tasks */}
          <div className="flex flex-col gap-1.5 justify-center pt-3 lg:pl-4">
            <span className="text-[10px] font-bold uppercase tracking-wider text-content-muted">
              {hasTasks ? "Remaining Tasks" : "Errors Encountered"}
            </span>
            <div className="flex items-center gap-1.5 text-sm font-semibold text-foreground">
              {hasTasks ? (
                <>
                  <ListTodo size={14} className="text-sky-500" />
                  <span>{remainingTasks}</span>
                </>
              ) : (
                <>
                  <AlertTriangle size={14} className={workflowStatusCounts.failed > 0 ? "text-rose-500" : "text-content-muted"} />
                  <span className={workflowStatusCounts.failed > 0 ? "text-rose-500 font-bold" : ""}>
                    {workflowStatusCounts.failed}
                  </span>
                </>
              )}
            </div>
          </div>

          {/* Elapsed Time */}
          <div className="flex flex-col gap-1.5 justify-center pt-3 lg:pl-4">
            <span className="text-[10px] font-bold uppercase tracking-wider text-content-muted">Elapsed Time</span>
            <div className="flex items-center gap-1.5 text-sm font-semibold text-foreground">
              <Clock size={14} className="text-brand-primary" />
              <span className="font-mono">{elapsedTime}</span>
            </div>
          </div>
        </div>
      </div>
      <CurrentImplementationCard />
    </div>
  );
}
