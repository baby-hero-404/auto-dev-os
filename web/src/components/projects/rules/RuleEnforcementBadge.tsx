import type { Rule } from "@/lib/types";

interface RuleEnforcementBadgeProps {
  enforcement: Rule["enforcement"];
}

export function RuleEnforcementBadge({ enforcement }: RuleEnforcementBadgeProps) {
  const cls =
    enforcement === "strict"
      ? "border-rose-500/20 bg-rose-500/10 text-rose-600 dark:text-rose-400"
      : "border-amber-500/20 bg-amber-500/10 text-amber-600 dark:text-amber-400";
  return (
    <span className={`rounded border px-2 py-0.5 font-mono text-[9px] font-bold uppercase tracking-wider ${cls}`}>
      {enforcement}
    </span>
  );
}
