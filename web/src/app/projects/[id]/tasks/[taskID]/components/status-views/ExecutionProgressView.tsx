"use client";

import { useMemo } from "react";
import { useTaskDetail } from "../TaskDetailContext";
import { Check } from "lucide-react";

/** context_loading, analyzing */
export function ExecutionProgressView() {
  const { task, workflow, workflowSteps } = useTaskDetail();
  const st = task?.status || "todo";

  const completedCheckpoints = useMemo(() => {
    if (!workflow?.checkpoints) return [];
    const seen = new Set<string>();
    const list: typeof workflow.checkpoints = [];
    const currentRunningStep = workflow?.job?.status === "running" ? workflow?.job?.step : undefined;
    for (const cp of workflow.checkpoints) {
      if (!cp.step || seen.has(cp.step)) continue;
      if (!workflowSteps.includes(cp.step)) continue;
      if (cp.step === currentRunningStep || cp.state?.status === "running") continue;
      const status = cp.state?.status;
      if (status === "success" || status === "recorded" || status === "skipped" || !status) {
        seen.add(cp.step);
        list.push(cp);
      }
    }
    return list;
  }, [workflow, workflowSteps]);

  return (
    <div className="flex flex-col gap-4">
      <div className="bg-linear-to-br from-blue-500/10 via-blue-500/5 to-slate-500/5 border border-blue-500/20 rounded-2xl p-5.5 shadow-sm relative overflow-hidden animate-fade-in">
        <div className="absolute -top-10 -right-10 w-24 h-24 bg-blue-500/10 rounded-full blur-2xl pointer-events-none" />
        <div className="flex items-center gap-3 mb-4 z-10">
          <span className="w-5 h-5 rounded-full border-2 border-blue-500/20 border-t-blue-600 dark:border-t-blue-400 animate-spin shrink-0"></span>
          <span className="text-sm font-bold text-blue-700 dark:text-blue-400 tracking-wide capitalize">
            {st === "context_loading" ? "Loading Context..." : "Analyzing Requirements..."}
          </span>
        </div>
        <div className="flex flex-col gap-2 pl-1 z-10">
          {completedCheckpoints.map((cp, idx) => (
            <div key={idx} className="flex items-center gap-2.5 py-1 text-xs font-mono text-emerald-800 dark:text-emerald-400/90">
              <span className="w-4 h-4 flex items-center justify-center rounded-full bg-emerald-500/10 border border-emerald-500/20 shrink-0">
                <Check className="h-3 w-3 text-emerald-600 dark:text-emerald-400" />
              </span>
              <span className="font-semibold">{cp.step.replace(/^cli_/, "CLI ").replace(/_/g, " ")}</span>
            </div>
          ))}
        </div>
      </div>
    </div>
  );
}
