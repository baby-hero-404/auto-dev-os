"use client";

import type { LucideIcon } from "lucide-react";

interface StatCard {
  label: string;
  value: string;
  detail?: string;
  icon: LucideIcon;
}

export function StatsCards({ stats }: { stats: StatCard[] }) {
  return (
    <div className="mb-6 grid gap-3 sm:grid-cols-2 xl:grid-cols-4">
      {stats.map((stat) => (
        <div
          key={stat.label}
          className="group min-h-28 rounded-lg border border-stroke bg-card p-4 transition duration-200 hover:border-brand-primary/40"
        >
          <div className="flex items-center justify-between gap-3">
            <div className="text-sm font-medium text-content-muted">{stat.label}</div>
            <div className="grid size-8 place-items-center rounded-md bg-brand-primary-muted text-brand-primary">
              <stat.icon size={16} />
            </div>
          </div>
          <div className="mt-3 font-mono text-3xl font-semibold leading-none transition group-hover:text-brand-primary">
            {stat.value}
          </div>
          {stat.detail && <div className="mt-2 text-xs text-content-muted">{stat.detail}</div>}
        </div>
      ))}
    </div>
  );
}
