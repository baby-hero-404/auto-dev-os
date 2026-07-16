"use client";

import { useTaskDetail, formatStepName, getStepDescription } from "./TaskDetailContext";
import { Loader2, Bot, ArrowRight } from "lucide-react";

export function ActiveWorkflowBanner() {
  const { workflow, workflowSteps, analysisData } = useTaskDetail();

  const isRunning = workflow?.job?.status === "running";
  if (!isRunning || !workflow?.job?.step) {
    return null;
  }

  const currentStep = workflow.job.step;
  const currentStepName = formatStepName(currentStep, analysisData);
  const currentStepDesc = getStepDescription(currentStep, analysisData);

  const currentIdx = workflowSteps.indexOf(currentStep);
  const nextStep = currentIdx !== -1 && currentIdx < workflowSteps.length - 1
    ? workflowSteps[currentIdx + 1]
    : null;

  const nextStepName = nextStep ? formatStepName(nextStep, analysisData) : null;

  return (
    <div className="relative overflow-hidden rounded-xl border border-sky-500/30 bg-sky-500/5 backdrop-blur-xl p-4 shadow-[0_0_20px_rgba(14,165,233,0.1)] flex flex-col sm:flex-row items-start sm:items-center justify-between gap-4 group transition-all">
      <div className="absolute inset-0 bg-gradient-to-r from-sky-500/0 via-sky-500/10 to-sky-500/0 -translate-x-full animate-[shimmer_2s_infinite] pointer-events-none" />
      <div className="relative flex items-start gap-3 z-10">
        <div className="p-2 rounded-lg bg-sky-500/10 text-sky-500 shrink-0 mt-0.5 border border-sky-500/20 shadow-[0_0_10px_rgba(14,165,233,0.2)]">
          <Bot size={18} />
        </div>
        <div>
          <h3 className="text-sm font-semibold text-foreground flex items-center gap-2">
            AI is currently working on <span className="text-sky-500 font-mono font-bold bg-sky-500/10 px-1.5 rounded">{currentStepName}</span>
            <span className="relative flex h-2.5 w-2.5 shrink-0 ml-1">
              <span className="animate-ping absolute inline-flex h-full w-full rounded-full bg-sky-400 opacity-75"></span>
              <span className="relative inline-flex rounded-full h-2.5 w-2.5 bg-sky-500 shadow-[0_0_5px_currentColor]"></span>
            </span>
          </h3>
          <p className="mt-1 text-xs text-content-muted leading-relaxed">
            {currentStepDesc}
          </p>
        </div>
      </div>

      {nextStepName && (
        <div className="relative z-10 flex items-center gap-1.5 text-[11px] font-semibold text-content-muted bg-surface/50 backdrop-blur border border-stroke/40 rounded-full px-3 py-1 self-stretch sm:self-auto justify-center">
          <span>Next</span>
          <ArrowRight size={12} className="text-content-muted" />
          <span className="font-mono text-foreground font-bold">{nextStepName}</span>
        </div>
      )}
    </div>
  );
}
