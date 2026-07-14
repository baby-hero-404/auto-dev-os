import * as React from "react";
import { cn } from "@/lib/cn";

export interface FieldProps {
  label?: string;
  htmlFor?: string;
  error?: string;
  hint?: string;
  children: React.ReactNode;
  className?: string;
}

export function Field({
  label,
  htmlFor,
  error,
  hint,
  children,
  className,
}: FieldProps) {
  return (
    <div className={cn("flex flex-col gap-1.5 w-full", className)}>
      {label && (
        <label
          htmlFor={htmlFor}
          className="text-[10px] font-mono uppercase tracking-wider text-muted font-semibold"
        >
          {label}
        </label>
      )}
      {children}
      {hint && !error && (
        <span className="text-xs text-muted leading-normal">{hint}</span>
      )}
      {error && (
        <span className="text-xs text-danger leading-normal font-medium">{error}</span>
      )}
    </div>
  );
}
