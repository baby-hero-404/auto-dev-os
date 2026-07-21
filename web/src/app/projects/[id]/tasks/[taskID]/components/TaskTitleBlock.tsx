"use client";

import { useState, useEffect } from "react";
import { useTaskDetail } from "./TaskDetailContext";
import { DescriptionBody } from "./DescriptionBody";

export function TaskTitleBlock() {
  const { task, workflow, isPaused } = useTaskDetail();
  const [elapsedSeconds, setElapsedSeconds] = useState(0);

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
  const P: Record<string, [string, string, string, string, string]> = {
    todo:            ['Todo','todo','var(--surface)','var(--content-muted)','Preparation'],
    context_loading: ['Loading Context','context_loading','#e0efff','#005bb8','Preparation'],
    analyzing:       ['Analyzing','analyzing','#e0efff','#005bb8','Preparation'],
    planning:        ['Planning','planning','#e0efff','#005bb8','Preparation'],
    spec_review:     ['Spec Review','spec_review','#fef3c6','#795800','Preparation · Gate'],
    coding:          ['Coding','coding','#e0efff','#005bb8','Execution'],
    testing:         ['Testing','testing','#e0efff','#005bb8','Execution'],
    reviewing:       ['Reviewing','reviewing','#f3e8ff','#7f22fe','Execution'],
    fixing:          ['Fixing','fixing','#fff1e0','#b75000','Execution'],
    pr_ready:        ['PR Ready','pr_ready','#d9f5e7','#007956','Finalization'],
    human_review:    ['Human Review','human_review','#fef3c6','#795800','Finalization · Gate'],
    merged:          ['Merged','merged','#e6f4ea','#00590e','Finalization'],
    failed:          ['Failed','failed','#ffe2e2','#bf000f','Finalization'],
  };
  const [label, , bg, fg, group] = P[st] || P.todo;
  const running = ['context_loading','analyzing','planning','coding','testing','reviewing','fixing'].includes(st);
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
          <span className="inline-flex px-3 py-1 rounded-full text-xs font-medium bg-slate-500/5 text-content-muted border border-stroke/15 shadow-sm">
            {group}
          </span>
          <span className="inline-flex px-3 py-1 rounded-full text-xs font-bold bg-rose-500/10 text-rose-600 dark:text-rose-400 border border-rose-500/20 shadow-sm">
            P{task?.priority || 0}
          </span>
        </div>
        <div className="mt-4 bg-slate-500/[0.02] dark:bg-slate-900/10 rounded-2xl border border-stroke/10 p-4 shadow-sm">
          <DescriptionBody />
        </div>
      </div>
      <div className="md:text-right shrink-0 self-start md:self-center bg-slate-500/5 dark:bg-slate-900/30 border border-stroke/10 rounded-xl px-4 py-2.5 shadow-sm hover:shadow-md transition-all duration-200">
        <div className="text-[10px] uppercase font-bold tracking-wider text-content-muted mb-0.5">Elapsed</div>
        <div className="font-mono text-base font-bold text-foreground flex items-center md:justify-end gap-1.5">
          <span className={`w-1.5 h-1.5 rounded-full bg-emerald-500 ${running && !paused ? 'animate-ping' : ''}`}></span>
          {formatTime(elapsedSeconds)}
        </div>
      </div>
    </div>
  );
}
