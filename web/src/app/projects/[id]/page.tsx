"use client";

import Link from "next/link";
import { use, useEffect, useState } from "react";
import { GitBranch, Plus, ShieldCheck, Workflow, Bot, CheckCircle2, AlertCircle, X, type LucideIcon } from "lucide-react";
import { useSession } from "@/lib/session";
import { api } from "@/lib/api";
import type { Agent, Task } from "@/lib/types";
import { workflowStageCounts, isActiveTask, isDoneStatus } from "@/lib/utils/task-utils";
import { CreateTaskPanel } from "@/components/projects/create-task-panel";
import { TasksTab } from "@/components/projects/tasks-tab";
import { ProjectStatusSummary } from "@/components/projects/project-status-summary";
import { ProjectProvider, useProjectContext } from "@/components/projects/project-context";
import { ProjectSidebar } from "@/components/projects/project-sidebar";
import { AgentsView } from "@/components/projects/agents-view";
import { RepositoriesView } from "@/components/projects/repositories-view";
import { RulesView } from "@/components/projects/rules-view";
import { ProjectProfile } from "@/components/projects/project-profile";

type ProjectView = "tasks" | "agents" | "repositories" | "rules" | "settings";

const projectViews: { id: ProjectView; label: string }[] = [
  { id: "tasks", label: "Tasks" },
  { id: "agents", label: "Agents" },
  { id: "repositories", label: "Repositories" },
  { id: "rules", label: "Rules" },
  { id: "settings", label: "Settings" },
];

export default function ProjectDetailRoute({ params }: { params: Promise<{ id: string }> }) {
  const { id: projectID } = use(params);
  const session = useSession();

  if (!session) {
    return (
      <main className="grid min-h-screen place-items-center p-6">
        <div className="rounded-lg border border-stroke bg-card p-6">
          <p className="mb-4 text-sm text-content-muted">Login from the dashboard before opening a project.</p>
          <Link className="rounded-md bg-brand-primary px-4 py-2 font-semibold text-slate-950" href="/">Back to login</Link>
        </div>
      </main>
    );
  }

  return (
    <ProjectProvider projectID={projectID}>
      <ProjectDetailContent />
    </ProjectProvider>
  );
}

