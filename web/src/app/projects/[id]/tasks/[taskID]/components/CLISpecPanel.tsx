"use client";

import { useState, useEffect } from "react";
import { FileText, ChevronDown, ChevronUp, Maximize2, Minimize2 } from "lucide-react";
import { Markdown } from "@/components/ui/markdown";
import { tasks as tasksApi } from "@/lib/api/projects";
import { useAuthedSWR } from "@/lib/use-authed-swr";
import { useTaskDetail } from "./TaskDetailContext";

type SpecTab = "proposal" | "specs" | "design" | "tasks";

export function CLISpecPanel() {
  const { taskID } = useTaskDetail();
  const [tab, setTab] = useState<SpecTab>("proposal");
  const [isOpen, setIsOpen] = useState(true);
  const [isFullscreen, setIsFullscreen] = useState(false);

  // Prevent background scrolling when in fullscreen
  useEffect(() => {
    if (isFullscreen) {
      document.body.style.overflow = 'hidden';
    } else {
      document.body.style.overflow = 'unset';
    }
    return () => { document.body.style.overflow = 'unset'; };
  }, [isFullscreen]);

  const { data: spec, error } = useAuthedSWR(
    ["task-spec", taskID],
    (token) => tasksApi.getSpec(taskID, token),
  );

  if (error || !spec) {
    return null;
  }

  const isEmptySpec = !spec.proposal && !spec.specs && !spec.design && !spec.tasks;

  if (isEmptySpec) {
    return (
      <div className="relative overflow-hidden rounded-xl border border-stroke/50 bg-card/60 backdrop-blur-xl p-8 shadow-lg flex flex-col items-center justify-center text-center gap-3">
        <FileText size={32} className="text-content-muted/50" />
        <div>
          <h2 className="font-heading text-base font-bold text-foreground">No Specification Data</h2>
          <p className="text-sm text-content-muted mt-1">There is no specification data available for this task yet.</p>
        </div>
      </div>
    );
  }

  const progressPct = spec.progress.total > 0 ? Math.round((spec.progress.done / spec.progress.total) * 100) : 0;

  const tabs: { id: SpecTab; label: string; content: string }[] = [
    { id: "proposal", label: "Proposal", content: spec.proposal },
    { id: "specs", label: "Specs", content: spec.specs },
    { id: "design", label: "Design", content: spec.design },
    { id: "tasks", label: "Tasks", content: spec.tasks },
  ];

  const containerClasses = isFullscreen
    ? "fixed inset-0 z-50 bg-background/95 backdrop-blur-xl p-6 md:p-12 flex flex-col overflow-hidden"
    : "relative overflow-hidden rounded-xl border border-stroke/50 bg-card/60 backdrop-blur-xl p-5 shadow-lg";

  return (
    <div className={containerClasses}>
      <div className={`flex flex-wrap items-center justify-between gap-4 ${isOpen || isFullscreen ? "mb-4 border-b border-stroke/40 pb-3" : ""}`}>
        <div className="flex items-center gap-2">
          {!isFullscreen && (
            <button
              type="button"
              onClick={() => setIsOpen((v) => !v)}
              className="flex items-center cursor-pointer text-left mr-1"
              aria-expanded={isOpen}
            >
              {isOpen ? <ChevronUp size={18} className="text-content-muted hover:text-foreground" /> : <ChevronDown size={18} className="text-content-muted hover:text-foreground" />}
            </button>
          )}
          <FileText size={18} className="text-brand-primary" />
          <h2 className="font-heading text-base font-bold text-foreground">OpenSpec (CLI Flow)</h2>
        </div>

        <div className="flex items-center gap-4">
          <div className="flex items-center gap-2 bg-surface/50 px-3 py-1.5 rounded-full border border-stroke/50">
            <div className="h-1.5 w-24 rounded-full bg-surface overflow-hidden">
              <div className="h-full bg-brand-primary transition-all" style={{ width: `${progressPct}%` }} />
            </div>
            <span className="text-[10px] font-mono font-semibold text-content-muted">
              {spec.progress.done}/{spec.progress.total}
            </span>
          </div>
          
          <button
            type="button"
            onClick={() => {
              setIsFullscreen(!isFullscreen);
              if (!isFullscreen && !isOpen) setIsOpen(true);
            }}
            className="p-1.5 rounded-md text-content-muted hover:text-foreground hover:bg-surface border border-transparent hover:border-stroke transition-colors cursor-pointer"
            title={isFullscreen ? "Exit fullscreen" : "View fullscreen"}
          >
            {isFullscreen ? <Minimize2 size={16} /> : <Maximize2 size={16} />}
          </button>
        </div>
      </div>

      {(isOpen || isFullscreen) && (
        <div className={`flex flex-col ${isFullscreen ? "flex-1 min-h-0" : ""}`}>
          <div className="flex gap-1.5 bg-surface/60 p-1.5 rounded-lg border border-stroke shadow-inner overflow-x-auto hide-scrollbar mb-4 shrink-0">
            {tabs.map((t) => (
              <button
                key={t.id}
                onClick={() => setTab(t.id)}
                className={`px-4 py-2 rounded-md text-xs font-bold uppercase tracking-wider transition-all duration-200 cursor-pointer whitespace-nowrap ${
                  tab === t.id ? "bg-card text-brand-primary shadow-sm ring-1 ring-stroke" : "text-content-muted hover:text-foreground hover:bg-card/50"
                }`}
              >
                {t.label}
              </button>
            ))}
          </div>

          <div className={`rounded-lg border border-stroke bg-card p-6 md:p-8 overflow-auto leading-relaxed shadow-inner text-sm ${
            isFullscreen ? "flex-1 min-h-0" : "max-h-[500px]"
          }`}>
            <Markdown content={tabs.find((t) => t.id === tab)?.content || ""} />
          </div>
        </div>
      )}
    </div>
  );
}
