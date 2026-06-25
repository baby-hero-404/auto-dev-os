"use client";

import { useState } from "react";
import { AlertTriangle, CheckCircle2, Clock3, GitPullRequest, Search, SlidersHorizontal, Workflow } from "lucide-react";
import type { Task, Agent } from "@/lib/types";
import { Badge } from "@/components/ui/badge";
import { TaskAction } from "./task-action";
import { isActiveTask, isFailedTask, needsReview } from "@/lib/utils/task-utils";
import { formatRelativeTime } from "@/lib/utils/time";

interface TasksTabProps {
  tasks: Task[];
  projectAgents: Agent[];
  projectID: string;
  isTasksLoading: boolean;
  isLoadingTask: Record<string, boolean>;
  onTaskAction: (taskId: string, action: "analyze" | "execute" | "delete") => Promise<any>;
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
    if (filter === "review") return needsReview(task);
    if (filter === "failed") return isFailedTask(task);
    return true;
  });

  const filterChips: { id: FilterStatus; label: string; count: number }[] = [
    { id: "all", label: "All Tasks", count: tasks.length },
    { id: "active", label: "Active", count: tasks.filter(isActiveTask).length },
    {
      id: "review",
      label: "Needs Review",
      count: tasks.filter(needsReview).length,
    },
    { id: "failed", label: "Failed", count: tasks.filter(isFailedTask).length },
  ];

  return (
    <section className="rounded-lg border border-stroke bg-card">
      {/* Search and Filters Toolbar */}
      <div className="border-b border-stroke p-4">
        <div className="flex flex-col gap-3 xl:flex-row xl:items-center xl:justify-between">
        <div className="relative flex-1 xl:max-w-lg">
          <Search className="absolute left-3 top-2.5 h-4 w-4 text-content-muted" />
          <input
            type="text"
            placeholder="Search tasks by title or description..."
            value={searchQuery}
            onChange={(e) => setSearchQuery(e.target.value)}
            className="w-full rounded-md border border-stroke bg-background pl-9 pr-4 py-2 text-sm text-foreground placeholder-content-muted/50 transition-all focus:border-brand-primary focus:outline-none"
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
      </div>

      {/* Tasks Content */}
      {isTasksLoading ? (
        <div className="p-4">
          <TasksSkeleton />
        </div>
      ) : filteredTasks.length === 0 ? (
        <div className="m-4 flex flex-col items-center justify-center rounded-lg border border-dashed border-stroke bg-background py-12 text-center">
          <SlidersHorizontal className="h-8 w-8 text-content-muted/60" />
          <p className="mt-4 font-sans text-base font-semibold text-foreground">No tasks found.</p>
          <p className="mt-1 max-w-sm text-xs text-content-muted">
            {searchQuery || filter !== "all"
              ? "Try adjusting your search query or status filter."
              : "Create a task to get started."}
          </p>
        </div>
      ) : (
        <div className="divide-y divide-stroke">
          {filteredTasks.map((task) => {
            const avatar = getAgentAvatar(task.agent_id, projectAgents);
            const statusDot = getStatusDot(task);
            const stage = getTaskStage(task);
            const priority = getPriorityLabel(task.priority);

            return (
              <article
                key={task.id}
                className="group grid gap-4 p-4 transition hover:bg-surface/30 xl:grid-cols-[1fr_auto] xl:items-center"
              >
                <div className="flex min-w-0 items-start gap-3">
                  {/* Status dot indicator */}
                  <span className={`mt-1.5 h-2 w-2 shrink-0 rounded-full ${statusDot}`} />
                  
                  <div className="min-w-0 flex-1">
                    <div className="flex flex-wrap items-center gap-2">
                      <h3 className="min-w-0 truncate font-sans font-semibold text-foreground transition duration-150 group-hover:text-brand-primary">
                        {task.title}
                      </h3>
                      <Badge value={task.complexity} />
                      <Badge value={task.status} />
                    </div>
                    <div className="mt-2 grid gap-2 text-xs text-content-muted sm:grid-cols-2 lg:grid-cols-4">
                      <TaskMeta icon={stage.icon} label={stage.label} accent={stage.accent} />
                      <TaskMeta icon={Clock3} label={formatRelativeTime(task.updated_at)} />
                      <TaskMeta icon={AlertTriangle} label={priority} />
                      <TaskMeta icon={Workflow} label={avatar.name || "Auto-assign"} />
                    </div>
                    {task.description && (
                      <p className="mt-2 line-clamp-2 max-w-4xl text-sm leading-6 text-content-muted">
                        {task.description}
                      </p>
                    )}
                  </div>
                </div>

                <div className="flex items-center justify-between gap-4 xl:justify-end">
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
    </section>
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
  if (isFailedTask(task)) return "bg-rose-500";
  if (needsReview(task)) return "bg-amber-500 animate-pulse";
  if (isActiveTask(task)) return "bg-emerald-500 animate-pulse-dot";
  return "bg-slate-300 dark:bg-slate-700";
}

function getTaskStage(task: Task) {
  if (task.status === "merged") {
    return { label: "Merged", icon: CheckCircle2, accent: "text-emerald-500" };
  }
  if (needsReview(task)) {
    return { label: "Review gate", icon: GitPullRequest, accent: "text-amber-500" };
  }
  if (isFailedTask(task)) {
    return { label: "Needs inspection", icon: AlertTriangle, accent: "text-rose-500" };
  }
  if (isActiveTask(task)) {
    return { label: "In workflow", icon: Workflow, accent: "text-brand-primary" };
  }
  return { label: "Ready", icon: Clock3, accent: "text-content-muted" };
}

function getPriorityLabel(priority: number) {
  if (priority >= 4) return "P1 Urgent";
  if (priority === 3) return "P2 High";
  if (priority === 2) return "P3 Medium";
  return "P4 Low";
}

function TaskMeta({
  icon: Icon,
  label,
  accent = "text-content-muted",
}: {
  icon: typeof Workflow;
  label: string;
  accent?: string;
}) {
  return (
    <span className="inline-flex min-w-0 items-center gap-1.5">
      <Icon size={13} className={`shrink-0 ${accent}`} />
      <span className="truncate">{label}</span>
    </span>
  );
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
