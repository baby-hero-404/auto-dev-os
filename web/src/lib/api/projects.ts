import { request } from "./client";
import type { CLIEngineConfig, ExecutionEngine, GitAccount, Project, Repository, Rule, Task, WorkflowJob, WorkflowStatus, TaskLog, WorkflowArtifact, TaskAnalysis, TaskSpec } from "../types";

export function list(orgID: string, token: string) {
  return request<Project[]>(`/organizations/${orgID}/projects`, { token });
}

export function create(orgID: string, token: string, input: { name: string; description: string }) {
  return request<Project>(`/organizations/${orgID}/projects`, {
    method: "POST",
    token,
    body: JSON.stringify(input),
  });
}

export function get(projectID: string, token: string) {
  return request<Project>(`/projects/${projectID}`, { token });
}

export function update(projectID: string, token: string, input: {
  name?: string;
  description?: string;
  default_model_level?: string;
  default_autonomy?: string;
  auto_review_policy?: string;
  max_retries?: number;
  max_review_fix_cycles?: number;
  default_branch?: string;
  execution_engine?: ExecutionEngine;
  cli_engine_config?: CLIEngineConfig;
}) {
  return request<Project>(`/projects/${projectID}`, {
    method: "PATCH",
    token,
    body: JSON.stringify(input),
  });
}

export function remove(projectID: string, token: string) {
  return request<void>(`/projects/${projectID}`, { method: "DELETE", token });
}

export const repositories = {
  list(projectID: string, token: string) {
    return request<Repository[]>(`/projects/${projectID}/repositories`, { token });
  },
  create(
    projectID: string,
    token: string,
    input: { url: string; provider: string; branch: string; token?: string; git_account_id?: string },
  ) {
    return request<Repository>(`/projects/${projectID}/repositories`, {
      method: "POST",
      token,
      body: JSON.stringify(input),
    });
  },
  validate(repoID: string, token: string) {
    return request<{ valid: boolean }>(`/repositories/${repoID}/validate`, { method: "POST", token });
  },
  clone(repoID: string, token: string) {
    return request<Repository>(`/repositories/${repoID}/clone`, { method: "POST", token });
  },
  update(
    repoID: string,
    token: string,
    input: { url?: string; provider?: string; branch?: string; token?: string; git_account_id?: string; display_name?: string },
  ) {
    return request<Repository>(`/repositories/${repoID}`, {
      method: "PATCH",
      token,
      body: JSON.stringify(input),
    });
  },
  getBranches(
    token: string,
    input: { url: string; token?: string; git_account_id?: string },
  ) {
    return request<{ branches: string[] }>(`/repositories/branches`, {
      method: "POST",
      token,
      body: JSON.stringify(input),
    });
  },
  delete(repoID: string, token: string) {
    return request<void>(`/repositories/${repoID}`, {
      method: "DELETE",
      token,
    });
  },
};

class StreamFatalError extends Error {
  status: number;
  constructor(status: number) {
    super(`Stream failed: ${status}`);
    this.status = status;
  }
}

