"use client";

import { use, useEffect, useMemo, useRef, useState } from "react";
import useSWR from "swr";
import Link from "next/link";
import {
  ArrowLeft,
  CheckCircle2,
  Circle,
  Loader2,
  Play,
  XCircle,
  Pause,
  Merge,
  Bot,
  Terminal,
  ShieldCheck,
} from "lucide-react";
import { api } from "@/lib/api";
import { useSession } from "@/lib/session";
import { useRealtimeLogStore, type RealtimeLog } from "@/lib/store/use-realtime-log-store";
import type { TaskLog, WorkflowStatus } from "@/lib/types";

const STEPS = [
  { id: "analyze", label: "Analyze", icon: Circle },
  { id: "plan", label: "Plan", icon: Circle },
  { id: "code_backend", label: "Backend", icon: Terminal },
  { id: "code_frontend", label: "Frontend", icon: Terminal },
  { id: "merge", label: "Merge", icon: Merge },
  { id: "review", label: "Review", icon: ShieldCheck },
  { id: "fix", label: "Fix", icon: Circle },
  { id: "test", label: "Test", icon: CheckCircle2 },
  { id: "pr", label: "PR", icon: Circle },
];

function stepStatus(
  stepId: string,
  currentStep: string | undefined,
  jobStatus: string | undefined,
  checkpoints: { step: string }[]
) {
  const completedSteps = checkpoints.map((cp) => cp.step);
  if (completedSteps.includes(stepId)) return "done";
  if (currentStep === stepId) {
    if (jobStatus === "failed") return "failed";
    if (jobStatus === "paused") return "paused";
    return "running";
  }
  return "pending";
}

function StepBadge({ status }: { status: string }) {
  switch (status) {
    case "done":
      return <CheckCircle2 size={18} className="text-emerald-400" />;
    case "running":
      return <Loader2 size={18} className="animate-spin text-sky-400" />;
    case "failed":
      return <XCircle size={18} className="text-red-400" />;
    case "paused":
      return <Pause size={18} className="text-amber-400" />;
    default:
      return <Circle size={18} className="text-content-muted" />;
  }
}

