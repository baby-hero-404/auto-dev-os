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

import { TASK_STATUS_BADGES } from "@/lib/status";

export const STATUS_COLORS: Record<string, string> = Object.fromEntries(
  Object.entries(TASK_STATUS_BADGES).map(([status, badge]) => [
    status,
    badge.chartColor || "#64748b"
  ])
);
