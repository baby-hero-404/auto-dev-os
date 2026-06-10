"use client";

import { Wrench, ShieldCheck, AlertTriangle } from "lucide-react";
import { DashboardLayout } from "@/components/dashboard/dashboard-layout";

export default function RulesPage() {
  // Rules are per-project — this page shows global guidance.
  return (
    <DashboardLayout>
      <div className="mb-5">
        <h2 className="font-mono text-2xl font-semibold">Rules</h2>
        <p className="mt-1 text-sm text-content-muted">
          Behavioral constraints and coding standards for AI agents.
        </p>
      </div>

      <div className="grid gap-4 md:grid-cols-2">
        <RuleCard
          icon={ShieldCheck}
          title="Strict Rules"
          description="Agents must never violate these. Includes security constraints, testing mandates, and safety guidelines."
          badge="Enforced"
          badgeColor="emerald"
        />
        <RuleCard
          icon={AlertTriangle}
          title="Advisory Rules"
          description="Best-practice recommendations. Agents follow these unless overridden by task-specific context."
          badge="Advisory"
          badgeColor="amber"
        />
      </div>

      <div className="mt-6 rounded-lg border border-stroke bg-panel p-5">
        <div className="mb-3 flex items-center gap-2">
          <Wrench size={18} className="text-brand-primary" />
          <h3 className="font-mono font-semibold">Default Rules (seeded per project)</h3>
        </div>
        <ul className="space-y-3 text-sm text-slate-300">
          <li className="flex items-start gap-3 rounded-md border border-stroke bg-slate-950 p-3">
            <span className="mt-0.5 rounded bg-emerald-400/10 px-2 py-0.5 text-xs text-emerald-300">strict</span>
            <span>Follow clean code principles: self-documenting code, meaningful variable names, small focused functions.</span>
          </li>
          <li className="flex items-start gap-3 rounded-md border border-stroke bg-slate-950 p-3">
            <span className="mt-0.5 rounded bg-emerald-400/10 px-2 py-0.5 text-xs text-emerald-300">strict</span>
            <span>All code changes must include tests. No PR may be merged without passing CI.</span>
          </li>
          <li className="flex items-start gap-3 rounded-md border border-stroke bg-slate-950 p-3">
            <span className="mt-0.5 rounded bg-amber-400/10 px-2 py-0.5 text-xs text-amber-300">advisory</span>
            <span>Use conventional commit messages: feat:, fix:, docs:, refactor:, test:, chore:.</span>
          </li>
          <li className="flex items-start gap-3 rounded-md border border-stroke bg-slate-950 p-3">
            <span className="mt-0.5 rounded bg-emerald-400/10 px-2 py-0.5 text-xs text-emerald-300">strict</span>
            <span>Security first: never log secrets, validate all inputs, use parameterized queries.</span>
          </li>
          <li className="flex items-start gap-3 rounded-md border border-stroke bg-slate-950 p-3">
            <span className="mt-0.5 rounded bg-amber-400/10 px-2 py-0.5 text-xs text-amber-300">advisory</span>
            <span>Document architectural decisions in ADRs. Update ARCHITECTURE.md when adding new packages.</span>
          </li>
        </ul>
      </div>
    </DashboardLayout>
  );
}

function RuleCard({
  icon: Icon,
  title,
  description,
  badge,
  badgeColor,
}: {
  icon: React.ComponentType<{ size?: number }>;
  title: string;
  description: string;
  badge: string;
  badgeColor: "emerald" | "amber";
}) {
  const colorMap = {
    emerald: "bg-emerald-400/10 text-emerald-300",
    amber: "bg-amber-400/10 text-amber-300",
  };
  return (
    <div className="rounded-lg border border-stroke bg-panel p-5">
      <div className="mb-3 flex items-center justify-between">
        <div className="flex items-center gap-2">
          <Icon size={18} />
          <h3 className="font-mono font-semibold">{title}</h3>
        </div>
        <span className={`rounded px-2 py-1 text-xs ${colorMap[badgeColor]}`}>{badge}</span>
      </div>
      <p className="text-sm text-content-muted">{description}</p>
    </div>
  );
}
