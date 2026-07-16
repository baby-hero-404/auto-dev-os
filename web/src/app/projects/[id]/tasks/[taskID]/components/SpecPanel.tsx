"use client";

import { useCallback, useEffect, useState } from "react";
import { Sparkles, Check, AlertCircle, ChevronDown, ChevronUp } from "lucide-react";
import { Markdown } from "@/components/ui/markdown";
import { TaskClarificationForm } from "@/components/projects/task-clarification-form";
import { useTaskDetail, isAffectedFile } from "./TaskDetailContext";

interface SpecPanelProps {
  /** Controlled collapse state (REQ-005). When omitted, the panel self-manages. */
  isExpanded?: boolean;
  onToggle?: () => void;
}

export function SpecPanel({ isExpanded, onToggle }: SpecPanelProps = {}) {
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

  // Outer collapse: default-collapsed. Uncontrolled fallback when no props given.
  const [internalOpen, setInternalOpen] = useState(false);
  const isOpen = isExpanded ?? internalOpen;
  const toggleOpen = onToggle ?? (() => setInternalOpen((v) => !v));

  const presenceChips = [
    { label: "Scope", present: !!analysisData.scope },
    { label: "Recommendation", present: !!analysisData.proposal_md },
    { label: "Architecture", present: !!analysisData.design_md },
    { label: "Risks", present: !!(analysisData.risk_domains && analysisData.risk_domains.length > 0) },
  ].filter((c) => c.present);

  const [isDescCollapsed, setIsDescCollapsed] = useState(true);
  const [isRisksCollapsed, setIsRisksCollapsed] = useState(true);
  const [isBoundariesCollapsed, setIsBoundariesCollapsed] = useState(true);
  const [isExpandedOpen, setIsExpandedOpen] = useState(false);

  useEffect(() => {
    if (!taskID || taskID === "undefined") return;
    if (typeof window !== "undefined") {
      const descVal = localStorage.getItem(`task-desc-collapsed-${taskID}`);
      const boundariesVal = localStorage.getItem(`task-boundaries-collapsed-${taskID}`);
      const storedRisks = localStorage.getItem(`task-risks-collapsed-${taskID}`);

      setIsDescCollapsed(descVal !== null ? descVal === "true" : true);
      setIsBoundariesCollapsed(boundariesVal !== null ? boundariesVal === "true" : true);

      if (storedRisks !== null) {
        setIsRisksCollapsed(storedRisks === "true");
      } else {
        const hasRisks = !!(analysisData.risk_domains && analysisData.risk_domains.length > 0);
        setIsRisksCollapsed(!hasRisks);
      }
    }
  }, [taskID, analysisData.risk_domains]);

  const toggleDescCollapse = () => {
    if (!taskID || taskID === "undefined") return;
    const nextState = !isDescCollapsed;
    setIsDescCollapsed(nextState);
    localStorage.setItem(`task-desc-collapsed-${taskID}`, String(nextState));
  };

  const toggleRisksCollapse = () => {
    if (!taskID || taskID === "undefined") return;
    const nextState = !isRisksCollapsed;
    setIsRisksCollapsed(nextState);
    localStorage.setItem(`task-risks-collapsed-${taskID}`, String(nextState));
  };

  const toggleBoundariesCollapse = () => {
    if (!taskID || taskID === "undefined") return;
    const nextState = !isBoundariesCollapsed;
    setIsBoundariesCollapsed(nextState);
    localStorage.setItem(`task-boundaries-collapsed-${taskID}`, String(nextState));
  };

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
    <div className="relative overflow-hidden rounded-xl border border-stroke/50 bg-card/60 backdrop-blur-xl p-5 shadow-lg hover:shadow-xl transition-all group">
      <div className="absolute inset-0 bg-gradient-to-br from-brand-primary/5 via-transparent to-transparent opacity-0 group-hover:opacity-100 transition-opacity duration-500 pointer-events-none" />
      <div className={`relative flex flex-wrap items-center justify-between gap-4 z-10 ${isOpen ? "mb-4 border-b border-stroke/40 pb-3" : ""}`}>
        <button
          type="button"
          onClick={toggleOpen}
          className="flex items-center gap-2 cursor-pointer text-left"
          aria-expanded={isOpen}
        >
          {isOpen ? <ChevronUp size={18} className="text-content-muted" /> : <ChevronDown size={18} className="text-content-muted" />}
          <Sparkles size={18} className="text-brand-primary" />
          <h2 className="font-heading text-base font-bold text-foreground">
            Proposed Task Specification
          </h2>
        </button>
        {!isOpen && (
          <div className="flex flex-wrap items-center gap-2">
            {presenceChips.map((c) => (
              <span key={c.label} className="inline-flex items-center gap-1 rounded-full border border-emerald-500/30 bg-emerald-500/10 px-2 py-0.5 text-[10px] font-semibold text-emerald-600 dark:text-emerald-400">
                <Check size={10} />
                {c.label}
              </span>
            ))}
            <span className="text-[10px] font-bold uppercase tracking-wider text-brand-primary">View details</span>
          </div>
        )}
        {isOpen && hasMarkdownData && (
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

      {isOpen && (activeSpecTab === "summary" ? (
        <div className="space-y-4">
          {analysisData.scope && (
            <div className="relative">
              <h3 className="font-mono text-[10px] font-bold uppercase tracking-wider text-content-muted mb-1">
                Scope
              </h3>
              <div
                className="relative overflow-hidden transition-all duration-300 ease-in-out"
                style={{ maxHeight: isDescCollapsed ? "3em" : "1000px" }}
              >
                <p className="text-sm leading-relaxed text-foreground">{analysisData.scope}</p>
                {isDescCollapsed && (
                  <div className="absolute bottom-0 left-0 right-0 h-6 bg-gradient-to-t from-card to-transparent pointer-events-none" />
                )}
              </div>
              <button
                onClick={toggleDescCollapse}
                className="text-[10px] font-bold text-brand-primary hover:text-brand-primary/80 transition-colors mt-1 cursor-pointer block"
              >
                {isDescCollapsed ? "Show Description" : "Hide Description"}
              </button>
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
              {((analysisData.risks_details && analysisData.risks_details.length > 0) || (analysisData.risks && analysisData.risks.length > 0)) && (
                <div>
                  <button
                    onClick={toggleRisksCollapse}
                    className="flex w-full items-center justify-between font-mono text-[10px] font-bold uppercase tracking-wider text-content-muted mb-2 cursor-pointer hover:text-foreground transition-colors"
                  >
                    <span>Risks Assessment ({analysisData.risks_details?.length || analysisData.risks?.length || 0})</span>
                    {!isRisksCollapsed ? <ChevronUp size={14} /> : <ChevronDown size={14} />}
                  </button>
                  <div
                    className="overflow-hidden transition-all duration-300 ease-in-out"
                    style={{ maxHeight: !isRisksCollapsed ? "400px" : "0px" }}
                  >
                    {analysisData.risks_details && analysisData.risks_details.length > 0 ? (
                      <ul className="space-y-2 max-h-[180px] overflow-y-auto pr-1.5 custom-scrollbar">
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
                    ) : (
                      <ul className="space-y-1.5 max-h-[150px] overflow-y-auto pr-1.5 custom-scrollbar">
                        {analysisData.risks?.map((risk, idx) => (
                          <li key={idx} className="flex items-start gap-2 text-xs text-content-muted">
                            <span className="mt-1.5 size-1.5 shrink-0 rounded-full bg-amber-500" />
                            <span className="leading-5">{risk}</span>
                          </li>
                        ))}
                      </ul>
                    )}
                  </div>
                </div>
              )}

              {analysisData.affected_files && analysisData.affected_files.length > 0 && (
                <div>
                  <h3 className="font-mono text-[10px] font-bold uppercase tracking-wider text-content-muted mb-2">
                    Estimated Affected Files
                  </h3>
                  <div className="flex flex-wrap gap-1.5 max-h-[110px] overflow-y-auto pr-1 custom-scrollbar">
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
                  <button
                    onClick={toggleBoundariesCollapse}
                    className="flex w-full items-center justify-between font-mono text-[10px] font-bold uppercase tracking-wider text-content-muted mb-2 cursor-pointer hover:text-foreground transition-colors"
                  >
                    <span>Execution Boundaries ({analysisData.execution_boundaries.length})</span>
                    {!isBoundariesCollapsed ? <ChevronUp size={14} /> : <ChevronDown size={14} />}
                  </button>
                  <div
                    className="overflow-hidden transition-all duration-300 ease-in-out"
                    style={{ maxHeight: !isBoundariesCollapsed ? "400px" : "0px" }}
                  >
                    <div className="space-y-2 max-h-[200px] overflow-y-auto pr-1.5 custom-scrollbar">
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
                </div>
              )}

              {analysisData.expanded_boundaries && analysisData.expanded_boundaries.length > 0 && (
                <div className="mt-4">
                  <button
                    onClick={() => setIsExpandedOpen(!isExpandedOpen)}
                    className="flex w-full items-center justify-between font-mono text-[10px] font-bold uppercase tracking-wider text-content-muted mb-2 cursor-pointer hover:text-foreground transition-colors"
                  >
                    <span>JIT Expanded Boundaries (Audit Trail) ({analysisData.expanded_boundaries.length})</span>
                    {isExpandedOpen ? <ChevronUp size={14} /> : <ChevronDown size={14} />}
                  </button>
                  {isExpandedOpen && (
                    <div className="space-y-2 max-h-[250px] overflow-y-auto pr-1.5 custom-scrollbar transition-all duration-300">
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
                  )}
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
      ))}
    </div>
  );
}
