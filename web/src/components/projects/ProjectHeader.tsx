"use client";

import Link from "next/link";
import { ChevronLeft, Copy, Plus } from "lucide-react";
import { Button } from "@/components/ui/button";
import { toast } from "sonner";

type ProjectView = "tasks" | "agents" | "repositories" | "rules" | "settings";

const projectViews: { id: ProjectView; label: string }[] = [
  { id: "tasks", label: "Tasks" },
  { id: "agents", label: "Agents" },
  { id: "repositories", label: "Repositories" },
  { id: "rules", label: "Rules" },
  { id: "settings", label: "Settings" },
];

interface ProjectHeaderProps {
  projectName: string;
  projectID: string;
  activeView: ProjectView;
  onCreateTaskClick: () => void;
}

export function ProjectHeader({
  projectName,
  projectID,
  activeView,
  onCreateTaskClick,
}: ProjectHeaderProps) {
  const handleCopyID = () => {
    navigator.clipboard.writeText(projectID);
    toast.success("Project ID copied to clipboard!");
  };

  return (
    <header className="shrink-0 border-b border-stroke bg-card/95 px-5 py-4 shadow-sm md:px-6">
      <div className="flex flex-col gap-4 xl:flex-row xl:items-start xl:justify-between">
        <div className="min-w-0">
          {/* Row 1: Breadcrumb / Mobile back */}
          <div className="flex items-center gap-1.5 text-xs text-content-muted">
            <Link href="/" className="hover:text-foreground transition-colors flex items-center gap-1">
              <ChevronLeft size={14} className="md:hidden" />
              <span>Projects</span>
            </Link>
            <span className="hidden md:inline">/</span>
            <span className="truncate font-semibold text-foreground hidden md:inline">{projectName || "Loading..."}</span>
          </div>

          {/* Row 2: Title & Copyable ID */}
          <div className="mt-2 flex flex-wrap items-center gap-2">
            <h1 className="truncate text-2xl font-semibold tracking-tight text-foreground">
              {projectName || "Project workspace"}
            </h1>
            <button
              onClick={handleCopyID}
              title="Copy Project ID"
              className="flex items-center gap-1 rounded-md border border-stroke bg-surface px-2 py-0.5 font-mono text-[11px] text-content-muted hover:bg-stroke hover:text-foreground transition cursor-pointer"
            >
              <span>{projectID.slice(0, 8)}</span>
              <Copy size={10} />
            </button>
          </div>
        </div>

        {/* Row 2: Right Action Button */}
        <div className="flex shrink-0 flex-wrap items-center gap-3">
          <Button
            onClick={onCreateTaskClick}
            size="sm"
          >
            <Plus size={15} /> Create Task
          </Button>
        </div>
      </div>

      {/* Row 3 (md:hidden): horizontal scrollable link tab strip */}
      <div className="mt-4 overflow-x-auto pb-1 md:hidden">
        <div className="flex border-b border-stroke min-w-max">
          {projectViews.map((view) => {
            const isActive = activeView === view.id;
            return (
              <Link
                key={view.id}
                href={`/projects/${projectID}?view=${view.id}`}
                className={`px-4 py-2 text-sm font-medium transition ${
                  isActive
                    ? "text-brand-primary border-b-2 border-brand-primary font-semibold"
                    : "text-content-muted hover:text-foreground"
                }`}
              >
                {view.label}
              </Link>
            );
          })}
        </div>
      </div>
    </header>
  );
}