function ProjectDetailContent() {
  const [activeView, setActiveView] = useState<ProjectView>("tasks");
  const [isTaskPanelOpen, setIsTaskPanelOpen] = useState(false);

  const {
    projectID,
    token,
    orgID,
    project,
    repositories,
    tasks,
    projectAgents,
    rules,
    isProjectLoading,
    isReposLoading,
    isTasksLoading,
    isAgentsLoading,
    isRulesLoading,
    mutateRepos,
    mutateProjectAgents,
    mutateRules,
    mutateProject,
    taskActions,
    repoActions,
    handleUpdateProject,
    projectError,
  } = useProjectContext();
  const activeTasks = tasks.filter((task) => isActiveTask(task)).length;
  const completedTasks = tasks.filter((task) => isDoneStatus(task.status)).length;

  // Keyboard Shortcuts (1-5) for Switching Views
  useEffect(() => {
    function handleKeyDown(e: KeyboardEvent) {
      if (
        document.activeElement?.tagName === "INPUT" ||
        document.activeElement?.tagName === "TEXTAREA" ||
        document.activeElement?.tagName === "SELECT"
      ) {
        return;
      }

      const keyMap: Record<string, ProjectView> = {
        "1": "tasks",
        "2": "agents",
        "3": "repositories",
        "4": "rules",
        "5": "settings",
      };

      if (keyMap[e.key]) {
        e.preventDefault();
        setActiveView(keyMap[e.key]);
      }
    }

    window.addEventListener("keydown", handleKeyDown);
    return () => window.removeEventListener("keydown", handleKeyDown);
  }, []);

  // Agents Actions
  async function handleAssignAgent(staff: Agent) {
    if (!projectID || !token) return;
    await api.createAgent(projectID, token, {
      name: staff.name,
      role: staff.role,
      goal: staff.goal,
      autonomy_level: staff.autonomy_level,
      model_level_group: staff.model_level_group,
      assignment_strategy: staff.assignment_strategy,
      agent_id: staff.id,
    });
    mutateProjectAgents();
  }

  // Repositories Actions
  const handleCloneRepository = async (repoID: string) => {
    await repoActions.cloneRepository(repoID);
  };

  const handleValidateRepository = async (repoID: string) => {
    await repoActions.validateRepository(repoID);
  };

  // Rules Actions
  async function handleAddRule(content: string, enforcement: "strict" | "advisory") {
    const created = await api.createRule(projectID, token, { content, scope: "project", enforcement });
    mutateRules([created, ...rules], false);
    return created;
  }

  async function handleUpdateRule(ruleId: string, content: string, enforcement: "strict" | "advisory") {
    const updated = await api.updateRule(ruleId, token, { content, enforcement });
    mutateRules(rules.map((r) => (r.id === ruleId ? updated : r)), false);
  }

  async function handleDeleteRule(ruleId: string) {
    await api.deleteRule(ruleId, token);
    mutateRules(rules.filter((r) => r.id !== ruleId), false);
  }

  async function handleSeedRules() {
    await api.seedRules(projectID, token);
    mutateRules();
  }

  return (
    <div className="flex h-screen overflow-hidden bg-background">
      <ProjectSidebar
        projectName={project?.name ?? ""}
        activeView={activeView}
        onViewChange={setActiveView}
        rulesCount={rules.length}
        agentsCount={projectAgents.length}
      />

      <section className="flex min-h-0 min-w-0 flex-1 flex-col">
        <header className="shrink-0 border-b border-stroke bg-card/95 px-5 py-4 shadow-sm md:px-6">
          <div className="flex flex-col gap-4 xl:flex-row xl:items-start xl:justify-between">
            <div className="min-w-0">
              <div className="flex items-center gap-1.5 text-xs text-content-muted">
                <span>Projects</span>
                <span>/</span>
                <span className="truncate font-semibold text-foreground">{project?.name || "Loading..."}</span>
              </div>
              <div className="mt-2 flex flex-wrap items-center gap-2">
                <h1 className="truncate text-2xl font-semibold tracking-tight text-foreground">
                  {project?.name || "Project workspace"}
                </h1>
                <span className="rounded-md border border-stroke bg-surface px-2 py-0.5 font-mono text-[11px] text-content-muted">
                  {projectID.slice(0, 8)}
                </span>
              </div>
              <div className="mt-3 flex flex-wrap items-center gap-2 text-xs text-content-muted">
                <WorkspaceSignal icon={GitBranch} label={`${repositories.length} repos`} />
                <WorkspaceSignal icon={Workflow} label={`${tasks.length} tasks`} />
                <WorkspaceSignal icon={Bot} label={`${projectAgents.length} agents`} />
                <WorkspaceSignal icon={ShieldCheck} label={`${rules.length} rules`} />
                <WorkspaceSignal icon={CheckCircle2} label={`${completedTasks}/${tasks.length} done`} />
              </div>
            </div>

            <div className="flex shrink-0 flex-wrap items-center gap-3">
              <div className="hidden rounded-lg border border-stroke bg-background px-3 py-2 text-xs text-content-muted lg:block">
                <span className="font-mono text-foreground">{activeTasks}</span> active now
              </div>
              <button
                onClick={() => {
                  taskActions.setTaskError("");
                  if (repositories.length === 0) {
                    setActiveView("repositories");
                    taskActions.setTaskError("You must add a repository to the project before creating tasks.");
                    return;
                  }
                  setIsTaskPanelOpen(true);
                }}
                className="flex items-center gap-1.5 rounded-md bg-brand-primary px-3.5 py-2 text-sm font-semibold text-white transition hover:opacity-90"
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
              onChange={(event) => setActiveView(event.target.value as ProjectView)}
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

        <div className="min-h-0 flex-1 overflow-y-auto bg-surface/10 p-4 md:p-6">
          {taskActions.taskError && (
            <div className="mb-4 rounded-md border border-red-500/20 bg-red-500/5 p-4 flex items-center justify-between gap-3 text-sm text-red-600 dark:text-red-400 animate-fade-in">
              <div className="flex items-center gap-2">
                <AlertCircle size={16} />
                <span>{taskActions.taskError}</span>
              </div>
              <button onClick={() => taskActions.setTaskError("")} className="text-content-muted hover:text-foreground cursor-pointer">
                <X size={16} />
              </button>
            </div>
          )}
          {projectError ? (
            <div className="flex flex-col items-center justify-center py-12 text-center rounded-lg border border-red-500/20 bg-red-500/5 p-6 max-w-lg mx-auto mt-8">
              <p className="font-sans text-base font-semibold text-red-600 dark:text-red-400">Failed to load project details.</p>
              <p className="mt-1 text-xs text-content-muted mb-4">Please check your configuration or network and try again.</p>
              <button
                onClick={() => mutateProject()}
                className="inline-flex items-center gap-1.5 rounded-md bg-brand-primary px-4 py-2 text-sm font-semibold text-slate-950 hover:opacity-90 transition cursor-pointer"
              >
                Retry Load
              </button>
            </div>
          ) : isProjectLoading ? (
            <div className="space-y-4">
              <div className="skeleton-shimmer h-12 w-full rounded" />
              <div className="skeleton-shimmer h-40 w-full rounded" />
            </div>
          ) : (
            <div className="space-y-6 animate-fade-in">
              {activeView === "tasks" && (
                <>
                  <ProjectStatusSummary tasks={tasks} projectAgents={projectAgents} />
                  <TasksTab
                    tasks={tasks}
                    projectAgents={projectAgents}
                    projectID={projectID}
                    isTasksLoading={isTasksLoading}
                    isLoadingTask={taskActions.isLoadingTask}
                    onTaskAction={taskActions.handleTaskAction}
                  />
                </>
              )}

              {activeView === "agents" && (
                <AgentsView
                  orgID={orgID}
                  projectAgents={projectAgents}
                  isLoading={isAgentsLoading}
                  onAssignAgent={handleAssignAgent}
                />
              )}

              {activeView === "repositories" && (
                <RepositoriesView
                  projectID={projectID}
                  project={project}
                  token={token}
                  orgID={orgID}
                  repositories={repositories}
                  isLoading={isReposLoading}
                  isLinking={repoActions.isLinkingRepository}
                  error={repoActions.repoError}
                  repoLoading={repoActions.repoLoading}
                  repoOperation={repoActions.repoOperation}
                  repoErrors={repoActions.repoErrors}
                  onLinkRepository={repoActions.createRepository}
                  onCloneRepository={handleCloneRepository}
                  onValidateRepository={handleValidateRepository}
                  onDeleteRepository={repoActions.deleteRepository}
                  onRefresh={mutateRepos}
                />
              )}

              {activeView === "rules" && (
                <RulesView
                  rules={rules}
                  isRulesLoading={isRulesLoading}
                  onAddRule={handleAddRule}
                  onUpdateRule={handleUpdateRule}
                  onDeleteRule={handleDeleteRule}
                  onSeedRules={handleSeedRules}
                />
              )}

              {activeView === "settings" && (
                <ProjectProfile
                  project={project}
                  onUpdateProject={handleUpdateProject}
                />
              )}
            </div>
          )}
        </div>
      </section>

      <CreateTaskPanel
        agents={projectAgents}
        repositories={repositories}
        isOpen={isTaskPanelOpen}
        isSubmitting={taskActions.isCreatingTask}
        error={taskActions.taskError}
        onClose={() => setIsTaskPanelOpen(false)}
        onSubmit={async (p) => {
          const success = await taskActions.createTask(p);
          if (success) {
            setIsTaskPanelOpen(false);
            setActiveView("tasks");
          }
          return success;
        }}
      />
    </div>
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
