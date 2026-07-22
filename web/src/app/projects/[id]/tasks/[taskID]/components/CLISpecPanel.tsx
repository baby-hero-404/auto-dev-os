"use client";

import { useState } from "react";
import { FileText, ChevronDown, ChevronUp } from "lucide-react";
import { Markdown } from "@/components/ui/markdown";
import { tasks as tasksApi } from "@/lib/api/projects";
import { useAuthedSWR } from "@/lib/use-authed-swr";
import { useTaskDetail } from "./TaskDetailContext";

type SpecTab = "proposal" | "specs" | "design" | "tasks";

/**
 * Live spec panel for the CLI spec-first flow: reads proposal/specs/design/
 * tasks.md straight off the task's worktree (as authored by cli_spec),
 * unlike SpecPanel which renders the API-native flow's task.analysis JSON.
 */
export function CLISpecPanel() {
  const { taskID, task } = useTaskDetail();
  const [tab, setTab] = useState<SpecTab>("proposal");
  const [isOpen, setIsOpen] = useState(true);

  const { data: spec, error } = useAuthedSWR(
    task?.execution_engine === "cli" ? ["task-spec", taskID] : null,
    (token) => tasksApi.getSpec(taskID, token),
  );

  if (task?.execution_engine !== "cli" || error || !spec) {
    return null;
  }

  const progressPct = spec.progress.total > 0 ? Math.round((spec.progress.done / spec.progress.total) * 100) : 0;

  const tabs: { id: SpecTab; label: string; content: string }[] = [
    { id: "proposal", label: "Proposal", content: spec.proposal },
    { id: "specs", label: "Specs", content: spec.specs },
    { id: "design", label: "Design", content: spec.design },
    { id: "tasks", label: "Tasks", content: spec.tasks },
  ];

  return (
    <div className="relative overflow-hidden rounded-xl border border-stroke/50 bg-card/60 backdrop-blur-xl p-5 shadow-lg">
      <div className={`flex flex-wrap items-center justify-between gap-4 ${isOpen ? "mb-4 border-b border-stroke/40 pb-3" : ""}`}>
        <button
          type="button"
          onClick={() => setIsOpen((v) => !v)}
          className="flex items-center gap-2 cursor-pointer text-left"
          aria-expanded={isOpen}
        >
          {isOpen ? <ChevronUp size={18} className="text-content-muted" /> : <ChevronDown size={18} className="text-content-muted" />}
          <FileText size={18} className="text-brand-primary" />
          <h2 className="font-heading text-base font-bold text-foreground">OpenSpec (CLI Flow)</h2>
        </button>

        <div className="flex items-center gap-3">
          <div className="flex items-center gap-2">
            <div className="h-1.5 w-24 rounded-full bg-surface overflow-hidden">
              <div className="h-full bg-brand-primary transition-all" style={{ width: `${progressPct}%` }} />
            </div>
            <span className="text-[10px] font-mono font-semibold text-content-muted">
              {spec.progress.done}/{spec.progress.total}
            </span>
          </div>
        </div>
      </div>

      {isOpen && (
        <>
          <div className="flex gap-1.5 bg-surface/60 p-1.5 rounded-lg border border-stroke shadow-inner overflow-x-auto hide-scrollbar mb-4">
            {tabs.map((t) => (
              <button
                key={t.id}
                onClick={() => setTab(t.id)}
                className={`px-3 py-1.5 rounded-md text-[11px] font-bold uppercase tracking-wider transition-all duration-200 cursor-pointer whitespace-nowrap ${
                  tab === t.id ? "bg-card text-brand-primary shadow-sm ring-1 ring-stroke" : "text-content-muted hover:text-foreground hover:bg-card/50"
                }`}
              >
                {t.label}
              </button>
            ))}
          </div>

          <div className="rounded-lg border border-stroke bg-card p-5 overflow-auto max-h-[500px] leading-relaxed shadow-inner text-sm">
            <Markdown content={tabs.find((t) => t.id === tab)?.content || ""} />
          </div>
        </>
      )}
    </div>
  );
}
