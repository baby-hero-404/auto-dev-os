"use client";

import { useState } from "react";
import { Search, SlidersHorizontal } from "lucide-react";
import type { Task, Agent } from "@/lib/types";
import { Badge } from "@/components/ui/badge";
import { TaskAction } from "./task-action";
import { isActiveTask, timeAgo } from "@/lib/utils/task-utils";

interface TasksTabProps {
  tasks: Task[];
  projectAgents: Agent[];
  projectID: string;
  isTasksLoading: boolean;
  isLoadingTask: Record<string, boolean>;
  onTaskAction: (taskId: string, action: "analyze" | "execute") => Promise<void>;
}

type FilterStatus = "all" | "active" | "review" | "failed";

export function TasksTab({
  tasks,
  projectAgents,
  projectID,
  isTasksLoading,
  isLoadingTask,
  onTaskAction,
}: TasksTabProps) {
  const [searchQuery, setSearchQuery] = useState("");
  const [filter, setFilter] = useState<FilterStatus>("all");

  // Filter & Search Logic
  const filteredTasks = tasks.filter((task) => {
    const matchesSearch =
      task.title.toLowerCase().includes(searchQuery.toLowerCase()) ||
      (task.description && task.description.toLowerCase().includes(searchQuery.toLowerCase()));

    if (!matchesSearch) return false;

    if (filter === "all") return true;
    if (filter === "active") return isActiveTask(task);
    if (filter === "review") {
      return (
        task.status === "spec_review" ||
        task.status === "human_review" ||
        task.spec_status === "pending_review"
      );
    }
    if (filter === "failed") return task.status === "failed";
    return true;
  });

  const filterChips: { id: FilterStatus; label: string; count: number }[] = [
    { id: "all", label: "All Tasks", count: tasks.length },
    { id: "active", label: "Active", count: tasks.filter(isActiveTask).length },
    {
      id: "review",
      label: "Needs Review",
      count: tasks.filter(
        (t) => t.status === "spec_review" || t.status === "human_review" || t.spec_status === "pending_review"
      ).length,
    },
    { id: "failed", label: "Failed", count: tasks.filter((t) => t.status === "failed").length },
  ];

  return (
    <div className="space-y-4">
      {/* Search and Filters Toolbar */}
      <div className="flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between">
        <div className="relative flex-1 max-w-md">
          <Search className="absolute left-3 top-2.5 h-4 w-4 text-content-muted" />
          <input
            type="text"
            placeholder="Search and Filter..."
            value={searchQuery}
            onChange={(e) => setSearchQuery(e.target.value)}
            className="w-full rounded-md border border-stroke bg-card pl-9 pr-4 py-2 text-sm text-foreground placeholder-content-muted/50 focus:border-brand-primary focus:outline-none transition-all"
          />
        </div>

        {/* Filter chips */}
        <div className="flex flex-wrap items-center gap-1.5">
          {filterChips.map((chip) => (
            <button
              key={chip.id}
              onClick={() => setFilter(chip.id)}
              className={`inline-flex items-center gap-1.5 rounded-full px-3 py-1 text-xs font-medium border transition cursor-pointer ${
                filter === chip.id
                  ? "bg-brand-primary/10 border-brand-primary text-brand-primary font-semibold"
                  : "bg-card border-stroke text-content-muted hover:text-foreground hover:border-stroke-focus"
              }`}
            >
              <span>{chip.label}</span>
              <span className={`rounded-full px-1.5 py-0.2 text-[10px] ${
                filter === chip.id
                  ? "bg-brand-primary/20 text-brand-primary"
                  : "bg-surface text-content-muted"
              }`}>
                {chip.count}
              </span>
            </button>
          ))}
        </div>
      </div>

      {/* Tasks Content */}
      {isTasksLoading ? (
        <TasksSkeleton />
      ) : filteredTasks.length === 0 ? (
        <div className="flex flex-col items-center justify-center rounded-lg border border-dashed border-stroke bg-card py-12 text-center">
          <SlidersHorizontal className="h-8 w-8 text-content-muted/60" />
          <p className="mt-4 font-sans text-base font-semibold text-foreground">No tasks found.</p>
          <p className="mt-1 max-w-sm text-xs text-content-muted">
            {searchQuery || filter !== "all"
              ? "Try adjusting your search query or status filter."
              : "Create a task to get started."}
          </p>
        </div>
      ) : (
        <div className="divide-y divide-stroke border border-stroke bg-card rounded-lg overflow-hidden">
          {filteredTasks.map((task) => {
            const avatar = getAgentAvatar(task.agent_id, projectAgents);
            const statusDot = getStatusDot(task);

            return (
              <article
                key={task.id}
                className="group flex flex-col justify-between gap-3 p-4 md:flex-row md:items-center transition hover:bg-surface/30"
              >
                <div className="flex items-start gap-3 min-w-0 flex-1">
                  {/* Status dot indicator */}
                  <span className={`mt-1.5 h-2 w-2 shrink-0 rounded-full ${statusDot}`} />
                  
                  <div className="min-w-0 flex-1">
                    <div className="flex flex-wrap items-center gap-2">
                      <h3 className="font-sans font-semibold text-foreground truncate group-hover:text-brand-primary transition duration-150">
                        {task.title}
                      </h3>
                      <Badge value={task.complexity} />
                      <Badge value={task.status} />
                    </div>
                    <div className="mt-1.5 flex flex-wrap items-center gap-x-2 gap-y-1 text-xs text-content-muted">
                      <span>{timeAgo(task.updated_at)}</span>
                      <span>•</span>
                      <span>Agent: {avatar.name || "Auto-assign"}</span>
                    </div>
                    {task.description && (
                      <p className="mt-2 text-sm text-content-muted line-clamp-1 max-w-3xl">
                        {task.description}
                      </p>
                    )}
                  </div>
                </div>

                <div className="flex items-center gap-4 shrink-0 justify-between md:justify-end">
                  {/* Agent avatar badge */}
                  <div
                    className={`flex h-7 w-7 items-center justify-center rounded-full border text-[11px] font-bold ${avatar.color}`}
                    title={avatar.name || "Auto-assign"}
                  >
                    {avatar.initials}
                  </div>

                  <TaskAction
                    task={task}
                    projectID={projectID}
                    isLoading={isLoadingTask[task.id]}
                    onAction={(action) => onTaskAction(task.id, action)}
                  />
                </div>
              </article>
            );
          })}
        </div>
      )}
    </div>
  );
}

