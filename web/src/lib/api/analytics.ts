import { request } from "./client";
import type { AgentStats, OverviewStats, TaskAnalytics, TokenUsageSummary, WorkflowAnalytics } from "../types";

export function tokenUsage(token: string, days = 30) {
  return request<TokenUsageSummary[]>(`/analytics/token-usage?days=${days}`, { token });
}

export function overview(token: string, orgID?: string) {
  const params = orgID ? `?org_id=${orgID}` : "";
  return request<OverviewStats>(`/analytics/overview${params}`, { token });
}

export function agents(token: string, projectID?: string) {
  const params = projectID ? `?project_id=${projectID}` : "";
  return request<AgentStats[]>(`/analytics/agents${params}`, { token });
}

export function tasks(token: string, projectID?: string, days = 30) {
  const params = new URLSearchParams({ days: days.toString() });
  if (projectID) params.set("project_id", projectID);
  return request<TaskAnalytics>(`/analytics/tasks?${params}`, { token });
}

export function workflows(token: string, projectID?: string) {
  const params = projectID ? `?project_id=${projectID}` : "";
  return request<WorkflowAnalytics>(`/analytics/workflows${params}`, { token });
}
