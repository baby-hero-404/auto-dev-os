"use client";

import Link from "next/link";
import { useTaskDetail } from "./TaskDetailContext";

export function TaskHeader() {
  const {
    projectID,
    task,
    workflow,
    execute,
    analyze,
    retry,
    pause,
    isPaused,
    isExecutionReady,
  } = useTaskDetail();

  const jobStatus = workflow?.job?.status?.toLowerCase();
  const canPause = jobStatus === "running";
  const canResume = !!(
    task &&
    isPaused &&
    task.status !== "pr_ready" &&
    task.status !== "human_review" &&
    task.status !== "merged" &&
    task.spec_status !== "pending_review" &&
    task.spec_status !== "changes_requested" &&
    task.spec_status !== "clarification_required"
  );
  const showAnalyze = !!(task && (task.status === "todo" || task.status === "failed") && !isExecutionReady);
  const showExecute = !!(task && isExecutionReady);
  const st = task?.status || "todo";

  return (
    <div className="flex items-center justify-between gap-4 px-8 py-3.5 bg-card border-b border-stroke">
      <div className="flex items-center gap-2 text-sm text-content-muted">
        <Link href={`/projects/${projectID}`} className="inline-flex items-center gap-1.5 hover:text-foreground transition">
          <svg xmlns="http://www.w3.org/2000/svg" width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round"><path d="m12 19-7-7 7-7"/><path d="M19 12H5"/></svg>
          Back to Project
        </Link>
      </div>
      <div className="flex items-center gap-2">
        {canPause && (
          <button onClick={pause} className="flex items-center gap-1.5 px-3.5 py-1.5 rounded-lg border border-stroke bg-card text-[13px] font-medium text-foreground hover:bg-surface cursor-pointer">
            ⏸ Pause
          </button>
        )}
        {canResume && (
          <button onClick={execute} className="flex items-center gap-1.5 px-3.5 py-1.5 rounded-lg border border-stroke bg-card text-[13px] font-medium text-foreground hover:bg-surface cursor-pointer">
            ▶ Resume
          </button>
        )}
        {showAnalyze && (
          <button onClick={analyze} className="px-4 py-1.5 rounded-lg border-none bg-brand-primary text-slate-950 text-[13px] font-semibold hover:opacity-90 cursor-pointer">
            ▶ Start Analysis
          </button>
        )}
        {showExecute && (
          <button onClick={execute} className="px-4 py-1.5 rounded-lg border-none bg-brand-primary text-slate-950 text-[13px] font-semibold hover:opacity-90 cursor-pointer">
            ▶ Start Execution
          </button>
        )}
        {st === 'failed' && (
          <button onClick={retry} className="px-4 py-1.5 rounded-lg border-none bg-[#bf000f] text-white text-[13px] font-semibold hover:bg-[#e40014] cursor-pointer">
            ↻ Restart Task
          </button>
        )}
      </div>
    </div>
  );
}
