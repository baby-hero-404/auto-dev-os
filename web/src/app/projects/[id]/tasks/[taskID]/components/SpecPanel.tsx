"use client";

import { useCallback } from "react";
import { Sparkles, Check, AlertCircle } from "lucide-react";
import { Markdown } from "@/components/ui/markdown";
import { TaskClarificationForm } from "@/components/projects/task-clarification-form";
import { useTaskDetail, isAffectedFile } from "./TaskDetailContext";

export function SpecPanel() {
  const {
    taskID,
    token,
    task,
    activeSpecTab,
    setActiveSpecTab,
    analysisData,
    clarificationQuestions,
    mutateWorkflow,
    completedPlanSteps,
    togglePlanStep,
  } = useTaskDetail();

  const handleSelectSummary = useCallback(() => setActiveSpecTab("summary"), [setActiveSpecTab]);
  const handleSelectProposal = useCallback(() => setActiveSpecTab("proposal"), [setActiveSpecTab]);
  const handleSelectSpecs = useCallback(() => setActiveSpecTab("specs"), [setActiveSpecTab]);
  const handleSelectDesign = useCallback(() => setActiveSpecTab("design"), [setActiveSpecTab]);
  const handleSelectTasks = useCallback(() => setActiveSpecTab("tasks"), [setActiveSpecTab]);
  const handleAnswersSubmitted = useCallback(async () => {
    await mutateWorkflow();
  }, [mutateWorkflow]);

  const hasMarkdownData = !!(
    analysisData.proposal_md ||
    analysisData.specs_md ||
    analysisData.design_md ||
    analysisData.tasks_md
  );

  if (!task?.analysis || !task?.spec_status || task.spec_status === "none" || Object.keys(analysisData).length === 0) {
    return null;
  }

  return (
    <div className="rounded-xl border border-stroke bg-card p-5 shadow-sm">
      <div className="mb-4 flex flex-wrap items-center justify-between gap-4 border-b border-stroke pb-3">
        <div className="flex items-center gap-2">
          <Sparkles size={18} className="text-brand-primary" />
          <h2 className="font-heading text-base font-bold text-foreground">
            Proposed Task Specification
          </h2>
        </div>
        {hasMarkdownData && (
          <div className="flex gap-1.5 bg-surface/60 p-1.5 rounded-lg border border-stroke shadow-inner overflow-x-auto hide-scrollbar">
            <button
              onClick={handleSelectSummary}
              className={`px-3 py-1.5 rounded-md text-[11px] font-bold uppercase tracking-wider transition-all duration-200 cursor-pointer whitespace-nowrap ${activeSpecTab === "summary" ? "bg-card text-brand-primary shadow-sm ring-1 ring-stroke" : "text-content-muted hover:text-foreground hover:bg-card/50"
                }`}
            >
              Summary
            </button>
            {analysisData.proposal_md && (
              <button
                onClick={handleSelectProposal}
                className={`px-3 py-1.5 rounded-md text-[11px] font-bold uppercase tracking-wider transition-all duration-200 cursor-pointer whitespace-nowrap ${activeSpecTab === "proposal" ? "bg-card text-brand-primary shadow-sm ring-1 ring-stroke" : "text-content-muted hover:text-foreground hover:bg-card/50"
                  }`}
              >
                Proposal
              </button>
            )}
            {analysisData.specs_md && (
              <button
                onClick={handleSelectSpecs}
                className={`px-3 py-1.5 rounded-md text-[11px] font-bold uppercase tracking-wider transition-all duration-200 cursor-pointer whitespace-nowrap ${activeSpecTab === "specs" ? "bg-card text-brand-primary shadow-sm ring-1 ring-stroke" : "text-content-muted hover:text-foreground hover:bg-card/50"
                  }`}
              >
                Specs
              </button>
            )}
            {analysisData.design_md && (
              <button
                onClick={handleSelectDesign}
                className={`px-3 py-1.5 rounded-md text-[11px] font-bold uppercase tracking-wider transition-all duration-200 cursor-pointer whitespace-nowrap ${activeSpecTab === "design" ? "bg-card text-brand-primary shadow-sm ring-1 ring-stroke" : "text-content-muted hover:text-foreground hover:bg-card/50"
                  }`}
              >
                Design
              </button>
            )}
            {analysisData.tasks_md && (
              <button
                onClick={handleSelectTasks}
                className={`px-3 py-1.5 rounded-md text-[11px] font-bold uppercase tracking-wider transition-all duration-200 cursor-pointer whitespace-nowrap ${activeSpecTab === "tasks" ? "bg-card text-brand-primary shadow-sm ring-1 ring-stroke" : "text-content-muted hover:text-foreground hover:bg-card/50"
                  }`}
              >
                Tasks
              </button>
            )}
          </div>
        )}
      </div>

      {activeSpecTab === "summary" ? (
        <div className="space-y-4">
          {analysisData.scope && (
            <div>
              <h3 className="font-mono text-[10px] font-bold uppercase tracking-wider text-content-muted mb-1">
                Scope
              </h3>
              <p className="text-sm leading-relaxed text-foreground">{analysisData.scope}</p>
            </div>
          )}

          {analysisData.complexity_details && (
            <div className="grid grid-cols-3 gap-2 mt-3">
              <div className="rounded border border-stroke bg-surface p-2 flex flex-col">
                <span className="text-[9px] uppercase tracking-wider font-bold text-content-muted">Architecture</span>
                <span className="text-xs font-semibold capitalize">{analysisData.complexity_details.architecture}</span>
              </div>
              <div className={`rounded border p-2 flex flex-col ${analysisData.complexity_details.data_migration ? 'border-amber-500/30 bg-amber-500/10' : 'border-stroke bg-surface'}`}>
                <span className="text-[9px] uppercase tracking-wider font-bold text-content-muted">Data Migration</span>
                <span className={`text-xs font-semibold ${analysisData.complexity_details.data_migration ? 'text-amber-500' : 'text-content-muted'}`}>{analysisData.complexity_details.data_migration ? "Yes" : "No"}</span>
              </div>
              <div className={`rounded border p-2 flex flex-col ${analysisData.complexity_details.breaking_change ? 'border-rose-500/30 bg-rose-500/10' : 'border-stroke bg-surface'}`}>
                <span className="text-[9px] uppercase tracking-wider font-bold text-content-muted">Breaking Change</span>
                <span className={`text-xs font-semibold ${analysisData.complexity_details.breaking_change ? 'text-rose-500' : 'text-content-muted'}`}>{analysisData.complexity_details.breaking_change ? "Yes" : "No"}</span>
              </div>
            </div>
          )}

          {analysisData.risk_domains && analysisData.risk_domains.length > 0 && (
            <div>
              <h3 className="font-mono text-[10px] font-bold uppercase tracking-wider text-content-muted mb-2">
                Risk Domains
              </h3>
              <div className="flex flex-wrap gap-1.5">
                {analysisData.risk_domains.map((domain) => (
                  <span
                    key={domain}
                    className="rounded-full border border-amber-500/20 bg-amber-500/10 px-2.5 py-0.5 text-[10px] font-semibold text-amber-600 dark:text-amber-400"
                  >
                    {domain}
                  </span>
                ))}
              </div>
            </div>
          )}

          <TaskClarificationForm
            taskID={taskID}
            specStatus={task?.spec_status}
            token={token}
            clarificationQuestions={clarificationQuestions}
            onAnswersSubmitted={handleAnswersSubmitted}
          />

          {task?.spec_status === "changes_requested" && clarificationQuestions.length === 0 && (
            <div className="rounded-lg border border-sky-500/20 bg-sky-500/5 p-4">
              <div className="flex items-start gap-2">
                <AlertCircle size={14} className="mt-0.5 text-sky-500" />
                <div>
                  <h3 className="text-xs font-bold uppercase tracking-wider text-sky-700 dark:text-sky-400">
                    Spec changes requested
                  </h3>
                  <p className="mt-1 text-xs leading-relaxed text-content-muted">
                    This task was sent back for a spec update, but there were no clarification questions to answer.
                  </p>
                </div>
              </div>
            </div>
          )}

          <div className="grid md:grid-cols-2 gap-5 pt-2">
            {analysisData.execution_plan && analysisData.execution_plan.length > 0 && (
              <div>
                <h3 className="font-mono text-[10px] font-bold uppercase tracking-wider text-content-muted mb-2.5 flex items-center gap-1.5">
                  <Check size={14} className="text-brand-primary" />
                  Interactive Execution Plan
                </h3>
                <div className="space-y-2 max-h-[350px] overflow-y-auto pr-2 custom-scrollbar">
                  {analysisData.execution_plan.map((step, idx) => {
                    const isDone = !!completedPlanSteps[idx];
                    return (
                      <label
                        key={idx}
                        className={`group flex items-start gap-3 rounded-xl border p-3.5 transition-all duration-300 cursor-pointer select-none relative overflow-hidden ${isDone
                          ? "border-emerald-500/30 bg-emerald-500/10 text-content-muted shadow-sm"
                          : "border-stroke bg-surface hover:border-brand-primary/50 text-foreground hover:shadow-md hover:bg-surface/80"
                          }`}
                      >
                        <input
                          type="checkbox"
                          checked={isDone}
                          onChange={() => togglePlanStep(idx)}
                          className="hidden"
                        />
                        <div className={`mt-0.5 flex size-5 shrink-0 items-center justify-center rounded-[6px] border transition-all duration-300 ${isDone ? "bg-emerald-500 border-emerald-500 text-slate-950 scale-110" : "border-stroke/80 bg-background group-hover:border-brand-primary group-hover:bg-brand-primary/10"
                          }`}>
                          {isDone && <Check size={14} strokeWidth={3.5} />}
                        </div>
                        <div className={`flex-1 text-sm leading-relaxed transition-all duration-300 [&_p]:mb-0 ${isDone ? "line-through opacity-70" : ""}`}>
                          <Markdown content={step} />
                        </div>
                        {isDone && <div className="absolute inset-0 bg-gradient-to-r from-emerald-500/0 via-emerald-500/5 to-emerald-500/0 opacity-0 group-hover:opacity-100 transition-opacity pointer-events-none" />}
                      </label>
                    );
                  })}
                </div>
              </div>
            )}

            <div className="space-y-4">
              {analysisData.risks_details && analysisData.risks_details.length > 0 ? (
                <div>
                  <h3 className="font-mono text-[10px] font-bold uppercase tracking-wider text-content-muted mb-2">
                    Risks Assessment
                  </h3>
                  <ul className="space-y-2">
                    {analysisData.risks_details.map((risk, idx) => (
                      <li key={idx} className="flex flex-col gap-1 rounded border border-amber-500/20 bg-amber-500/5 p-2 text-xs">
                        <div className="flex items-center justify-between">
                          <span className="font-semibold text-amber-600 dark:text-amber-400">{risk.risk}</span>
                          <div className="flex gap-1.5">
                            <span className="rounded bg-background px-1.5 py-0.5 text-[9px] uppercase">{risk.probability} prob</span>
                            <span className="rounded bg-background px-1.5 py-0.5 text-[9px] uppercase">{risk.severity} sev</span>
                          </div>
                        </div>
                        <div className="text-content-muted/80 mt-1"><span className="font-medium text-content-muted">Mitigation:</span> {risk.mitigation}</div>
                        <div className="text-[10px] font-mono text-brand-primary/70 mt-0.5">Owner: {risk.owner}</div>
                      </li>
                    ))}
                  </ul>
                </div>
              ) : analysisData.risks && analysisData.risks.length > 0 && (
                <div>
                  <h3 className="font-mono text-[10px] font-bold uppercase tracking-wider text-content-muted mb-2">
                    Risks
                  </h3>
                  <ul className="space-y-1.5">
                    {analysisData.risks.map((risk, idx) => (
                      <li key={idx} className="flex items-start gap-2 text-xs text-content-muted">
                        <span className="mt-1.5 size-1.5 shrink-0 rounded-full bg-amber-500" />
                        <span className="leading-5">{risk}</span>
                      </li>
                    ))}
                  </ul>
                </div>
              )}

              {analysisData.affected_files && analysisData.affected_files.length > 0 && (
                <div>
                  <h3 className="font-mono text-[10px] font-bold uppercase tracking-wider text-content-muted mb-2">
                    Estimated Affected Files
                  </h3>
                  <div className="flex flex-wrap gap-1.5">
                    {analysisData.affected_files.map((fileObj, idx) => {
                      const path = isAffectedFile(fileObj) ? fileObj.file : typeof fileObj === "string" ? fileObj : "";
                      const repo = isAffectedFile(fileObj) && fileObj.repo ? fileObj.repo : null;
                      return (
                        <span
                          key={idx}
                          className="rounded border border-stroke bg-surface px-2 py-0.5 font-mono text-[10px] text-content-muted flex items-center gap-1"
                        >
                          {repo && <span className="text-brand-primary/70">{repo}/</span>}
                          {path}
                        </span>
                      );
                    })}
                  </div>
                </div>
              )}

              {analysisData.execution_boundaries && analysisData.execution_boundaries.length > 0 && (
                <div className="mt-4">
                  <h3 className="font-mono text-[10px] font-bold uppercase tracking-wider text-content-muted mb-2">
                    Execution Boundaries
                  </h3>
                  <div className="space-y-2">
                    {analysisData.execution_boundaries.map((boundary, idx) => (
                      <div key={idx} className="rounded-lg border border-stroke bg-surface p-3 text-[11px]">
                        <div className="flex items-center justify-between font-mono font-bold text-content mb-1">
                          <span>
                            {boundary.repo_name && <span className="text-brand-primary/70">{boundary.repo_name}/</span>}
                            {boundary.root || "./"}
                          </span>
                          <span className="text-[9px] px-1.5 py-0.5 rounded-full bg-brand-primary/10 text-brand-primary uppercase">
                            {boundary.module}
                          </span>
                        </div>
                        <div className="flex flex-wrap gap-1 mt-1.5">
                          {boundary.capabilities.map((cap, cIdx) => (
                            <span key={cIdx} className="rounded bg-content/5 text-content-muted px-1.5 py-0.5 font-mono text-[9px]">
                              {cap}
                            </span>
                          ))}
                        </div>
                      </div>
                    ))}
                  </div>
                </div>
              )}

              {analysisData.expanded_boundaries && analysisData.expanded_boundaries.length > 0 && (
                <div className="mt-4">
                  <h3 className="font-mono text-[10px] font-bold uppercase tracking-wider text-content-muted mb-2">
                    JIT Expanded Boundaries (Audit Trail)
                  </h3>
                  <div className="space-y-2">
                    {analysisData.expanded_boundaries.map((expanded, idx) => {
                      const isHighRisk = expanded.risk === "HIGH" || expanded.risk === "CRITICAL";
                      const isMediumRisk = expanded.risk === "MEDIUM";
                      const riskColor = isHighRisk 
                        ? "bg-red-500/10 text-red-500 border-red-500/20" 
                        : isMediumRisk 
                          ? "bg-amber-500/10 text-amber-500 border-amber-500/20" 
                          : "bg-emerald-500/10 text-emerald-500 border-emerald-500/20";
                      return (
                        <div key={idx} className="rounded-lg border border-stroke bg-surface/50 p-2.5 text-[11px] flex flex-col gap-1.5">
                          <div className="flex items-start justify-between">
                            <span className="font-mono font-bold text-content break-all">{expanded.file}</span>
                            <span className={`text-[8px] font-bold uppercase px-1.5 py-0.5 rounded border ${riskColor}`}>
                              {expanded.risk || "LOW"}
                            </span>
                          </div>
                          <p className="text-content-muted italic text-[10px]">{expanded.reason}</p>
                          {expanded.capability && (
                            <div className="flex items-center gap-1 mt-0.5">
                              <span className="text-[9px] text-content-muted">Capability:</span>
                              <span className="rounded bg-content/5 px-1 py-0.5 font-mono text-[9px] text-content font-bold">
                                {expanded.capability}
                              </span>
                            </div>
                          )}
                        </div>
                      );
                    })}
                  </div>
                </div>
              )}
            </div>
          </div>
        </div>
      ) : (
        <div className="rounded-lg border border-stroke bg-card p-5 overflow-auto max-h-[500px] leading-relaxed animate-fade-in shadow-inner text-sm">
          {activeSpecTab === "proposal" && <Markdown content={analysisData.proposal_md || ""} />}
          {activeSpecTab === "specs" && <Markdown content={analysisData.specs_md || ""} />}
          {activeSpecTab === "design" && <Markdown content={analysisData.design_md || ""} />}
          {activeSpecTab === "tasks" && <Markdown content={analysisData.tasks_md || ""} />}
        </div>
      )}
    </div>
  );
}
