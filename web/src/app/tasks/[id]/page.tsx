"use client";

import Link from "next/link";
import { use, useEffect, useMemo, useState } from "react";
import useSWR from "swr";
import {
  AlertTriangle,
  ArrowLeft,
  Bot,
  Clock,
  Play,
  TerminalSquare,
  GitPullRequest,
  Check,
  AlertCircle,
  MessageSquare,
  Sparkles,
} from "lucide-react";
import { api, ApiError } from "@/lib/api";
import { useSession } from "@/lib/session";
import { useRealtimeLogStore, type RealtimeLog } from "@/lib/store/use-realtime-log-store";
import type { TaskLog } from "@/lib/types";
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

const RISK_BADGES: Record<string, string> = {
  low: "bg-emerald-400/10 text-emerald-300 border-emerald-400/20",
  medium: "bg-amber-400/10 text-amber-300 border-amber-400/20",
  high: "bg-red-400/10 text-red-300 border-red-400/20",
  critical: "bg-purple-400/10 text-purple-300 border-purple-400/20 animate-pulse",
};

export default function TaskWorkflowPage({ params }: { params: Promise<{ id: string }> }) {
  const { id: taskID } = use(params);
  const session = useSession();
  const token = session?.token ?? "";
  const [error, setError] = useState("");
  const [feedback, setFeedback] = useState("");
  const [submittingPR, setSubmittingPR] = useState(false);
  const [rawSelectedFile, setSelectedFile] = useState<string | null>(null);
  const realtimeLogs = useRealtimeLogStore((state) => state.logs);
  const appendLogs = useRealtimeLogStore((state) => state.appendLogs);
  const clearLogs = useRealtimeLogStore((state) => state.clearLogs);

  const { data: workflow, mutate: mutateWorkflow } = useSWR(
    taskID ? ["workflow", taskID] : null,
    () => api.taskWorkflow(taskID, token),
    { refreshInterval: (latestWorkflow) => (isWorkflowTerminal(latestWorkflow?.job?.status) ? 0 : 2500) },
  );
  const { data: fetchedLogs, mutate: mutateLogs } = useSWR(
    taskID ? ["task-logs", taskID] : null,
    () => api.taskLogs(taskID, token),
    { refreshInterval: () => (isWorkflowTerminal(workflow?.job?.status) ? 0 : 3000) },
  );

  const logs = useMemo(
    () => realtimeLogs.filter((log) => log.streamId === taskID).slice(-200),
    [realtimeLogs, taskID],
  );

  useEffect(() => {
    clearLogs(taskID);
  }, [clearLogs, taskID]);

  useEffect(() => {
    if (!fetchedLogs) return;
    appendLogs(fetchedLogs.map((log) => toRealtimeLog(taskID, log)));
  }, [appendLogs, fetchedLogs, taskID]);

  const task = workflow?.task;
  const latest = new Map<string, string>();
  for (const checkpoint of workflow?.checkpoints ?? []) {
    const status = checkpoint.state?.status;
    latest.set(checkpoint.step, typeof status === "string" ? status : "recorded");
  }

  // Parse task analysis
  let analysisData: { affected_files?: string[]; risks?: string[]; execution_plan?: string[] } = {};
  try {
    if (task?.analysis) {
      analysisData = typeof task.analysis === "string" ? JSON.parse(task.analysis) : task.analysis;
    }
  } catch {}

  const affectedFiles = analysisData.affected_files || [];
  const selectedFile = rawSelectedFile || affectedFiles[0] || null;

  // Compute risk level dynamically matching the backend
  const getRiskAssessment = (complexity: string, files: string[]) => {
    const fileCount = files.length;
    let hasMigration = false;
    let hasConfig = false;
    for (const f of files) {
      const lower = f.toLowerCase();
      if (lower.includes("migration/") || lower.includes(".sql")) hasMigration = true;
      if (lower.includes("config") || lower.includes(".env") || lower.includes("docker")) hasConfig = true;
    }

    if (hasMigration && complexity === "hard") {
      return { level: "critical", reason: "Database migration in a hard-complexity task requires careful review" };
    }
    if (hasMigration) {
      return { level: "high", reason: "Contains database migration files" };
    }
    if (complexity === "hard" || fileCount > 15) {
      return { level: "high", reason: `Hard complexity task affecting ${fileCount} files` };
    }
    if (hasConfig) {
      return { level: "medium", reason: "Modifies configuration or infrastructure files" };
    }
    if (complexity === "medium" || fileCount > 5) {
      return { level: "medium", reason: `Medium complexity task affecting ${fileCount} files` };
    }
    return { level: "low", reason: `Simple change affecting ${fileCount} files` };
  };

  const riskAssessment = getRiskAssessment(task?.complexity ?? "easy", affectedFiles);

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

  async function approvePR() {
    if (!token) return;
    setError("");
    setSubmittingPR(true);
    try {
      await api.approvePR(taskID, token);
      await mutateWorkflow();
    } catch (err) {
      setError(err instanceof ApiError ? err.message : "Failed to approve PR");
    } finally {
      setSubmittingPR(false);
    }
  }

  async function rejectPR() {
    if (!token) return;
    if (!feedback.trim()) {
      setError("Feedback is required to reject the PR");
      return;
    }
    setError("");
    setSubmittingPR(true);
    try {
      await api.rejectPR(taskID, token, feedback.trim());
      setFeedback("");
      await mutateWorkflow();
    } catch (err) {
      setError(err instanceof ApiError ? err.message : "Failed to reject PR");
    } finally {
      setSubmittingPR(false);
    }
  }

  const getMockDiff = (filename: string) => {
    if (filename.includes("migration") || filename.includes(".sql")) {
      return [
        { type: "normal", line: 1, text: "-- +migrate Up" },
        { type: "add", line: 2, text: "+CREATE TABLE audit_logs (" },
        { type: "add", line: 3, text: "+    id UUID PRIMARY KEY DEFAULT uuid_generate_v4()," },
        { type: "add", line: 4, text: "+    action VARCHAR(255) NOT NULL," },
        { type: "add", line: 5, text: "+    details JSONB NOT NULL DEFAULT '{}'," },
        { type: "add", line: 6, text: "+    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()" },
        { type: "add", line: 7, text: "+);" },
        { type: "normal", line: 8, text: "" },
        { type: "add", line: 9, text: "+CREATE INDEX idx_audit_logs_action ON audit_logs(action);" }
      ];
    }
    if (filename.includes("model") || filename.includes("types") || filename.includes("struct")) {
      return [
        { type: "normal", line: 12, text: "type Task struct {" },
        { type: "normal", line: 13, text: "\tID string `json:\"id\"`" },
        { type: "del", line: 14, text: "-\tStatus string `json:\"status\"`" },
        { type: "add", line: 15, text: "+\tStatus string `json:\"status\" gorm:\"default:'todo'\"`" },
        { type: "add", line: 16, text: "+\tSpecStatus string `json:\"spec_status\" gorm:\"default:'none'\"`" },
        { type: "normal", line: 17, text: "}" }
      ];
    }
    return [
      { type: "normal", line: 1, text: "// Automated change representation" },
      { type: "del", line: 2, text: "-// Old code logic" },
      { type: "add", line: 3, text: "+// New optimized logic using Auto Code OS" },
      { type: "normal", line: 4, text: "func Process() error { return nil }" }
    ];
  };

  if (!session) {
    return (
      <main className="grid min-h-screen place-items-center p-6">
        <Link className="rounded-md bg-brand-primary px-4 py-2 font-semibold text-slate-950" href="/">
          Back to login
        </Link>
      </main>
    );
  }

  const isReviewWaiting = task?.status === "human_review";
  const isPRMerged = task?.status === "merged";

  return (
    <main className="min-h-screen p-5">
      <header className="mb-6 flex flex-col justify-between gap-4 border-b border-stroke pb-5 md:flex-row md:items-end">
        <div>
          <Link href={task ? `/projects/${task.project_id}` : "/"} className="mb-4 inline-flex items-center gap-2 text-sm text-content-muted transition hover:text-white">
            <ArrowLeft size={16} />
            Project
          </Link>
          <h1 className="font-mono text-3xl font-semibold">{task?.title ?? "Task workflow"}</h1>
          <p className="mt-1 max-w-3xl text-sm text-content-muted">{task?.description ?? "Loading task details..."}</p>
        </div>
        <div className="flex flex-wrap gap-2">
          <button
            className="inline-flex items-center gap-2 rounded-md bg-brand-primary px-3 py-2 text-sm font-semibold text-slate-950 transition hover:opacity-90"
            onClick={execute}
            type="button"
          >
            <Play size={15} />
            Execute DAG
          </button>
        </div>
      </header>

      {error && (
        <div className="mb-5 rounded-lg border border-red-400/30 bg-red-950/40 p-3 text-sm text-red-200 flex items-center gap-2">
          <AlertCircle size={16} />
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

      {/* Pull Request & Review Center (Task 5) */}
      {(isReviewWaiting || isPRMerged) && (
        <section className="mb-6 rounded-lg border border-stroke bg-panel overflow-hidden">
          <div className="border-b border-stroke bg-slate-900/60 p-5 flex flex-wrap items-center justify-between gap-4">
            <div className="flex items-center gap-2.5">
              <div className="grid size-9 place-items-center rounded-md bg-purple-500/10 text-purple-400">
                <GitPullRequest size={18} />
              </div>
              <div>
                <div className="font-mono text-sm uppercase tracking-wider text-content-muted">Task Pull Request</div>
                <h2 className="font-mono font-semibold text-lg text-white">
                  [Auto Code OS] {task?.title}
                </h2>
              </div>
            </div>
            <span className={`inline-flex rounded border px-2 py-0.5 text-xs font-semibold font-mono uppercase ${
              isPRMerged ? "bg-emerald-400/10 text-emerald-300 border-emerald-400/20" : "bg-purple-400/10 text-purple-300 border-purple-400/20 animate-pulse"
            }`}>
              {isPRMerged ? "Merged" : "Awaiting Review"}
            </span>
          </div>

          <div className="grid lg:grid-cols-[380px_1fr] border-b border-stroke">
            {/* PR Summary Details */}
            <div className="border-r border-stroke p-5 space-y-5 bg-slate-950/20">
              <div>
                <h3 className="font-mono text-xs uppercase tracking-wider text-content-muted mb-2 flex items-center gap-1">
                  <Sparkles size={12} className="text-purple-400" /> AI PR Summary
                </h3>
                <p className="text-sm leading-relaxed text-slate-300">
                  Automated changes generated for this execution run. The agent completed the code-backend, code-frontend, and successfully compiled all builds.
                </p>
              </div>

              <div>
                <h3 className="font-mono text-xs uppercase tracking-wider text-content-muted mb-2">Risk Assessment</h3>
                <div className={`rounded-md border p-3 ${RISK_BADGES[riskAssessment.level]}`}>
                  <div className="flex items-center justify-between mb-1.5">
                    <span className="font-mono text-xs font-bold uppercase tracking-wider">Level: {riskAssessment.level}</span>
                  </div>
                  <p className="text-xs">{riskAssessment.reason}</p>
                </div>
              </div>

              <div>
                <h3 className="font-mono text-xs uppercase tracking-wider text-content-muted mb-2">Changed Files ({affectedFiles.length})</h3>
                <div className="space-y-1 max-h-48 overflow-y-auto">
                  {affectedFiles.map((file) => (
                    <button
                      key={file}
                      onClick={() => setSelectedFile(file)}
                      className={`flex w-full items-center justify-between rounded px-2.5 py-1.5 text-left text-xs font-mono transition ${
                        selectedFile === file
                          ? "bg-brand-primary/10 text-brand-primary"
                          : "text-slate-300 hover:bg-slate-900"
                      }`}
                    >
                      <span className="truncate">{file.split("/").pop()}</span>
                      <span className="text-[10px] text-content-muted truncate max-w-[120px]">{file}</span>
                    </button>
                  ))}
                  {affectedFiles.length === 0 && <p className="text-xs text-content-muted">No file modifications detected.</p>}
                </div>
              </div>
            </div>

            {/* Interactive Code Diff Review Block */}
            <div className="p-5 flex flex-col min-w-0">
              <div className="mb-3 flex items-center justify-between">
                <span className="font-mono text-xs text-content-muted">
                  Diff Review &mdash; <span className="text-slate-200">{selectedFile || "Select a file"}</span>
                </span>
                <span className="text-[10px] bg-slate-800 text-slate-400 px-2 py-0.5 rounded uppercase font-mono">
                  Git Diff
                </span>
              </div>

              <div className="flex-1 min-h-[250px] overflow-auto rounded-md border border-stroke bg-slate-950 p-4 font-mono text-xs leading-relaxed">
                {selectedFile ? (
                  getMockDiff(selectedFile).map((line, idx) => (
                    <div
                      key={idx}
                      className={`px-2 py-0.5 -mx-4 flex gap-4 ${
                        line.type === "add" ? "bg-emerald-950/40 text-emerald-300" :
                        line.type === "del" ? "bg-red-950/40 text-red-300" :
                        "text-slate-300"
                      }`}
                    >
                      <span className="w-8 shrink-0 text-right select-none text-slate-600">{line.line}</span>
                      <span className="whitespace-pre">{line.text}</span>
                    </div>
                  ))
                ) : (
                  <div className="h-full flex items-center justify-center text-content-muted">
                    Select a file on the left to inspect git changes.
                  </div>
                )}
              </div>
            </div>
          </div>

          {/* Action Footer */}
          {isReviewWaiting && (
            <div className="p-5 bg-slate-900/40 flex flex-col gap-4">
              <div className="flex items-start gap-3">
                <MessageSquare size={16} className="text-content-muted mt-2.5 shrink-0" />
                <textarea
                  value={feedback}
                  onChange={(e) => setFeedback(e.target.value)}
                  placeholder="Leave rejection feedback to trigger a fix cycle..."
                  className="flex-1 rounded-md border border-stroke bg-slate-950 p-3 text-sm text-white placeholder-slate-500 focus:border-brand-primary focus:outline-none min-h-[80px]"
                />
              </div>
              <div className="flex justify-end gap-2.5">
                <button
                  onClick={rejectPR}
                  disabled={submittingPR || !feedback.trim()}
                  className="inline-flex items-center gap-1.5 rounded-md border border-orange-500/30 px-4 py-2 text-sm font-semibold text-orange-200 transition hover:bg-orange-500/10 disabled:opacity-50"
                >
                  <AlertCircle size={15} />
                  Reject &amp; Request Fixes
                </button>
                <button
                  onClick={approvePR}
                  disabled={submittingPR}
                  className="inline-flex items-center gap-1.5 rounded-md bg-brand-primary px-4 py-2 text-sm font-semibold text-slate-950 transition hover:opacity-90 disabled:opacity-50"
                >
                  <Check size={15} />
                  Approve &amp; Merge
                </button>
              </div>
            </div>
          )}
        </section>
      )}

      <div className="grid gap-5 xl:grid-cols-[1fr_420px]">
        <section className="space-y-5">
          <div className="rounded-lg border border-stroke bg-panel p-5">
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
                  <div key={step} className="rounded-md border border-stroke bg-slate-950 p-3">
                    <div className="mb-2 flex items-center justify-between">
                      <span className="font-mono text-sm">{step}</span>
                      <WorkflowDot status={status} />
                    </div>
                    <div className="text-xs uppercase tracking-wide text-content-muted">{status}</div>
                  </div>
                );
              })}
            </div>
          </div>

          <div className="rounded-lg border border-stroke bg-panel p-5">
            <div className="mb-4 flex items-center gap-2">
              <TerminalSquare size={18} className="text-brand-primary" />
              <h2 className="font-mono text-lg font-semibold">Execution Logs</h2>
            </div>
            <div className="max-h-[520px] overflow-auto rounded-md bg-slate-950 p-4 font-mono text-xs">
              {logs.map((log) => (
                <div key={log.id} className="mb-2 grid gap-2 border-b border-stroke/50 pb-2 md:grid-cols-[150px_70px_1fr]">
                  <span className="text-content-muted">{new Date(log.createdAtEpoch).toLocaleTimeString()}</span>
                  <span className={log.level === "error" ? "text-red-300" : log.level === "warn" ? "text-amber-300" : "text-emerald-300"}>{log.level}</span>
                  <span className="whitespace-pre-wrap">{log.message}</span>
                </div>
              ))}
              {logs.length === 0 && <p className="text-content-muted">No logs yet. Execute the workflow to start.</p>}
            </div>
          </div>
        </section>

        <aside className="space-y-5">
          <div className="rounded-lg border border-stroke bg-panel p-5">
            <div className="mb-4 flex items-center gap-2">
              <Bot size={18} className="text-brand-primary" />
              <h2 className="font-mono text-lg font-semibold">Agent Activity</h2>
            </div>
            <dl className="space-y-3 text-sm">
              <InfoRow label="Assigned agent" value={workflow?.job?.agent_id ?? task?.agent_id ?? "Unassigned"} />
              <InfoRow label="Current step" value={workflow?.job?.step ?? "none"} />
              <InfoRow label="Attempts" value={String(workflow?.job?.attempts ?? 0)} />
              <InfoRow label="Last error" value={workflow?.job?.last_error || "none"} />
            </dl>
          </div>

          <div className="rounded-lg border border-stroke bg-panel p-5">
            <div className="mb-4 flex items-center gap-2">
              <Clock size={18} className="text-brand-primary" />
              <h2 className="font-mono text-lg font-semibold">Checkpoints</h2>
            </div>
            <div className="space-y-2 text-sm">
              {(workflow?.checkpoints ?? []).slice().reverse().map((checkpoint) => (
                <div key={checkpoint.id} className="rounded-md border border-stroke bg-slate-950 p-3">
                  <div className="font-mono text-brand-primary">{checkpoint.step}</div>
                  <div className="text-xs text-content-muted">{new Date(checkpoint.created_at).toLocaleString()}</div>
                </div>
              ))}
              {(workflow?.checkpoints ?? []).length === 0 && <p className="text-content-muted">No checkpoints recorded.</p>}
            </div>
          </div>
        </aside>
      </div>
    </main>
  );
}

function isWorkflowTerminal(status?: string) {
  return status === "done" || status === "completed" || status === "failed" || status === "merged";
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
      <dt className="text-xs uppercase tracking-wide text-content-muted">{label}</dt>
      <dd className="mt-1 break-all font-mono">{value}</dd>
    </div>
  );
}
