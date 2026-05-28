"use client";

export function StatsCards({ stats }: { stats: { label: string; value: string }[] }) {
  return (
    <div className="mb-6 grid gap-3 sm:grid-cols-2 xl:grid-cols-4">
      {stats.map((stat) => (
        <div
          key={stat.label}
          className="group rounded-lg border border-[var(--border)] bg-[var(--primary)] p-4 transition hover:border-[var(--accent)]/40"
        >
          <div className="text-sm text-[var(--muted)]">{stat.label}</div>
          <div className="mt-2 font-mono text-3xl font-semibold transition group-hover:text-[var(--accent)]">
            {stat.value}
          </div>
        </div>
      ))}
    </div>
  );
}
