"use client";

import Link from "next/link";
import { ArrowLeft, Bot, GitBranch, List, ShieldCheck, Settings } from "lucide-react";
import { cn } from "@/lib/cn";

type ProjectView = "tasks" | "agents" | "repositories" | "rules" | "settings";

interface ProjectSidebarProps {
  projectID: string;
  projectName: string;
  activeView: ProjectView;
  rulesCount: number;
  agentsCount: number;
}

export function ProjectSidebar({
  projectID,
  projectName,
  activeView,
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
      {/* Title / Project Name / Home Link */}
      <div className="mb-8 px-3 py-2">
        <Link
          href="/"
          className="group mb-4 flex items-center gap-2 text-[10px] font-mono font-bold uppercase tracking-widest text-content-muted/60 hover:text-brand-primary transition-colors"
        >
          <span>← Auto Code OS</span>
        </Link>
        <div className="text-[10px] font-mono font-bold uppercase tracking-widest text-content-muted/60">
          Current Project
        </div>
        <h2 className="mt-1 truncate font-sans text-lg font-bold text-foreground">
          {projectName || "Loading project..."}
        </h2>
      </div>

      {/* Navigation menu */}
      <nav className="flex-1 space-y-1">
        {menuItems.map((item, index) => {
          const Icon = item.icon;
          const isActive = activeView === item.id;
          return (
            <Link
              key={item.id}
              href={`/projects/${projectID}?view=${item.id}`}
              aria-current={isActive ? "page" : undefined}
              className={cn(
                "relative flex w-full items-center justify-between rounded-lg px-3 py-2.5 text-sm font-medium transition-all duration-150 cursor-pointer",
                isActive
                  ? "bg-brand-primary-muted text-brand-primary font-semibold"
                  : "text-content-muted hover:bg-surface hover:text-foreground"
              )}
            >
              {isActive && (
                <div className="absolute left-0 top-2 bottom-2 w-0.75 rounded-r bg-brand-primary" />
              )}
              <div className="flex items-center gap-2.5">
                <Icon size={16} />
                <span>{item.label}</span>
              </div>
              <div className="flex items-center gap-1.5">
                {item.badge !== null && (
                  <span className="rounded bg-background border border-stroke px-1.5 py-0.5 text-[10px] font-semibold text-content-muted">
                    {item.badge}
                  </span>
                )}
                <kbd className="hidden rounded bg-stroke/30 px-1 py-0.5 text-[9px] font-mono text-content-muted lg:inline-block">
                  {index + 1}
                </kbd>
              </div>
            </Link>
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
