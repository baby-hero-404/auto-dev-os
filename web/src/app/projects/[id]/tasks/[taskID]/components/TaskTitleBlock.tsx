"use client";

import { useState, useEffect } from "react";
import useSWR from "swr";
import { ShieldCheck, Loader2 } from "lucide-react";
import { attestations as attestationsApi } from "@/lib/api";
import { useTaskDetail } from "./TaskDetailContext";
import { DescriptionBody } from "./DescriptionBody";
import { getTaskStatusBadge, isActiveStatus } from "@/lib/status";

export function TaskTitleBlock() {
  const { task, workflow, isPaused, token } = useTaskDetail();
  const [elapsedSeconds, setElapsedSeconds] = useState(0);

  const { data: attestations, isLoading: isLoadingAttestations } = useSWR(
    task?.status === "merged" && task?.id && token ? [`/tasks/${task.id}/attestations`, token] : null,
    () => attestationsApi.listByTask(task!.id, token)
  );

  const hasAttestations = attestations && attestations.length > 0;

  useEffect(() => {
    if (!workflow?.checkpoints || workflow.checkpoints.length === 0) {
      // eslint-disable-next-line react-hooks/set-state-in-effect
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
  const label = badge.label;
  const bg = badge.bg || "var(--surface)";
  const fg = badge.fg || "var(--content-muted)";
  const group = badge.group || "Preparation";
  const running = isActiveStatus(st);
  const paused = isPaused;

  return (
    <div className="flex flex-col md:flex-row md:items-center justify-between gap-4 mb-6 pb-5 border-b border-stroke/10">
      <div className="flex-1">
        <h1 className="m-0 mb-3 text-2xl md:text-3xl font-extrabold tracking-tight text-foreground bg-gradient-to-r from-slate-950 via-slate-900 to-slate-800 dark:from-white dark:via-slate-100 dark:to-slate-300 bg-clip-text text-transparent">
          {task?.title || "Task"}
        </h1>
        <div className="flex flex-wrap items-center gap-2 mb-3">
          <span className="inline-flex items-center gap-1.5 px-3 py-1 rounded-full text-xs font-semibold border border-stroke/20 shadow-sm transition-all" style={{ background: paused && running ? '#fef3c6' : bg, color: paused && running ? '#795800' : fg }}>
            <span className={`w-1.5 h-1.5 rounded-full ${running && !paused ? 'animate-pulse' : ''}`} style={{ background: paused && running ? '#795800' : fg }}></span>
            {paused && running ? 'Paused' : label}
          </span>
          <span className="inline-flex px-3 py-1 rounded-full text-xs font-medium bg-surface/50 text-content-muted border border-stroke/15 shadow-sm">
            {group}
          </span>
          <span className="inline-flex px-3 py-1 rounded-full text-xs font-bold bg-rose-500/10 text-rose-600 dark:text-rose-400 border border-rose-500/20 shadow-sm">
            P{task?.priority || 0}
          </span>

          {task?.spec_status === "ready_with_warnings" && (
            <span
              className="inline-flex items-center gap-1 px-3 py-1 rounded-full text-xs font-semibold bg-amber-500/10 text-amber-600 dark:text-amber-400 border border-amber-500/20 shadow-sm"
              title="Definition-of-Ready gate bypassed (hotfix label or max clarification rounds exhausted)"
            >
              <svg className="w-3.5 h-3.5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M12 9v2m0 4h.01m-6.938 4h13.856c1.54 0 2.502-1.667 1.732-3L13.732 4c-.77-1.333-2.694-1.333-3.464 0L3.34 16c-.77 1.333.192 3 1.732 3z" />
              </svg>
              DoR Bypassed
            </span>
          )}
          {task?.status === "merged" && (
            isLoadingAttestations ? (
              <span className="inline-flex items-center gap-1 px-3 py-1 rounded-full text-xs font-semibold bg-surface text-content-muted border border-stroke/20 shadow-sm">
                <Loader2 size={12} className="animate-spin" /> Verifying
              </span>
            ) : hasAttestations ? (
              <span
                className="inline-flex items-center gap-1 px-3 py-1 rounded-full text-xs font-semibold bg-emerald-500/10 text-emerald-600 dark:text-emerald-400 border border-emerald-500/20 shadow-sm"
                title="Code cryptographically attested"
              >
                <ShieldCheck size={12} /> Verified
              </span>
            ) : null
          )}
        </div>
        <div className="mt-4 bg-surface/20 dark:bg-surface/20 rounded-2xl border border-stroke/10 p-4 shadow-sm">
          <DescriptionBody />
        </div>
      </div>
      <div className="md:text-right shrink-0 self-start md:self-center bg-surface/50 dark:bg-surface/50 border border-stroke/10 rounded-xl px-4 py-2.5 shadow-sm hover:shadow-md transition-all duration-200">
        <div className="text-[10px] uppercase font-bold tracking-wider text-content-muted mb-0.5">Elapsed</div>
        <div className="font-mono text-base font-bold text-foreground flex items-center md:justify-end gap-1.5">
          <span className={`w-1.5 h-1.5 rounded-full bg-emerald-500 ${running && !paused ? 'animate-ping' : ''}`}></span>
          {formatTime(elapsedSeconds)}
        </div>
      </div>
    </div>
  );
}