export const tasks = {
  list(projectID: string, token: string) {
    return request<Task[]>(`/projects/${projectID}/tasks`, { token });
  },
  get(taskID: string, token: string) {
    return request<Task>(`/tasks/${taskID}`, { token });
  },
  create(
    projectID: string,
    token: string,
    input: { title: string; description: string; complexity: string; priority: number; labels: string[]; agent_id?: string; repository_id?: string; execution_engine?: ExecutionEngine },
  ) {
    return request<Task>(`/projects/${projectID}/tasks`, {
      method: "POST",
      token,
      body: JSON.stringify(input),
    });
  },
  analyze(taskID: string, token: string) {
    return request<Task>(`/tasks/${taskID}/analyze`, { method: "POST", token });
  },
  update(
    taskID: string,
    token: string,
    input: { title?: string; description?: string; complexity?: string; priority?: number; labels?: string[]; agent_id?: string; repository_id?: string; analysis?: TaskAnalysis; spec_status?: string },
  ) {
    return request<Task>(`/tasks/${taskID}`, {
      method: "PATCH",
      token,
      body: JSON.stringify(input),
    });
  },
  delete(taskID: string, token: string) {
    return request<{ status: string }>(`/tasks/${taskID}`, { method: "DELETE", token });
  },
  approveAnalysis(taskID: string, token: string) {
    return request<Task>(`/tasks/${taskID}/analysis/approve`, { method: "POST", token });
  },
  requestChanges(taskID: string, token: string, context: string) {
    return request<Task>(`/tasks/${taskID}/analysis/request-changes`, {
      method: "POST",
      token,
      body: JSON.stringify({ context }),
    });
  },
  getSpec(taskID: string, token: string) {
    return request<TaskSpec>(`/tasks/${taskID}/spec`, { token });
  },
  specReview(taskID: string, token: string, action: "approve" | "request_changes", comment?: string) {
    return request<Task>(`/tasks/${taskID}/spec-review`, {
      method: "POST",
      token,
      body: JSON.stringify({ action, comment: comment ?? "" }),
    });
  },
  clarify(taskID: string, token: string, context: string) {
    return request<Task>(`/tasks/${taskID}/clarify`, {
      method: "POST",
      token,
      body: JSON.stringify({ context }),
    });
  },
  execute(taskID: string, token: string) {
    return request<WorkflowJob>(`/tasks/${taskID}/execute`, { method: "POST", token });
  },
  retry(taskID: string, token: string) {
    return request<WorkflowJob>(`/tasks/${taskID}/retry`, { method: "POST", token });
  },
  pause(taskID: string, token: string) {
    return request<{ status: string }>(`/tasks/${taskID}/pause`, { method: "POST", token });
  },
  cancel(taskID: string, token: string) {
    return request<{ status: string }>(`/tasks/${taskID}/cancel`, { method: "POST", token });
  },
  workflow(taskID: string, token: string) {
    return request<WorkflowStatus>(`/tasks/${taskID}/workflow`, { token });
  },
  logs(taskID: string, token: string) {
    return request<TaskLog[]>(`/tasks/${taskID}/logs`, { token });
  },
  async streamLogs(
    taskID: string,
    token: string,
    signal: AbortSignal,
    onLog: (log: TaskLog) => void,
    onFatalError?: (err: Error) => void,
  ) {
    const API_BASE = process.env.NEXT_PUBLIC_API_URL ?? "http://localhost:32080/api/v1";
    let retryDelay = 1000;

    while (!signal.aborted) {
      try {
        const res = await fetch(`${API_BASE}/tasks/${taskID}/logs/stream`, {
          headers: { Authorization: `Bearer ${token}` },
          signal,
        });

        if (!res.ok) {
          if (res.status === 401 || res.status === 403 || res.status === 404) {
            throw new StreamFatalError(res.status);
          }
          throw new Error(`Stream failed: ${res.status}`);
        }

        retryDelay = 1000;
        if (!res.body) throw new Error("No body");

        const reader = res.body.getReader();
        const decoder = new TextDecoder();
        let buffer = "";

        while (true) {
          const { done, value } = await reader.read();
          if (done) break;
          buffer += decoder.decode(value, { stream: true });

          const lines = buffer.split("\n");
          buffer = lines.pop() ?? "";

          let currentEvent = "";
          for (const line of lines) {
            if (line.startsWith("event: ")) {
              currentEvent = line.slice(7).trim();
            } else if (line.startsWith("data: ")) {
              const dataStr = line.slice(6);
              if (currentEvent === "log") {
                try {
                  onLog(JSON.parse(dataStr));
                } catch {}
              }
            }
          }
        }
      } catch (err) {
        const error = err as Error;
        if (error.name === "AbortError" || signal.aborted) {
          return;
        }
        if (err instanceof StreamFatalError) {
          onFatalError?.(err);
          return;
        }
      }

      if (signal.aborted) return;

      await new Promise(resolve => setTimeout(resolve, retryDelay));
      retryDelay = Math.min(retryDelay * 2, 5000);
    }
  },
  approveWorkflow(taskID: string, token: string) {
    return request<Task>(`/tasks/${taskID}/approve`, { method: "POST", token });
  },
  approvePR(taskID: string, token: string) {
    return request<Task>(`/tasks/${taskID}/pr/approve`, { method: "POST", token });
  },
  startReview(taskID: string, token: string) {
    return request<Task>(`/tasks/${taskID}/pr/start-review`, { method: "POST", token });
  },
  rejectPR(taskID: string, token: string, feedback: string) {
    return request<Task>(`/tasks/${taskID}/pr/reject`, {
      method: "POST",
      token,
      body: JSON.stringify({ feedback }),
    });
  },
  artifacts(jobID: string, token: string) {
    return request<WorkflowArtifact[]>(`/workflows/${jobID}/artifacts`, { token });
  },
};

export const rules = {
  listGlobal(orgID: string, token: string) {
    return request<Rule[]>(`/organizations/${orgID}/rules`, { token });
  },
  createGlobal(orgID: string, token: string, input: { content: string; enforcement: string }) {
    return request<Rule>(`/organizations/${orgID}/rules`, {
      method: "POST",
      token,
      body: JSON.stringify(input),
    });
  },
  seedGlobal(orgID: string, token: string) {
    return request<Rule[]>(`/organizations/${orgID}/rules/seed`, { method: "POST", token });
  },
  list(projectID: string, token: string) {
    return request<Rule[]>(`/projects/${projectID}/rules`, { token });
  },
  seed(projectID: string, token: string) {
    return request<Rule[]>(`/projects/${projectID}/rules/seed`, { method: "POST", token });
  },
  create(projectID: string, token: string, input: { scope: string; content: string; enforcement: string }) {
    return request<Rule>(`/projects/${projectID}/rules`, {
      method: "POST",
      token,
      body: JSON.stringify(input),
    });
  },
  update(ruleID: string, token: string, input: { content?: string; enforcement?: string }) {
    return request<Rule>(`/rules/${ruleID}`, {
      method: "PATCH",
      token,
      body: JSON.stringify(input),
    });
  },
  remove(ruleID: string, token: string) {
    return request<void>(`/rules/${ruleID}`, { method: "DELETE", token });
  },
};

export const gitAccounts = {
  list(orgID: string, token: string) {
    return request<GitAccount[]>(`/organizations/${orgID}/git-accounts`, { token });
  },
  create(
    orgID: string,
    token: string,
    input: { provider: string; display_name: string; base_url?: string; token: string },
  ) {
    return request<GitAccount>(`/organizations/${orgID}/git-accounts`, {
      method: "POST",
      token,
      body: JSON.stringify(input),
    });
  },
  remove(accID: string, token: string) {
    return request<void>(`/git-accounts/${accID}`, { method: "DELETE", token });
  },
  test(accID: string, token: string) {
    return request<{ status: string }>(`/git-accounts/${accID}/test`, { method: "POST", token });
  },
};
