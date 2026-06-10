import { request } from "./client";
import type { EpisodicMemory, KnowledgeEdge, LearningSuggestion, MemorySearchResult } from "../types";

export function list(agentID: string, token: string, tier?: string) {
  const params = tier ? `?tier=${tier}` : "";
  return request<{ memories: EpisodicMemory[] }>(`/agents/${agentID}/memories${params}`, { token });
}

export function search(agentID: string, query: string, token: string) {
  const params = new URLSearchParams({ q: query });
  return request<{ results: MemorySearchResult[] }>(`/agents/${agentID}/memories/search?${params}`, { token });
}

export function get(memoryID: string, token: string) {
  return request<{ memory: EpisodicMemory; edges?: KnowledgeEdge[] }>(`/memories/${memoryID}`, { token });
}

export function remove(memoryID: string, token: string) {
  return request<void>(`/memories/${memoryID}`, { method: "DELETE", token });
}

export const suggestions = {
  list(agentID: string, token: string, status?: string) {
    const params = status ? `?status=${status}` : "";
    return request<{ suggestions: LearningSuggestion[] }>(`/agents/${agentID}/suggestions${params}`, { token });
  },
  get(suggestionID: string, token: string) {
    return request<{ suggestion: LearningSuggestion }>(`/suggestions/${suggestionID}`, { token });
  },
  approve(suggestionID: string, token: string) {
    return request<{ suggestion: LearningSuggestion }>(`/suggestions/${suggestionID}/approve`, { method: "POST", token });
  },
  reject(suggestionID: string, token: string, feedback?: string) {
    return request<{ suggestion: LearningSuggestion }>(`/suggestions/${suggestionID}/reject`, {
      method: "POST",
      token,
      body: JSON.stringify({ feedback: feedback ?? "" }),
    });
  },
};
