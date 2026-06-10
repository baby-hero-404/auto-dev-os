import { request } from "./client";
import type { AuditLog } from "../types";

export function logs(
  token: string,
  filters: { org_id?: string; action?: string; entity_type?: string; days?: number; limit?: number } = {},
) {
  const params = new URLSearchParams();
  if (filters.org_id) params.set("org_id", filters.org_id);
  if (filters.action) params.set("action", filters.action);
  if (filters.entity_type) params.set("entity_type", filters.entity_type);
  if (filters.days) params.set("days", filters.days.toString());
  if (filters.limit) params.set("limit", filters.limit.toString());
  return request<AuditLog[]>(`/audit/logs?${params}`, { token });
}

export function summary(token: string, orgID?: string) {
  const params = orgID ? `?org_id=${orgID}` : "";
  return request<Record<string, number>>(`/audit/summary${params}`, { token });
}
