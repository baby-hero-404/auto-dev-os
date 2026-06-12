"use client";

import Link from "next/link";
import { useRouter } from "next/navigation";
import { FormEvent, RefObject, useEffect, useMemo, useRef, useState } from "react";
import useSWR from "swr";
import { ArrowLeft, Bot, CheckCircle2, Clock, FolderGit, GitBranch, Loader2, Plus, RefreshCw, X } from "lucide-react";
import { Toaster, toast } from "sonner";
import { api, ApiError } from "@/lib/api";
import { useSession } from "@/lib/session";
import { useAuthedSWR } from "@/lib/use-authed-swr";
import type { Project, Task } from "@/lib/types";
import { DashboardLayout } from "@/components/dashboard/dashboard-layout";
import { StatsCards } from "@/components/dashboard/stats-cards";
import { SetupChecklist } from "@/components/dashboard/setup-checklist";

export default function Home() {
  const session = useSession();
  const router = useRouter();
  const [showModal, setShowModal] = useState(false);
  const [modalStep, setModalStep] = useState<1 | 2>(1);
  const [projectName, setProjectName] = useState("");
  const [projectDescription, setProjectDescription] = useState("");
  const [repoURL, setRepoURL] = useState("");
  const [repoBranch, setRepoBranch] = useState("main");
  const [selectedGitAccountID, setSelectedGitAccountID] = useState("");
  const [creationError, setCreationError] = useState("");
  const [isSubmitting, setIsSubmitting] = useState(false);
  const [fetchedBranches, setFetchedBranches] = useState<string[]>([]);
  const [isFetchingBranches, setIsFetchingBranches] = useState(false);
  const [fetchBranchesError, setFetchBranchesError] = useState("");

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
      { label: "Projects", value: (overview?.total_projects ?? projects.length).toString() },
      { label: "Active Tasks", value: (overview?.active_tasks ?? 0).toString() },
      { label: "Running Agents", value: (overview?.running_agents ?? 0).toString() },
      { label: "Open PRs", value: (overview?.open_prs ?? 0).toString() },
    ],
    [projects.length, overview],
  );

  function resetProjectModal() {
    setModalStep(1);
    setProjectName("");
    setProjectDescription("");
    setRepoURL("");
    setRepoBranch("main");
    setSelectedGitAccountID("");
    setCreationError("");
    setFetchedBranches([]);
    setFetchBranchesError("");
  }

  function openProjectModal() {
    setCreationError("");
    setModalStep(1);
    setShowModal(true);
  }

  function closeProjectModal() {
    if (isSubmitting) return;
    setShowModal(false);
  }

  function handleProjectInfoNext(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    if (!session) return;
    
    const trimmedName = projectName.trim();
    if (!trimmedName) {
      setCreationError("Project name is required.");
      return;
    }

    setCreationError("");
    setModalStep(2);
  }

  async function handleFetchBranches() {
    const trimmedURL = repoURL.trim();
    if (!trimmedURL) return;

    setIsFetchingBranches(true);
    setFetchBranchesError("");
    setFetchedBranches([]);

    try {
      const res = await api.getRemoteBranches(token, {
        url: trimmedURL,
        git_account_id: selectedGitAccountID || undefined,
      });
      setFetchedBranches(res.branches || []);
      if (res.branches && res.branches.length > 0) {
        const hasMain = res.branches.includes("main");
        const hasMaster = res.branches.includes("master");
        setRepoBranch(hasMain ? "main" : (hasMaster ? "master" : res.branches[0]));
      }
    } catch (err: any) {
      setFetchBranchesError(err?.message || "Failed to fetch branches. Check URL/Git Account.");
    } finally {
      setIsFetchingBranches(false);
    }
  }

  async function createProjectWithOptionalRepo(linkRepository: boolean) {
    if (!session) return;

    const trimmedName = projectName.trim();
    if (!trimmedName) {
      setCreationError("Project name is required.");
      setModalStep(1);
      return;
    }
    if (linkRepository && repoURL.trim() && gitAccounts.length === 0) {
      setCreationError("Connect a Git account before linking a repository.");
      return;
    }
    if (linkRepository && repoURL.trim() && !selectedGitAccountID) {
      setCreationError("Select a Git account to link this repository.");
      return;
    }

    setCreationError("");
    setIsSubmitting(true);
    try {
      const createdProject = await api.createProject(orgID, token, {
        name: trimmedName,
        description: projectDescription.trim(),
      });

      if (linkRepository && repoURL.trim()) {
        try {
          await api.createRepository(createdProject.id, token, {
            url: repoURL.trim(),
            provider: "github",
            branch: repoBranch.trim() || "main",
            git_account_id: selectedGitAccountID || undefined,
          });
        } catch {
          toast.error("Project created, but repo could not be linked. You can add it later from the project page.");
        }
      }

      resetProjectModal();
      setShowModal(false);
      await mutate();
      router.push(`/projects/${createdProject.id}`);
    } catch (err) {
      setCreationError(err instanceof ApiError ? err.message : "Failed to create project");
    } finally {
      setIsSubmitting(false);
    }
  }

  return (
    <DashboardLayout>
      <Toaster richColors position="top-right" />
      <SetupChecklist />
      <StatsCards stats={stats} />

      <div className="mb-6 flex flex-col justify-between gap-4 sm:flex-row sm:items-end">
        <div>
          <h2 className="font-mono text-2xl font-semibold">Projects</h2>
          <p className="mt-1 text-sm text-content-muted">
            Link repositories, create tasks, and configure agents for execution.
          </p>
        </div>
        <button
          onClick={openProjectModal}
          className="flex items-center justify-center gap-2 rounded-md bg-brand-primary px-4 py-2.5 text-sm font-semibold text-white transition hover:opacity-90 cursor-pointer shadow-[0_0_15px_rgba(34,197,94,0.2)] self-start sm:self-auto"
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

      {showModal && (
        <div className="fixed inset-0 z-50 flex items-center justify-center p-4">
          <div
            className="absolute inset-0 bg-slate-950/80 backdrop-blur-sm transition-opacity duration-300"
            onClick={closeProjectModal}
          />

          <div className="relative w-full max-w-md transform overflow-hidden rounded-xl border border-stroke bg-card p-6 shadow-2xl transition-all duration-300 animate-modal-in">
            <div className="flex items-center justify-between border-b border-stroke pb-4">
              <div>
                <h3 className="text-lg font-semibold text-foreground">
                  {modalStep === 1 ? "Create New Project" : "Link a Repository"}
                </h3>
                <p className="mt-1 text-xs text-content-muted">Step {modalStep} of 2</p>
              </div>
              <button
                onClick={closeProjectModal}
                className="rounded-md p-1 text-content-muted transition hover:bg-surface hover:text-foreground cursor-pointer"
                disabled={isSubmitting}
                type="button"
              >
                <X size={18} />
              </button>
            </div>

            {modalStep === 1 ? (
              <form className="mt-4 flex flex-col gap-4" onSubmit={handleProjectInfoNext}>
                <div className="flex flex-col gap-1.5">
                  <label className="text-xs font-semibold uppercase tracking-wider text-content-muted">
                    Name <span className="text-brand-primary">*</span>
                  </label>
                  <input
                    value={projectName}
                    onChange={(e) => setProjectName(e.target.value)}
                    placeholder="e.g. api-backend"
                    className="rounded-md border border-stroke bg-background px-3 py-2 text-sm text-foreground focus:outline-none focus:border-brand-primary transition"
                    disabled={isSubmitting}
                    required
                    autoFocus
                  />
                </div>

                <div className="flex flex-col gap-1.5">
                  <label className="text-xs font-semibold uppercase tracking-wider text-content-muted">
                    Description
                  </label>
                  <textarea
                    value={projectDescription}
                    onChange={(e) => setProjectDescription(e.target.value)}
                    placeholder="Optional goal, scope, or repository context."
                    className="min-h-[100px] rounded-md border border-stroke bg-background px-3 py-2 text-sm text-foreground focus:outline-none focus:border-brand-primary transition resize-none"
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
                    onClick={closeProjectModal}
                    className="rounded-md border border-stroke bg-transparent px-4 py-2 text-sm font-semibold text-foreground transition hover:bg-surface cursor-pointer disabled:opacity-50"
                    disabled={isSubmitting}
                    type="button"
                  >
                    Cancel
                  </button>
                  <button
                    className="flex items-center gap-2 rounded-md bg-brand-primary px-4 py-2 text-sm font-semibold text-white transition hover:opacity-90 cursor-pointer disabled:opacity-50"
                    disabled={isSubmitting}
                    type="submit"
                  >
                    Next
                    <ArrowLeft size={16} className="rotate-180" />
                  </button>
                </div>
              </form>
            ) : (
              <form
                className="mt-4 flex flex-col gap-4"
                onSubmit={(event) => {
                  event.preventDefault();
                  createProjectWithOptionalRepo(true);
                }}
              >
                <div className="flex flex-col gap-1.5">
                  <label className="text-xs font-semibold uppercase tracking-wider text-content-muted">Repository URL</label>
                  <input
                    value={repoURL}
                    onChange={(e) => setRepoURL(e.target.value)}
                    placeholder="https://github.com/org/repo.git"
                    className="rounded-md border border-stroke bg-background px-3 py-2 text-sm text-foreground focus:outline-none focus:border-brand-primary transition"
                    disabled={isSubmitting}
                  />
                </div>

                {gitAccounts.length === 0 ? (
                  <div className="rounded-md border border-amber-500/20 bg-amber-500/10 p-3 text-sm text-amber-700 dark:text-amber-200">
                    Connect a Git account before linking a repository.{" "}
                    <Link className="font-semibold underline underline-offset-2" href="/git-accounts">
                      Connect a Git account first
                    </Link>
                  </div>
                ) : (
                  <div className="flex flex-col gap-1.5">
                    <label className="text-xs font-semibold uppercase tracking-wider text-content-muted">Git Account</label>
                    <select
                      value={selectedGitAccountID}
                      onChange={(e) => setSelectedGitAccountID(e.target.value)}
                      className="rounded-md border border-stroke bg-background px-3 py-2 text-sm text-foreground focus:outline-none focus:border-brand-primary transition"
                      disabled={isSubmitting}
                    >
                      <option value="">Select an account</option>
                      {gitAccounts.map((account) => (
                        <option key={account.id} value={account.id}>
                          {account.display_name} ({account.base_url ? "GitHub Enterprise" : "GitHub"})
                        </option>
                      ))}
                    </select>
                  </div>
                )}

                <div className="flex gap-2">
                  <button
                    type="button"
                    onClick={handleFetchBranches}
                    disabled={isFetchingBranches || !repoURL.trim()}
                    className="rounded border border-stroke bg-slate-100 dark:bg-slate-900 px-3 py-2 text-xs font-semibold hover:bg-slate-200 dark:hover:bg-slate-800 disabled:opacity-50 flex items-center gap-1.5 transition cursor-pointer"
                  >
                    {isFetchingBranches ? <Loader2 size={13} className="animate-spin" /> : <RefreshCw size={13} />}
                    Fetch Branches
                  </button>
                </div>

                {fetchBranchesError && (
                  <p className="text-xs text-red-400">{fetchBranchesError}</p>
                )}

                <div className="flex flex-col gap-1.5">
                  <label className="text-xs font-semibold uppercase tracking-wider text-content-muted">Branch</label>
                  {fetchedBranches.length > 0 ? (
                    <select
                      value={repoBranch}
                      onChange={(e) => setRepoBranch(e.target.value)}
                      className="rounded-md border border-stroke bg-background px-3 py-2 text-sm text-foreground focus:outline-none focus:border-brand-primary transition cursor-pointer"
                      disabled={isSubmitting}
                    >
                      {fetchedBranches.map((b) => (
                        <option key={b} value={b}>{b}</option>
                      ))}
                    </select>
                  ) : (
                    <input
                      value={repoBranch}
                      onChange={(e) => setRepoBranch(e.target.value)}
                      placeholder="main"
                      className="rounded-md border border-stroke bg-background px-3 py-2 text-sm text-foreground focus:outline-none focus:border-brand-primary transition"
                      disabled={isSubmitting}
                    />
                  )}
                </div>

                {creationError && (
                  <p className="rounded-md border border-red-500/20 bg-red-950/40 p-3 text-xs text-red-200">
                    {creationError}
                  </p>
                )}

                <div className="mt-2 flex flex-wrap items-center justify-between gap-3 border-t border-stroke pt-4">
                  <button
                    onClick={() => setModalStep(1)}
                    className="inline-flex items-center gap-2 rounded-md border border-stroke bg-transparent px-4 py-2 text-sm font-semibold text-foreground transition hover:bg-surface cursor-pointer disabled:opacity-50"
                    disabled={isSubmitting}
                    type="button"
                  >
                    <ArrowLeft size={16} />
                    Back
                  </button>
                  <div className="flex flex-wrap justify-end gap-3">
                    <button
                      onClick={() => createProjectWithOptionalRepo(false)}
                      className="rounded-md border border-stroke bg-transparent px-4 py-2 text-sm font-semibold text-foreground transition hover:bg-surface cursor-pointer disabled:opacity-50"
                      disabled={isSubmitting}
                      type="button"
                    >
                      Skip for now
                    </button>
                    <button
                      className="flex items-center gap-2 rounded-md bg-brand-primary px-4 py-2 text-sm font-semibold text-white transition hover:opacity-90 cursor-pointer disabled:opacity-50"
                      disabled={isSubmitting || !repoURL.trim() || gitAccounts.length === 0 || !selectedGitAccountID}
                      type="submit"
                    >
                      {isSubmitting ? (
                        <>
                          <Loader2 size={16} className="animate-spin" />
                          Creating...
                        </>
                      ) : (
                        "Create"
                      )}
                    </button>
                  </div>
                </div>
              </form>
            )}
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
      className="group glow-on-hover flex min-h-[230px] flex-col justify-between rounded-lg border border-stroke bg-card p-5 transition hover:border-brand-primary/40"
    >
      <div>
        <div className="mb-4 flex items-start justify-between gap-3">
          <div className="min-w-0">
            <h3 className="truncate font-mono text-lg font-semibold text-foreground transition duration-150 group-hover:text-brand-primary">
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
        <div className="h-1.5 overflow-hidden rounded-full bg-background">
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

function ProjectCardsSkeleton() {
  return (
    <div className="grid gap-4 md:grid-cols-2 xl:grid-cols-3">
      {[0, 1, 2].map((item) => (
        <div key={item} className="min-h-[230px] rounded-lg border border-stroke bg-card p-5">
          <div className="mb-4 flex items-start justify-between gap-3">
            <div className="min-w-0 flex-1 space-y-2">
              <div className="skeleton-shimmer h-6 w-40 rounded" />
              <div className="skeleton-shimmer h-4 w-full max-w-[260px] rounded" />
            </div>
            <div className="skeleton-shimmer h-5 w-16 rounded" />
          </div>
          <div className="grid grid-cols-3 gap-2">
            <div className="skeleton-shimmer h-14 rounded-md" />
            <div className="skeleton-shimmer h-14 rounded-md" />
            <div className="skeleton-shimmer h-14 rounded-md" />
          </div>
          <div className="mt-5">
            <div className="skeleton-shimmer h-1.5 rounded-full" />
            <div className="mt-3 flex items-center justify-between">
              <div className="skeleton-shimmer h-3 w-20 rounded" />
              <div className="skeleton-shimmer h-3 w-24 rounded" />
            </div>
          </div>
        </div>
      ))}
    </div>
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
    <div className="rounded-md border border-stroke bg-background/50 p-2">
      <div className="mb-1 flex items-center gap-1.5">
        <Icon size={13} className="text-brand-primary" />
        <span>{label}</span>
      </div>
      <div className="font-mono text-sm font-semibold text-foreground">{value}</div>
    </div>
  );
}

function StatusBadge({ status }: { status: string }) {
  const styles: Record<string, string> = {
    loading: "border-stroke bg-surface text-content-muted",
    idle: "border-stroke bg-surface text-content-muted",
    active: "border-cyan-400/20 bg-cyan-400/10 text-cyan-700 dark:text-cyan-300",
    blocked: "border-red-400/20 bg-red-400/10 text-red-700 dark:text-red-300",
    done: "border-emerald-400/20 bg-emerald-400/10 text-emerald-700 dark:text-emerald-300",
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
