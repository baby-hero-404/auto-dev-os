import { request } from "./client";
import type { Agent, RoleTemplate, Skill, SkillSource } from "../types";

type AgentInput = {
  name: string;
  role: string;
  goal: string;
  autonomy_level: string;
  context_config?: Record<string, unknown>;
  model_level_group: string;
  assignment_strategy?: string;
  agent_id?: string;
};

type UpdateAgentInput = Partial<Omit<AgentInput, "agent_id">>;

export function list(projectID: string, token: string) {
  return request<Agent[]>(`/projects/${projectID}/agents`, { token });
}

export function create(projectID: string, token: string, input: AgentInput) {
  return request<Agent>(`/projects/${projectID}/agents`, {
    method: "POST",
    token,
    body: JSON.stringify(input),
  });
}

export function listOrganization(orgID: string, token: string) {
  return request<Agent[]>(`/organizations/${orgID}/agents`, { token });
}

export function hire(orgID: string, token: string, input: Omit<AgentInput, "agent_id">) {
  return request<Agent>(`/organizations/${orgID}/agents`, {
    method: "POST",
    token,
    body: JSON.stringify(input),
  });
}

export function roleTemplates(token: string) {
  return request<RoleTemplate[]>("/role-templates", { token });
}

export function update(agentID: string, token: string, input: UpdateAgentInput) {
  return request<Agent>(`/agents/${agentID}`, {
    method: "PATCH",
    token,
    body: JSON.stringify(input),
  });
}

export function remove(agentID: string, token: string) {
  return request<void>(`/agents/${agentID}`, { method: "DELETE", token });
}

export const skills = {
  list(token: string) {
    return request<Skill[]>("/skills", { token });
  },
  seed(token: string) {
    return request<Skill[]>("/skills/seed", { method: "POST", token });
  },
  listSources(token: string) {
    return request<SkillSource[]>("/skills/sources", { token });
  },
  addSource(token: string, input: { url: string }) {
    return request<SkillSource>("/skills/sources", {
      method: "POST",
      token,
      body: JSON.stringify(input),
    });
  },
  deleteSource(sourceID: string, token: string) {
    return request<void>(`/skills/sources/${sourceID}`, { method: "DELETE", token });
  },
  syncSource(sourceID: string, token: string) {
    return request<SkillSource>(`/skills/sources/${sourceID}/sync`, { method: "POST", token });
  },
  listSourceFiles(sourceID: string, path: string, token: string) {
    return request<Array<{ name: string; path: string; is_dir: boolean; size: number }>>(`/skills/sources/${sourceID}/files?path=${encodeURIComponent(path)}`, { token });
  },
  getSourceFileContent(sourceID: string, path: string, token: string) {
    return request<{ content: string; path: string }>(`/skills/sources/${sourceID}/file-content?path=${encodeURIComponent(path)}`, { token });
  },
};
