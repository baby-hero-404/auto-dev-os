"use client";
 
import { useTaskDetail } from "./TaskDetailContext";
import { AlertTriangle, Bot, Milestone, Calendar, Info } from "lucide-react";

const STEP_LABELS: Record<string, string> = {
  cli_analyze: "Analyze (CLI)",
  cli_spec: "Author Spec (CLI)",
  cli_implement: "Implement (CLI)",
  cli_mr: "Merge Request (CLI)",
};

function friendlyStepName(step: string): string {
  return STEP_LABELS[step] ?? step;
}

export function CheckpointsPanel() {
  const { task, workflow } = useTaskDetail();
 
  if (!workflow) return null;
 
  const agentName = task?.agent_id || "Unassigned";
  const attempts = workflow.job?.attempts ?? 0;
  const lastError = workflow.job?.last_error;
  const checkpoints = workflow.checkpoints || [];
  
  // reversed checkpoints
  const reversedCheckpoints = [...checkpoints].reverse();
  const isPauseReason = lastError && (
    lastError.includes("workflow paused for human spec review") ||
    lastError.includes("workflow paused for human task clarification")
  );
 
  return (
    <div className="space-y-4 text-left">
      {/* Agent details */}
      <div className="grid grid-cols-1 sm:grid-cols-2 gap-3">
        <div className="rounded-2xl border border-stroke/10 bg-slate-500/5 p-4 flex items-center gap-3.5 shadow-sm">
          <Bot className="text-brand-primary shrink-0" size={18} />
          <div>
            <div className="text-[10px] font-bold uppercase tracking-wider text-content-muted">Assigned Agent</div>
            <div className="text-xs font-bold font-mono text-foreground mt-1">{agentName}</div>
          </div>
        </div>
 
        <div className="rounded-2xl border border-stroke/10 bg-slate-500/5 p-4 flex items-center gap-3.5 shadow-sm">
          <Milestone className="text-brand-primary shrink-0" size={18} />
          <div>
            <div className="text-[10px] font-bold uppercase tracking-wider text-content-muted">Execution Attempts</div>
            <div className="text-xs font-bold font-mono text-foreground mt-1">{attempts}</div>
          </div>
        </div>
      </div>
  
      {lastError && (
        <div className={`rounded-2xl border p-4 flex items-start gap-3 shadow-sm ${
          isPauseReason
            ? "border-amber-500/20 bg-amber-500/5 text-amber-500"
            : "border-rose-500/20 bg-rose-500/5 text-rose-500"
        }`}>
          {isPauseReason ? (
            <Info className="text-amber-500 shrink-0 mt-0.5" size={16} />
          ) : (
            <AlertTriangle className="text-rose-500 shrink-0 mt-0.5" size={16} />
          )}
          <div className="min-w-0 flex-1">
            <div className={`text-[10px] font-bold uppercase tracking-wider ${
              isPauseReason ? "text-amber-600 dark:text-amber-500" : "text-rose-600 dark:text-rose-500"
            }`}>
              {isPauseReason ? "Pause Reason" : "Last Error"}
            </div>
            <p className={`text-xs font-mono rounded-xl p-3 mt-2.5 break-all whitespace-pre-wrap ${
              isPauseReason
                ? "bg-amber-500/10 border border-amber-500/20 text-amber-800 dark:text-amber-200"
                : "bg-rose-500/10 border border-rose-500/20 text-rose-800 dark:text-rose-200"
            }`}>
              {lastError}
            </p>
          </div>
        </div>
      )}
 
      {/* Checkpoints list */}
      <div>
        <h4 className="text-[10px] font-bold uppercase tracking-wider text-content-muted mb-2.5 px-0.5">
          Checkpoint History ({checkpoints.length})
        </h4>
 
        {reversedCheckpoints.length === 0 ? (
          <p className="text-xs text-content-muted italic px-0.5">No checkpoints recorded yet.</p>
        ) : (
          <div className="border border-stroke/10 rounded-2xl overflow-hidden bg-slate-500/[0.02] divide-y divide-stroke/10 shadow-sm">
            {reversedCheckpoints.map((cp, idx) => {
              const status = typeof cp.state?.status === "string" ? cp.state.status : "recorded";
              const error = typeof cp.state?.error === "string" ? cp.state.error : undefined;
              
              // status styles
              let statusBadge = "bg-slate-100 text-slate-700 dark:bg-slate-900 dark:text-slate-400";
              if (status === "success" || status === "recorded" || status === "skipped" || status === "waiting_approval") {
                statusBadge = "bg-emerald-500/10 text-emerald-600 dark:text-emerald-400 border border-emerald-500/20";
              } else if (status === "running") {
                statusBadge = "bg-sky-500/10 text-sky-600 dark:text-sky-400 border border-sky-500/20";
              } else if (status === "failed") {
                statusBadge = "bg-rose-500/10 text-rose-600 dark:text-rose-400 border border-rose-500/20";
              }
 
              const formattedTime = new Date(cp.created_at).toLocaleString();
 
              return (
                <div key={idx} className="p-3.5 text-xs flex flex-col gap-1.5 hover:bg-slate-500/5 transition-colors duration-150">
                  <div className="flex items-center justify-between gap-3">
                    <span className="font-mono font-bold text-foreground truncate">{friendlyStepName(cp.step)}</span>
                    <span className={`text-[9px] font-bold uppercase px-2 py-0.5 rounded-full ${statusBadge}`}>
                      {status}
                    </span>
                  </div>
                  <div className="flex items-center gap-1.5 text-content-muted text-[10px]">
                    <Calendar size={11} />
                    <span>{formattedTime}</span>
                  </div>
                  {error && (
                    <p className="text-[10px] font-mono bg-rose-500/5 text-rose-500 rounded-lg p-2.5 mt-1.5 border border-rose-500/10 max-h-[100px] overflow-y-auto custom-scrollbar">
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
