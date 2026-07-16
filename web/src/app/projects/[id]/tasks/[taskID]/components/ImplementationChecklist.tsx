"use client";

import { useTaskDetail } from "./TaskDetailContext";
import { CheckCircle2, Circle, FileCode, ListTodo } from "lucide-react";

interface ImplementationChecklistProps {
  /**
   * Expands the (default-collapsed) LogConsole and scrolls to the matching
   * log group (REQ-006). When omitted, falls back to a direct scroll — which
   * only resolves if the log is already open.
   */
  expandAndScrollToLog?: (stepId: string) => void;
}

export function ImplementationChecklist({ expandAndScrollToLog }: ImplementationChecklistProps = {}) {
  const { implementationItems } = useTaskDetail();

  if (!implementationItems || implementationItems.length === 0) {
    return null;
  }

  const runningItems = implementationItems.filter((item) => item.status === "running");
  const pendingItems = implementationItems.filter((item) => item.status === "pending");
  const completedItems = implementationItems.filter((item) => item.status === "done");

  const scrollToLog = (stepId: string) => {
    if (expandAndScrollToLog) {
      expandAndScrollToLog(stepId);
      return;
    }
    const el = document.getElementById(`log-group-${stepId}`);
    if (el) {
      el.scrollIntoView({ behavior: "smooth", block: "center" });
    }
  };

  const renderItem = (item: typeof implementationItems[0]) => {
    let icon = <Circle className="text-content-muted shrink-0" size={18} />;
    let statusClass = "text-content-muted";
    let bgClass = "bg-transparent";
    let borderClass = "border-stroke/40";

    if (item.status === "done") {
      icon = <CheckCircle2 className="text-emerald-500 shrink-0" size={18} />;
      statusClass = "text-content-muted line-through opacity-80";
      bgClass = "bg-emerald-500/5 dark:bg-emerald-500/5";
      borderClass = "border-emerald-500/20";
    } else if (item.status === "running") {
      icon = (
        <span className="relative flex h-4 w-4 shrink-0 mt-0.5">
          <span className="animate-ping absolute inline-flex h-full w-full rounded-full bg-sky-400 opacity-75"></span>
          <span className="relative inline-flex rounded-full h-4 w-4 bg-sky-500"></span>
        </span>
      );
      statusClass = "text-foreground font-semibold";
      bgClass = "bg-sky-500/10 dark:bg-sky-500/10 border border-sky-500/30";
      borderClass = "border-sky-500/30";
    }

    const filesCount = item.affectedFiles?.length || 0;

    return (
      <div
        key={item.id}
        onClick={() => scrollToLog(item.stepId)}
        className={`relative flex items-start gap-4 p-4 rounded-xl border ${borderClass} ${bgClass} hover:bg-slate-50 dark:hover:bg-slate-900/60 hover:scale-[1.01] hover:shadow-lg transition-all duration-300 cursor-pointer group overflow-hidden`}
      >
        <div className="mt-0.5">{icon}</div>
        <div className="flex-1 min-w-0">
          <div className="flex items-center gap-2 justify-between">
            <span className={`text-sm ${statusClass} group-hover:text-brand-primary transition-colors truncate`}>
              {item.name}
            </span>
            {filesCount > 0 && (
              <span className="flex items-center gap-1 text-[10px] font-mono px-2 py-0.5 rounded-full bg-slate-100 dark:bg-slate-800 text-content-muted border border-stroke/20">
                <FileCode size={10} />
                <span>{filesCount} {filesCount === 1 ? "file" : "files"}</span>
              </span>
            )}
          </div>
          {item.description && (
            <p className="text-xs text-content-muted mt-1 leading-relaxed line-clamp-2">
              {item.description}
            </p>
          )}
        </div>
      </div>
    );
  };

  return (
    <section className="relative overflow-hidden rounded-xl border border-stroke/50 bg-card/60 backdrop-blur-xl shadow-lg hover:shadow-xl transition-all group p-6">
      <div className="absolute inset-0 bg-gradient-to-br from-brand-primary/5 via-transparent to-transparent opacity-0 group-hover:opacity-100 transition-opacity duration-500 pointer-events-none" />
      <div className="relative flex items-center gap-2 mb-6 border-b border-stroke/40 pb-4 z-10">
        <ListTodo size={20} className="text-brand-primary" />
        <h2 className="font-sans font-bold text-lg text-foreground">Implementation Checklist</h2>
      </div>

      <div className="space-y-6">
        {runningItems.length > 0 && (
          <div>
            <h3 className="text-xs font-bold uppercase tracking-wider text-sky-500 mb-3 flex items-center gap-1.5 font-mono">
              <span className="relative flex h-2 w-2">
                <span className="animate-ping absolute inline-flex h-full w-full rounded-full bg-sky-400 opacity-75"></span>
                <span className="relative inline-flex rounded-full h-2 w-2 bg-sky-500"></span>
              </span>
              In Progress ({runningItems.length})
            </h3>
            <div className="grid gap-3 md:grid-cols-2">
              {runningItems.map(renderItem)}
            </div>
          </div>
        )}

        {pendingItems.length > 0 && (
          <div>
            <h3 className="text-xs font-bold uppercase tracking-wider text-content-muted mb-3 font-mono">
              Pending ({pendingItems.length})
            </h3>
            <div className="grid gap-3 md:grid-cols-2">
              {pendingItems.map(renderItem)}
            </div>
          </div>
        )}

        {completedItems.length > 0 && (
          <div>
            <h3 className="text-xs font-bold uppercase tracking-wider text-emerald-500 mb-3 font-mono">
              Completed ({completedItems.length})
            </h3>
            <div className="grid gap-3 md:grid-cols-2">
              {completedItems.map(renderItem)}
            </div>
          </div>
        )}
      </div>
    </section>
  );
}
