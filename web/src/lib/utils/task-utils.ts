import type { Task, Agent } from "@/lib/types";

export function isActiveTask(task: Task) {
  return ["analyzing", "running", "assigned", "planning", "coding", "reviewing", "fixing", "testing", "in_progress"].includes(task.status);
}

export function agentName(task: Task, agents: Agent[]) {
  if (!task.agent_id) return "Auto-assign";
  return agents.find((a) => a.id === task.agent_id)?.name ?? task.agent_id.slice(0, 8);
}

export function timeAgo(value: string) {
  const ts = new Date(value).getTime();
  if (Number.isNaN(ts)) return "recently";
  const s = Math.max(0, Math.floor((Date.now() - ts) / 1000));
  if (s < 60) return `${s}s ago`;
  const m = Math.floor(s / 60);
  if (m < 60) return `${m}m ago`;
  const h = Math.floor(m / 60);
  if (h < 24) return `${h}h ago`;
  return `${Math.floor(h / 24)}d ago`;
}