// ─── Deterministic Avatar & Status helpers ───────────────────

function getAgentAvatar(agentId?: string, agents: Agent[] = []) {
  if (!agentId) return { initials: "AA", color: "bg-slate-100 dark:bg-slate-900 border-stroke text-content-muted", name: "Auto-assign" };
  const agent = agents.find((a) => a.id === agentId);
  if (!agent) return { initials: "AG", color: "bg-slate-100 dark:bg-slate-900 border-stroke text-content-muted", name: "Agent" };
  const initials = agent.name
    .split(/\s+/)
    .map((n) => n[0])
    .join("")
    .slice(0, 2)
    .toUpperCase();

  const colors = [
    "bg-blue-500/10 text-blue-500 border-blue-500/20 dark:bg-blue-400/10 dark:text-blue-400 dark:border-blue-400/25",
    "bg-purple-500/10 text-purple-500 border-purple-500/20 dark:bg-purple-400/10 dark:text-purple-400 dark:border-purple-400/25",
    "bg-pink-500/10 text-pink-500 border-pink-500/20 dark:bg-pink-400/10 dark:text-pink-400 dark:border-pink-400/25",
    "bg-indigo-500/10 text-indigo-500 border-indigo-500/20 dark:bg-indigo-400/10 dark:text-indigo-400 dark:border-indigo-400/25",
    "bg-cyan-500/10 text-cyan-500 border-cyan-500/20 dark:bg-cyan-400/10 dark:text-cyan-400 dark:border-cyan-400/25",
    "bg-teal-500/10 text-teal-500 border-teal-500/20 dark:bg-teal-400/10 dark:text-teal-400 dark:border-teal-400/25",
  ];
  const index = agent.id.split("").reduce((acc, char) => acc + char.charCodeAt(0), 0) % colors.length;
  return { initials, color: colors[index], name: agent.name };
}

function getStatusDot(task: Task) {
  if (task.status === "failed") return "bg-rose-500";
  if (
    task.status === "spec_review" ||
    task.status === "human_review" ||
    task.spec_status === "pending_review"
  ) {
    return "bg-amber-500 animate-pulse";
  }
  if (isActiveTask(task)) return "bg-emerald-500 animate-pulse-dot";
  return "bg-slate-300 dark:bg-slate-700";
}

// ─── Helpers ──────────────────────────────────────────────────

function TasksSkeleton() {
  return (
    <div className="space-y-3">
      {[0, 1, 2].map((i) => (
        <div key={i} className="rounded-lg border border-stroke bg-card p-4">
          <div className="flex justify-between gap-3">
            <div className="flex-1 space-y-3">
              <div className="skeleton-shimmer h-5 w-2/3 rounded" />
              <div className="skeleton-shimmer h-3 w-1/2 rounded" />
            </div>
            <div className="hidden gap-2 sm:flex">
              <div className="skeleton-shimmer h-9 w-20 rounded" />
              <div className="skeleton-shimmer h-9 w-24 rounded" />
            </div>
          </div>
        </div>
      ))}
    </div>
  );
}
