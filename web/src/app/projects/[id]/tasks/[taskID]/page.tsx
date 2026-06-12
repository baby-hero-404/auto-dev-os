"use client";

import Link from "next/link";
import { use, useMemo, useState } from "react";
import {
  ArrowLeft,
  Bot,
  Clock,
  Play,
  GitPullRequest,
  Check,
  AlertCircle,
  MessageSquare,
  Sparkles,
} from "lucide-react";
import { Badge } from "@/components/ui/badge";
import { useTaskWorkflow } from "@/lib/hooks/use-task-workflow";
import { SpecReviewSection } from "@/components/projects/spec-review-section";
import { LogConsole } from "@/components/dashboard/log-console";
import { getRiskAssessment } from "@/lib/utils/tasks";

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
  low: "bg-success/10 text-success border-success/20",
  medium: "bg-warning/10 text-warning border-warning/20",
  high: "bg-danger/10 text-danger border-danger/20",
  critical: "bg-danger/20 text-danger border-danger/30 animate-pulse",
};

export default function ProjectTaskDetailPage({
  params,
}: {
  params: Promise<{ id: string; taskID: string }>;
}) {
  const { id: projectID, taskID } = use(params);
  const [activeSpecTab, setActiveSpecTab] = useState<"summary" | "proposal" | "design" | "tasks">("summary");
  const [rawSelectedFile, setSelectedFile] = useState<string | null>(null);
  const [completedPlanSteps, setCompletedPlanSteps] = useState<Record<number, boolean>>({});

  const togglePlanStep = (idx: number) => {
    setCompletedPlanSteps((prev) => ({ ...prev, [idx]: !prev[idx] }));
  };

  const {
    task,
    workflow,
    logs,
    error,
    submittingPR,
    feedback,
    setFeedback,
    isRequestingChanges,
    setIsRequestingChanges,
    specFeedbackText,
    setSpecFeedbackText,
    execute,
    approveSpec,
    requestSpecChanges,
    submitSpecChanges,
    approvePR,
    rejectPR,
  } = useTaskWorkflow(taskID);

  // Compute latest status map for workflow progress dots
  const latest = useMemo(() => {
    const map = new Map<string, string>();
    for (const checkpoint of workflow?.checkpoints ?? []) {
      const status = checkpoint.state?.status;
      map.set(checkpoint.step, typeof status === "string" ? status : "recorded");
    }
    return map;
  }, [workflow]);

  // Parse task analysis
  const analysisData = useMemo(() => {
    let data: {
      complexity?: string;
      scope?: string;
      affected_files?: string[];
      risks?: string[];
      execution_plan?: string[];
      clarification_questions?: string[];
      proposal_md?: string;
      design_md?: string;
      tasks_md?: string;
    } = {};
    try {
      if (task?.analysis) {
        data = typeof task.analysis === "string" ? JSON.parse(task.analysis) : task.analysis;
      }
    } catch {}
    return data;
  }, [task]);

  const affectedFiles = analysisData.affected_files || [];
  const selectedFile = rawSelectedFile || affectedFiles[0] || null;
  const riskAssessment = getRiskAssessment(task?.complexity ?? "easy", affectedFiles);

  const isReviewWaiting = task?.status === "human_review";
  const isPRMerged = task?.status === "merged";

  return (
    <main className="min-h-screen p-5">
      <header className="mb-6 flex flex-col justify-between gap-4 border-b border-stroke pb-5 md:flex-row md:items-end">
        <div>
          <Link
            href={`/projects/${projectID}`}
            className="mb-4 inline-flex items-center gap-2 text-sm text-content-muted transition hover:text-foreground dark:hover:text-white"
          >
            <ArrowLeft size={16} />
            Back to Project
          </Link>
          <h1 className="font-mono text-3xl font-semibold text-foreground dark:text-white">
            {task?.title ?? "Task workflow"}
          </h1>
          <p className="mt-1 max-w-3xl text-sm text-content-muted">{task?.description ?? "Loading task details..."}</p>
        </div>
        <div className="flex flex-wrap gap-2">
          {task &&
            (task.spec_status === "approved" ||
              task.spec_status === "auto_approved" ||
              task.status === "todo" ||
              task.status === "approved") && (
              <button
                className="inline-flex items-center gap-2 rounded-md bg-brand-primary px-3 py-2 text-sm font-semibold text-slate-950 transition hover:opacity-90 cursor-pointer"
                onClick={execute}
                type="button"
              >
                <Play size={15} />
                Execute DAG
              </button>
            )}
        </div>
      </header>

      {error && (
        <div className="mb-5 rounded-lg border border-red-400/30 bg-red-950/40 p-3 text-sm text-red-200 flex items-center gap-2">
          <AlertCircle size={16} />
          {error}
        </div>
      )}

      <SpecReviewSection
        specStatus={task?.spec_status}
        onRequestChanges={requestSpecChanges}
        onApproveSpec={approveSpec}
      />

      {/* Pull Request & Review Center */}
      {(isReviewWaiting || isPRMerged) && (
        <section className="mb-6 rounded-lg border border-stroke bg-panel overflow-hidden">
          <div className="border-b border-stroke bg-slate-50 dark:bg-slate-900/60 p-5 flex flex-wrap items-center justify-between gap-4">
            <div className="flex items-center gap-2.5">
              <div className="grid size-9 place-items-center rounded-md bg-purple-500/10 text-purple-400">
                <GitPullRequest size={18} />
              </div>
              <div>
                <div className="font-mono text-sm uppercase tracking-wider text-content-muted">Task Pull Request</div>
                <h2 className="font-mono font-semibold text-lg text-foreground dark:text-white">
                  [Auto Code OS] {task?.title}
                </h2>
              </div>
            </div>
            <span
              className={`inline-flex rounded border px-2 py-0.5 text-xs font-semibold font-mono uppercase ${
                isPRMerged
                  ? "bg-emerald-400/10 text-emerald-600 dark:text-emerald-300 border-emerald-400/20"
                  : "bg-purple-400/10 text-purple-600 dark:text-purple-300 border-purple-400/20 animate-pulse"
              }`}
            >
              {isPRMerged ? "Merged" : "Awaiting Review"}
            </span>
          </div>

          <div className="grid lg:grid-cols-[380px_1fr] border-b border-stroke">
            {/* PR Summary Details */}
            <div className="border-r border-stroke p-5 space-y-5 bg-slate-50/50 dark:bg-slate-950/20">
              <div>
                <h3 className="font-mono text-xs uppercase tracking-wider text-content-muted mb-2 flex items-center gap-1">
                  <Sparkles size={12} className="text-purple-500 dark:text-purple-400" /> AI PR Summary
                </h3>
                <p className="text-sm leading-relaxed text-slate-600 dark:text-slate-300">
                  Automated changes generated for this execution run. The agent completed the code-backend,
                  code-frontend, and successfully compiled all builds.
                </p>
              </div>

              <div>
                <h3 className="font-mono text-xs uppercase tracking-wider text-content-muted mb-2 font-bold">
                  Risk Assessment
                </h3>
                <div className={`rounded-md border p-3 ${RISK_BADGES[riskAssessment.level]}`}>
                  <div className="flex items-center justify-between mb-1.5">
                    <span className="font-mono text-xs font-bold uppercase tracking-wider">
                      Level: {riskAssessment.level}
                    </span>
                  </div>
                  <p className="text-xs">{riskAssessment.reason}</p>
                </div>
              </div>

              <div>
                <h3 className="font-mono text-xs uppercase tracking-wider text-content-muted mb-2 font-bold">
                  Changed Files ({affectedFiles.length})
                </h3>
                <div className="space-y-1 max-h-48 overflow-y-auto">
                  {affectedFiles.map((file) => (
                    <button
                      key={file}
                      onClick={() => setSelectedFile(file)}
                      className={`flex w-full items-center justify-between rounded px-2.5 py-1.5 text-left text-xs font-mono transition cursor-pointer ${
                        selectedFile === file
                          ? "bg-brand-primary/10 text-brand-primary"
                          : "text-slate-600 dark:text-slate-300 hover:bg-slate-100 dark:hover:bg-slate-900"
                      }`}
                    >
                      <span className="truncate">{file.split("/").pop()}</span>
                      <span className="text-[10px] text-content-muted truncate max-w-[120px]">{file}</span>
                    </button>
                  ))}
                  {affectedFiles.length === 0 && (
                    <p className="text-xs text-content-muted">No file modifications detected.</p>
                  )}
                </div>
              </div>
            </div>

            {/* Interactive Code Diff Review Block */}
            <div className="p-5 flex flex-col min-w-0">
              <div className="mb-3 flex items-center justify-between">
                <span className="font-mono text-xs text-content-muted">
                  Diff Review &mdash;{" "}
                  <span className="text-foreground dark:text-slate-200">{selectedFile || "Select a file"}</span>
                </span>
                <span className="text-[10px] bg-slate-100 dark:bg-slate-800 text-slate-500 dark:text-slate-400 px-2 py-0.5 rounded uppercase font-mono">
                  Git Diff
                </span>
              </div>

              <div className="flex-1 min-h-[250px] overflow-auto rounded-md border border-stroke bg-slate-50 dark:bg-slate-950 p-4 font-mono text-xs leading-relaxed">
                {selectedFile ? (
                  <div className="h-full flex flex-col items-center justify-center text-content-muted">
                    <p className="text-sm">Diff is not available yet. Open PR on Git provider.</p>
                  </div>
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
            <div className="p-5 bg-slate-50 dark:bg-slate-900/40 flex flex-col gap-4">
              <div className="flex items-start gap-3">
                <MessageSquare size={16} className="text-content-muted mt-2.5 shrink-0" />
                <textarea
                  value={feedback}
                  onChange={(e) => setFeedback(e.target.value)}
                  placeholder="Leave rejection feedback to trigger a fix cycle..."
                  className="flex-1 rounded-md border border-stroke bg-slate-50 dark:bg-slate-950 p-3 text-sm text-foreground dark:text-white placeholder-slate-400 dark:placeholder-slate-500 focus:border-brand-primary focus:outline-none min-h-[80px]"
                />
              </div>
              <div className="flex justify-end gap-2.5">
                <button
                  onClick={rejectPR}
                  disabled={submittingPR || !feedback.trim()}
                  className="inline-flex items-center gap-1.5 rounded-md border border-orange-500/30 px-4 py-2 text-sm font-semibold text-orange-700 dark:text-orange-200 transition hover:bg-orange-50 dark:hover:bg-orange-500/10 disabled:opacity-50 cursor-pointer"
                >
                  <AlertCircle size={15} />
                  Reject &amp; Request Fixes
                </button>
                <button
                  onClick={approvePR}
                  disabled={submittingPR}
                  className="inline-flex items-center gap-1.5 rounded-md bg-brand-primary px-4 py-2 text-sm font-semibold text-slate-950 transition hover:opacity-90 disabled:opacity-50 cursor-pointer"
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
          {task?.analysis && (
            <div className="rounded-lg border border-stroke bg-panel p-5">
              <div className="mb-4 flex flex-wrap items-center justify-between gap-4 border-b border-stroke pb-3">
                <div className="flex items-center gap-2">
                  <Sparkles size={18} className="text-brand-primary" />
                  <h2 className="font-mono text-lg font-semibold text-foreground dark:text-white">
                    Proposed Task Specification
                  </h2>
                </div>
                {(analysisData.proposal_md || analysisData.design_md || analysisData.tasks_md) && (
                  <div className="flex gap-1 bg-surface p-1 rounded border border-stroke text-xs font-mono">
                    <button
                      onClick={() => setActiveSpecTab("summary")}
                      className={`px-2.5 py-1 rounded transition-colors ${
                        activeSpecTab === "summary" ? "bg-brand-primary text-white" : "text-content-muted hover:text-foreground"
                      }`}
                    >
                      Summary
                    </button>
                    {analysisData.proposal_md && (
                      <button
                        onClick={() => setActiveSpecTab("proposal")}
                        className={`px-2.5 py-1 rounded transition-colors ${
                          activeSpecTab === "proposal" ? "bg-brand-primary text-white" : "text-content-muted hover:text-foreground"
                        }`}
                      >
                        proposal.md
                      </button>
                    )}
                    {analysisData.design_md && (
                      <button
                        onClick={() => setActiveSpecTab("design")}
                        className={`px-2.5 py-1 rounded transition-colors ${
                          activeSpecTab === "design" ? "bg-brand-primary text-white" : "text-content-muted hover:text-foreground"
                        }`}
                      >
                        design.md
                      </button>
                    )}
                    {analysisData.tasks_md && (
                      <button
                        onClick={() => setActiveSpecTab("tasks")}
                        className={`px-2.5 py-1 rounded transition-colors ${
                          activeSpecTab === "tasks" ? "bg-brand-primary text-white" : "text-content-muted hover:text-foreground"
                        }`}
                      >
                        tasks.md
                      </button>
                    )}
                  </div>
                )}
              </div>

              {activeSpecTab === "summary" ? (
                <div className="space-y-4">
                  {analysisData.scope && (
                    <div>
                      <h3 className="font-mono text-xs uppercase tracking-wider text-content-muted mb-1 font-bold">
                        Scope
                      </h3>
                      <p className="text-sm leading-relaxed text-slate-700 dark:text-slate-300">{analysisData.scope}</p>
                    </div>
                  )}

                  {analysisData.clarification_questions && analysisData.clarification_questions.length > 0 && (
                    <div className="rounded border border-amber-500/20 bg-amber-500/5 p-3 space-y-1">
                      <h3 className="font-mono text-xs font-bold text-amber-700 dark:text-amber-300 flex items-center gap-1.5">
                        <AlertCircle size={14} />
                        Questions / Clarifications Required
                      </h3>
                      <ul className="list-disc list-inside space-y-1 text-xs text-amber-800 dark:text-amber-200">
                        {analysisData.clarification_questions.map((q, idx) => (
                          <li key={idx}>{q}</li>
                        ))}
                      </ul>
                    </div>
                  )}

                  <div className="grid md:grid-cols-2 gap-4">
                    {analysisData.execution_plan && analysisData.execution_plan.length > 0 && (
                      <div>
                        <h3 className="font-mono text-xs uppercase tracking-wider text-content-muted mb-2.5 font-bold flex items-center gap-1.5">
                          <Check size={14} className="text-brand-primary" />
                          Interactive Execution Plan
                        </h3>
                        <div className="space-y-1.5 max-h-[200px] overflow-y-auto pr-1">
                          {analysisData.execution_plan.map((step, idx) => {
                            const isDone = !!completedPlanSteps[idx];
                            return (
                              <label
                                key={idx}
                                className={`flex items-start gap-2.5 rounded-md border p-2 transition cursor-pointer select-none ${
                                  isDone
                                    ? "border-emerald-500/20 bg-emerald-500/5 text-slate-500 dark:text-slate-400 line-through"
                                    : "border-stroke bg-white dark:bg-slate-950 hover:border-brand-primary/50 text-slate-700 dark:text-slate-200"
                                }`}
                              >
                                <input
                                  type="checkbox"
                                  checked={isDone}
                                  onChange={() => togglePlanStep(idx)}
                                  className="mt-0.5 rounded border-stroke text-brand-primary focus:ring-brand-primary"
                                />
                                <span className="text-xs leading-relaxed">{step}</span>
                              </label>
                            );
                          })}
                        </div>
                      </div>
                    )}

                    <div className="space-y-3">
                      {analysisData.risks && analysisData.risks.length > 0 && (
                        <div>
                          <h3 className="font-mono text-xs uppercase tracking-wider text-content-muted mb-1 font-bold">
                            Risks
                          </h3>
                          <ul className="space-y-1">
                            {analysisData.risks.map((risk, idx) => (
                              <li key={idx} className="flex items-start gap-1.5 text-xs text-slate-600 dark:text-slate-300">
                                <span className="mt-1.5 size-1 shrink-0 rounded-full bg-amber-500" />
                                <span className="leading-4">{risk}</span>
                              </li>
                            ))}
                          </ul>
                        </div>
                      )}

                      {analysisData.affected_files && analysisData.affected_files.length > 0 && (
                        <div>
                          <h3 className="font-mono text-xs uppercase tracking-wider text-content-muted mb-1 font-bold">
                            Affected Files
                          </h3>
                          <div className="flex flex-wrap gap-1">
                            {analysisData.affected_files.map((file) => (
                              <span
                                key={file}
                                className="rounded border border-stroke bg-surface px-1.5 py-0.5 font-mono text-[10px] text-content-muted"
                              >
                                {file}
                              </span>
                            ))}
                          </div>
                        </div>
                      )}
                    </div>
                  </div>
                </div>
              ) : (
                <div className="rounded-md border border-stroke bg-slate-50 dark:bg-slate-950 p-4 font-mono text-xs text-slate-800 dark:text-slate-200 overflow-auto max-h-[450px] whitespace-pre-wrap leading-relaxed animate-fade-in">
                  {activeSpecTab === "proposal" && analysisData.proposal_md}
                  {activeSpecTab === "design" && analysisData.design_md}
                  {activeSpecTab === "tasks" && analysisData.tasks_md}
                </div>
              )}
            </div>
          )}

          <div className="rounded-lg border border-stroke bg-panel p-5">
            <div className="mb-4 flex flex-wrap items-center gap-2">
              <h2 className="font-mono text-lg font-semibold text-foreground dark:text-white">Workflow Progress</h2>
              {task && <Badge value={task.status} />}
              {task && <Badge value={task.spec_status} />}
              {workflow?.job && <Badge value={workflow.job.status} />}
            </div>
            <div className="grid gap-3 md:grid-cols-3">
              {workflowSteps.map((step) => {
                const status = latest.get(step) ?? "pending";
                return (
                  <div key={step} className="rounded-md border border-stroke bg-white dark:bg-slate-950 p-3">
                    <div className="mb-2 flex items-center justify-between">
                      <span className="font-mono text-sm text-foreground dark:text-white">{step}</span>
                      <WorkflowDot status={status} />
                    </div>
                    <div className="text-xs uppercase tracking-wide text-content-muted">{status}</div>
                  </div>
                );
              })}
            </div>
          </div>

          <LogConsole logs={logs} />
        </section>

        <aside className="space-y-5">
          <div className="rounded-lg border border-stroke bg-panel p-5">
            <div className="mb-4 flex items-center gap-2">
              <Bot size={18} className="text-brand-primary" />
              <h2 className="font-mono text-lg font-semibold text-foreground dark:text-white">Agent Activity</h2>
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
              <h2 className="font-mono text-lg font-semibold text-foreground dark:text-white">Checkpoints</h2>
            </div>
            <div className="space-y-2 text-sm">
              {(workflow?.checkpoints ?? [])
                .slice()
                .reverse()
                .map((checkpoint) => (
                  <div key={checkpoint.id} className="rounded-md border border-stroke bg-white dark:bg-slate-950 p-3">
                    <div className="font-mono text-brand-primary">{checkpoint.step}</div>
                    <div className="text-xs text-content-muted">{new Date(checkpoint.created_at).toLocaleString()}</div>
                  </div>
                ))}
              {(workflow?.checkpoints ?? []).length === 0 && (
                <p className="text-content-muted">No checkpoints recorded.</p>
              )}
            </div>
          </div>
        </aside>
      </div>

      {isRequestingChanges && (
        <div className="fixed inset-0 z-[100] flex items-center justify-center bg-black/70 backdrop-blur-md p-4 animate-fade-in">
          <div className="relative w-full max-w-lg rounded-xl border border-stroke bg-panel/90 p-6 shadow-2xl backdrop-blur-md animate-scale-up space-y-4">
            <div className="flex items-center gap-2.5 text-amber-500 border-b border-stroke pb-3">
              <MessageSquare size={20} />
              <h2 className="font-mono text-lg font-bold">Request Specification Changes</h2>
            </div>

            <p className="text-sm text-content-muted leading-relaxed">
              Describe what adjustments are needed for this task&apos;s specification, plan, or scope before starting execution.
            </p>

            <textarea
              className="w-full h-32 rounded-lg border border-stroke bg-slate-50 dark:bg-slate-950 p-3 font-mono text-sm text-foreground placeholder-slate-400 outline-none focus:border-brand-primary focus:ring-1 focus:ring-brand-primary transition-all duration-200 resize-none"
              placeholder="e.g. Please use Go standard library and add comprehensive unit tests for the edge cases..."
              value={specFeedbackText}
              onChange={(e) => setSpecFeedbackText(e.target.value)}
              disabled={submittingPR}
              autoFocus
            />

            <div className="flex justify-end gap-2 pt-2 border-t border-stroke">
              <button
                type="button"
                onClick={() => {
                  setIsRequestingChanges(false);
                  setSpecFeedbackText("");
                }}
                disabled={submittingPR}
                className="rounded border border-stroke bg-surface px-4 py-2 text-sm font-semibold text-content hover:bg-slate-100 dark:hover:bg-slate-800 transition cursor-pointer disabled:opacity-50"
              >
                Cancel
              </button>
              <button
                type="button"
                onClick={submitSpecChanges}
                disabled={submittingPR || !specFeedbackText.trim()}
                className="rounded bg-brand-primary px-4 py-2 text-sm font-semibold text-slate-950 hover:opacity-90 transition cursor-pointer disabled:opacity-50 flex items-center gap-1.5"
              >
                {submittingPR ? (
                  <span className="size-3.5 animate-spin rounded-full border-2 border-slate-950 border-t-transparent" />
                ) : (
                  <Check size={14} />
                )}
                Submit Request
              </button>
            </div>
          </div>
        </div>
      )}
    </main>
  );
}

function WorkflowDot({ status }: { status: string }) {
  const color =
    status === "success"
      ? "bg-emerald-400"
      : status === "running"
      ? "bg-sky-400"
      : status === "paused"
      ? "bg-amber-400"
      : status === "failed"
      ? "bg-red-400"
      : "bg-slate-500";
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
