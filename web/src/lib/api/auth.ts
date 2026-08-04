import { request } from "./client";
import type { AuthResponse, ExecutionProviderConfig, Organization } from "../types";

export function register(input: { email: string; password: string; org_name?: string }) {
  return request<AuthResponse>("/auth/register", {
    method: "POST",
    body: JSON.stringify(input),
  });
}

export function login(input: { email: string; password: string }) {
  return request<AuthResponse>("/auth/login", {
    method: "POST",
    body: JSON.stringify(input),
  });
}

export function getOrganization(orgID: string, token: string) {
  return request<Organization>(`/organizations/${orgID}`, { token });
}

export function updateOrganization(
  orgID: string,
  token: string,
  input: {
    name?: string;
    description?: string;
    default_execution_providers?: ExecutionProviderConfig[];
    allow_agent_web_search?: boolean;
  },
) {
  return request<Organization>(`/organizations/${orgID}`, {
    method: "PATCH",
    token,
    body: JSON.stringify(input),
  });
}

export function refresh(input: { refresh_token: string }) {
  return request<AuthResponse>("/auth/refresh", {
    method: "POST",
    body: JSON.stringify(input),
  });
}
