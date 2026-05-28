"use client";

import Link from "next/link";
import { FormEvent, use, useState } from "react";
import useSWR from "swr";
import { ArrowLeft, CheckCircle2, GitBranch, Play, Plus, RefreshCw, ShieldAlert } from "lucide-react";
import { api, ApiError } from "@/lib/api";
import { useSession } from "@/lib/session";
import type { Repository, Task } from "@/lib/types";
import { Badge } from "@/components/ui/badge";
import { InfoBlock } from "@/components/ui/info-block";

export default function ProjectDetail({ params }: { params: Promise<{ id: string }> }) {
  const { id: projectID } = use(params);
  const session = useSession();
  const [repoURL, setRepoURL] = useState("");
  const [repoToken, setRepoToken] = useState("");
  const [taskTitle, setTaskTitle] = useState("");
  const [taskDescription, setTaskDescription] = useState("");
  const [changeRequest, setChangeRequest] = useState("");
  const [repoError, setRepoError] = useState("");
  const [taskError, setTaskError] = useState("");

  const token = session?.token ?? "";
  const { data: project } = useSWR(projectID && token ? ["project", projectID, token] : null, ([, id, t]) => api.getProject(id, t));
  const { data: repositories = [], mutate: mutateRepos } = useSWR(projectID && token ? ["repositories", projectID, token] : null, ([, id, t]) => api.listRepositories(id, t));
  const { data: tasks = [], mutate: mutateTasks } = useSWR(projectID && token ? ["tasks", projectID, token] : null, ([, id, t]) => api.listTasks(id, t));

  async function createRepository(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    if (!projectID || !token) return;
    
    const url = repoURL.trim();
    if (!url) {
      setRepoError("Repository URL is required.");
      return;
    }

    setRepoError("");
    try {
      await api.createRepository(projectID, token, {
        url,
        provider: "github",
        branch: "main",
        token: repoToken.trim(),
      });
      setRepoURL("");
      setRepoToken("");
      mutateRepos();
    } catch (err) {
      setRepoError(errorMessage(err, "Failed to link repository"));
    }
  }

  async function createTask(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    if (!projectID || !token) return;
    
    const title = taskTitle.trim();
    if (!title) {
      setTaskError("Task title is required.");
      return;
    }

    setTaskError("");
    try {
      await api.createTask(projectID, token, {
        title,
        description: taskDescription.trim(),
        complexity: "easy",
        priority: 0,
        labels: [],
      });
      setTaskTitle("");
      setTaskDescription("");
      mutateTasks();
    } catch (err) {
      setTaskError(errorMessage(err, "Failed to create task"));
    }
  }

  async function analyze(taskID: string) {
    await api.analyzeTask(taskID, token);
    mutateTasks();
  }

  async function approve(taskID: string) {
    await api.approveTaskAnalysis(taskID, token);
    mutateTasks();
  }

  async function requestChanges(taskID: string) {
    await api.requestTaskChanges(taskID, token, changeRequest || "Please refine the task spec before execution.");
    setChangeRequest("");
    mutateTasks();
  }

  async function cloneRepository(repoID: string) {
    await api.cloneRepository(repoID, token);
    mutateRepos();
  }

  if (!session) {
    return (
      <main className="grid min-h-screen place-items-center p-6">
        <div className="rounded-lg border border-[var(--border)] bg-[var(--primary)] p-6">
          <p className="mb-4 text-sm text-[var(--muted)]">Login from the dashboard before opening a project.</p>
          <Link className="rounded-md bg-[var(--accent)] px-4 py-2 font-semibold text-slate-950" href="/">
            Back to login
          </Link>
        </div>
      </main>
    );
  }

  return (
    <main className="min-h-screen p-5">
      <header className="mb-6 flex flex-col justify-between gap-4 border-b border-[var(--border)] pb-5 md:flex-row md:items-end">
        <div>
          <Link href="/" className="mb-4 inline-flex items-center gap-2 text-sm text-[var(--muted)] transition hover:text-white">
            <ArrowLeft size={16} />
            Projects
          </Link>
          <h1 className="font-mono text-3xl font-semibold">{project?.name ?? "Project"}</h1>
          <p className="mt-1 text-sm text-[var(--muted)]">{project?.description ?? "Repository and task workspace"}</p>
        </div>
        <div className="rounded-md border border-[var(--border)] bg-[var(--primary)] px-3 py-2 text-sm text-[var(--muted)]">Project ID: {projectID}</div>
      </header>

      <div className="grid gap-5 xl:grid-cols-[420px_1fr]">
        <section className="space-y-5">
          <div className="rounded-lg border border-[var(--border)] bg-[var(--primary)] p-5">
            <div className="mb-4 flex items-center gap-2">
              <GitBranch size={18} className="text-[var(--accent)]" />
              <h2 className="font-mono text-lg font-semibold">Repositories</h2>
            </div>
            <form className="space-y-3" onSubmit={createRepository}>
              <input value={repoURL} onChange={(e) => setRepoURL(e.target.value)} placeholder="https://github.com/org/repo.git" className="w-full rounded-md border border-[var(--border)] bg-slate-950 px-3 py-2 text-sm text-white" />
              <input value={repoToken} onChange={(e) => setRepoToken(e.target.value)} placeholder="GitHub token" type="password" className="w-full rounded-md border border-[var(--border)] bg-slate-950 px-3 py-2 text-sm text-white" />
              {repoError && (
                <p className="rounded border border-red-400/30 bg-red-950/40 p-2 text-xs text-red-200">
                  {repoError}
                </p>
              )}
              <button className="flex w-full items-center justify-center gap-2 rounded-md bg-[var(--accent)] px-3 py-2 text-sm font-semibold text-slate-950" type="submit">
                <Plus size={16} />
                Link repository
              </button>
            </form>
            <div className="mt-4 space-y-3">
              {repositories.map((repo: Repository) => (
                <div key={repo.id} className="rounded-md border border-[var(--border)] bg-slate-950 p-3">
                  <div className="break-all text-sm">{repo.url}</div>
                  <div className="mt-2 flex items-center justify-between text-xs text-[var(--muted)]">
                    <span>{repo.clone_status}</span>
                    <button className="inline-flex items-center gap-1 rounded border border-[var(--border)] px-2 py-1 transition hover:bg-[var(--secondary)]" onClick={() => cloneRepository(repo.id)} type="button">
                      <RefreshCw size={13} />
                      Clone
                    </button>
                  </div>
                </div>
              ))}
            </div>
          </div>

          <div className="rounded-lg border border-[var(--border)] bg-[var(--primary)] p-5">
            <h2 className="mb-4 font-mono text-lg font-semibold">Create task</h2>
            <form className="space-y-3" onSubmit={createTask}>
              <input value={taskTitle} onChange={(e) => setTaskTitle(e.target.value)} placeholder="Task title" className="w-full rounded-md border border-[var(--border)] bg-slate-950 px-3 py-2 text-sm text-white" />
              <textarea value={taskDescription} onChange={(e) => setTaskDescription(e.target.value)} placeholder="Description, context, files, expected behavior" rows={5} className="w-full rounded-md border border-[var(--border)] bg-slate-950 px-3 py-2 text-sm text-white" />
              {taskError && (
                <p className="rounded border border-red-400/30 bg-red-950/40 p-2 text-xs text-red-200">
                  {taskError}
                </p>
              )}
              <button className="flex w-full items-center justify-center gap-2 rounded-md bg-[var(--accent)] px-3 py-2 text-sm font-semibold text-slate-950" type="submit">
                <Plus size={16} />
                Create task
              </button>
            </form>
          </div>
        </section>

        <section className="rounded-lg border border-[var(--border)] bg-[var(--primary)] p-5">
          <h2 className="mb-4 font-mono text-lg font-semibold">Tasks</h2>
          <div className="space-y-4">
            {tasks.map((task: Task) => (
              <article key={task.id} className="rounded-lg border border-[var(--border)] bg-slate-950 p-4">
                <div className="flex flex-col justify-between gap-3 md:flex-row md:items-start">
                  <div>
                    <div className="flex flex-wrap items-center gap-2">
                      <h3 className="font-mono font-semibold">{task.title}</h3>
                      <Badge value={task.complexity} />
                      <Badge value={task.spec_status} />
                      <Badge value={task.status} />
                    </div>
                    <p className="mt-2 text-sm text-[var(--muted)]">{task.description || "No description"}</p>
                  </div>
                  <div className="flex shrink-0 flex-wrap gap-2">
                    <Link className="inline-flex items-center gap-2 rounded-md border border-[var(--border)] px-3 py-2 text-sm transition hover:bg-[var(--secondary)]" href={`/projects/${projectID}/tasks/${task.id}/monitor`}>
                      Workflow
                    </Link>
                    <button className="inline-flex items-center gap-2 rounded-md border border-[var(--border)] px-3 py-2 text-sm transition hover:bg-[var(--secondary)]" onClick={() => analyze(task.id)} type="button">
                      <Play size={15} />
                      Analyze
                    </button>
                    <button className="inline-flex items-center gap-2 rounded-md border border-emerald-400/40 px-3 py-2 text-sm text-emerald-200 transition hover:bg-emerald-400/10" onClick={() => approve(task.id)} type="button">
                      <CheckCircle2 size={15} />
                      Approve
                    </button>
                  </div>
                </div>

                {task.analysis && (
                  <div className="mt-4 grid gap-3 lg:grid-cols-2">
                    <InfoBlock title="Scope" items={[task.analysis.scope]} />
                    <InfoBlock title="Risks" items={task.analysis.risks ?? []} />
                    <InfoBlock title="Plan" items={task.analysis.execution_plan ?? []} />
                    <InfoBlock title="Questions" items={task.analysis.clarification_questions ?? []} />
                  </div>
                )}

                <div className="mt-4 flex flex-col gap-2 md:flex-row">
                  <input value={changeRequest} onChange={(e) => setChangeRequest(e.target.value)} placeholder="Request spec changes" className="min-w-0 flex-1 rounded-md border border-[var(--border)] bg-[var(--background)] px-3 py-2 text-sm" />
                  <button className="inline-flex items-center justify-center gap-2 rounded-md border border-amber-400/40 px-3 py-2 text-sm text-amber-200 transition hover:bg-amber-400/10" onClick={() => requestChanges(task.id)} type="button">
                    <ShieldAlert size={15} />
                    Request changes
                  </button>
                </div>
              </article>
            ))}
          </div>
        </section>
      </div>
    </main>
  );
}

function errorMessage(err: unknown, fallback: string) {
  return err instanceof ApiError ? err.message : fallback;
}
