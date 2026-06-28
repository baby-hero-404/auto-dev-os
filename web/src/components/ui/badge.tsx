"use client";

const colorMap: Record<string, string> = {
  easy: "bg-emerald-400/10 text-emerald-300 border-emerald-400/20",
  medium: "bg-amber-400/10 text-amber-300 border-amber-400/20",
  hard: "bg-red-400/10 text-red-300 border-red-400/20",
  todo: "bg-panel text-content-muted border-stroke",
  context_loading: "bg-indigo-400/10 text-indigo-300 border-indigo-400/20",
  analyzing: "bg-blue-400/10 text-blue-300 border-blue-400/20",
  spec_review: "bg-purple-400/10 text-purple-300 border-purple-400/20",
  planning: "bg-indigo-400/10 text-indigo-300 border-indigo-400/20",
  coding: "bg-cyan-400/10 text-cyan-300 border-cyan-400/20",
  reviewing: "bg-violet-400/10 text-violet-300 border-violet-400/20",
  fixing: "bg-orange-400/10 text-orange-300 border-orange-400/20",
  testing: "bg-teal-400/10 text-teal-300 border-teal-400/20",
  pr_ready: "bg-purple-400/10 text-purple-300 border-purple-400/20",
  human_review: "bg-yellow-400/10 text-yellow-300 border-yellow-400/20",
  merged: "bg-emerald-400/10 text-emerald-300 border-emerald-400/20",
  none: "bg-panel text-content-muted border-stroke",
  draft: "bg-blue-400/10 text-blue-300 border-blue-400/20",
  pending_review: "bg-amber-400/10 text-amber-300 border-amber-400/20",
  changes_requested: "bg-orange-400/10 text-orange-300 border-orange-400/20",
  approved: "bg-emerald-400/10 text-emerald-300 border-emerald-400/20",
  auto_approved: "bg-emerald-400/10 text-emerald-300 border-emerald-400/20",
  strict: "bg-emerald-400/10 text-emerald-300",
  advisory: "bg-amber-400/10 text-amber-300",
  active: "bg-emerald-400/10 text-emerald-300",
  idle: "bg-panel text-content-muted border-stroke",
  busy: "bg-cyan-400/10 text-cyan-300 border-cyan-400/20",
  assigned: "bg-blue-400/10 text-blue-300 border-blue-400/20",
  running: "bg-amber-400/10 text-amber-300 border-amber-400/20",
  offline: "bg-panel text-content-muted border-stroke",
};

const fallback = "bg-panel text-content";

export function Badge({ value }: { value: string | number }) {
  const strVal = String(value);
  const color = colorMap[strVal] ?? fallback;
  return (
    <span className={`inline-flex rounded border px-2 py-0.5 text-xs font-medium ${color}`}>
      {strVal.replaceAll("_", " ")}
    </span>
  );
}
