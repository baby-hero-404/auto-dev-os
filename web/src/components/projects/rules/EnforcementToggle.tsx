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
                ? "border-danger/40 bg-danger/10 text-danger font-semibold"
                : "border-warning/40 bg-warning/10 text-warning font-semibold"
              : "border-stroke bg-card text-content-muted hover:text-foreground"
          }`}
          type="button"
        >
          <span className={`size-1.5 rounded-full ${value === opt ? "bg-current animate-pulse" : "bg-stroke"}`} />
          {opt}
        </button>
      ))}
    </div>
  );
}
