"use client";

import { useState, useEffect } from "react";
import useSWR from "swr";
import { ShieldCheck, Loader2, CheckCircle2, CircleDashed, Circle } from "lucide-react";
import { attestations as attestationsApi } from "@/lib/api";
import { useTaskDetail } from "./TaskDetailContext";
import { DescriptionBody } from "./DescriptionBody";
import { getTaskStatusBadge, isActiveStatus } from "@/lib/status";

// High-level pipeline steps for the Stepper
const PIPELINE_STEPS = [
  { id: "reported", label: "Bug found", statuses: ["todo", "context_loading"] },
  { id: "proposal", label: "Proposal", statuses: ["analyzing", "spec_review"] },
  { id: "implementing", label: "Implementing", statuses: ["coding", "review"] },
  { id: "testing", label: "Testing", statuses: ["testing", "pr_ready", "human_review", "merged"] },
  { id: "released", label: "Released", statuses: ["merged"] },
];

export function TaskSummaryHeader() {
  const { task, workflow, isPaused, token } = useTaskDetail();
  const [elapsedSeconds, setElapsedSeconds] = useState(0);

  const { data: attestations, isLoading: isLoadingAttestations } = useSWR(
    task?.status === "merged" && task?.id && token ? [`/tasks/${task.id}/attestations`, token] : null,
    () => attestationsApi.listByTask(task!.id, token)
  );

  const hasAttestations = attestations && attestations.length > 0;

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

  const formatTime = (totalSeconds: number) => {
    if (totalSeconds === 0) return "—";
    const days = Math.floor(totalSeconds / 86400);
    const hrs = Math.floor((totalSeconds % 86400) / 3600);
    const mins = Math.floor((totalSeconds % 3600) / 60);
    if (days > 0) return `${days}d ${hrs}h ${mins}m`;
    if (hrs > 0) return `${hrs}h ${mins}m`;
    return `${mins}m ${totalSeconds % 60}s`;
  };

  const st = task?.status || "todo";
  const badge = getTaskStatusBadge(st);
  const running = isActiveStatus(st);
  const paused = isPaused;

  // Determine current step index
  let currentStepIdx = 0;
  for (let i = 0; i < PIPELINE_STEPS.length; i++) {
    if (PIPELINE_STEPS[i].statuses.includes(st)) {
      currentStepIdx = i;
      break;
    }
  }
  if (st === "merged") {
    currentStepIdx = 4;
  }

  return (
    <div className="flex flex-col gap-5 mb-8 pb-6 border-b border-stroke/15">
      {/* Top Row: Title & Timer */}
      <div className="flex flex-col md:flex-row md:items-start justify-between gap-4">
        <div className="flex-1">
          <h1 className="m-0 mb-3 text-2xl md:text-3xl font-extrabold tracking-tight text-foreground bg-gradient-to-r from-slate-950 via-slate-900 to-slate-800 dark:from-white dark:via-slate-100 dark:to-slate-300 bg-clip-text text-transparent">
            {task?.title || "Task"}
          </h1>
          
          <div className="flex flex-wrap items-center gap-2 mb-3">
            <span className="inline-flex px-3 py-1 rounded-full text-[13px] font-bold bg-rose-500/10 text-rose-600 dark:text-rose-400 border border-rose-500/20 shadow-sm">
              P{task?.priority || 0}
            </span>
            <span className="inline-flex items-center gap-1.5 px-3 py-1 rounded-full text-[13px] font-semibold border border-stroke/20 shadow-sm transition-all" style={{ background: paused && running ? '#fef3c6' : badge.bg, color: paused && running ? '#795800' : badge.fg }}>
              <span className={`w-1.5 h-1.5 rounded-full ${running && !paused ? 'animate-pulse' : ''}`} style={{ background: paused && running ? '#795800' : badge.fg }}></span>
              {paused && running ? 'Paused' : badge.label}
            </span>
            <span className="inline-flex px-3 py-1 rounded-full text-[13px] font-medium bg-surface/50 text-content-muted border border-stroke/15 shadow-sm">
              Owner: AI Agent
            </span>
            {task?.status === "merged" && (
              isLoadingAttestations ? (
                <span className="inline-flex items-center gap-1 px-3 py-1 rounded-full text-[13px] font-semibold bg-surface text-content-muted border border-stroke/20 shadow-sm">
                  <Loader2 size={12} className="animate-spin" /> Verifying
                </span>
              ) : hasAttestations ? (
                <span
                  className="inline-flex items-center gap-1 px-3 py-1 rounded-full text-[13px] font-semibold bg-emerald-500/10 text-emerald-600 dark:text-emerald-400 border border-emerald-500/20 shadow-sm"
                  title="Code cryptographically attested"
                >
                  <ShieldCheck size={12} /> Verified
                </span>
              ) : null
            )}
          </div>
        </div>

        <div className="md:text-right shrink-0 self-start md:self-center bg-surface/50 dark:bg-surface/50 border border-stroke/10 rounded-xl px-4 py-2.5 shadow-sm hover:shadow-md transition-all duration-200">
          <div className="text-[11px] uppercase font-bold tracking-wider text-content-muted mb-0.5">Elapsed</div>
          <div className="font-mono text-base font-bold text-foreground flex items-center md:justify-end gap-1.5">
            <span className={`w-1.5 h-1.5 rounded-full bg-emerald-500 ${running && !paused ? 'animate-ping' : ''}`}></span>
            {formatTime(elapsedSeconds)}
          </div>
        </div>
      </div>

      {/* Description */}
      <div className="bg-surface/30 dark:bg-surface/30 rounded-2xl border border-stroke/10 p-4 shadow-sm text-sm">
        <DescriptionBody />
      </div>

      {/* Progress Stepper */}
      <div className="w-full mt-2 hidden md:block">
        <div className="flex items-center justify-between relative">
          {/* Connector Line */}
          <div className="absolute left-0 right-0 top-1/2 h-[2px] -translate-y-1/2 bg-stroke/50 z-0 mx-8"></div>
          <div 
            className="absolute left-0 top-1/2 h-[2px] -translate-y-1/2 bg-brand-primary z-0 transition-all duration-500 mx-8" 
            style={{ width: `${(currentStepIdx / (PIPELINE_STEPS.length - 1)) * 100}%` }}
          ></div>

          {/* Steps */}
          {PIPELINE_STEPS.map((step, idx) => {
            const isCompleted = idx < currentStepIdx;
            const isCurrent = idx === currentStepIdx;
            const isPending = idx > currentStepIdx;
            
            return (
              <div key={step.id} className="relative z-10 flex flex-col items-center gap-2 bg-background px-4">
                <div 
                  className={`flex items-center justify-center w-8 h-8 rounded-full border-2 transition-colors bg-background ${
                    isCompleted ? "border-brand-primary text-brand-primary" :
                    isCurrent ? "border-brand-primary text-brand-primary shadow-[0_0_0_3px_rgba(var(--brand-primary-rgb),0.15)]" :
                    "border-stroke text-stroke"
                  }`}
                >
                  {isCompleted ? <CheckCircle2 size={16} /> : isCurrent && running ? <Loader2 size={16} className="animate-spin" /> : isCurrent ? <CircleDashed size={16} /> : <Circle size={10} className="fill-current" />}
                </div>
                <span className={`text-[11px] uppercase tracking-wider font-bold ${
                  isCurrent ? "text-brand-primary" :
                  isCompleted ? "text-foreground" :
                  "text-content-muted"
                }`}>
                  {step.label}
                </span>
              </div>
            );
          })}
        </div>
      </div>
    </div>
  );
}
