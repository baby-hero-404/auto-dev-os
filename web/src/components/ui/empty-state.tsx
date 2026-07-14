"use client";

import type { LucideIcon } from "lucide-react";

export function EmptyState({
  icon: Icon,
  title,
  description,
  action,
}: {
  icon: LucideIcon;
  title: string;
  description: string;
  action?: React.ReactNode;
}) {
  return (
    <div className="rounded-lg border border-dashed border-stroke bg-panel/50 p-8 text-center flex flex-col items-center justify-center">
      <Icon size={32} className="mb-3 text-content-muted" />
      <h3 className="font-mono font-semibold">{title}</h3>
      <p className="mt-2 text-sm text-content-muted max-w-sm">{description}</p>
      {action && <div className="mt-5">{action}</div>}
    </div>
  );
}
