"use client";

import { useMemo } from "react";
import { ChevronDown, ChevronRight, Check, AlertCircle, AlertTriangle } from "lucide-react";
import { useTaskDetail } from "./TaskDetailContext";
import { SpecPanel } from "./SpecPanel";
import { LogConsole, parseMilestones } from "@/components/dashboard/log-console";
import { DescriptionBody } from "./DescriptionBody";

interface AccordionItemProps {
  title: string;
  summary: React.ReactNode;
  isOpen: boolean;
  onToggle: () => void;
  children: React.ReactNode;
  keepMounted?: boolean;
}

function AccordionItem({ title, summary, isOpen, onToggle, children, keepMounted = false }: AccordionItemProps) {
  const Icon = isOpen ? ChevronDown : ChevronRight;

  return (
    <div className="border border-stroke/40 rounded-xl bg-card shadow-sm overflow-hidden text-foreground">
      {/* Accordion Header */}
      <button
        type="button"
        onClick={onToggle}
        className="w-full flex items-center justify-between gap-4 p-4 text-left hover:bg-slate-50 dark:hover:bg-slate-900/40 transition-colors cursor-pointer select-none"
        aria-expanded={isOpen}
      >
        <div className="flex items-center gap-2 min-w-0">
          <Icon size={18} className="text-content-muted shrink-0" />
          <h3 className="font-heading text-sm font-bold shrink-0">{title}</h3>
          {!isOpen && (
            <div className="flex-1 min-w-0 truncate text-xs text-content-muted flex items-center gap-1.5 ml-2 border-l border-stroke/40 pl-2.5">
              {summary}
            </div>
          )}
        </div>
        {!isOpen && (
          <span className="text-[10px] font-bold uppercase tracking-wider text-brand-primary shrink-0 hover:underline">
            Expand
          </span>
        )}
      </button>

      {/* Accordion Body */}
      {keepMounted ? (
        <div className={isOpen ? "border-t border-stroke/30 p-5 bg-card/40" : "hidden"}>
          {children}
        </div>
      ) : (
        isOpen && (
          <div className="border-t border-stroke/30 p-5 bg-card/40">
            {children}
          </div>
        )
      )}
    </div>
  );
}

interface SupportingAccordionProps {
  openSections: Record<string, boolean>;
  onToggleSection: (key: string) => void;
}

export function SupportingAccordion({ openSections, onToggleSection }: SupportingAccordionProps) {
  const {
    task,
    workflow,
    logs,
    analysisData,
    descriptionParts,
  } = useTaskDetail();

  // Specification presence chips
  const presenceChips = useMemo(() => {
    return [
      { label: "Scope", present: !!analysisData.scope },
      { label: "Recommendation", present: !!analysisData.proposal_md },
      { label: "Architecture", present: !!analysisData.design_md },
      { label: "Risks", present: !!(analysisData.risk_domains && analysisData.risk_domains.length > 0) },
    ].filter((c) => c.present);
  }, [analysisData]);

  // Log latest event summary
  const logSummary = useMemo(() => {
    if (!logs || logs.length === 0) return <span className="font-mono text-[11px] text-content-muted">No logs yet</span>;
    
    const milestones = parseMilestones(logs);
    const latestMilestone = milestones.length > 0 ? milestones[milestones.length - 1] : null;
    const latestEventText = latestMilestone
      ? latestMilestone.message
      : logs[logs.length - 1].message;
      
    const latestEventType = latestMilestone?.type ?? "info";
    const isError = latestEventType === "failed" || logs.some(l => l.level === "error");

    let textStyle = "text-content-muted";
    if (isError) textStyle = "text-rose-500 font-semibold";
    else if (latestEventType === "success") textStyle = "text-emerald-500";
    else if (latestEventType === "running") textStyle = "text-sky-500";
    else if (latestEventType === "paused") textStyle = "text-amber-500";

    return (
      <div className="flex items-center gap-1.5 min-w-0 max-w-full font-mono text-[11px]">
        {isError && <AlertTriangle size={13} className="text-rose-500 shrink-0" />}
        <span className={`truncate ${textStyle}`}>{latestEventText}</span>
      </div>
    );
  }, [logs]);

  // Description summary
  const descriptionSummary = useMemo(() => {
    if (!descriptionParts.body || !descriptionParts.body.trim()) {
      return <span className="italic text-content-muted/80">No description provided</span>;
    }
    const cleanDesc = descriptionParts.body.replace(/[#*`_-]/g, "").trim();
    if (cleanDesc.length <= 80) return cleanDesc;
    return `${cleanDesc.substring(0, 80)}...`;
  }, [descriptionParts.body]);

  // Checkpoints summary
  const checkpointCount = workflow?.checkpoints?.length || 0;

  return (
    <section className="flex flex-col gap-2 mt-8">
      <h2 className="text-xs font-semibold tracking-wider uppercase text-content-muted mb-2 px-1">
        Supporting Information
      </h2>

      {/* Accordion 1: Specification */}
      {task?.status !== 'spec_review' && (
        <AccordionItem
          title="Specification"
          isOpen={!!openSections.specification}
          onToggle={() => onToggleSection("specification")}
          summary={
            <div className="flex flex-wrap gap-1">
              {presenceChips.map((c) => (
                <span
                  key={c.label}
                  className="inline-flex items-center gap-0.5 rounded-full border border-emerald-500/20 bg-emerald-500/10 px-1.5 py-0.5 text-[9px] font-semibold text-emerald-600 dark:text-emerald-400"
                >
                  <Check size={8} />
                  {c.label}
                </span>
              ))}
            </div>
          }
        >
          <SpecPanel isExpanded={true} onToggle={() => {}} />
        </AccordionItem>
      )}

      {/* Accordion 2: Execution Logs */}
      {!['coding', 'testing', 'fixing', 'failed', 'merged', 'pr_ready', 'human_review'].includes(task?.status || '') && (
        <AccordionItem
          title="Execution Logs"
          isOpen={!!openSections.logs}
          onToggle={() => onToggleSection("logs")}
          summary={logSummary}
          keepMounted={true}
        >
          <LogConsole
            logs={logs}
            isWorkflowRunning={workflow?.job?.status === "running"}
            isExpanded={true}
            onToggle={() => {}}
          />
        </AccordionItem>
      )}

    </section>
  );
}
