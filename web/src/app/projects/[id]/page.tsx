"use client";

import Link from "next/link";
import { use, useEffect, useState } from "react";
import { Plus } from "lucide-react";
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

type ProjectView = "tasks" | "agents" | "repositories" | "rules" | "settings";

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
    taskActions,
    repoActions,
    handleUpdateProject,
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
  const handleCloneRepository = (repoID: string) => {
    api.cloneRepository(repoID, token);
    mutateRepos();
  };

  const handleValidateRepository = (repoID: string) => {
    api.validateRepository(repoID, token);
    mutateRepos();
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
        {/* Header bar */}
        <header className="flex items-center justify-between border-b border-stroke px-6 py-4 bg-card shrink-0">
          <div>
            <div className="flex items-center gap-1.5 text-xs text-content-muted">
              <span>Projects</span>
              <span>/</span>
              <span className="font-semibold text-foreground">{project?.name || "Loading..."}</span>
            </div>
          </div>
          <div className="flex items-center gap-3">
            <button
              onClick={() => {
                taskActions.setTaskError("");
                setIsTaskPanelOpen(true);
              }}
              className="flex items-center gap-1.5 rounded-md bg-brand-primary px-3.5 py-2 text-sm font-semibold text-slate-950 hover:opacity-90 transition cursor-pointer"
              type="button"
            >
              <Plus size={15} /> Create Task
            </button>
            <div className="text-[11px] text-content-muted font-mono bg-surface border border-stroke rounded px-2.5 py-1 hidden sm:block">
              ID: {projectID}
            </div>
          </div>
        </header>

        {/* Scrollable View Content */}
        <div className="min-h-0 flex-1 overflow-y-auto p-6 bg-surface/10">
          {isProjectLoading ? (
            <div className="space-y-4">
              <div className="skeleton-shimmer h-12 w-full rounded" />
              <div className="skeleton-shimmer h-40 w-full rounded" />
            </div>
          ) : (
            <div className="space-y-6 animate-fade-in">
              {activeView === "tasks" && (
                <>
                  <ProjectStatusSummary tasks={tasks} projectAgents={projectAgents} />
                  <div className="rounded-lg border border-stroke bg-card p-5">
                    <TasksTab
                      tasks={tasks}
                      projectAgents={projectAgents}
                      projectID={projectID}
                      isTasksLoading={isTasksLoading}
                      isLoadingTask={taskActions.isLoadingTask}
                      onTaskAction={taskActions.handleTaskAction}
                    />
                  </div>
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
                  project={project!}
                  token={token}
                  orgID={orgID}
                  repositories={repositories}
                  isLoading={isReposLoading}
                  isLinking={repoActions.isLinkingRepository}
                  error={repoActions.repoError}
                  onLinkRepository={repoActions.createRepository}
                  onCloneRepository={handleCloneRepository}
                  onValidateRepository={handleValidateRepository}
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
