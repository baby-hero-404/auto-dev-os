import { request } from "./client";
import type { GitAccount, Project, Repository, Rule, Task, WorkflowJob, WorkflowStatus, TaskLog } from "../types";

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

export function update(projectID: string, token: string, input: { name?: string; description?: string }) {
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
    input: { url?: string; provider?: string; branch?: string; token?: string; git_account_id?: string },
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
};

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
    input: { title: string; description: string; complexity: string; priority: number; labels: string[]; agent_id?: string },
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
  execute(taskID: string, token: string) {
    return request<WorkflowJob>(`/tasks/${taskID}/execute`, { method: "POST", token });
  },
  workflow(taskID: string, token: string) {
    return request<WorkflowStatus>(`/tasks/${taskID}/workflow`, { token });
  },
  logs(taskID: string, token: string) {
    return request<TaskLog[]>(`/tasks/${taskID}/logs`, { token });
  },
  approveWorkflow(taskID: string, token: string) {
    return request<Task>(`/tasks/${taskID}/approve`, { method: "POST", token });
  },
  approvePR(taskID: string, token: string) {
    return request<Task>(`/tasks/${taskID}/pr/approve`, { method: "POST", token });
  },
  rejectPR(taskID: string, token: string, feedback: string) {
    return request<Task>(`/tasks/${taskID}/pr/reject`, {
      method: "POST",
      token,
      body: JSON.stringify({ feedback }),
    });
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
