export function compactNumber(value: number) {
  return new Intl.NumberFormat("en-US", { notation: "compact", maximumFractionDigits: 1 }).format(value);
}

export function formatCost(value: number) {
  return new Intl.NumberFormat("en-US", { style: "currency", currency: "USD", maximumFractionDigits: 2 }).format(value);
}

export function formatDuration(ms: number) {
  if (ms < 60_000) return `${Math.round(ms / 1000)}s`;
  return `${Math.round(ms / 60_000)}m`;
}

export function formatLatency(ms: number) {
  if (ms < 1000) return `${Math.round(ms)}ms`;
  return `${(ms / 1000).toFixed(1)}s`;
}

export const STATUS_COLORS: Record<string, string> = {
  todo: "#64748b",
  context_loading: "#3b82f6",
  analyzing: "#f59e0b",
  planning: "#818cf8",
  spec_review: "#a78bfa",
  assigned: "#38bdf8",
  coding: "#22c55e",
  reviewing: "#06b6d4",
  fixing: "#fb923c",
  testing: "#14b8a6",
  pr_ready: "#10b981",
  human_review: "#e879f9",
  merged: "#34d399",
  in_progress: "#60a5fa",
  failed: "#ef4444",
  completed: "#22c55e",
};
