"use client";

import Link from "next/link";
import { FormEvent, useMemo, useState } from "react";
import useSWR from "swr";
import { FolderGit, Plus, X, Loader2 } from "lucide-react";
import { api, ApiError } from "@/lib/api";
import { useSession } from "@/lib/session";
import type { Project } from "@/lib/types";
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
    session ? ["projects", orgID, token] : null,
    ([, oid, t]) => api.listProjects(oid, t),
  );

  const { data: overview } = useSWR(
    session ? ["analytics-overview", orgID, token] : null,
    ([, oid, t]) => api.analyticsOverview(t, oid),
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
          <p className="mt-1 text-sm text-[var(--muted)]">
            Link repositories, create tasks, and configure agents for execution.
          </p>
        </div>
        <button
          onClick={() => {
            setCreationError("");
            setShowModal(true);
          }}
          className="flex items-center justify-center gap-2 rounded-md bg-[var(--accent)] px-4 py-2.5 text-sm font-semibold text-slate-950 transition hover:opacity-90 cursor-pointer shadow-[0_0_15px_rgba(244,63,94,0.15)] self-start sm:self-auto"
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
        <div className="flex flex-col items-center justify-center rounded-lg border border-dashed border-[var(--border)] bg-[var(--primary)] p-12 text-center">
          <div className="grid size-12 place-items-center rounded-xl bg-slate-900 text-[var(--accent)]">
            <FolderGit size={24} />
          </div>
          <h3 className="mt-4 font-mono font-semibold">No projects configured</h3>
          <p className="mt-2 max-w-sm text-sm text-[var(--muted)]">
            Get started by creating a new project and linking it to a remote Git repository.
          </p>
          <button
            onClick={() => {
              setCreationError("");
              setShowModal(true);
            }}
            className="mt-5 flex items-center justify-center gap-2 rounded-md bg-[var(--accent)] px-4 py-2 text-sm font-semibold text-slate-950 transition hover:opacity-90 cursor-pointer"
            type="button"
          >
            <Plus size={16} />
            Create Project
          </button>
        </div>
      ) : (
        <div className="grid gap-4 md:grid-cols-2 xl:grid-cols-3">
          {projects.map((project: Project) => (
            <Link
              key={project.id}
              href={`/projects/${project.id}`}
              className="group rounded-lg border border-[var(--border)] bg-[var(--primary)] p-5 transition hover:border-[var(--accent)]/40 flex flex-col justify-between"
            >
              <div>
                <div className="mb-4 flex items-start justify-between gap-3">
                  <div>
                    <h3 className="font-mono text-lg font-semibold group-hover:text-[var(--accent)] transition duration-150">
                      {project.name}
                    </h3>
                    <p className="mt-2 line-clamp-2 text-sm text-[var(--muted)]">
                      {project.description || "No project description provided."}
                    </p>
                  </div>
                  <span className="rounded border border-emerald-400/20 bg-emerald-400/10 px-2 py-0.5 text-[10px] uppercase font-bold tracking-wider text-emerald-300">
                    active
                  </span>
                </div>
              </div>

              <div className="mt-4">
                <div className="h-1.5 rounded-full bg-slate-950 overflow-hidden">
                  <div className="h-full w-1/3 rounded-full bg-[var(--accent)]" />
                </div>
                <div className="mt-3 flex justify-between text-xs text-[var(--muted)] font-mono">
                  <span>Tasks Configured</span>
                  <span className="text-white">Active</span>
                </div>
              </div>
            </Link>
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
          <div className="relative w-full max-w-md transform overflow-hidden rounded-xl border border-[var(--border)] bg-slate-900 p-6 shadow-2xl transition-all duration-300 animate-[in_0.15s_ease-out]">
            <div className="flex items-center justify-between border-b border-[var(--border)] pb-4">
              <h3 className="font-mono text-lg font-semibold text-white">Create New Project</h3>
              <button
                onClick={() => {
                  if (!isSubmitting) setShowModal(false);
                }}
                className="rounded-md p-1 text-[var(--muted)] transition hover:bg-slate-800 hover:text-white cursor-pointer"
                disabled={isSubmitting}
                type="button"
              >
                <X size={18} />
              </button>
            </div>

            <form className="mt-4 flex flex-col gap-4" onSubmit={handleCreateProject}>
              <div className="flex flex-col gap-1.5">
                <label className="text-xs font-mono font-bold uppercase tracking-wider text-[var(--muted)]">
                  Project Name <span className="text-[var(--accent)]">*</span>
                </label>
                <input
                  value={projectName}
                  onChange={(e) => setProjectName(e.target.value)}
                  placeholder="e.g. e-commerce-backend"
                  className="rounded-md border border-[var(--border)] bg-slate-950 px-3 py-2 text-sm text-white focus:outline-none focus:border-[var(--accent)] transition"
                  disabled={isSubmitting}
                  required
                  autoFocus
                />
              </div>

              <div className="flex flex-col gap-1.5">
                <label className="text-xs font-mono font-bold uppercase tracking-wider text-[var(--muted)]">
                  Description
                </label>
                <textarea
                  value={projectDescription}
                  onChange={(e) => setProjectDescription(e.target.value)}
                  placeholder="Optional brief details about the project goals, repository scope, or rules."
                  className="min-h-[100px] rounded-md border border-[var(--border)] bg-slate-950 px-3 py-2 text-sm text-white focus:outline-none focus:border-[var(--accent)] transition resize-none"
                  disabled={isSubmitting}
                />
              </div>

              {creationError && (
                <p className="rounded-md border border-red-500/20 bg-red-950/40 p-3 text-xs text-red-200">
                  {creationError}
                </p>
              )}

              <div className="mt-2 flex items-center justify-end gap-3 border-t border-[var(--border)] pt-4">
                <button
                  onClick={() => setShowModal(false)}
                  className="rounded-md border border-[var(--border)] bg-transparent px-4 py-2 text-sm font-semibold text-white transition hover:bg-slate-800 cursor-pointer disabled:opacity-50"
                  disabled={isSubmitting}
                  type="button"
                >
                  Cancel
                </button>
                <button
                  className="flex items-center gap-2 rounded-md bg-[var(--accent)] px-4 py-2 text-sm font-semibold text-slate-950 transition hover:opacity-90 cursor-pointer disabled:opacity-50"
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
