import type {
  Agent,
  AgentStats,
  AuditLog,
  AuthResponse,
  Organization,
  MemorySearchResult,
  OverviewStats,
  Project,
  Repository,
  Rule,
  Skill,
  Task,
  TaskAnalytics,
  TaskLog,
  TokenUsageSummary,
  WorkflowAnalytics,
  WorkflowJob,
  WorkflowStatus,
  EpisodicMemory,
  KnowledgeEdge,
  LearningSuggestion,
} from "./types";

const API_BASE = process.env.NEXT_PUBLIC_API_URL ?? "http://localhost:32080/api/v1";

type RequestOptions = RequestInit & {
  token?: string | null;
};

export class ApiError extends Error {
  status: number;

  constructor(status: number, message: string) {
    super(message);
    this.status = status;
  }
}

async function request<T>(path: string, options: RequestOptions = {}): Promise<T> {
  const headers = new Headers(options.headers);
  headers.set("Content-Type", "application/json");
  if (options.token) {
    headers.set("Authorization", `Bearer ${options.token}`);
  }

  const response = await fetch(`${API_BASE}${path}`, {
    ...options,
    headers,
  });

  if (!response.ok) {
    let message = response.statusText;
    try {
      const body = (await response.json()) as { error?: string };
      message = body.error ?? message;
    } catch {
      // Keep status text when response is not JSON.
    }

    if (
      response.status === 401 ||
      message.includes("token expired") ||
      message === "validation: token expired"
    ) {
      if (typeof window !== "undefined") {
        // Clear session to force redirect to login
        import("./session").then(({ clearSession }) => {
          clearSession();
        });
      }
    }

    throw new ApiError(response.status, message);
  }

  if (response.status === 204) {
    return undefined as T;
  }
  return response.json() as Promise<T>;
}

