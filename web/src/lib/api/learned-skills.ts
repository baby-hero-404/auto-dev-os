import { request } from "./client";
import type { LearnedSkill } from "../types";

export function listByProject(projectID: string, token: string) {
  return request<{ skills: LearnedSkill[] }>(`/projects/${projectID}/learned-skills`, { token })
    .then((res) => res.skills || []);
}

export function update(skillID: string, token: string, input: { status?: string; title?: string; content?: string; trigger_keywords?: string[] }) {
  return request<{ skill: LearnedSkill }>(`/learned-skills/${skillID}`, {
    method: "PATCH",
    token,
    body: JSON.stringify(input),
  }).then((res) => res.skill);
}

export function remove(skillID: string, token: string) {
  return request<void>(`/learned-skills/${skillID}`, {
    method: "DELETE",
    token,
  });
}
