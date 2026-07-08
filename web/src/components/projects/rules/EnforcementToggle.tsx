import type { Rule } from "@/lib/types";

interface EnforcementToggleProps {
  value: Rule["enforcement"];
  onChange: (v: Rule["enforcement"]) => void;
  disabled?: boolean;
}

export function EnforcementToggle({ value, onChange, disabled }: EnforcementToggleProps) {
  return (
    <div className="flex flex-wrap items-center gap-2">
      <span className="font-mono text-[10px] font-bold uppercase tracking-wider text-content-muted">
        Enforcement
      </span>
      {(["strict", "advisory"] as const).map((opt) => (
        <button
          key={opt}
          onClick={() => onChange(opt)}
          disabled={disabled}
          className={`inline-flex items-center gap-1.5 rounded-md border px-3 py-1.5 text-xs font-semibold transition disabled:opacity-50 cursor-pointer ${
            value === opt
              ? opt === "strict"
                ? "border-rose-500/40 bg-rose-500/10 text-rose-700 dark:text-rose-400 font-semibold"
                : "border-amber-500/40 bg-amber-500/10 text-amber-700 dark:text-amber-400 font-semibold"
              : "border-stroke bg-card text-content-muted hover:text-foreground"
          }`}
          type="button"
        >
          <span className={`size-1.5 rounded-full ${value === opt ? "bg-current animate-pulse" : "bg-slate-300 dark:bg-slate-700"}`} />
          {opt}
        </button>
      ))}
    </div>
  );
}
