"use client";

import { useMemo, useState } from "react";
import useSWR from "swr";
import { Bot, FolderGit, GitPullRequest, Plus, Workflow } from "lucide-react";
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
import { Button } from "@/components/ui/button";
import { EmptyState } from "@/components/ui/empty-state";

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
        <SetupChecklist onCreateProjectClick={openProjectModal} />
        <StatsCards stats={stats} />

        <div className="flex flex-col justify-between gap-4 sm:flex-row sm:items-end">
          <div>
            <h2 className="font-mono text-2xl font-semibold">Projects</h2>
            <p className="mt-1 text-sm text-content-muted">
              Link repositories, create tasks, and configure agents for execution.
            </p>
          </div>
          {projects.length > 0 && (
            <Button
              onClick={openProjectModal}
              className="self-start sm:self-auto"
            >
              <Plus size={16} />
              New Project
            </Button>
          )}
        </div>

        {error && (
          <p className="rounded-md border border-danger/20 bg-danger/5 p-3 text-sm text-danger">
            {error.message}
          </p>
        )}

        {isProjectsLoading ? (
          <ProjectCardsSkeleton />
        ) : projects.length === 0 ? (
          <EmptyState
            icon={FolderGit}
            title="No projects yet."
            description="Create your first project to start running AI tasks."
            action={
              <Button onClick={openProjectModal}>
                <Plus size={16} />
                Create Project
              </Button>
            }
          />
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