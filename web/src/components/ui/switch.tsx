import * as React from "react";
import { cn } from "@/lib/cn";

export interface SwitchProps {
  id?: string;
  checked: boolean;
  onChange: (checked: boolean) => void;
  disabled?: boolean;
  className?: string;
  label?: string;
}

export function Switch({
  id,
  checked,
  onChange,
  disabled = false,
  className,
  label,
}: SwitchProps) {
  function handleKeyDown(e: React.KeyboardEvent<HTMLButtonElement>) {
    if (e.key === " " || e.key === "Enter") {
      e.preventDefault();
      if (!disabled) {
        onChange(!checked);
      }
    }
  }

  return (
    <div className="inline-flex items-center gap-3">
      <button
        type="button"
        id={id}
        role="switch"
        aria-checked={checked}
        disabled={disabled}
        onClick={() => !disabled && onChange(!checked)}
        onKeyDown={handleKeyDown}
        className={cn(
          "relative inline-flex h-6 w-11 shrink-0 cursor-pointer rounded-full border-2 border-transparent transition-colors duration-200 ease-in-out focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-brand-primary focus-visible:ring-offset-2 disabled:cursor-not-allowed disabled:opacity-50",
          checked ? "bg-brand-primary" : "bg-slate-300 dark:bg-slate-700",
          className
        )}
      >
        <span
          className={cn(
            "pointer-events-none inline-block h-5 w-5 transform rounded-full bg-white shadow-lg ring-0 transition duration-200 ease-in-out",
            checked ? "translate-x-5" : "translate-x-0"
          )}
        />
      </button>
      {label && (
        <label
          htmlFor={id}
          className={cn(
            "text-sm font-medium text-foreground cursor-pointer select-none",
            disabled && "opacity-50 cursor-not-allowed"
          )}
        >
          {label}
        </label>
      )}
    </div>
  );
}
