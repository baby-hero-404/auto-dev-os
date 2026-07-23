"use client";

import Link from "next/link";
import { use, useEffect, useState, Suspense, useCallback } from "react";
import { useRouter, usePathname, useSearchParams } from "next/navigation";
import { AlertCircle, X } from "lucide-react";
import { useSession } from "@/lib/session";
import { api } from "@/lib/api";
import type { Agent } from "@/lib/types";
import { CreateTaskPanel } from "@/components/projects/create-task-panel";
import { TasksTab } from "@/components/projects/tasks-tab";
import { ProjectStatusSummary } from "@/components/projects/project-status-summary";
import { ProjectProvider, useProjectContext } from "@/components/projects/project-context";
import { ProjectSidebar } from "@/components/projects/project-sidebar";
import { AgentsView } from "@/components/projects/agents-view";
import { RepositoriesView } from "@/components/projects/repositories-view";
import { RulesView } from "@/components/projects/rules-view";
import { ProjectProfile } from "@/components/projects/project-profile";
import { ProjectHeader } from "@/components/projects/ProjectHeader";
import { Skeleton } from "@/components/ui/skeleton";
import { Button } from "@/components/ui/button";

type ProjectView = "tasks" | "agents" | "repositories" | "rules" | "settings";

export default function ProjectDetailRoute({ params }: { params: Promise<{ id: string }> }) {
  const { id: projectID } = use(params);
  const session = useSession();

  if (!session) {
    return (
      <main className="grid min-h-screen place-items-center p-6">
        <div className="rounded-lg border border-stroke bg-card p-6">
          <p className="mb-4 text-sm text-content-muted">Login from the dashboard before opening a project.</p>
          <Button asChild variant="primary">
            <Link href="/">Back to login</Link>
          </Button>
        </div>
      </main>
    );
  }

  return (
    <ProjectProvider projectID={projectID}>
      <Suspense fallback={
        <div className="flex h-screen bg-background items-center justify-center">
          <span className="h-8 w-8 animate-spin rounded-full border-2 border-brand-primary border-t-transparent" />
        </div>
      }>
        <ProjectDetailContent />
      </Suspense>
    </ProjectProvider>
  );
}

function ProjectDetailContent() {
  const router = useRouter();
  const pathname = usePathname();
  const searchParams = useSearchParams();
  const [isTaskPanelOpen, setIsTaskPanelOpen] = useState(false);

  const rawView = searchParams.get("view");
  const activeView: ProjectView = (rawView === "tasks" || rawView === "agents" || rawView === "repositories" || rawView === "rules" || rawView === "settings") ? rawView : "tasks";

  const setActiveView = useCallback((view: ProjectView) => {
    const params = new URLSearchParams(searchParams.toString());
    params.set("view", view);
    router.replace(`${pathname}?${params.toString()}`, { scroll: false });
  }, [searchParams, pathname, router]);

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
  }, [setActiveView]);

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
        projectID={projectID}
        projectName={project?.name ?? ""}
        activeView={activeView}
        rulesCount={rules.length}
        agentsCount={projectAgents.length}
      />

      <section className="flex min-h-0 min-w-0 flex-1 flex-col">
        <ProjectHeader
          projectName={project?.name ?? ""}
          projectID={projectID}
          activeView={activeView}
          onCreateTaskClick={() => {
            taskActions.setTaskError("");
            if (repositories.length === 0) {
              setActiveView("repositories");
              taskActions.setTaskError("You must add a repository to the project before creating tasks.");
              return;
            }
            setIsTaskPanelOpen(true);
          }}
        />

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
            <div className="flex flex-col items-center justify-center py-12 text-center rounded-lg border border-danger/20 bg-danger/5 p-6 max-w-lg mx-auto mt-8">
              <p className="font-sans text-base font-semibold text-danger">Failed to load project details.</p>
              <p className="mt-1 text-xs text-content-muted mb-4">Please check your configuration or network and try again.</p>
              <Button
                onClick={() => mutateProject()}
                size="sm"
              >
                Retry Load
              </Button>
            </div>
          ) : isProjectLoading ? (
            <div className="space-y-4">
              <Skeleton className="h-12 w-full" />
              <Skeleton className="h-40 w-full" />
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
                  token={token}
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
