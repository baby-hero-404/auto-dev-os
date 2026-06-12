import useSWR from "swr";
import { api } from "@/lib/api";
import { useSession } from "@/lib/session";
import type { Task } from "@/lib/types";

export function useProjectData(projectID: string) {
  const session = useSession();
  const token = session?.token ?? "";

  const { data: project, mutate: mutateProject, isLoading: isProjectLoading } = useSWR(
    projectID && token ? ["project", projectID] : null,
    () => api.getProject(projectID, token),
  );

  const { data: repositories = [], mutate: mutateRepos, isLoading: isReposLoading } = useSWR(
    projectID && token ? ["repositories", projectID] : null,
    () => api.listRepositories(projectID, token),
  );

  const { data: tasks = [], mutate: mutateTasks, isLoading: isTasksLoading } = useSWR(
    projectID && token ? ["tasks", projectID] : null,
    () => api.listTasks(projectID, token),
    { refreshInterval: (latest?: Task[]) => (latest?.some(isActiveTask) ? 5000 : 0) },
  );

  const { data: projectAgents = [], mutate: mutateProjectAgents, isLoading: isAgentsLoading } = useSWR(
    projectID && token ? ["project-agents", projectID] : null,
    () => api.listAgents(projectID, token),
  );

  const { data: rules = [], mutate: mutateRules, isLoading: isRulesLoading } = useSWR(
    projectID && token ? ["rules", projectID] : null,
    () => api.listRules(projectID, token),
  );

  return {
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
  };
}

function isActiveTask(task: Task) {
  return ["analyzing", "running", "assigned", "planning", "coding", "reviewing", "fixing", "testing", "in_progress"].includes(task.status);
}
