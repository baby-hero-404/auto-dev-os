"use client";

import Link from "next/link";
import { FormEvent, RefObject, useEffect, useMemo, useRef, useState } from "react";
import useSWR from "swr";
import { Bot, CheckCircle2, Clock, FolderGit, GitBranch, Loader2, Plus, X } from "lucide-react";
import { api, ApiError } from "@/lib/api";
import { useSession } from "@/lib/session";
import { useAuthedSWR } from "@/lib/use-authed-swr";
import type { Project, Task } from "@/lib/types";
import { DashboardLayout } from "@/components/dashboard/dashboard-layout";
import { StatsCards } from "@/components/dashboard/stats-cards";

export default function Home() {
  const session = useSession();
  const [showModal, setShowModal] = useState(false);
  const [projectName, setProjectName] = useState("");
  const [projectDescription, setProjectDescription] = useState("");
  const [creationError, setCreationError] = useState("");
  const [isSubmitting, setIsSubmitting] = useState(false);

  const token = session?.token ?? "";
  const orgID = session?.user.org_id ?? "";

  const { data: projects = [], mutate, error } = useSWR(
    session ? ["projects", orgID] : null,
    () => api.listProjects(orgID, token),
  );

  const { data: overview } = useAuthedSWR(
    orgID ? ["analytics-overview", orgID] : null,
    (t) => api.analyticsOverview(t, orgID),
  );

  const stats = useMemo(
    () => [
      { label: "Projects", value: (overview?.total_projects ?? projects.length).toString() },
      { label: "Active Tasks", value: (overview?.active_tasks ?? 0).toString() },
      { label: "Running Agents", value: (overview?.running_agents ?? 0).toString() },
      { label: "Open PRs", value: (overview?.open_prs ?? 0).toString() },
    ],
    [projects.length, overview],
  );

  async function handleCreateProject(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    if (!session) return;
    
    const trimmedName = projectName.trim();
    if (!trimmedName) {
      setCreationError("Project name is required.");
      return;
    }

    setCreationError("");
    setIsSubmitting(true);
    try {
      await api.createProject(orgID, token, {
        name: trimmedName,
        description: projectDescription.trim(),
      });
      setProjectName("");
      setProjectDescription("");
      setShowModal(false);
      mutate();
    } catch (err) {
      setCreationError(err instanceof ApiError ? err.message : "Failed to create project");
    } finally {
      setIsSubmitting(false);
    }
  }

  return (
    <DashboardLayout>
      <StatsCards stats={stats} />

      <div className="mb-6 flex flex-col justify-between gap-4 sm:flex-row sm:items-end">
        <div>
          <h2 className="font-mono text-2xl font-semibold">Projects</h2>
          <p className="mt-1 text-sm text-content-muted">
            Link repositories, create tasks, and configure agents for execution.
          </p>
        </div>
        <button
          onClick={() => {
            setCreationError("");
            setShowModal(true);
          }}
          className="flex items-center justify-center gap-2 rounded-md bg-brand-primary px-4 py-2.5 text-sm font-semibold text-slate-950 transition hover:opacity-90 cursor-pointer shadow-[0_0_15px_rgba(34,197,94,0.2)] self-start sm:self-auto"
          type="button"
        >
          <Plus size={16} />
          New Project
        </button>
      </div>

      {error && (
        <p className="mb-4 rounded-md border border-red-400/40 bg-red-950/40 p-3 text-sm text-red-100">
          {error.message}
        </p>
      )}

      {projects.length === 0 ? (
        <div className="flex flex-col items-center justify-center rounded-lg border border-dashed border-stroke bg-panel p-12 text-center">
          <div className="grid size-12 place-items-center rounded-xl bg-slate-900 text-brand-primary">
            <FolderGit size={24} />
          </div>
          <h3 className="mt-4 font-mono font-semibold">No projects configured</h3>
          <p className="mt-2 max-w-sm text-sm text-content-muted">
            Get started by creating a new project and linking it to a remote Git repository.
          </p>
          <button
            onClick={() => {
              setCreationError("");
              setShowModal(true);
            }}
            className="mt-5 flex items-center justify-center gap-2 rounded-md bg-brand-primary px-4 py-2 text-sm font-semibold text-slate-950 transition hover:opacity-90 cursor-pointer"
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

      {/* Modern Dialog/Modal for project creation */}
      {showModal && (
        <div className="fixed inset-0 z-50 flex items-center justify-center p-4">
          {/* Backdrop Blur overlay */}
          <div
            className="absolute inset-0 bg-slate-950/80 backdrop-blur-sm transition-opacity duration-300"
            onClick={() => {
              if (!isSubmitting) setShowModal(false);
            }}
          />

          {/* Modal Container */}
          <div className="relative w-full max-w-md transform overflow-hidden rounded-xl border border-stroke bg-slate-900 p-6 shadow-2xl transition-all duration-300 animate-[in_0.15s_ease-out]">
            <div className="flex items-center justify-between border-b border-stroke pb-4">
              <h3 className="font-mono text-lg font-semibold text-white">Create New Project</h3>
              <button
                onClick={() => {
                  if (!isSubmitting) setShowModal(false);
                }}
                className="rounded-md p-1 text-content-muted transition hover:bg-slate-800 hover:text-white cursor-pointer"
                disabled={isSubmitting}
                type="button"
              >
                <X size={18} />
              </button>
            </div>

            <form className="mt-4 flex flex-col gap-4" onSubmit={handleCreateProject}>
              <div className="flex flex-col gap-1.5">
                <label className="text-xs font-mono font-bold uppercase tracking-wider text-content-muted">
                  Project Name <span className="text-brand-primary">*</span>
                </label>
                <input
                  value={projectName}
                  onChange={(e) => setProjectName(e.target.value)}
                  placeholder="e.g. e-commerce-backend"
                  className="rounded-md border border-stroke bg-slate-950 px-3 py-2 text-sm text-white focus:outline-none focus:border-brand-primary transition"
                  disabled={isSubmitting}
                  required
                  autoFocus
                />
              </div>

              <div className="flex flex-col gap-1.5">
                <label className="text-xs font-mono font-bold uppercase tracking-wider text-content-muted">
                  Description
                </label>
                <textarea
                  value={projectDescription}
                  onChange={(e) => setProjectDescription(e.target.value)}
                  placeholder="Optional brief details about the project goals, repository scope, or rules."
                  className="min-h-[100px] rounded-md border border-stroke bg-slate-950 px-3 py-2 text-sm text-white focus:outline-none focus:border-brand-primary transition resize-none"
                  disabled={isSubmitting}
                />
              </div>

              {creationError && (
                <p className="rounded-md border border-red-500/20 bg-red-950/40 p-3 text-xs text-red-200">
                  {creationError}
                </p>
              )}

              <div className="mt-2 flex items-center justify-end gap-3 border-t border-stroke pt-4">
                <button
                  onClick={() => setShowModal(false)}
                  className="rounded-md border border-stroke bg-transparent px-4 py-2 text-sm font-semibold text-white transition hover:bg-slate-800 cursor-pointer disabled:opacity-50"
                  disabled={isSubmitting}
                  type="button"
                >
                  Cancel
                </button>
                <button
                  className="flex items-center gap-2 rounded-md bg-brand-primary px-4 py-2 text-sm font-semibold text-slate-950 transition hover:opacity-90 cursor-pointer disabled:opacity-50"
                  disabled={isSubmitting}
                  type="submit"
                >
                  {isSubmitting ? (
                    <>
                      <Loader2 size={16} className="animate-spin" />
                      Creating...
                    </>
                  ) : (
                    "Create Project"
                  )}
                </button>
              </div>
            </form>
          </div>
        </div>
      )}
    </DashboardLayout>
  );
}

function ProjectCard({ project, token }: { project: Project; token: string }) {
  const cardRef = useRef<HTMLAnchorElement>(null);
  const isVisible = useIsNearViewport(cardRef);
  const hasHydratedCounts =
    project.repositories_count !== undefined ||
    project.agents_count !== undefined ||
    project.tasks_total_count !== undefined ||
    project.tasks_done_count !== undefined;
  const { data: meta } = useSWR(
    token && isVisible && !hasHydratedCounts ? ["project-card-meta", project.id] : null,
    async () => {
      const [repositories, agents, tasks] = await Promise.allSettled([
        api.listRepositories(project.id, token),
        api.listAgents(project.id, token),
        api.listTasks(project.id, token),
      ]);

      return {
        repositories: repositories.status === "fulfilled" ? repositories.value : null,
        agents: agents.status === "fulfilled" ? agents.value : null,
        tasks: tasks.status === "fulfilled" ? tasks.value : null,
      };
    },
  );

  const tasks = meta?.tasks;
  const totalTasks = project.tasks_total_count ?? tasks?.length ?? 0;
  const doneTasks = project.tasks_done_count ?? tasks?.filter((task) => isDoneStatus(task.status)).length ?? 0;
  const repositoriesCount = project.repositories_count ?? meta?.repositories?.length;
  const agentsCount = project.agents_count ?? meta?.agents?.length;
  const progress = totalTasks === 0 ? 0 : Math.round((doneTasks / totalTasks) * 100);
  const status = tasks ? deriveProjectStatus(tasks) : deriveHydratedProjectStatus(doneTasks, totalTasks, hasHydratedCounts);
  const lastActivity = tasks ? latestActivity(tasks, project.updated_at) : project.updated_at;

  return (
    <Link
      ref={cardRef}
      href={`/projects/${project.id}`}
      className="group glow-on-hover flex min-h-[230px] flex-col justify-between rounded-lg border border-stroke bg-panel p-5 transition hover:border-brand-primary/40"
    >
      <div>
        <div className="mb-4 flex items-start justify-between gap-3">
          <div className="min-w-0">
            <h3 className="truncate font-mono text-lg font-semibold transition duration-150 group-hover:text-brand-primary">
              {project.name}
            </h3>
            <p className="mt-2 line-clamp-2 text-sm text-content-muted">
              {project.description || "No project description provided."}
            </p>
          </div>
          <StatusBadge status={status} />
        </div>

        <div className="grid grid-cols-3 gap-2 text-xs text-content-muted">
          <CardStat icon={GitBranch} label="Repos" value={repositoriesCount !== undefined ? repositoriesCount.toString() : "--"} />
          <CardStat icon={Bot} label="Agents" value={agentsCount !== undefined ? agentsCount.toString() : "--"} />
          <CardStat icon={CheckCircle2} label="Tasks" value={hasHydratedCounts || tasks ? `${doneTasks}/${totalTasks}` : "--"} />
        </div>
      </div>

      <div className="mt-5">
        <div className="h-1.5 overflow-hidden rounded-full bg-slate-950">
          <div className="h-full rounded-full bg-brand-primary transition-all" style={{ width: `${progress}%` }} />
        </div>
        <div className="mt-3 flex items-center justify-between gap-3 font-mono text-xs text-content-muted">
          <span>{progress}% complete</span>
          <span className="inline-flex min-w-0 items-center gap-1 text-right">
            <Clock size={12} />
            <span className="truncate">{formatRelativeTime(lastActivity)}</span>
          </span>
        </div>
      </div>
    </Link>
  );
}

function deriveHydratedProjectStatus(doneTasks: number, totalTasks: number, hasHydratedCounts: boolean) {
  if (!hasHydratedCounts) return "loading";
  if (totalTasks === 0) return "idle";
  if (doneTasks === totalTasks) return "done";
  return "active";
}

function useIsNearViewport<T extends Element>(ref: RefObject<T | null>) {
  const [isVisible, setIsVisible] = useState(false);

  useEffect(() => {
    if (isVisible) return;
    const node = ref.current;
    if (!node) return;

    const observer = new IntersectionObserver(
      ([entry]) => {
        if (entry.isIntersecting) {
          setIsVisible(true);
          observer.disconnect();
        }
      },
      { rootMargin: "360px 0px" },
    );

    observer.observe(node);
    return () => observer.disconnect();
  }, [isVisible, ref]);

  return isVisible;
}

function CardStat({
  icon: Icon,
  label,
  value,
}: {
  icon: typeof GitBranch;
  label: string;
  value: string;
}) {
  return (
    <div className="rounded-md border border-stroke bg-slate-950/50 p-2">
      <div className="mb-1 flex items-center gap-1.5">
        <Icon size={13} className="text-brand-primary" />
        <span>{label}</span>
      </div>
      <div className="font-mono text-sm font-semibold text-white">{value}</div>
    </div>
  );
}

function StatusBadge({ status }: { status: string }) {
  const styles: Record<string, string> = {
    loading: "border-slate-600/30 bg-slate-800 text-slate-300",
    idle: "border-slate-600/30 bg-slate-800 text-slate-300",
    active: "border-cyan-400/20 bg-cyan-400/10 text-cyan-300",
    blocked: "border-red-400/20 bg-red-400/10 text-red-300",
    done: "border-emerald-400/20 bg-emerald-400/10 text-emerald-300",
  };

  return (
    <span className={`shrink-0 rounded border px-2 py-0.5 text-[10px] font-bold uppercase tracking-wider ${styles[status] || styles.idle}`}>
      {status}
    </span>
  );
}

function deriveProjectStatus(tasks: Task[]) {
  if (tasks.length === 0) return "idle";
  if (tasks.some((task) => ["failed", "blocked", "needs_changes", "changes_requested"].includes(task.status))) {
    return "blocked";
  }
  if (tasks.some((task) => ["running", "in_progress", "approved", "queued", "analyzing", "coding", "reviewing", "testing"].includes(task.status))) {
    return "active";
  }
  if (tasks.every((task) => isDoneStatus(task.status))) return "done";
  return "idle";
}

function isDoneStatus(status: string) {
  return ["done", "completed", "merged"].includes(status);
}

function latestActivity(tasks: Task[], fallback: string) {
  return tasks.reduce((latest, task) => {
    return new Date(task.updated_at).getTime() > new Date(latest).getTime() ? task.updated_at : latest;
  }, fallback);
}

function formatRelativeTime(value: string) {
  const timestamp = new Date(value).getTime();
  if (Number.isNaN(timestamp)) return "No activity";
  const diffMs = Date.now() - timestamp;
  const minute = 60 * 1000;
  const hour = 60 * minute;
  const day = 24 * hour;

  if (diffMs < minute) return "just now";
  if (diffMs < hour) return `${Math.floor(diffMs / minute)}m ago`;
  if (diffMs < day) return `${Math.floor(diffMs / hour)}h ago`;
  return `${Math.floor(diffMs / day)}d ago`;
}
