import type { Rule } from "@/lib/types";

export function RuleEnforcementBadge({ enforcement }: { enforcement: Rule["enforcement"] }) {
  const classes = enforcement === "strict"
    ? "border-rose-500/20 bg-rose-500/5 text-rose-700 dark:border-rose-500/30 dark:bg-rose-500/10 dark:text-rose-300"
    : "border-amber-500/20 bg-amber-500/5 text-amber-700 dark:border-amber-500/30 dark:bg-amber-500/10 dark:text-amber-300";

  return (
    <span className={`rounded-full border px-2.5 py-0.5 font-mono text-[9px] font-bold uppercase tracking-wider shadow-sm ${classes}`}>
      {enforcement}
    </span>
  );
}

export function RulesSkeleton() {
  return (
    <div className="space-y-2">
      {[0, 1, 2].map((i) => (
        <div key={i} className="rounded-lg border border-stroke bg-panel p-4">
          <div className="skeleton-shimmer h-4 w-5/6 rounded" />
          <div className="mt-3 flex gap-2">
            <div className="skeleton-shimmer h-5 w-16 rounded" />
            <div className="skeleton-shimmer h-5 w-14 rounded" />
          </div>
        </div>
      ))}
    </div>
  );
}

export function EnforcementToggle({
  value,
  onChange,
  disabled,
  showDescriptions = false,
}: {
  value: Rule["enforcement"];
  onChange: (val: Rule["enforcement"]) => void;
  disabled?: boolean;
  showDescriptions?: boolean;
}) {
  const options: Array<{ value: Rule["enforcement"]; label: string; description: string }> = [
    {
      value: "strict",
      label: "Strict",
      description: "Required guardrail. Agents must follow it and should refuse work that conflicts with it.",
    },
    {
      value: "advisory",
      label: "Advisory",
      description: "Guidance preference. Agents should follow it when possible, but it can yield to task context.",
    },
  ];

  return (
    <div className={`grid gap-2 ${showDescriptions ? "sm:grid-cols-2" : "grid-cols-2"}`}>
      {options.map((option) => {
        const isSelected = value === option.value;
        const selectedClasses = option.value === "strict"
          ? "border-rose-500/40 bg-rose-500/10 text-rose-700 dark:text-rose-200"
          : "border-amber-500/40 bg-amber-500/10 text-amber-700 dark:text-amber-200";
        return (
          <button
            key={option.value}
            onClick={() => onChange(option.value)}
            disabled={disabled}
            className={`rounded border px-3 py-2 text-left transition cursor-pointer disabled:opacity-50 ${
              isSelected ? selectedClasses : "border-stroke bg-background text-content-muted hover:text-foreground"
            }`}
            type="button"
          >
            <span className="block text-xs font-semibold">{option.label}</span>
            {showDescriptions && (
              <span className="mt-1 block text-xs leading-5 text-content-muted">{option.description}</span>
            )}
          </button>
        );
      })}
    </div>
  );
}