export default function MonitorPage({
  params,
}: {
  params: Promise<{ id: string; taskID: string }>;
}) {
  const { id: projectID, taskID } = use(params);
  const session = useSession();
  const token = session?.token ?? "";
  const logEndRef = useRef<HTMLDivElement>(null);
  const [autoScroll, setAutoScroll] = useState(true);
  const realtimeLogs = useRealtimeLogStore((state) => state.logs);
  const appendLogs = useRealtimeLogStore((state) => state.appendLogs);
  const clearLogs = useRealtimeLogStore((state) => state.clearLogs);

  const { data: wfStatus, mutate: mutateStatus } = useSWR<WorkflowStatus>(
    taskID ? ["wf-status", taskID] : null,
    () => api.taskWorkflow(taskID as string, token),
    { refreshInterval: 3000 }
  );

  const { data: fetchedLogs, mutate: mutateLogs } = useSWR<TaskLog[]>(
    taskID ? ["wf-logs", taskID] : null,
    () => api.taskLogs(taskID as string, token),
    { refreshInterval: 2000 }
  );

  const logs = useMemo(
    () => realtimeLogs.filter((log) => log.streamId === taskID),
    [realtimeLogs, taskID],
  );

  useEffect(() => {
    clearLogs(taskID);
  }, [clearLogs, taskID]);

  useEffect(() => {
    if (!fetchedLogs) return;
    appendLogs(fetchedLogs.map((log) => toRealtimeLog(taskID, log)));
  }, [appendLogs, fetchedLogs, taskID]);

  useEffect(() => {
    if (autoScroll && logEndRef.current) {
      logEndRef.current.scrollIntoView({ behavior: "smooth" });
    }
  }, [logs, autoScroll]);

  async function handleExecute() {
    if (!token) return;
    await api.executeTask(taskID, token);
    mutateStatus();
    mutateLogs();
  }

  async function handleApprove() {
    if (!token) return;
    await api.approveTaskWorkflow(taskID, token);
    mutateStatus();
  }

  const task = wfStatus?.task;
  const job = wfStatus?.job;
  const checkpoints = wfStatus?.checkpoints ?? [];

  if (!session) {
    return (
      <main className="grid min-h-screen place-items-center p-6">
        <div className="rounded-lg border border-stroke bg-panel p-6">
          <p className="mb-4 text-sm text-content-muted">
            Login required to view workflow monitor.
          </p>
          <Link
            className="rounded-md bg-brand-primary px-4 py-2 font-semibold text-slate-950"
            href="/"
          >
            Back to login
          </Link>
        </div>
      </main>
    );
  }

  return (
    <main className="min-h-screen p-5">
      {/* Header */}
      <header className="mb-6 border-b border-stroke pb-5">
        <Link
          href={`/projects/${projectID}`}
          className="mb-3 inline-flex items-center gap-2 text-sm text-content-muted transition hover:text-white"
        >
          <ArrowLeft size={16} />
          Back to project
        </Link>
        <div className="flex flex-col justify-between gap-4 md:flex-row md:items-end">
          <div>
            <h1 className="font-mono text-2xl font-semibold">
              {task?.title ?? "Workflow Monitor"}
            </h1>
            <p className="mt-1 text-sm text-content-muted">
              {task?.description?.slice(0, 120) ?? "Loading..."}
            </p>
          </div>
          <div className="flex gap-2">
            {(!job || job.status === "failed") && (
              <button
                className="inline-flex items-center gap-2 rounded-md bg-brand-primary px-4 py-2 text-sm font-semibold text-slate-950 transition hover:opacity-90"
                onClick={handleExecute}
              >
                <Play size={16} />
                Execute
              </button>
            )}
            {task?.status === "human_review" && (
              <button
                className="inline-flex items-center gap-2 rounded-md border border-emerald-400/40 px-4 py-2 text-sm font-semibold text-emerald-200 transition hover:bg-emerald-400/10"
                onClick={handleApprove}
              >
                <CheckCircle2 size={16} />
                Approve Merge
              </button>
            )}
          </div>
        </div>
      </header>

      {/* Spec Review Banner */}
      {task?.spec_status === "pending_review" && (
        <div className="mb-5 rounded-lg border border-amber-400/30 bg-amber-950/30 p-4 text-sm text-amber-200">
          ⚠️ This task requires spec approval before execution can proceed.
          Please review the analysis and approve or request changes.
        </div>
      )}

      <div className="grid gap-5 lg:grid-cols-[1fr_320px]">
        {/* Left Column: Progress + Logs */}
        <div className="space-y-5">
          {/* Step Progress Bar */}
          <section className="rounded-lg border border-stroke bg-panel p-5">
            <h2 className="mb-4 font-mono text-lg font-semibold">
              Workflow Progress
            </h2>
            <div className="flex items-center gap-1 overflow-x-auto pb-2">
              {STEPS.map((step, i) => {
                const status = job
                  ? stepStatus(step.id, job.step, job.status, checkpoints)
                  : "pending";
                return (
                  <div key={step.id} className="flex items-center">
                    <div
                      className={`flex flex-col items-center rounded-lg px-3 py-2 transition ${
                        status === "running"
                          ? "bg-sky-950/50 ring-1 ring-sky-400/30"
                          : status === "done"
                            ? "bg-emerald-950/30"
                            : status === "failed"
                              ? "bg-red-950/30"
                              : ""
                      }`}
                    >
                      <StepBadge status={status} />
                      <span className="mt-1 text-[11px] font-medium text-content-muted">
                        {step.label}
                      </span>
                    </div>
                    {i < STEPS.length - 1 && (
                      <div
                        className={`mx-0.5 h-px w-4 ${
                          status === "done"
                            ? "bg-emerald-400/60"
                            : "bg-stroke"
                        }`}
                      />
                    )}
                  </div>
                );
              })}
            </div>
            {task && (
              <div className="mt-3 flex flex-wrap gap-2 text-xs">
                <span className="rounded-full border border-stroke px-2 py-0.5">
                  {task.complexity}
                </span>
                <span className="rounded-full border border-stroke px-2 py-0.5">
                  {task.spec_status}
                </span>
                <span className="rounded-full border border-stroke px-2 py-0.5">
                  {task.status}
                </span>
              </div>
            )}
          </section>

          {/* Real-time Log Stream */}
          <section className="rounded-lg border border-stroke bg-panel p-5">
            <div className="mb-3 flex items-center justify-between">
              <h2 className="font-mono text-lg font-semibold">
                Execution Logs
              </h2>
              <button
                className="rounded border border-stroke px-2 py-1 text-xs text-content-muted transition hover:bg-panel-muted"
                onClick={() => setAutoScroll(!autoScroll)}
              >
                {autoScroll ? "Pause scroll" : "Resume scroll"}
              </button>
            </div>
            <div className="max-h-[420px] overflow-y-auto rounded-md bg-slate-950 p-3 font-mono text-sm">
              {logs.length === 0 && (
                <p className="text-content-muted">No logs yet. Execute the task to see output.</p>
              )}
              {logs.map((log) => (
                <div
                  key={log.id}
                  className={`py-0.5 ${
                    log.level === "error"
                      ? "text-red-300"
                      : log.level === "warn"
                        ? "text-amber-300"
                        : "text-slate-300"
                  }`}
                >
                  <span className="mr-2 text-content-muted">
                    {new Date(log.createdAtEpoch).toLocaleTimeString()}
                  </span>
                  <span className="mr-2 font-semibold uppercase text-content-muted">
                    [{log.level}]
                  </span>
                  {log.message}
                </div>
              ))}
              <div ref={logEndRef} />
            </div>
          </section>
        </div>

        {/* Right Column: Agent Panel + Job Details */}
        <aside className="space-y-5">
          {/* Agent Activity Panel */}
          <section className="rounded-lg border border-stroke bg-panel p-5">
            <div className="mb-3 flex items-center gap-2">
              <Bot size={18} className="text-brand-primary" />
              <h2 className="font-mono text-lg font-semibold">Agent</h2>
            </div>
            {job?.agent_id ? (
              <div className="space-y-2 text-sm">
                <div className="flex justify-between">
                  <span className="text-content-muted">Agent ID</span>
                  <span className="font-mono text-xs">
                    {job.agent_id.slice(0, 8)}…
                  </span>
                </div>
                <div className="flex justify-between">
                  <span className="text-content-muted">Current Step</span>
                  <span className="font-semibold">{job.step}</span>
                </div>
                <div className="flex justify-between">
                  <span className="text-content-muted">Status</span>
                  <span
                    className={`font-semibold ${
                      job.status === "done"
                        ? "text-emerald-400"
                        : job.status === "failed"
                          ? "text-red-400"
                          : job.status === "running"
                            ? "text-sky-400"
                            : "text-amber-400"
                    }`}
                  >
                    {job.status}
                  </span>
                </div>
                <div className="flex justify-between">
                  <span className="text-content-muted">Attempts</span>
                  <span>{job.attempts}</span>
                </div>
              </div>
            ) : (
              <p className="text-sm text-content-muted">
                No agent assigned yet.
              </p>
            )}
          </section>

          {/* Job Error */}
          {job?.last_error && (
            <section className="rounded-lg border border-red-400/30 bg-red-950/30 p-4">
              <h3 className="mb-2 text-sm font-semibold text-red-200">
                Last Error
              </h3>
              <p className="text-xs text-red-300">{job.last_error}</p>
            </section>
          )}

          {/* Checkpoints */}
          <section className="rounded-lg border border-stroke bg-panel p-5">
            <h2 className="mb-3 font-mono text-lg font-semibold">
              Checkpoints
            </h2>
            {checkpoints.length === 0 ? (
              <p className="text-sm text-content-muted">
                No checkpoints recorded.
              </p>
            ) : (
              <div className="space-y-2">
                {checkpoints.map((cp) => (
                  <div
                    key={cp.id}
                    className="rounded-md border border-stroke bg-slate-950 p-2 text-xs"
                  >
                    <div className="flex justify-between">
                      <span className="font-semibold">{cp.step}</span>
                      <span className="text-content-muted">
                        {new Date(cp.created_at).toLocaleTimeString()}
                      </span>
                    </div>
                  </div>
                ))}
              </div>
            )}
          </section>
        </aside>
      </div>
    </main>
  );
}

function toRealtimeLog(taskID: string, log: TaskLog): RealtimeLog {
  return {
    id: log.id,
    streamId: taskID,
    source: "workflow",
    level: log.level,
    message: log.message,
    createdAt: log.created_at,
    createdAtEpoch: Date.parse(log.created_at),
  };
}
