import useSWR from "swr";
import { api } from "@/lib/api";
import { useSession } from "@/lib/session";
import { isActiveStatus } from "@/lib/status";
import type { Task } from "@/lib/types";

export function useProjectData(projectID: string) {
  const session = useSession();
  const token = session?.token ?? "";

  const { data: project, mutate: mutateProject, isLoading: isProjectLoading, error: projectError } = useSWR(
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
    { refreshInterval: (latest?: Task[]) => (latest?.some((task) => isActiveStatus(task.status)) ? 5000 : 0) },
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
    repositories: repositories ?? [],
    tasks: tasks ?? [],
    projectAgents: projectAgents ?? [],
    rules: rules ?? [],
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
    projectError,
  };
}
