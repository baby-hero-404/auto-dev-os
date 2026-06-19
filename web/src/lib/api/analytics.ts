import { request } from "./client";
import type { AgentStats, OverviewStats, RecentFailure, TaskAnalytics, TokenUsageSummary, WorkflowAnalytics } from "../types";

export function tokenUsage(token: string, orgID?: string, days = 30) {
  const params = new URLSearchParams({ days: days.toString() });
  if (orgID) params.set("org_id", orgID);
  return request<TokenUsageSummary[]>(`/analytics/token-usage?${params}`, { token });
}

export function overview(token: string, orgID?: string) {
  const params = orgID ? `?org_id=${orgID}` : "";
  return request<OverviewStats>(`/analytics/overview${params}`, { token });
}

export function agents(token: string, orgID?: string, projectID?: string) {
  const params = new URLSearchParams();
  if (orgID) params.set("org_id", orgID);
  if (projectID) params.set("project_id", projectID);
  const query = params.toString();
  return request<AgentStats[]>(`/analytics/agents${query ? `?${query}` : ""}`, { token });
}

export function tasks(token: string, orgID?: string, projectID?: string, days = 30) {
  const params = new URLSearchParams({ days: days.toString() });
  if (orgID) params.set("org_id", orgID);
  if (projectID) params.set("project_id", projectID);
  return request<TaskAnalytics>(`/analytics/tasks?${params}`, { token });
}

export function workflows(token: string, orgID?: string, projectID?: string) {
  const params = new URLSearchParams();
  if (orgID) params.set("org_id", orgID);
  if (projectID) params.set("project_id", projectID);
  const query = params.toString();
  return request<WorkflowAnalytics>(`/analytics/workflows${query ? `?${query}` : ""}`, { token });
}

export function failures(token: string, orgID?: string, projectID?: string, limit = 5) {
  const params = new URLSearchParams({ limit: limit.toString() });
  if (orgID) params.set("org_id", orgID);
  if (projectID) params.set("project_id", projectID);
  return request<RecentFailure[]>(`/analytics/failures?${params}`, { token });
}
