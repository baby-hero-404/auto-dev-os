import React from "react";

export type TaskComplexity = "easy" | "medium" | "hard";

interface ComplexitySelectionProps {
  complexity: TaskComplexity;
  onChange: (val: TaskComplexity) => void;
  disabled?: boolean;
}

export function ComplexitySelection({
  complexity,
  onChange,
  disabled,
}: ComplexitySelectionProps) {
  const options = ["easy", "medium", "hard"] as const;

  const activeColors = {
    easy: "border-emerald-500/40 bg-emerald-500/10 text-emerald-600 dark:text-emerald-400 font-semibold shadow-sm",
    medium: "border-amber-500/40 bg-amber-500/10 text-amber-600 dark:text-amber-400 font-semibold shadow-sm",
    hard: "border-rose-500/40 bg-rose-500/10 text-rose-600 dark:text-rose-400 font-semibold shadow-sm",
  };

  const dotColors = {
    easy: "bg-emerald-500",
    medium: "bg-amber-500",
    hard: "bg-rose-500",
  };

  return (
    <div className="grid grid-cols-3 gap-2" role="radiogroup" aria-label="Task complexity">
      {options.map((option) => {
        const isSelected = complexity === option;
        return (
          <button
            key={option}
            onClick={() => onChange(option)}
            role="radio"
            aria-checked={isSelected}
            className={`flex items-center justify-center gap-1.5 rounded-lg border py-2 text-xs capitalize transition-all duration-200 cursor-pointer focus:outline-none focus:ring-1 focus:ring-stroke-focus ${
              isSelected
                ? activeColors[option]
                : "border-stroke bg-surface text-content-muted hover:border-stroke-focus hover:text-foreground hover:bg-surface/50"
            }`}
            disabled={disabled}
            type="button"
          >
            <span className={`h-1.5 w-1.5 rounded-full ${dotColors[option]}`} />
            {option}
          </button>
        );
      })}
    </div>
  );
}
