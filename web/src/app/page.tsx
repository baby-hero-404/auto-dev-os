"use client";

import Link from "next/link";
import { FormEvent, useMemo, useState } from "react";
import useSWR from "swr";
import { Plus } from "lucide-react";
import { api, ApiError } from "@/lib/api";
import { useSession } from "@/lib/session";
import type { Project } from "@/lib/types";
import { DashboardLayout } from "@/components/dashboard/dashboard-layout";
import { StatsCards } from "@/components/dashboard/stats-cards";

export default function Home() {
  const session = useSession();
  const [projectName, setProjectName] = useState("");
  const [projectDescription, setProjectDescription] = useState("");
  const [creationError, setCreationError] = useState("");

  const { data: projects = [], mutate, error } = useSWR(
    session ? ["projects", session.user.org_id, session.token] : null,
    ([, orgID, token]) => api.listProjects(orgID, token),
  );

  const stats = useMemo(
    () => [
      { label: "Projects", value: projects.length.toString() },
      { label: "Active tasks", value: "0" },
      { label: "Running agents", value: "0" },
      { label: "Open PRs", value: "0" },
    ],
    [projects.length],
  );

  async function createProject(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    if (!session) return;
    
    const trimmedName = projectName.trim();
    if (!trimmedName) {
      setCreationError("Project name is required.");
      return;
    }

    setCreationError("");
    try {
      await api.createProject(session.user.org_id, session.token, {
        name: trimmedName,
        description: projectDescription.trim(),
      });
      setProjectName("");
      setProjectDescription("");
      mutate();
    } catch (err) {
      setCreationError(err instanceof ApiError ? err.message : "Failed to create project");
    }
  }

  return (
    <DashboardLayout>
      <StatsCards stats={stats} />

      <div className="mb-5 flex flex-col justify-between gap-4 lg:flex-row lg:items-end">
        <div>
          <h2 className="font-mono text-2xl font-semibold">Projects</h2>
          <p className="mt-1 text-sm text-[var(--muted)]">
            Link repositories, create tasks, and prepare specs for agent execution.
          </p>
        </div>
        <form
          className="grid gap-2 rounded-lg border border-[var(--border)] bg-[var(--primary)] p-3 md:grid-cols-[180px_260px_auto]"
          onSubmit={createProject}
        >
          <input
            value={projectName}
            onChange={(e) => setProjectName(e.target.value)}
            placeholder="Project name"
            className="rounded-md border border-[var(--border)] bg-slate-950 px-3 py-2 text-sm text-white"
          />
          <input
            value={projectDescription}
            onChange={(e) => setProjectDescription(e.target.value)}
            placeholder="Description"
            className="rounded-md border border-[var(--border)] bg-slate-950 px-3 py-2 text-sm text-white"
          />
          <button
            className="flex items-center justify-center gap-2 rounded-md bg-[var(--accent)] px-3 py-2 text-sm font-semibold text-slate-950 transition hover:opacity-90"
            type="submit"
          >
            <Plus size={16} />
            Create
          </button>
        </form>
      </div>

      {creationError && (
        <p className="mb-4 rounded-md border border-red-400/40 bg-red-950/40 p-3 text-sm text-red-100">
          {creationError}
        </p>
      )}

      {error && (
        <p className="mb-4 rounded-md border border-red-400/40 bg-red-950/40 p-3 text-sm text-red-100">
          {error.message}
        </p>
      )}

      <div className="grid gap-4 md:grid-cols-2 xl:grid-cols-3">
        {projects.map((project: Project) => (
          <Link
            key={project.id}
            href={`/projects/${project.id}`}
            className="rounded-lg border border-[var(--border)] bg-[var(--primary)] p-5 transition hover:border-[var(--accent)]"
          >
            <div className="mb-4 flex items-start justify-between gap-3">
              <div>
                <h3 className="font-mono text-lg font-semibold">{project.name}</h3>
                <p className="mt-1 line-clamp-2 text-sm text-[var(--muted)]">
                  {project.description || "No description"}
                </p>
              </div>
              <span className="rounded bg-emerald-400/10 px-2 py-1 text-xs text-emerald-300">
                active
              </span>
            </div>
            <div className="h-2 rounded-full bg-slate-950">
              <div className="h-2 w-1/3 rounded-full bg-[var(--accent)]" />
            </div>
            <div className="mt-4 flex justify-between text-xs text-[var(--muted)]">
              <span>Tasks ready</span>
              <span>Open</span>
            </div>
          </Link>
        ))}
      </div>
    </DashboardLayout>
  );
}
