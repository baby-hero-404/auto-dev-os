import * as React from "react";
import { cn } from "@/lib/cn";
import { getTaskStatusBadge, getSpecStatusBadge } from "@/lib/status";
import type { TaskStatus, TaskSpecStatus } from "@/lib/types";

export interface BadgeProps {
  children?: React.ReactNode;
  value?: string | number;
  variant?: "neutral" | "accent" | "success" | "warning" | "danger" | "info" | "purple" | "cyan" | "orange" | "violet" | "indigo" | "teal" | "yellow";
  className?: string;
}

export function Badge({
  children,
  value,
  variant = "neutral",
  className,
}: BadgeProps) {
  const baseStyles = "inline-flex items-center rounded border px-2 py-0.5 text-xs font-medium";

  const variants = {
    neutral: "bg-secondary text-foreground border-stroke",
    accent: "bg-brand-primary-muted text-brand-primary border-brand-primary/20",
    success: "bg-success/10 text-success border-success/20",
    warning: "bg-warning/10 text-warning border-warning/20",
    danger: "bg-danger/10 text-danger border-danger/20",
    info: "bg-info/10 text-info border-info/20",
    purple: "bg-purple-500/10 text-purple-600 dark:text-purple-400 border-purple-500/20",
    cyan: "bg-cyan-500/10 text-cyan-600 dark:text-cyan-400 border-cyan-500/20",
    orange: "bg-orange-500/10 text-orange-600 dark:text-orange-400 border-orange-500/20",
    violet: "bg-violet-500/10 text-violet-600 dark:text-violet-400 border-violet-500/20",
    indigo: "bg-indigo-500/10 text-indigo-600 dark:text-indigo-400 border-indigo-500/20",
    teal: "bg-teal-500/10 text-teal-600 dark:text-teal-400 border-teal-500/20",
    yellow: "bg-yellow-500/10 text-yellow-600 dark:text-yellow-400 border-yellow-500/20",
  };

  const content = children ?? (value !== undefined ? String(value).replaceAll("_", " ") : "");

  return (
    <span className={cn(baseStyles, variants[variant], className)}>
      {content}
    </span>
  );
}

export function taskStatusBadge(status: TaskStatus): { variant: "neutral" | "accent" | "success" | "warning" | "danger" | "info" | "purple" | "cyan" | "orange" | "violet" | "indigo" | "teal" | "yellow"; label: string } {
  return getTaskStatusBadge(status);
}

export function prStatusBadge(status: TaskSpecStatus): { variant: "neutral" | "accent" | "success" | "warning" | "danger" | "info" | "purple" | "cyan" | "orange" | "violet" | "indigo" | "teal" | "yellow"; label: string } {
  return getSpecStatusBadge(status);
}

export function ruleEnforcementBadge(enforcement: string): { variant: "neutral" | "accent" | "success" | "warning" | "danger" | "info" | "purple" | "cyan" | "orange" | "violet" | "indigo" | "teal" | "yellow"; label: string } {
  const norm = enforcement.toLowerCase();
  switch (norm) {
    case "strict":
      return { variant: "danger", label: "Strict" };
    case "advisory":
      return { variant: "warning", label: "Advisory" };
    default:
      return { variant: "neutral", label: enforcement };
  }
}

export function agentStatusBadge(status: string): { variant: "neutral" | "accent" | "success" | "warning" | "danger" | "info" | "purple" | "cyan" | "orange" | "violet" | "indigo" | "teal" | "yellow"; label: string } {
  const norm = status.toLowerCase();
  switch (norm) {
    case "active":
      return { variant: "success", label: "Active" };
    case "idle":
      return { variant: "neutral", label: "Idle" };
    case "busy":
      return { variant: "cyan", label: "Busy" };
    case "offline":
      return { variant: "neutral", label: "Offline" };
    default:
      return { variant: "neutral", label: status };
  }
}

export function projectStatusBadge(status: string): { variant: "neutral" | "accent" | "success" | "warning" | "danger" | "info" | "purple" | "cyan" | "orange" | "violet" | "indigo" | "teal" | "yellow"; label: string } {
  const norm = status.toLowerCase();
  switch (norm) {
    case "active":
      return { variant: "success", label: "Active" };
    case "idle":
      return { variant: "neutral", label: "Idle" };
    case "busy":
      return { variant: "cyan", label: "Busy" };
    case "offline":
      return { variant: "neutral", label: "Offline" };
    case "assigned":
      return { variant: "info", label: "Assigned" };
    case "running":
      return { variant: "warning", label: "Running" };
    default:
      return { variant: "neutral", label: status };
  }
}
