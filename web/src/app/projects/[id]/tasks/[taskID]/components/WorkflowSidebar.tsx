"use client";

import { useMemo, useCallback } from "react";
import { Bot, Clock } from "lucide-react";
import { useTaskDetail, formatStepName } from "./TaskDetailContext";
import { TaskActions } from "./TaskActions";

function InfoRow({ label, value }: { label: string; value: string }) {
  return (
    <div>
      <dt className="text-[10px] font-bold uppercase tracking-wider text-content-muted">{label}</dt>
      <dd className="mt-1 break-all font-mono text-[11px] text-foreground bg-surface/30 border border-stroke/60 rounded px-2 py-1">{value}</dd>
    </div>
  );
}

export function WorkflowSidebar() {
  const {
    task,
    workflow,
    analysisData,
  } = useTaskDetail();

  const checkpointsList = useMemo(() => {
    return [...(workflow?.checkpoints ?? [])].reverse();
  }, [workflow?.checkpoints]);

  const agentName = task?.agent_id || "Unassigned";
  const currentStep = workflow?.job?.step || "none";
  const attemptsCount = String(workflow?.job?.attempts ?? 0);
  const lastErrorText = workflow?.job?.last_error || "none";

  return (
    <aside className="space-y-6">
      <TaskActions />
      <div className="rounded-xl border border-stroke bg-card p-5 shadow-sm">
        <div className="mb-4 flex items-center gap-2 border-b border-stroke pb-3">
          <Bot size={16} className="text-brand-primary" />
          <h2 className="font-heading text-base font-bold text-foreground">Agent Activity</h2>
        </div>
        <dl className="space-y-3.5 text-xs">
          <InfoRow label="Assigned agent" value={agentName} />
          <InfoRow label="Current step" value={currentStep} />
          <InfoRow label="Attempts" value={attemptsCount} />
          <InfoRow label="Last error" value={lastErrorText} />
        </dl>
      </div>

      <div className="rounded-xl border border-stroke bg-card p-5 shadow-sm">
        <div className="mb-4 flex items-center gap-2 border-b border-stroke pb-3">
          <Clock size={16} className="text-brand-primary" />
          <h2 className="font-heading text-base font-bold text-foreground">Checkpoints</h2>
        </div>
        <div className="relative space-y-4 max-h-[350px] overflow-y-auto pr-2 custom-scrollbar pl-2 mt-2">
          <div className="absolute left-3.5 top-2 bottom-2 w-[2px] bg-stroke/60 rounded-full" />
          {checkpointsList.map((checkpoint) => (
            <div key={checkpoint.id} className="relative pl-7 group">
              <div className="absolute left-[-2px] top-2.5 size-[11px] rounded-full border-2 border-card bg-brand-primary ring-2 ring-transparent group-hover:ring-brand-primary/30 transition-all" />
              <div className="rounded-lg border border-stroke bg-surface/40 p-2.5 hover:bg-surface/80 transition-colors shadow-sm">
                <div className="font-mono text-[11px] font-bold text-brand-primary capitalize tracking-wide">
                  {formatStepName(checkpoint.step, analysisData)}
                </div>
                <div className="text-[10px] text-content-muted mt-1 font-medium">
                  {new Date(checkpoint.created_at).toLocaleString()}
                </div>
              </div>
            </div>
          ))}
          {checkpointsList.length === 0 && (
            <p className="text-xs text-content-muted italic pl-6">No checkpoints recorded.</p>
          )}
        </div>
      </div>
    </aside>
  );
}
