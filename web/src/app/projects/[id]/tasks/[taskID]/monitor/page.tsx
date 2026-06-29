"use client";

import { use, useRef, useState, useEffect, useMemo } from "react";
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
  Database,
  Sparkles,
  AlertCircle,
  LucideIcon,
} from "lucide-react";
import { useTaskWorkflow } from "@/lib/hooks/use-task-workflow";
import { useSession } from "@/lib/session";

type StepItem = {
  id: string;
  label: string;
  icon: LucideIcon;
};

const EASY_STEPS: StepItem[] = [
  { id: "context_load", label: "Context", icon: Database },
  { id: "analyze", label: "Analyze", icon: Circle },
  { id: "code_backend", label: "Code", icon: Terminal },
  { id: "test", label: "Test", icon: CheckCircle2 },
  { id: "pr", label: "PR", icon: Circle },
];

const STANDARD_STEPS: StepItem[] = [
  { id: "context_load", label: "Context", icon: Database },
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

function getWorkflowSteps(complexity: string | undefined): StepItem[] {
  if (complexity === "easy") {
    return EASY_STEPS;
  }
  if (complexity === "medium" || complexity === "hard") {
    return STANDARD_STEPS;
  }
  return EASY_STEPS.slice(0, 2);
}

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
  const logEndRef = useRef<HTMLDivElement>(null);
  const [autoScroll, setAutoScroll] = useState(true);

  const session = useSession();

  const {
    task,
    workflow,
    logs,
    execute,
    approvePR,
    startReview,
    submittingPR,
    mutateWorkflow,
    isLoading: isTaskLoading,
    workflowError,
  } = useTaskWorkflow(taskID);

  const job = workflow?.job;
  const checkpoints = workflow?.checkpoints ?? [];
  const steps = useMemo(() => getWorkflowSteps(task?.complexity), [task?.complexity]);

  useEffect(() => {
    if (autoScroll && logEndRef.current) {
      logEndRef.current.scrollIntoView({ behavior: "smooth" });
    }
  }, [logs, autoScroll]);

  if (!session) {
    return (
      <main className="grid min-h-screen place-items-center p-6">
        <div className="rounded-lg border border-stroke bg-card p-6">
          <p className="mb-4 text-sm text-content-muted">Login from the dashboard before opening the monitor.</p>
          <Link className="rounded-md bg-brand-primary px-4 py-2 font-semibold text-slate-950" href="/">Back to login</Link>
        </div>
      </main>
    );
  }

  if (isTaskLoading) {
    return (
      <main className="min-h-screen bg-slate-950 p-6 flex flex-col justify-center items-center gap-4">
        <Loader2 className="h-8 w-8 animate-spin text-brand-primary" />
        <p className="text-sm font-mono text-content-muted animate-pulse">Loading task monitor...</p>
      </main>
    );
  }

  if (workflowError) {
    return (
      <main className="grid min-h-screen place-items-center p-6">
        <div className="rounded-lg border border-red-500/20 bg-red-500/5 p-6 max-w-lg text-center">
          <AlertCircle className="h-10 w-10 text-red-500 mx-auto mb-4" />
          <p className="font-sans text-base font-semibold text-red-600 dark:text-red-400">Failed to load task monitor.</p>
          <p className="mt-1 text-xs text-content-muted mb-4">{workflowError.message || "An unexpected error occurred."}</p>
          <div className="flex justify-center gap-3">
            <Link className="rounded-md border border-stroke bg-panel px-4 py-2 text-sm font-semibold text-foreground hover:bg-surface transition" href={`/projects/${projectID}`}>
              Back to Project
            </Link>
            <button onClick={() => mutateWorkflow()} className="rounded-md bg-brand-primary px-4 py-2 text-sm font-semibold text-slate-950 hover:opacity-90 transition">
              Retry Load
            </button>
          </div>
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
          className="mb-3 inline-flex items-center gap-2 text-sm text-content-muted transition hover:text-foreground dark:hover:text-white"
        >
          <ArrowLeft size={16} />
          Back to project
        </Link>
        <div className="flex flex-col justify-between gap-4 md:flex-row md:items-end">
          <div>
            <h1 className="font-mono text-2xl font-semibold text-foreground dark:text-white">
              {task?.title ?? "Workflow Monitor"}
            </h1>
            <p className="mt-1 text-sm text-content-muted">
              {task?.description?.slice(0, 120) ?? "Loading..."}
            </p>
          </div>
          <div className="flex gap-2">
            {(!job || job.status === "failed") &&
              task &&
              (task.spec_status === "approved" ||
                task.spec_status === "auto_approved") &&
              (task.status === "todo" || task.status === "failed") && (
                <button
                  className="inline-flex items-center gap-2 rounded-md bg-brand-primary px-4 py-2 text-sm font-semibold text-slate-950 transition hover:opacity-90 cursor-pointer"
                  onClick={execute}
                >
                  <Play size={16} />
                  Execute
                </button>
              )}
            {task?.status === "pr_ready" && (
              <>
                <button
                  disabled={submittingPR}
                  className="inline-flex items-center gap-2 rounded-md border border-brand-primary bg-transparent px-4 py-2 text-sm font-semibold text-brand-primary transition hover:bg-brand-primary/10 cursor-pointer disabled:opacity-50"
                  onClick={startReview}
                >
                  <Sparkles size={16} />
                  Start Review
                </button>
                <button
                  disabled={submittingPR}
                  className="inline-flex items-center gap-2 rounded-md bg-brand-primary px-4 py-2 text-sm font-semibold text-slate-950 transition hover:opacity-90 cursor-pointer disabled:opacity-50"
                  onClick={approvePR}
                >
                  <CheckCircle2 size={16} />
                  Approve Merge
                </button>
              </>
            )}
            {task?.status === "human_review" && (
              <button
                disabled={submittingPR}
                className="inline-flex items-center gap-2 rounded-md border border-success/40 px-4 py-2 text-sm font-semibold text-success transition hover:bg-success/10 cursor-pointer disabled:opacity-50"
                onClick={approvePR}
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
            <h2 className="mb-4 font-mono text-lg font-semibold text-foreground dark:text-white">
              Workflow Progress
            </h2>
            <div className="flex items-center gap-1 overflow-x-auto pb-2">
              {steps.map((step, i) => {
                const status = job
                  ? stepStatus(step.id, job.step, job.status, checkpoints)
                  : "pending";
                return (
                  <div key={step.id} className="flex items-center">
                    <div
                      className={`flex flex-col items-center rounded-lg px-3 py-2 transition ${
                        status === "running"
                          ? "bg-sky-50 dark:bg-sky-950/50 ring-1 ring-sky-400/30"
                          : status === "done"
                          ? "bg-emerald-50 dark:bg-emerald-950/30"
                          : status === "failed"
                          ? "bg-red-50 dark:bg-red-950/30"
                          : "bg-white dark:bg-slate-950"
                      } border border-stroke`}
                    >
                      <StepBadge status={status} />
                      <span className="mt-1 text-[11px] font-semibold text-foreground dark:text-white">
                        {step.label}
                      </span>
                    </div>
                    {i < steps.length - 1 && (
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
                <span className="rounded-full border border-stroke px-2 py-0.5 text-content-muted">
                  {task.complexity}
                </span>
                <span className="rounded-full border border-stroke px-2 py-0.5 text-content-muted">
                  {task.spec_status}
                </span>
                <span className="rounded-full border border-stroke px-2 py-0.5 text-content-muted">
                  {task.status}
                </span>
              </div>
            )}
          </section>

          {/* Real-time Log Stream */}
          <section className="rounded-lg border border-stroke bg-panel p-5">
            <div className="mb-3 flex items-center justify-between">
              <h2 className="font-mono text-lg font-semibold text-foreground dark:text-white">
                Execution Logs
              </h2>
              <button
                className="rounded border border-stroke px-2 py-1 text-xs text-content-muted transition hover:bg-panel-muted cursor-pointer"
                onClick={() => setAutoScroll(!autoScroll)}
              >
                {autoScroll ? "Pause scroll" : "Resume scroll"}
              </button>
            </div>
            <div className="max-h-[420px] overflow-y-auto rounded-md bg-slate-50 dark:bg-slate-950 p-3 font-mono text-xs border border-stroke">
              {logs.length === 0 && (
                <p className="text-content-muted">No logs yet. Execute the task to see output.</p>
              )}
              {logs.map((log) => (
                <div
                  key={log.id}
                  className={`py-0.5 ${
                    log.level === "error"
                      ? "text-red-600 dark:text-red-300"
                      : log.level === "warn"
                      ? "text-amber-600 dark:text-amber-300"
                      : "text-slate-800 dark:text-slate-300"
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
              <h2 className="font-mono text-lg font-semibold text-foreground dark:text-white">Agent</h2>
            </div>
            {job?.agent_id ? (
              <div className="space-y-2 text-sm">
                <div className="flex justify-between">
                  <span className="text-content-muted">Agent ID</span>
                  <span className="font-mono text-xs text-foreground dark:text-white">
                    {job.agent_id.slice(0, 8)}…
                  </span>
                </div>
                <div className="flex justify-between">
                  <span className="text-content-muted">Current Step</span>
                  <span className="font-semibold text-foreground dark:text-white">{job.step}</span>
                </div>
                <div className="flex justify-between">
                  <span className="text-content-muted">Status</span>
                  <span
                    className={`font-semibold ${
                      job.status === "done"
                        ? "text-emerald-600 dark:text-emerald-400"
                        : job.status === "failed"
                        ? "text-red-600 dark:text-red-400"
                        : job.status === "running"
                        ? "text-sky-600 dark:text-sky-400"
                        : "text-amber-600 dark:text-amber-400"
                    }`}
                  >
                    {job.status}
                  </span>
                </div>
                <div className="flex justify-between">
                  <span className="text-content-muted">Attempts</span>
                  <span className="text-foreground dark:text-white">{job.attempts}</span>
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
            <h2 className="mb-3 font-mono text-lg font-semibold text-foreground dark:text-white">
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
                    className="rounded-md border border-stroke bg-white dark:bg-slate-950 p-2 text-xs"
                  >
                    <div className="flex justify-between">
                      <span className="font-semibold text-foreground dark:text-white">{cp.step}</span>
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
