"use client";

import { useTaskDetail } from "../TaskDetailContext";
import { Clock, Sparkles, Check, AlertCircle } from "lucide-react";

export function TodoView() {
  const { task, isExecutionReady, analyze, execute } = useTaskDetail();

  const isClarification = task?.spec_status === "clarification_required";

  if (isClarification) {
    return (
      <div className="bg-linear-to-br from-amber-500/10 via-amber-500/[0.02] to-orange-500/5 border border-amber-500/25 rounded-2xl p-5.5 flex flex-col sm:flex-row sm:items-center sm:justify-between gap-4 shadow-md hover:shadow-lg transition-all duration-200 animate-fade-in">
        <div className="flex items-center gap-3.5">
          <span className="w-11 h-11 flex items-center justify-center rounded-xl bg-amber-500/10 text-amber-600 dark:text-amber-500 border border-amber-500/20 shrink-0 shadow-inner">
            <AlertCircle className="h-5.5 w-5.5" />
          </span>
          <div>
            <div className="text-base font-bold text-foreground">Clarification Required</div>
            <div className="text-xs text-content-muted mt-0.5">The agent has asked one or more questions about the task. Please provide answers in the Spec panel below.</div>
          </div>
        </div>
      </div>
    );
  }

  return (
    <div className="bg-linear-to-br from-slate-500/5 via-slate-500/[0.02] to-slate-500/10 border border-stroke/10 rounded-2xl p-5.5 flex flex-col sm:flex-row sm:items-center sm:justify-between gap-4 shadow-md hover:shadow-lg transition-all duration-200 animate-fade-in">
      <div className="flex items-center gap-3.5">
        <span className="w-11 h-11 flex items-center justify-center rounded-xl bg-surface/50 text-content-muted border border-stroke/15 shrink-0 shadow-inner">
          <Clock className="h-5.5 w-5.5" />
        </span>
        <div>
          <div className="text-base font-bold text-foreground">Ready to Start</div>
          <div className="text-xs text-content-muted mt-0.5">Review the task details and start the workflow.</div>
        </div>
      </div>
      <div className="self-end sm:self-center">
        {!isExecutionReady ? (
          <button onClick={analyze} className="px-5 py-2.5 rounded-xl border-none bg-linear-to-r from-brand-primary/80 to-brand-primary hover:from-brand-primary hover:to-brand-primary text-background text-xs font-extrabold transition-all duration-150 hover:shadow-md hover:shadow-brand-primary/20 hover:scale-[1.02] cursor-pointer whitespace-nowrap flex items-center gap-2">
            <Sparkles className="h-4 w-4" /> Start Analysis
          </button>
        ) : (
          <button onClick={execute} className="px-5 py-2.5 rounded-xl border-none bg-linear-to-r from-emerald-500 to-teal-500 hover:from-emerald-400 hover:to-teal-400 text-white text-xs font-extrabold transition-all duration-150 hover:shadow-md hover:shadow-emerald-500/20 hover:scale-[1.02] cursor-pointer whitespace-nowrap flex items-center gap-2">
            <Check className="h-4 w-4" /> Start Execution
          </button>
        )}
      </div>
    </div>
  );
}
