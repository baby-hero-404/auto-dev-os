"use client";

import { useMemo, useState } from "react";
import useSWR from "swr";
import { Bot, Cpu, FolderGit, GitPullRequest, Plus, ShieldCheck, Sparkles, Workflow } from "lucide-react";
import { Toaster } from "sonner";
import { api } from "@/lib/api";
import { useSession } from "@/lib/session";
import { useAuthedSWR } from "@/lib/use-authed-swr";
import type { Project } from "@/lib/types";
import { DashboardLayout } from "@/components/dashboard/dashboard-layout";
import { StatsCards } from "@/components/dashboard/stats-cards";
import { SetupChecklist } from "@/components/dashboard/setup-checklist";
import { ProjectCard, ProjectCardsSkeleton } from "@/components/dashboard/home/project-card";
import { CreateProjectModal } from "@/components/dashboard/home/create-project-modal";

export default function Home() {
  const session = useSession();
  const [showModal, setShowModal] = useState(false);

  const token = session?.token ?? "";
  const orgID = session?.user.org_id ?? "";

  const { data: projects = [], mutate, error, isLoading: isProjectsLoading } = useSWR(
    session ? ["projects", orgID] : null,
    () => api.listProjects(orgID, token),
  );

  const { data: overview } = useAuthedSWR(
    orgID ? ["analytics-overview", orgID] : null,
    (t) => api.analyticsOverview(t, orgID),
  );

  const { data: gitAccounts = [] } = useAuthedSWR(
    orgID ? ["git-accounts", orgID] : null,
    (t) => api.listGitAccounts(orgID, t),
  );

  const stats = useMemo(
    () => [
      {
        label: "Projects",
        value: (overview?.total_projects ?? projects.length).toString(),
        detail: "Repository workspaces",
        icon: FolderGit,
      },
      {
        label: "Active Tasks",
        value: (overview?.active_tasks ?? 0).toString(),
        detail: "Analyze, code, review, test",
        icon: Workflow,
      },
      {
        label: "Running Agents",
        value: (overview?.running_agents ?? 0).toString(),
        detail: "Role-based execution",
        icon: Bot,
      },
      {
        label: "Open PRs",
        value: (overview?.open_prs ?? 0).toString(),
        detail: "Ready for human review",
        icon: GitPullRequest,
      },
    ],
    [projects.length, overview],
  );

  function openProjectModal() {
    setShowModal(true);
  }

  return (
    <DashboardLayout>
      <div className="mx-auto w-full max-w-6xl space-y-6">
        <Toaster richColors position="top-right" />
        <DashboardHero />
        <SetupChecklist />
        <StatsCards stats={stats} />

        <div className="flex flex-col justify-between gap-4 sm:flex-row sm:items-end">
          <div>
            <h2 className="font-mono text-2xl font-semibold">Projects</h2>
            <p className="mt-1 text-sm text-content-muted">
              Link repositories, create tasks, and configure agents for execution.
            </p>
          </div>
          {projects.length > 0 && (
            <button
              onClick={openProjectModal}
              className="flex items-center justify-center gap-2 rounded-md bg-brand-primary px-4 py-2.5 text-sm font-semibold text-white transition hover:opacity-90 cursor-pointer shadow-[0_0_15px_rgba(34,197,94,0.2)] self-start sm:self-auto"
              type="button"
            >
              <Plus size={16} />
              New Project
            </button>
          )}
        </div>

        {error && (
          <p className="rounded-md border border-red-400/40 bg-red-950/40 p-3 text-sm text-red-100">
            {error.message}
          </p>
        )}

        {isProjectsLoading ? (
          <ProjectCardsSkeleton />
        ) : projects.length === 0 ? (
          <div className="flex flex-col items-center justify-center rounded-lg border border-dashed border-stroke bg-card p-12 text-center">
            <div className="grid size-12 place-items-center rounded-xl bg-surface text-brand-primary">
              <FolderGit size={24} />
            </div>
            <h3 className="mt-4 font-semibold text-foreground">No projects yet.</h3>
            <p className="mt-2 max-w-sm text-sm text-content-muted">
              Create your first project to start running AI tasks.
            </p>
            <button
              onClick={openProjectModal}
              className="mt-5 flex items-center justify-center gap-2 rounded-md bg-brand-primary px-4 py-2 text-sm font-semibold text-white transition hover:opacity-90 cursor-pointer"
              type="button"
            >
              <Plus size={16} />
              Create Project
            </button>
          </div>
        ) : (
          <div className="grid gap-4 md:grid-cols-2 xl:grid-cols-3">
            {projects.map((project: Project) => (
              <ProjectCard key={project.id} project={project} token={token} />
            ))}
          </div>
        )}
      </div>

      <CreateProjectModal
        isOpen={showModal}
        onClose={() => setShowModal(false)}
        gitAccounts={gitAccounts}
        token={token}
        orgID={orgID}
        onProjectCreated={async () => {
          await mutate();
        }}
      />
    </DashboardLayout>
  );
}

function DashboardHero() {
  const capabilities = [
    { label: "Gateway", detail: "Provider routing", icon: Cpu },
    { label: "Rules", detail: "Org guardrails", icon: ShieldCheck },
    { label: "Skills", detail: "JIT context", icon: Sparkles },
    { label: "Workflow", detail: "Review gates", icon: Workflow },
  ];

  return (
    <section className="rounded-lg border border-stroke bg-card p-5">
      <div className="flex flex-col gap-5 lg:flex-row lg:items-center lg:justify-between">
        <div className="max-w-2xl">
          <div className="mb-2 inline-flex items-center gap-2 rounded-md border border-stroke bg-surface px-2.5 py-1 text-xs font-medium text-content-muted">
            <Workflow size={14} className="text-brand-primary" />
            AI-native SDLC control plane
          </div>
          <h1 className="text-2xl font-semibold tracking-tight text-foreground sm:text-3xl">
            Coordinate tasks from spec to merged code.
          </h1>
          <p className="mt-2 max-w-xl text-sm text-content-muted">
            Plan work, enforce rules, assign agents, route models through the gateway, and keep PRs visible for final review.
          </p>
        </div>
      </div>

      <div className="mt-5 grid gap-2 sm:grid-cols-2 lg:grid-cols-4">
        {capabilities.map((item) => (
          <div key={item.label} className="rounded-md border border-stroke bg-background/50 p-3">
            <div className="flex items-center gap-2 text-sm font-semibold text-foreground">
              <item.icon size={15} className="text-brand-primary" />
              {item.label}
            </div>
            <div className="mt-1 text-xs text-content-muted">{item.detail}</div>
          </div>
        ))}
      </div>
    </section>
  );
}
