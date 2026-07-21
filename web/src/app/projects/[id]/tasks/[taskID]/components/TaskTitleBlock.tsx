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
    <div className="flex items-start justify-between gap-6 mb-5.5">
      <div className="flex-1">
        <h1 className="m-0 mb-2.5 text-[26px] font-bold tracking-tight text-foreground">{task?.title || "Task"}</h1>
        <div className="flex items-center gap-2 mb-3">
          <span className="inline-flex items-center gap-1.5 px-2.5 py-0.5 rounded-full text-xs font-semibold border border-stroke/20" style={{ background: paused && running ? '#fef3c6' : bg, color: paused && running ? '#795800' : fg }}>
            <span className={`w-1.5 h-1.5 rounded-full ${running && !paused ? 'animate-pulse' : ''}`} style={{ background: paused && running ? '#795800' : fg }}></span>
            {paused && running ? 'Paused' : label}
          </span>
          <span className="inline-flex px-2.5 py-0.5 rounded-full text-xs font-medium bg-surface text-content-muted border border-stroke">
            {group}
          </span>
          <span className="inline-flex px-2.5 py-0.5 rounded-full text-xs font-semibold bg-[#ffe2e2] text-[#bf000f]">
            P{task?.priority || 0}
          </span>
        </div>
        <div className="mt-4">
          <DescriptionBody />
        </div>
      </div>
      <div className="text-right shrink-0">
        <div className="text-xs text-content-muted">Elapsed</div>
        <div className="font-mono text-[15px] font-semibold">{formatTime(elapsedSeconds)}</div>
      </div>
    </div>
  );
}
