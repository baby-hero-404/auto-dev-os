"use client";

import { GitBranch, Plus, ShieldCheck, Workflow, Bot, CheckCircle2, type LucideIcon } from "lucide-react";
import type { Task } from "@/lib/types";
import { workflowStageCounts } from "@/lib/utils/task-utils";

type ProjectView = "tasks" | "agents" | "repositories" | "rules" | "settings";

const projectViews: { id: ProjectView; label: string }[] = [
  { id: "tasks", label: "Tasks" },
  { id: "agents", label: "Agents" },
  { id: "repositories", label: "Repositories" },
  { id: "rules", label: "Rules" },
  { id: "settings", label: "Settings" },
];

interface ProjectHeaderProps {
  projectName: string;
  projectID: string;
  repositoriesCount: number;
  tasksCount: number;
  agentsCount: number;
  rulesCount: number;
  completedTasksCount: number;
  activeTasksCount: number;
  activeView: ProjectView;
  onViewChange: (v: ProjectView) => void;
  onCreateTaskClick: () => void;
  tasks: Task[];
}

export function ProjectHeader({
  projectName,
  projectID,
  repositoriesCount,
  tasksCount,
  agentsCount,
  rulesCount,
  completedTasksCount,
  activeTasksCount,
  activeView,
  onViewChange,
  onCreateTaskClick,
  tasks,
}: ProjectHeaderProps) {
  return (
    <header className="shrink-0 border-b border-stroke bg-card/95 px-5 py-4 shadow-sm md:px-6">
      <div className="flex flex-col gap-4 xl:flex-row xl:items-start xl:justify-between">
        <div className="min-w-0">
          <div className="flex items-center gap-1.5 text-xs text-content-muted">
            <span>Projects</span>
            <span>/</span>
            <span className="truncate font-semibold text-foreground">{projectName || "Loading..."}</span>
          </div>
          <div className="mt-2 flex flex-wrap items-center gap-2">
            <h1 className="truncate text-2xl font-semibold tracking-tight text-foreground">
              {projectName || "Project workspace"}
            </h1>
            <span className="rounded-md border border-stroke bg-surface px-2 py-0.5 font-mono text-[11px] text-content-muted">
              {projectID.slice(0, 8)}
            </span>
          </div>
          <div className="mt-3 flex flex-wrap items-center gap-2 text-xs text-content-muted">
            <WorkspaceSignal icon={GitBranch} label={`${repositoriesCount} repos`} />
            <WorkspaceSignal icon={Workflow} label={`${tasksCount} tasks`} />
            <WorkspaceSignal icon={Bot} label={`${agentsCount} agents`} />
            <WorkspaceSignal icon={ShieldCheck} label={`${rulesCount} rules`} />
            <WorkspaceSignal icon={CheckCircle2} label={`${completedTasksCount}/${tasksCount} done`} />
          </div>
        </div>

        <div className="flex shrink-0 flex-wrap items-center gap-3">
          <div className="hidden rounded-lg border border-stroke bg-background px-3 py-2 text-xs text-content-muted lg:block">
            <span className="font-mono text-foreground">{activeTasksCount}</span> active now
          </div>
          <button
            onClick={onCreateTaskClick}
            className="flex items-center gap-1.5 rounded-md bg-brand-primary px-3.5 py-2 text-sm font-semibold text-white transition hover:opacity-90 cursor-pointer"
            type="button"
          >
            <Plus size={15} /> Create Task
          </button>
        </div>
      </div>

      <div className="mt-4 md:hidden">
        <label htmlFor="project-view" className="sr-only">
          Project view
        </label>
        <select
          id="project-view"
          value={activeView}
          onChange={(event) => onViewChange(event.target.value as ProjectView)}
          className="w-full rounded-md border border-stroke bg-background px-3 py-2 text-sm text-foreground"
        >
          {projectViews.map((view) => (
            <option key={view.id} value={view.id}>
              {view.label}
            </option>
          ))}
        </select>
      </div>

      <WorkflowStageStrip tasks={tasks} />
    </header>
  );
}

function WorkspaceSignal({ icon: Icon, label }: { icon: LucideIcon; label: string }) {
  return (
    <span className="inline-flex items-center gap-1.5 rounded-md border border-stroke bg-surface px-2 py-1">
      <Icon size={13} className="text-brand-primary" />
      {label}
    </span>
  );
}

function WorkflowStageStrip({ tasks }: { tasks: Task[] }) {
  const stages = workflowStageCounts(tasks);

  return (
    <div className="mt-4 overflow-x-auto pb-1">
      <div className="flex min-w-max gap-2">
        {stages.map((stage) => {
          const hasTasks = stage.count > 0;
          return (
            <div
              key={stage.label}
              className={`flex min-w-28 items-center justify-between gap-3 rounded-md border px-3 py-2 text-xs transition ${
                hasTasks
                  ? "border-brand-primary/30 bg-brand-primary-muted text-foreground"
                  : "border-stroke bg-surface text-content-muted"
              }`}
            >
              <span className="font-medium">{stage.label}</span>
              <span className="font-mono font-semibold">{stage.count}</span>
            </div>
          );
        })}
      </div>
    </div>
  );
}
