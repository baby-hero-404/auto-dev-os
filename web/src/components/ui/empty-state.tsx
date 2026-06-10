"use client";

import type { LucideIcon } from "lucide-react";

export function EmptyState({
  icon: Icon,
  title,
  description,
}: {
  icon: LucideIcon;
  title: string;
  description: string;
}) {
  return (
    <div className="rounded-lg border border-dashed border-stroke bg-panel/50 p-8 text-center">
      <Icon size={32} className="mx-auto mb-3 text-content-muted" />
      <h3 className="font-mono font-semibold">{title}</h3>
      <p className="mt-2 text-sm text-content-muted">{description}</p>
    </div>
  );
}
