import { request } from "./client";
import type { Agent, RoleTemplate, Skill } from "../types";

type AgentInput = {
  name: string;
  role: string;
  goal: string;
  autonomy_level: string;
  context_config?: Record<string, unknown>;
  model_route: string;
  assignment_strategy?: string;
  skill_ids?: string[];
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
  listForAgent(agentID: string, token: string) {
    return request<Skill[]>(`/agents/${agentID}/skills`, { token });
  },
  assignToAgent(agentID: string, skillID: string, token: string) {
    return request<{ status: string }>(`/agents/${agentID}/skills`, {
      method: "POST",
      token,
      body: JSON.stringify({ skill_id: skillID }),
    });
  },
};