export const api = {
  // ─── Auth ──────────────────────────────────────────────────────
  register(input: { email: string; password: string; org_name?: string }) {
    return request<AuthResponse>("/auth/register", {
      method: "POST",
      body: JSON.stringify(input),
    });
  },
  login(input: { email: string; password: string }) {
    return request<AuthResponse>("/auth/login", {
      method: "POST",
      body: JSON.stringify(input),
    });
  },

  // ─── Organizations ────────────────────────────────────────────
  getOrganization(orgID: string, token: string) {
    return request<Organization>(`/organizations/${orgID}`, { token });
  },

  // ─── Projects ─────────────────────────────────────────────────
  listProjects(orgID: string, token: string) {
    return request<Project[]>(`/organizations/${orgID}/projects`, { token });
  },
  createProject(orgID: string, token: string, input: { name: string; description: string }) {
    return request<Project>(`/organizations/${orgID}/projects`, {
      method: "POST",
      token,
      body: JSON.stringify(input),
    });
  },
  getProject(projectID: string, token: string) {
    return request<Project>(`/projects/${projectID}`, { token });
  },
  updateProject(
    projectID: string,
    token: string,
    input: { name?: string; description?: string }
  ) {
    return request<Project>(`/projects/${projectID}`, {
      method: "PATCH",
      token,
      body: JSON.stringify(input),
    });
  },
  deleteProject(projectID: string, token: string) {
    return request<void>(`/projects/${projectID}`, { method: "DELETE", token });
  },

  // ─── Repositories ─────────────────────────────────────────────
  listRepositories(projectID: string, token: string) {
    return request<Repository[]>(`/projects/${projectID}/repositories`, { token });
  },
  createRepository(projectID: string, token: string, input: { url: string; provider: string; branch: string; token?: string }) {
    return request<Repository>(`/projects/${projectID}/repositories`, {
      method: "POST",
      token,
      body: JSON.stringify(input),
    });
  },
  validateRepository(repoID: string, token: string) {
    return request<{ valid: boolean }>(`/repositories/${repoID}/validate`, { method: "POST", token });
  },
  cloneRepository(repoID: string, token: string) {
    return request<Repository>(`/repositories/${repoID}/clone`, { method: "POST", token });
  },

  // ─── Tasks ────────────────────────────────────────────────────
  listTasks(projectID: string, token: string) {
    return request<Task[]>(`/projects/${projectID}/tasks`, { token });
  },
  getTask(taskID: string, token: string) {
    return request<Task>(`/tasks/${taskID}`, { token });
  },
  createTask(projectID: string, token: string, input: { title: string; description: string; complexity: string; priority: number; labels: string[] }) {
    return request<Task>(`/projects/${projectID}/tasks`, {
      method: "POST",
      token,
      body: JSON.stringify(input),
    });
  },
  analyzeTask(taskID: string, token: string) {
    return request<Task>(`/tasks/${taskID}/analyze`, { method: "POST", token });
  },
  approveTaskAnalysis(taskID: string, token: string) {
    return request<Task>(`/tasks/${taskID}/analysis/approve`, { method: "POST", token });
  },
  requestTaskChanges(taskID: string, token: string, context: string) {
    return request<Task>(`/tasks/${taskID}/analysis/request-changes`, {
      method: "POST",
      token,
      body: JSON.stringify({ context }),
    });
  },
  executeTask(taskID: string, token: string) {
    return request<WorkflowJob>(`/tasks/${taskID}/execute`, { method: "POST", token });
  },
  taskWorkflow(taskID: string, token: string) {
    return request<WorkflowStatus>(`/tasks/${taskID}/workflow`, { token });
  },
  taskLogs(taskID: string, token: string) {
    return request<TaskLog[]>(`/tasks/${taskID}/logs`, { token });
  },
  approveTaskWorkflow(taskID: string, token: string) {
    return request<Task>(`/tasks/${taskID}/approve`, { method: "POST", token });
  },

  // ─── Agents ───────────────────────────────────────────────────
  listAgents(projectID: string, token: string) {
    return request<Agent[]>(`/projects/${projectID}/agents`, { token });
  },
  createAgent(
    projectID: string,
    token: string,
    input: {
      name: string;
      role: string;
      provider: string;
      model: string;
      level?: string;
      assignment_strategy?: string;
      agent_id?: string;
    }
  ) {
    return request<Agent>(`/projects/${projectID}/agents`, {
      method: "POST",
      token,
      body: JSON.stringify(input),
    });
  },
  listOrgAgents(orgID: string, token: string) {
    return request<Agent[]>(`/organizations/${orgID}/agents`, { token });
  },
  hireAgent(
    orgID: string,
    token: string,
    input: {
      name: string;
      role: string;
      provider: string;
      model: string;
      level?: string;
      assignment_strategy?: string;
    }
  ) {
    return request<Agent>(`/organizations/${orgID}/agents`, {
      method: "POST",
      token,
      body: JSON.stringify(input),
    });
  },
  deleteAgent(agentID: string, token: string) {
    return request<void>(`/agents/${agentID}`, { method: "DELETE", token });
  },

  // ─── Rules ────────────────────────────────────────────────────
  listRules(projectID: string, token: string) {
    return request<Rule[]>(`/projects/${projectID}/rules`, { token });
  },
  createRule(projectID: string, token: string, input: { scope: string; content: string; enforcement: string }) {
    return request<Rule>(`/projects/${projectID}/rules`, {
      method: "POST",
      token,
      body: JSON.stringify(input),
    });
  },

  // ─── Skills ───────────────────────────────────────────────────
  listSkills(token: string) {
    return request<Skill[]>("/skills", { token });
  },
  listAgentSkills(agentID: string, token: string) {
    return request<Skill[]>(`/agents/${agentID}/skills`, { token });
  },
  assignSkillToAgent(agentID: string, skillID: string, token: string) {
    return request<{ status: string }>(`/agents/${agentID}/skills`, {
      method: "POST",
      token,
      body: JSON.stringify({ skill_id: skillID }),
    });
  },

  // ─── Analytics ────────────────────────────────────────────────
  tokenUsage(token: string, days = 30) {
    return request<TokenUsageSummary[]>(`/analytics/token-usage?days=${days}`, { token });
  },
  analyticsOverview(token: string, orgID?: string) {
    const params = orgID ? `?org_id=${orgID}` : "";
    return request<OverviewStats>(`/analytics/overview${params}`, { token });
  },
  analyticsAgents(token: string, projectID?: string) {
    const params = projectID ? `?project_id=${projectID}` : "";
    return request<AgentStats[]>(`/analytics/agents${params}`, { token });
  },
  analyticsTasks(token: string, projectID?: string, days = 30) {
    const params = new URLSearchParams({ days: days.toString() });
    if (projectID) params.set("project_id", projectID);
    return request<TaskAnalytics>(`/analytics/tasks?${params}`, { token });
  },
  analyticsWorkflows(token: string, projectID?: string) {
    const params = projectID ? `?project_id=${projectID}` : "";
    return request<WorkflowAnalytics>(`/analytics/workflows${params}`, { token });
  },

  // ─── Audit ────────────────────────────────────────────────────
  auditLogs(token: string, filters: { org_id?: string; action?: string; entity_type?: string; days?: number; limit?: number } = {}) {
    const params = new URLSearchParams();
    if (filters.org_id) params.set("org_id", filters.org_id);
    if (filters.action) params.set("action", filters.action);
    if (filters.entity_type) params.set("entity_type", filters.entity_type);
    if (filters.days) params.set("days", filters.days.toString());
    if (filters.limit) params.set("limit", filters.limit.toString());
    return request<AuditLog[]>(`/audit/logs?${params}`, { token });
  },
  auditSummary(token: string, orgID?: string) {
    const params = orgID ? `?org_id=${orgID}` : "";
    return request<Record<string, number>>(`/audit/summary${params}`, { token });
  },

  // ─── PR Review ───────────────────────────────────────────────
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

  // ─── Episodic Memory ──────────────────────────────────────────
  listMemories(agentID: string, token: string, tier?: string) {
    const params = tier ? `?tier=${tier}` : "";
    return request<{ memories: EpisodicMemory[] }>(`/agents/${agentID}/memories${params}`, { token });
  },
  searchMemories(agentID: string, query: string, token: string) {
    const params = new URLSearchParams({ q: query });
    return request<{ results: MemorySearchResult[] }>(`/agents/${agentID}/memories/search?${params}`, { token });
  },
  getMemory(memoryID: string, token: string) {
    return request<{ memory: EpisodicMemory; edges?: KnowledgeEdge[] }>(`/memories/${memoryID}`, { token });
  },
  deleteMemory(memoryID: string, token: string) {
    return request<void>(`/memories/${memoryID}`, { method: "DELETE", token });
  },

  // ─── Learning Loop ────────────────────────────────────────────
  listSuggestions(agentID: string, token: string, status?: string) {
    const params = status ? `?status=${status}` : "";
    return request<{ suggestions: LearningSuggestion[] }>(`/agents/${agentID}/suggestions${params}`, { token });
  },
  getSuggestion(suggestionID: string, token: string) {
    return request<{ suggestion: LearningSuggestion }>(`/suggestions/${suggestionID}`, { token });
  },
  approveSuggestion(suggestionID: string, token: string) {
    return request<{ suggestion: LearningSuggestion }>(`/suggestions/${suggestionID}/approve`, { method: "POST", token });
  },
  rejectSuggestion(suggestionID: string, token: string, feedback?: string) {
    return request<{ suggestion: LearningSuggestion }>(`/suggestions/${suggestionID}/reject`, {
      method: "POST",
      token,
      body: JSON.stringify({ feedback: feedback ?? "" }),
    });
  },
};
