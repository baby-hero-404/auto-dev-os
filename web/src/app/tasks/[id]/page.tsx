"use client";

import Link from "next/link";
import { use, useState } from "react";
import useSWR from "swr";
import { AlertTriangle, ArrowLeft, Bot, CheckCircle2, Clock, Play, TerminalSquare } from "lucide-react";
import { api, ApiError } from "@/lib/api";
import { useSession } from "@/lib/session";
import { Badge } from "@/components/ui/badge";

const workflowSteps = [
  "analyze",
  "plan",
  "code_backend",
  "code_frontend",
  "merge",
  "review",
  "fix",
  "test",
  "pr",
];

export default function TaskWorkflowPage({ params }: { params: Promise<{ id: string }> }) {
  const { id: taskID } = use(params);
  const session = useSession();
  const token = session?.token ?? "";
  const [error, setError] = useState("");

  const { data: workflow, mutate: mutateWorkflow } = useSWR(
    taskID && token ? ["workflow", taskID, token] : null,
    ([, id, t]) => api.taskWorkflow(id, t),
    { refreshInterval: 1500 },
  );
  const { data: logs = [], mutate: mutateLogs } = useSWR(
    taskID && token ? ["task-logs", taskID, token] : null,
    ([, id, t]) => api.taskLogs(id, t),
    { refreshInterval: 1500 },
  );

  const task = workflow?.task;
  const latest = new Map<string, string>();
  for (const checkpoint of workflow?.checkpoints ?? []) {
    const status = checkpoint.state?.status;
    latest.set(checkpoint.step, typeof status === "string" ? status : "recorded");
  }

  async function execute() {
    if (!token) return;
    setError("");
    try {
      await api.executeTask(taskID, token);
      await Promise.all([mutateWorkflow(), mutateLogs()]);
    } catch (err) {
      setError(err instanceof ApiError ? err.message : "Failed to execute workflow");
    }
  }

  async function approveWorkflow() {
    if (!token) return;
    setError("");
    try {
      await api.approveTaskWorkflow(taskID, token);
      await mutateWorkflow();
    } catch (err) {
      setError(err instanceof ApiError ? err.message : "Failed to approve workflow");
    }
  }

  if (!session) {
    return (
      <main className="grid min-h-screen place-items-center p-6">
        <Link className="rounded-md bg-[var(--accent)] px-4 py-2 font-semibold text-slate-950" href="/">
          Back to login
        </Link>
      </main>
    );
  }

  return (
    <main className="min-h-screen p-5">
      <header className="mb-6 flex flex-col justify-between gap-4 border-b border-[var(--border)] pb-5 md:flex-row md:items-end">
        <div>
          <Link href={task ? `/projects/${task.project_id}` : "/"} className="mb-4 inline-flex items-center gap-2 text-sm text-[var(--muted)] transition hover:text-white">
            <ArrowLeft size={16} />
            Project
          </Link>
          <h1 className="font-mono text-3xl font-semibold">{task?.title ?? "Task workflow"}</h1>
          <p className="mt-1 max-w-3xl text-sm text-[var(--muted)]">{task?.description ?? "Loading task details..."}</p>
        </div>
        <div className="flex flex-wrap gap-2">
          <button className="inline-flex items-center gap-2 rounded-md bg-[var(--accent)] px-3 py-2 text-sm font-semibold text-slate-950" onClick={execute} type="button">
            <Play size={15} />
            Execute DAG
          </button>
          <button className="inline-flex items-center gap-2 rounded-md border border-emerald-400/40 px-3 py-2 text-sm text-emerald-200 transition hover:bg-emerald-400/10" onClick={approveWorkflow} type="button">
            <CheckCircle2 size={15} />
            Approve PR
          </button>
        </div>
      </header>

      {error && (
        <div className="mb-5 rounded-lg border border-red-400/30 bg-red-950/40 p-3 text-sm text-red-200">
          {error}
        </div>
      )}

      {task?.spec_status === "pending_review" || task?.spec_status === "changes_requested" ? (
        <div className="mb-5 flex items-start gap-3 rounded-lg border border-amber-400/30 bg-amber-950/30 p-4 text-amber-100">
          <AlertTriangle className="mt-0.5 shrink-0" size={18} />
          <div>
            <div className="font-mono font-semibold">Spec review required</div>
            <p className="text-sm text-amber-100/80">
              This task is paused until the analysis is approved or clarified.
            </p>
          </div>
        </div>
      ) : null}

      <div className="grid gap-5 xl:grid-cols-[1fr_420px]">
        <section className="space-y-5">
          <div className="rounded-lg border border-[var(--border)] bg-[var(--primary)] p-5">
            <div className="mb-4 flex flex-wrap items-center gap-2">
              <h2 className="font-mono text-lg font-semibold">Workflow Progress</h2>
              {task && <Badge value={task.status} />}
              {task && <Badge value={task.spec_status} />}
              {workflow?.job && <Badge value={workflow.job.status} />}
            </div>
            <div className="grid gap-3 md:grid-cols-3">
              {workflowSteps.map((step) => {
                const status = latest.get(step) ?? "pending";
                return (
                  <div key={step} className="rounded-md border border-[var(--border)] bg-slate-950 p-3">
                    <div className="mb-2 flex items-center justify-between">
                      <span className="font-mono text-sm">{step}</span>
                      <WorkflowDot status={status} />
                    </div>
                    <div className="text-xs uppercase tracking-wide text-[var(--muted)]">{status}</div>
                  </div>
                );
              })}
            </div>
          </div>

          <div className="rounded-lg border border-[var(--border)] bg-[var(--primary)] p-5">
            <div className="mb-4 flex items-center gap-2">
              <TerminalSquare size={18} className="text-[var(--accent)]" />
              <h2 className="font-mono text-lg font-semibold">Execution Logs</h2>
            </div>
            <div className="max-h-[520px] overflow-auto rounded-md bg-slate-950 p-4 font-mono text-xs">
              {logs.map((log) => (
                <div key={log.id} className="mb-2 grid gap-2 border-b border-[var(--border)]/50 pb-2 md:grid-cols-[150px_70px_1fr]">
                  <span className="text-[var(--muted)]">{new Date(log.created_at).toLocaleTimeString()}</span>
                  <span className={log.level === "error" ? "text-red-300" : log.level === "warn" ? "text-amber-300" : "text-emerald-300"}>{log.level}</span>
                  <span className="whitespace-pre-wrap">{log.message}</span>
                </div>
              ))}
              {logs.length === 0 && <p className="text-[var(--muted)]">No logs yet. Execute the workflow to start.</p>}
            </div>
          </div>
        </section>

        <aside className="space-y-5">
          <div className="rounded-lg border border-[var(--border)] bg-[var(--primary)] p-5">
            <div className="mb-4 flex items-center gap-2">
              <Bot size={18} className="text-[var(--accent)]" />
              <h2 className="font-mono text-lg font-semibold">Agent Activity</h2>
            </div>
            <dl className="space-y-3 text-sm">
              <InfoRow label="Assigned agent" value={workflow?.job?.agent_id ?? task?.agent_id ?? "Unassigned"} />
              <InfoRow label="Current step" value={workflow?.job?.step ?? "none"} />
              <InfoRow label="Attempts" value={String(workflow?.job?.attempts ?? 0)} />
              <InfoRow label="Last error" value={workflow?.job?.last_error || "none"} />
            </dl>
          </div>

          <div className="rounded-lg border border-[var(--border)] bg-[var(--primary)] p-5">
            <div className="mb-4 flex items-center gap-2">
              <Clock size={18} className="text-[var(--accent)]" />
              <h2 className="font-mono text-lg font-semibold">Checkpoints</h2>
            </div>
            <div className="space-y-2 text-sm">
              {(workflow?.checkpoints ?? []).slice().reverse().map((checkpoint) => (
                <div key={checkpoint.id} className="rounded-md border border-[var(--border)] bg-slate-950 p-3">
                  <div className="font-mono text-[var(--accent)]">{checkpoint.step}</div>
                  <div className="text-xs text-[var(--muted)]">{new Date(checkpoint.created_at).toLocaleString()}</div>
                </div>
              ))}
              {(workflow?.checkpoints ?? []).length === 0 && <p className="text-[var(--muted)]">No checkpoints recorded.</p>}
            </div>
          </div>
        </aside>
      </div>
    </main>
  );
}

function WorkflowDot({ status }: { status: string }) {
  const color =
    status === "success" ? "bg-emerald-400" :
    status === "running" ? "bg-sky-400" :
    status === "paused" ? "bg-amber-400" :
    status === "failed" ? "bg-red-400" :
    "bg-slate-500";
  return <span className={`size-2.5 rounded-full ${color}`} />;
}

function InfoRow({ label, value }: { label: string; value: string }) {
  return (
    <div>
      <dt className="text-xs uppercase tracking-wide text-[var(--muted)]">{label}</dt>
      <dd className="mt-1 break-all font-mono">{value}</dd>
    </div>
  );
}
