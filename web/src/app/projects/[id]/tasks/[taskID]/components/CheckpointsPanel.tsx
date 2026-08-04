"use client";

import { useState } from "react";
import { useTaskDetail } from "./TaskDetailContext";
import { AlertTriangle, Bot, Milestone, Calendar, Info, Terminal, ChevronDown, ChevronRight } from "lucide-react";
import { ReviewVerdictCard } from "./ReviewVerdictCard";
import type { ReviewVerdict, WorkflowArtifact } from "@/lib/types";

// Artifacts for retried steps are saved with a "_cycle_N" suffix appended to
// the step name (see checkpoint.Store.SaveArtifact), so a checkpoint's own
// step name only matches artifacts by prefix, not exact equality.
function artifactsForStep(artifacts: WorkflowArtifact[] | undefined, step: string, type: string, attempt?: number): WorkflowArtifact[] {
  if (!artifacts) return [];
  const targetStep = attempt && attempt > 1 ? `${step}_cycle_${attempt}` : step;
  return artifacts.filter(
    (a) => a.type === type && a.step === targetStep
  );
}

function CliOutputViewer({ output }: { output: string }) {
  const [expanded, setExpanded] = useState(false);
  return (
    <div className="mt-1.5 rounded-lg border border-stroke/10 bg-surface/20 dark:bg-surface/50 overflow-hidden">
      <button
        type="button"
        onClick={() => setExpanded((v) => !v)}
        className="w-full flex items-center gap-1.5 px-2.5 py-1.5 text-[10px] font-bold uppercase tracking-wider text-content-muted hover:text-foreground transition-colors"
      >
        {expanded ? <ChevronDown size={12} /> : <ChevronRight size={12} />}
        <Terminal size={11} />
        CLI Output
      </button>
      {expanded && (
        <pre className="text-[10px] font-mono whitespace-pre-wrap break-all p-2.5 pt-0 max-h-[320px] overflow-y-auto custom-scrollbar text-foreground/80">
          {output}
        </pre>
      )}
    </div>
  );
}

function CliPromptViewer({ prompt }: { prompt: string }) {
  const [expanded, setExpanded] = useState(false);
  return (
    <div className="mt-1.5 rounded-lg border border-stroke/10 bg-surface/20 dark:bg-surface/50 overflow-hidden">
      <button
        type="button"
        onClick={() => setExpanded((v) => !v)}
        className="w-full flex items-center gap-1.5 px-2.5 py-1.5 text-[10px] font-bold uppercase tracking-wider text-content-muted hover:text-foreground transition-colors"
      >
        {expanded ? <ChevronDown size={12} /> : <ChevronRight size={12} />}
        <Terminal size={11} />
        CLI Input (command, prompt, skills/rules)
      </button>
      {expanded && (
        <pre className="text-[10px] font-mono whitespace-pre-wrap break-all p-2.5 pt-0 max-h-[320px] overflow-y-auto custom-scrollbar text-foreground/80">
          {prompt}
        </pre>
      )}
    </div>
  );
}

const STEP_LABELS: Record<string, string> = {
  cli_analyze: "Analyze (CLI)",
  cli_spec: "Author Spec (CLI)",
  cli_implement: "Implement (CLI)",
  cross_review: "Cross Review",
  cli_mr: "Merge Request (CLI)",
};

function friendlyStepName(step: string): string {
  return STEP_LABELS[step] ?? step;
}

export function CheckpointsPanel() {
  const { task, workflow, artifacts } = useTaskDetail();
 
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
        <div className="rounded-2xl border border-stroke/10 bg-surface/50 p-4 flex items-center gap-3.5 shadow-sm">
          <Bot className="text-brand-primary shrink-0" size={18} />
          <div>
            <div className="text-[10px] font-bold uppercase tracking-wider text-content-muted">Assigned Agent</div>
            <div className="text-xs font-bold font-mono text-foreground mt-1">{agentName}</div>
          </div>
        </div>
 
        <div className="rounded-2xl border border-stroke/10 bg-surface/50 p-4 flex items-center gap-3.5 shadow-sm">
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
          <div className="border border-stroke/10 rounded-2xl overflow-hidden bg-surface/20 divide-y divide-stroke/10 shadow-sm">
            {reversedCheckpoints.map((cp, idx) => {
              const status = typeof cp.state?.status === "string" ? cp.state.status : "recorded";
              const error = typeof cp.state?.error === "string" ? cp.state.error : undefined;
              
              // status styles
              let statusBadge = "bg-surface/50 text-foreground dark:bg-surface dark:text-content-muted";
              if (status === "success" || status === "recorded" || status === "skipped" || status === "waiting_approval") {
                statusBadge = "bg-emerald-500/10 text-emerald-600 dark:text-emerald-400 border border-emerald-500/20";
              } else if (status === "running") {
                statusBadge = "bg-sky-500/10 text-sky-600 dark:text-sky-400 border border-sky-500/20";
              } else if (status === "failed") {
                statusBadge = "bg-rose-500/10 text-rose-600 dark:text-rose-400 border border-rose-500/20";
              }
 
              const formattedTime = new Date(cp.created_at).toLocaleString();
              /* eslint-disable @typescript-eslint/no-explicit-any */
              const reviewVerdict = (cp.state?.output as any)?.review_verdict as ReviewVerdict | undefined;
              /* eslint-enable @typescript-eslint/no-explicit-any */
              const attempt = typeof cp.state?.attempt === "number" ? cp.state.attempt : undefined;
              const cliOutputArtifacts = artifactsForStep(artifacts, cp.step, "cli_output", attempt);
              const cliPromptArtifacts = artifactsForStep(artifacts, cp.step, "cli_prompt", attempt);

              return (
                <div key={idx} className="p-3.5 text-xs flex flex-col gap-1.5 hover:bg-surface/50 transition-colors duration-150">
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
                  {reviewVerdict && <ReviewVerdictCard verdict={reviewVerdict} />}
                  {error && (
                    <p className="text-[10px] font-mono bg-rose-500/5 text-rose-500 rounded-lg p-2.5 mt-1.5 border border-rose-500/10 max-h-[100px] overflow-y-auto custom-scrollbar">
                      {error}
                    </p>
                  )}
                  {cliPromptArtifacts.map((art) => {
                    let prompt = "";
                    try {
                      prompt = typeof art.payload === "string" ? art.payload : JSON.stringify(art.payload, null, 2);
                    } catch {
                      prompt = String(art.payload);
                    }
                    return <CliPromptViewer key={art.id} prompt={prompt} />;
                  })}
                  {cliOutputArtifacts.map((art) => {
                    let output = "";
                    try {
                      output = typeof art.payload === "string" ? art.payload : JSON.stringify(art.payload, null, 2);
                    } catch {
                      output = String(art.payload);
                    }
                    return <CliOutputViewer key={art.id} output={output} />;
                  })}
                </div>
              );
            })}
          </div>
        )}
      </div>
    </div>
  );
}
