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
    <div className="rounded-lg border border-dashed border-[var(--border)] bg-[var(--primary)]/50 p-8 text-center">
      <Icon size={32} className="mx-auto mb-3 text-[var(--muted)]" />
      <h3 className="font-mono font-semibold">{title}</h3>
      <p className="mt-2 text-sm text-[var(--muted)]">{description}</p>
    </div>
  );
}
