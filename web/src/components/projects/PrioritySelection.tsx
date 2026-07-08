import React from "react";

interface PrioritySelectionProps {
  priority: number;
  onChange: (val: number) => void;
  disabled?: boolean;
}

export function PrioritySelection({
  priority,
  onChange,
  disabled,
}: PrioritySelectionProps) {
  const priorities = [
    { label: "Low", value: 1 },
    { label: "Medium", value: 2 },
    { label: "High", value: 3 },
    { label: "Urgent", value: 4 },
  ];

  return (
    <div className="grid grid-cols-4 gap-1.5" role="radiogroup" aria-label="Task priority">
      {priorities.map((option) => {
        const isSelected = priority === option.value;
        return (
          <button
            key={option.value}
            onClick={() => onChange(option.value)}
            role="radio"
            aria-checked={isSelected}
            className={`rounded-lg border py-2 text-xs transition-all duration-200 cursor-pointer focus:outline-none focus:ring-1 focus:ring-stroke-focus font-medium ${
              isSelected
                ? "border-brand-primary/40 bg-brand-primary-muted text-brand-primary font-semibold shadow-sm"
                : "border-stroke bg-surface text-content-muted hover:border-stroke-focus hover:text-foreground hover:bg-surface/50"
            }`}
            disabled={disabled}
            type="button"
          >
            {option.label}
          </button>
        );
      })}
    </div>
  );
}
