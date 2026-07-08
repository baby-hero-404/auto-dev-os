"use client";

import Link from "next/link";
import { CheckCircle2, Circle } from "lucide-react";
import type { CheckItem } from "./types";

interface ChecklistItemProps {
  check: CheckItem;
  isAnimating: boolean;
}

export function ChecklistItem({ check, isAnimating }: ChecklistItemProps) {
  const Icon = check.icon;

  const handleClick = (e: React.MouseEvent) => {
    if (check.onClick) {
      e.preventDefault();
      check.onClick();
    }
  };

  return (
    <Link
      href={check.href}
      onClick={handleClick}
      className={`group flex items-center gap-3 rounded-lg px-3 py-2.5 transition hover:bg-surface ${
        check.done ? "opacity-70" : ""
      }`}
    >
      {/* Status indicator */}
      <span className={isAnimating ? "animate-completion-pop" : ""}>
        {check.done ? (
          <CheckCircle2
            size={18}
            className={check.required ? "text-success" : "text-warning"}
          />
        ) : (
          <Circle size={18} className="text-content-muted" />
        )}
      </span>

      {/* Icon */}
      <span className="grid size-7 shrink-0 place-items-center rounded-md bg-surface">
        <Icon size={14} className="text-content-muted group-hover:text-brand-primary transition" />
      </span>

      {/* Label */}
      <span
        className={`flex-1 text-sm transition ${
          check.done
            ? "text-content-muted line-through decoration-content-muted/40"
            : "text-foreground group-hover:text-brand-primary"
        }`}
      >
        {check.label}
        {!check.required && (
          <span className="ml-1.5 text-[10px] font-semibold uppercase tracking-wider text-warning">
            recommended
          </span>
        )}
      </span>

      {/* Link hint */}
      {!check.done && (
        <span className="hidden text-xs text-content-muted group-hover:text-brand-primary sm:inline transition">
          {check.hrefLabel} →
        </span>
      )}
    </Link>
  );
}
