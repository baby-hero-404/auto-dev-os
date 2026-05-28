"use client";

import { Settings, Server, Shield, Palette } from "lucide-react";
import { DashboardLayout } from "@/components/dashboard/dashboard-layout";

const settingsCategories = [
  {
    icon: Server,
    title: "API Configuration",
    description: "Configure the backend API URL, LLM provider, and model selection.",
    status: "Active",
  },
  {
    icon: Shield,
    title: "Security",
    description: "Manage JWT secrets, webhook tokens, and API key rotation.",
    status: "Active",
  },
  {
    icon: Palette,
    title: "Appearance",
    description: "Theme preferences, dark mode settings, and font configuration.",
    status: "Default",
  },
  {
    icon: Settings,
    title: "Agent Defaults",
    description: "Default agent configuration: provider, model, token limits, and retry policies.",
    status: "Phase 4",
  },
];

export default function SettingsPage() {
  return (
    <DashboardLayout>
      <div className="mb-5">
        <h2 className="font-mono text-2xl font-semibold">Settings</h2>
        <p className="mt-1 text-sm text-[var(--muted)]">
          Platform configuration and preferences.
        </p>
      </div>

      <div className="grid gap-4 md:grid-cols-2">
        {settingsCategories.map((category) => {
          const Icon = category.icon;
          return (
            <div
              key={category.title}
              className="rounded-lg border border-[var(--border)] bg-[var(--primary)] p-5 transition hover:border-[var(--accent)]/40"
            >
              <div className="mb-3 flex items-center justify-between">
                <div className="flex items-center gap-2">
                  <Icon size={18} className="text-[var(--accent)]" />
                  <h3 className="font-mono font-semibold">{category.title}</h3>
                </div>
                <span className="rounded bg-slate-800 px-2 py-1 text-xs text-slate-200">
                  {category.status}
                </span>
              </div>
              <p className="text-sm text-[var(--muted)]">{category.description}</p>
            </div>
          );
        })}
      </div>
    </DashboardLayout>
  );
}
