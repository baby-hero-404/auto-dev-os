"use client";

import Link from "next/link";
import { ArrowLeft, Bot, GitBranch, List, ShieldCheck, Settings } from "lucide-react";

type ProjectView = "tasks" | "agents" | "repositories" | "rules" | "settings";

interface ProjectSidebarProps {
  projectName: string;
  activeView: ProjectView;
  onViewChange: (view: ProjectView) => void;
  rulesCount: number;
  agentsCount: number;
}

export function ProjectSidebar({
  projectName,
  activeView,
  onViewChange,
  rulesCount,
  agentsCount,
}: ProjectSidebarProps) {
  const menuItems = [
    { id: "tasks" as const, label: "Tasks", icon: List, badge: null },
    { id: "agents" as const, label: "Agents", icon: Bot, badge: agentsCount > 0 ? agentsCount : null },
    { id: "repositories" as const, label: "Repositories", icon: GitBranch, badge: null },
    { id: "rules" as const, label: "Rules", icon: ShieldCheck, badge: rulesCount > 0 ? rulesCount : null },
    { id: "settings" as const, label: "Settings", icon: Settings, badge: null },
  ];

  return (
    <aside className="hidden h-screen w-64 shrink-0 flex-col border-r border-stroke bg-card p-4 md:flex">
      {/* Title / Project Name */}
      <div className="mb-8 px-3 py-2">
        <div className="text-[10px] font-mono font-bold uppercase tracking-widest text-content-muted/60">
          Current Project
        </div>
        <h2 className="mt-1 truncate font-sans text-lg font-bold text-foreground">
          {projectName || "Loading project..."}
        </h2>
      </div>

      {/* Navigation menu */}
      <nav className="flex-1 space-y-1">
        {menuItems.map((item) => {
          const Icon = item.icon;
          const isActive = activeView === item.id;
          return (
            <button
              key={item.id}
              onClick={() => onViewChange(item.id)}
              className={`flex w-full items-center justify-between rounded-lg px-3 py-2.5 text-sm font-medium transition-all duration-150 cursor-pointer ${
                isActive
                  ? "bg-brand-primary/10 text-brand-primary shadow-[inset_3px_0_0_0_var(--accent)] font-semibold"
                  : "text-content-muted hover:bg-surface hover:text-foreground"
              }`}
            >
              <div className="flex items-center gap-2.5">
                <Icon size={16} />
                <span>{item.label}</span>
              </div>
              {item.badge !== null && (
                <span className="rounded bg-background border border-stroke px-1.5 py-0.5 text-[10px] font-semibold text-content-muted">
                  {item.badge}
                </span>
              )}
            </button>
          );
        })}
      </nav>

      {/* Footer / Back */}
      <div className="border-t border-stroke pt-4">
        <Link
          href="/"
          className="flex items-center gap-2 px-3 py-2 text-sm font-medium text-content-muted transition hover:text-foreground"
        >
          <ArrowLeft size={16} />
          <span>Back to Projects</span>
        </Link>
      </div>
    </aside>
  );
}
