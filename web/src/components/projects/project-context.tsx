"use client";

import { createContext, useContext, ReactNode } from "react";
import { useSession } from "@/lib/session";
import { useProjectData } from "@/lib/hooks/use-project-data";
import { useTaskActions } from "@/lib/hooks/project/use-task-actions";
import { useRepoActions } from "@/lib/hooks/project/use-repo-actions";
import { api } from "@/lib/api";
import type { Agent, Project, Repository, Rule, Task } from "@/lib/types";

// Define the shape of the context
interface ProjectContextValue {
  projectID: string;
  token: string;
  orgID: string;
  
  // Data
  project?: Project;
  repositories: Repository[];
  tasks: Task[];
  projectAgents: Agent[];
  rules: Rule[];
  
  // Loading states
  isProjectLoading: boolean;
  isReposLoading: boolean;
  isTasksLoading: boolean;
  isAgentsLoading: boolean;
  isRulesLoading: boolean;
  
  // Mutators
  mutateProject: () => void;
  mutateRepos: () => void;
  mutateTasks: () => void;
  mutateProjectAgents: () => void;
  mutateRules: (data?: any, shouldRevalidate?: boolean) => any;
  
  // Actions
  taskActions: ReturnType<typeof useTaskActions>;
  repoActions: ReturnType<typeof useRepoActions>;
  
  // Generic Project Actions
  handleUpdateProject: (name: string, description: string) => Promise<void>;
}

const ProjectContext = createContext<ProjectContextValue | null>(null);

export function ProjectProvider({ projectID, children }: { projectID: string; children: ReactNode }) {
  const session = useSession();
  const token = session?.token ?? "";
  const orgID = session?.user.org_id ?? "";

  const {
    project,
    repositories,
    tasks,
    projectAgents,
    rules,
    mutateProject,
    mutateRepos,
    mutateTasks,
    mutateProjectAgents,
    mutateRules,
    isProjectLoading,
    isReposLoading,
    isTasksLoading,
    isAgentsLoading,
    isRulesLoading,
  } = useProjectData(projectID);

  const taskActions = useTaskActions(projectID, token, mutateTasks);
  const repoActions = useRepoActions(projectID, token, mutateRepos);

  async function handleUpdateProject(name: string, description: string) {
    if (!projectID || !token) return;
    await api.updateProject(projectID, token, { name, description });
    mutateProject();
  }

  const value: ProjectContextValue = {
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
    mutateProject,
    mutateRepos,
    mutateTasks,
    mutateProjectAgents,
    mutateRules,
    taskActions,
    repoActions,
    handleUpdateProject,
  };

  return (
    <ProjectContext.Provider value={value}>
      {children}
    </ProjectContext.Provider>
  );
}

export function useProjectContext() {
  const context = useContext(ProjectContext);
  if (!context) {
    throw new Error("useProjectContext must be used within a ProjectProvider");
  }
  return context;
}
