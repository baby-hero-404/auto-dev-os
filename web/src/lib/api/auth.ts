import { request } from "./client";
import type { AuthResponse, Organization } from "../types";

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

export function refresh(input: { refresh_token: string }) {
  return request<AuthResponse>("/auth/refresh", {
    method: "POST",
    body: JSON.stringify(input),
  });
}
