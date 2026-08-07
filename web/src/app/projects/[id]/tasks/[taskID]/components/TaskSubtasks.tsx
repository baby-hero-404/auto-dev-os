"use client";

import { useEffect, useState } from "react";
import { useTaskDetail } from "./TaskDetailContext";
import { ListTodo, AlertTriangle, Check, GitFork } from "lucide-react";
import { EmptyState } from "@/components/ui/empty-state";
import { tasks as tasksApi } from "@/lib/api/projects";
import type { Task } from "@/lib/types";

export function TaskSubtasks() {
  const { task, taskID, token, implementationItems } = useTaskDetail();
  const [dbSubtasks, setDbSubtasks] = useState<Task[]>(task?.subtasks || []);


  useEffect(() => {
    if (task?.subtasks && task.subtasks.length > 0) {
      // eslint-disable-next-line react-hooks/set-state-in-effect
      setDbSubtasks(task.subtasks);
      return;
    }

    if (!token || !taskID) return;

    let isMounted = true;
    tasksApi
      .getSubtasks(taskID, token)
      .then((res) => {
        if (isMounted && Array.isArray(res)) {
          setDbSubtasks(res);
        }
      })
      .catch(() => {});

    return () => {
      isMounted = false;
    };
  }, [taskID, token, task?.subtasks]);

  const hasDbSubtasks = dbSubtasks.length > 0;
  const hasImplItems = implementationItems && implementationItems.length > 0;

  if (!hasDbSubtasks && !hasImplItems) {
    const st = task?.status || "todo";
    const hasSplitProposal = !!task?.analysis?.child_specs && task.analysis.child_specs.length > 0;
    const isCliTask = !!task?.analysis?.cli_spec_info;
    if (!isCliTask && (st === "coding" || st === "testing" || st === "reviewing" || st === "fixing" || hasSplitProposal)) {
      return (
        <EmptyState
          icon={ListTodo}
          title="No subtasks yet"
          description={hasSplitProposal ? "Review split proposal above..." : "Waiting for sub-tasks..."}
        />
      );
    }
    return null;
  }

  // Render DB Subtasks if present
  if (hasDbSubtasks) {
    const sorted = [...dbSubtasks].sort((a, b) => (a.sequence_index ?? 0) - (b.sequence_index ?? 0));
    const completedTasks = sorted.filter((s) => s.status === "merged").length;
    const totalTasks = sorted.length;
    const progressPct = totalTasks > 0 ? (completedTasks / totalTasks) * 100 : 0;

    return (
      <div className="bg-card border border-stroke/10 rounded-2xl p-5.5 shadow-sm hover:shadow-md transition-all duration-200">
        <div className="flex items-center justify-between mb-3">
          <div className="flex items-center gap-2">
            <GitFork className="w-4 h-4 text-amber-400" />
            <span className="text-sm font-bold text-foreground tracking-wide">Decomposed Sub-tasks</span>
          </div>
          <span className="text-xs font-semibold text-content-muted">
            {completedTasks} of {totalTasks} merged
          </span>
        </div>

        <div className="h-2 rounded-full bg-surface overflow-hidden mb-4 shadow-inner">
          <div
            className="h-full rounded-full bg-gradient-to-r from-amber-500 via-indigo-500 to-emerald-500 transition-all duration-500"
            style={{ width: `${progressPct}%` }}
          ></div>
        </div>

        <div className="flex flex-col gap-2.5">
          {sorted.map((item, idx) => {
            const isMerged = item.status === "merged";
            const isFailed = item.status === "failed";
            const isBlocked = item.status === "blocked";
            const isRunning = item.status === "coding" || item.status === "testing" || item.status === "reviewing";

            let itemClass =
              "flex items-center gap-3 px-3.5 py-3 rounded-xl border border-stroke/10 bg-surface/20 text-foreground transition-all duration-150";
            let indicator = (
              <span className="w-5 h-5 rounded-full border border-stroke/30 bg-background flex items-center justify-center text-[10px] font-mono text-content-muted shrink-0">
                {idx + 1}
              </span>
            );

            if (isMerged) {
              itemClass =
                "flex items-center gap-3 px-3.5 py-3 rounded-xl border border-emerald-500/20 bg-emerald-500/[0.04] text-emerald-800 dark:text-emerald-300 transition-all duration-150";
              indicator = (
                <span className="inline-flex items-center justify-center w-5 h-5 rounded-full text-xs font-bold bg-emerald-500 text-white shadow-sm shrink-0">
                  <Check className="w-3 h-3" />
                </span>
              );
            } else if (isFailed || isBlocked) {
              itemClass =
                "flex items-center gap-3 px-3.5 py-3 rounded-xl border border-red-500/20 bg-red-500/[0.04] text-red-800 dark:text-red-300 transition-all duration-150";
              indicator = (
                <span className="inline-flex items-center justify-center w-5 h-5 rounded-full text-xs font-bold bg-red-500 text-white shrink-0">
                  <AlertTriangle className="w-3 h-3" />
                </span>
              );
            } else if (isRunning) {
              itemClass =
                "flex items-center gap-3 px-3.5 py-3 rounded-xl border border-amber-500/30 bg-amber-500/5 text-amber-300 transition-all duration-150 shadow-sm";
              indicator = (
                <span className="w-5 h-5 rounded-full border-2 border-amber-500/20 border-t-amber-500 animate-spin shrink-0"></span>
              );
            }

            return (
              <div key={item.id} className={itemClass}>
                {indicator}
                <div className="flex-1 min-w-0">
                  <div className={`text-xs md:text-sm font-semibold ${isMerged ? "line-through opacity-60" : ""}`}>
                    {item.title}
                  </div>
                  {item.description && (
                    <div className="text-[11px] text-content-muted truncate font-mono mt-0.5">
                      {item.description}
                    </div>
                  )}
                </div>
                <span className="px-2 py-0.5 rounded text-[10px] font-mono font-bold bg-surface border border-stroke/20 text-content-muted uppercase">
                  {item.status}
                </span>
              </div>
            );
          })}
        </div>
      </div>
    );
  }

  // Classic fallback implementation items
  const completedTasks = implementationItems.filter((i) => i.status === "done").length;
  const totalTasks = implementationItems.length;
  const progressPct = totalTasks > 0 ? (completedTasks / totalTasks) * 100 : 0;

  return (
    <div className="bg-card border border-stroke/10 rounded-2xl p-5.5 shadow-sm hover:shadow-md transition-all duration-200">
      <div className="flex items-center justify-between mb-3">
        <span className="text-sm font-bold text-foreground tracking-wide">Subtasks</span>
        <span className="text-xs font-semibold text-content-muted">
          {completedTasks} of {totalTasks} completed
        </span>
      </div>
      <div className="h-2 rounded-full bg-surface overflow-hidden mb-4 shadow-inner">
        <div
          className="h-full rounded-full bg-gradient-to-r from-blue-500 via-indigo-500 to-emerald-500 transition-all duration-500"
          style={{ width: `${progressPct}%` }}
        ></div>
      </div>
      <div className="flex flex-col gap-2">
        {implementationItems.map((item, idx) => {
          const isDone = item.status === "done";
          const isRunning = item.status === "running";

          let itemClass =
            "flex items-center gap-3 px-3.5 py-3 rounded-xl border border-stroke/10 bg-surface/20 hover:bg-surface/50 text-foreground transition-all duration-150";
          let indicator = (
            <span className="w-5 h-5 rounded-full border border-stroke/30 bg-background shrink-0 transition-colors"></span>
          );

          if (isDone) {
            itemClass =
              "flex items-center gap-3 px-3.5 py-3 rounded-xl border border-emerald-500/20 bg-emerald-500/[0.04] text-emerald-800 dark:text-emerald-300 transition-all duration-150";
            indicator = (
              <span className="inline-flex items-center justify-center w-5 h-5 rounded-full text-xs font-bold bg-emerald-500 text-white shadow-sm shrink-0 animate-scale-in">
                ✓
              </span>
            );
          } else if (isRunning) {
            itemClass =
              "flex items-center gap-3 px-3.5 py-3 rounded-xl border border-blue-500/25 bg-blue-500/5 text-blue-800 dark:text-blue-300 transition-all duration-150 shadow-sm shadow-blue-500/5";
            indicator = (
              <span className="w-5 h-5 rounded-full border-2 border-blue-500/20 border-t-blue-500 animate-spin shrink-0"></span>
            );
          }

          return (
            <div key={item.id || idx} className={itemClass}>
              {indicator}
              <span
                className={`text-xs md:text-sm flex-1 leading-normal font-medium ${isDone ? "line-through opacity-60" : ""}`}
              >
                {item.name}
              </span>
            </div>
          );
        })}
      </div>
    </div>
  );
}
