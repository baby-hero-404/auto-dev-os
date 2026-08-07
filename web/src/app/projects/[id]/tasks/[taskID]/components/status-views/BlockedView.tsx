"use client";

import { useState } from "react";
import { AlertOctagon, RotateCcw, Loader2 } from "lucide-react";
import { useTaskDetail } from "../TaskDetailContext";
import { tasks as tasksApi } from "@/lib/api/projects";

/** blocked — recovery, not approval */
export function BlockedView() {
  const { task, taskID, token, mutateWorkflow, setError } = useTaskDetail();
  const [submitting, setSubmitting] = useState(false);

  const handleRetry = async () => {
    if (!token) return;
    setSubmitting(true);
    try {
      await tasksApi.retrySplit(taskID, token);
      await mutateWorkflow();
    } catch (err) {
      setError(err instanceof Error ? err.message : "Failed to retry blocked task");
    } finally {
      setSubmitting(false);
    }
  };

  return (
    <div className="bg-amber-500/10 border border-amber-500/30 rounded-2xl p-5 flex items-start gap-4 animate-fade-in">
      <div className="p-2.5 rounded-xl bg-amber-500/20 text-amber-400 shrink-0">
        <AlertOctagon className="w-6 h-6" />
      </div>

      <div className="flex-1 space-y-1">
        <div className="flex items-center justify-between">
          <h4 className="text-sm font-bold text-amber-400 uppercase tracking-wide">
            Execution Blocked
          </h4>
          <span className="px-2 py-0.5 rounded text-[10px] font-mono font-bold bg-amber-500/20 text-amber-300">
            BLOCKED
          </span>
        </div>
        <p className="text-xs text-content-muted leading-relaxed">
          {task?.block_reason === "security_review_required"
            ? "Execution paused for security review — a change touched a protected path or pattern requiring human sign-off."
            : "Parent execution is paused because a child task encountered an error. Review the failing child task below, update instructions if needed, then retry to resume execution."}
        </p>

        {task?.blocked_child_id && (
          <div className="text-xs font-mono text-amber-300/80 pt-1">
            Blocked child ID: <span className="text-foreground">{task.blocked_child_id}</span>
          </div>
        )}
      </div>

      <button
        onClick={handleRetry}
        disabled={submitting}
        className="px-4 py-2 rounded-xl bg-amber-500 hover:bg-amber-600 text-slate-950 font-bold text-xs shadow-md transition-all flex items-center gap-2 shrink-0 self-center disabled:opacity-50"
      >
        {submitting ? <Loader2 className="w-3.5 h-3.5 animate-spin" /> : <RotateCcw className="w-3.5 h-3.5" />}
        Retry
      </button>
    </div>
  );
}
