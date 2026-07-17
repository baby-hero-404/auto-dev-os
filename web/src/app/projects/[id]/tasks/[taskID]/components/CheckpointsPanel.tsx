"use client";

import { useTaskDetail } from "./TaskDetailContext";
import { AlertTriangle, Bot, Milestone, Calendar } from "lucide-react";

export function CheckpointsPanel() {
  const { task, workflow } = useTaskDetail();

  if (!workflow) return null;

  const agentName = task?.agent_id || "Unassigned";
  const attempts = workflow.job?.attempts ?? 0;
  const lastError = workflow.job?.last_error;
  const checkpoints = workflow.checkpoints || [];
  
  // reversed checkpoints
  const reversedCheckpoints = [...checkpoints].reverse();

  return (
    <div className="space-y-4 text-left">
      {/* Agent details */}
      <div className="grid grid-cols-1 sm:grid-cols-2 gap-3">
        <div className="rounded-lg border border-stroke/50 bg-surface/30 p-3.5 flex items-center gap-3">
          <Bot className="text-brand-primary shrink-0" size={18} />
          <div>
            <div className="text-[10px] font-bold uppercase tracking-wider text-content-muted">Assigned Agent</div>
            <div className="text-sm font-semibold font-mono text-foreground mt-0.5">{agentName}</div>
          </div>
        </div>

        <div className="rounded-lg border border-stroke/50 bg-surface/30 p-3.5 flex items-center gap-3">
          <Milestone className="text-brand-primary shrink-0" size={18} />
          <div>
            <div className="text-[10px] font-bold uppercase tracking-wider text-content-muted">Execution Attempts</div>
            <div className="text-sm font-semibold font-mono text-foreground mt-0.5">{attempts}</div>
          </div>
        </div>
      </div>

      {lastError && (
        <div className="rounded-lg border border-rose-500/20 bg-rose-500/5 p-3.5 flex items-start gap-2.5">
          <AlertTriangle className="text-rose-500 shrink-0 mt-0.5" size={16} />
          <div className="min-w-0 flex-1">
            <div className="text-[10px] font-bold uppercase tracking-wider text-rose-500">Last Error</div>
            <p className="text-xs font-mono bg-rose-500/10 border border-rose-500/20 rounded p-2.5 mt-1.5 break-all whitespace-pre-wrap text-rose-800 dark:text-rose-200">
              {lastError}
            </p>
          </div>
        </div>
      )}

      {/* Checkpoints list */}
      <div>
        <h4 className="text-xs font-bold uppercase tracking-wider text-content-muted mb-2.5">
          Checkpoint History ({checkpoints.length})
        </h4>

        {reversedCheckpoints.length === 0 ? (
          <p className="text-xs text-content-muted italic">No checkpoints recorded yet.</p>
        ) : (
          <div className="border border-stroke/50 rounded-lg overflow-hidden bg-surface/10 divide-y divide-stroke/30">
            {reversedCheckpoints.map((cp, idx) => {
              const status = typeof cp.state?.status === "string" ? cp.state.status : "recorded";
              const error = typeof cp.state?.error === "string" ? cp.state.error : undefined;
              
              // status styles
              let statusBadge = "bg-slate-100 text-slate-700 dark:bg-slate-900 dark:text-slate-400";
              if (status === "success" || status === "recorded" || status === "skipped") {
                statusBadge = "bg-emerald-500/10 text-emerald-600 dark:text-emerald-400 border border-emerald-500/20";
              } else if (status === "running") {
                statusBadge = "bg-sky-500/10 text-sky-600 dark:text-sky-400 border border-sky-500/20";
              } else if (status === "failed") {
                statusBadge = "bg-rose-500/10 text-rose-600 dark:text-rose-400 border border-rose-500/20";
              }

              const formattedTime = new Date(cp.created_at).toLocaleString();

              return (
                <div key={idx} className="p-3 text-xs flex flex-col gap-1.5">
                  <div className="flex items-center justify-between gap-3">
                    <span className="font-mono font-bold text-foreground truncate">{cp.step}</span>
                    <span className={`text-[9px] font-bold uppercase px-1.5 py-0.5 rounded ${statusBadge}`}>
                      {status}
                    </span>
                  </div>
                  <div className="flex items-center gap-1.5 text-content-muted text-[10px]">
                    <Calendar size={11} />
                    <span>{formattedTime}</span>
                  </div>
                  {error && (
                    <p className="text-[10px] font-mono bg-rose-500/5 text-rose-500 rounded p-1.5 mt-1 border border-rose-500/10 max-h-[80px] overflow-y-auto custom-scrollbar">
                      {error}
                    </p>
                  )}
                </div>
              );
            })}
          </div>
        )}
      </div>
    </div>
  );
}
