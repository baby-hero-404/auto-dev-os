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
import { Markdown } from "@/components/ui/markdown";
import { WorkflowArtifact } from "@/lib/types";
import { SpecReviewSection } from "@/components/projects/spec-review-section";
import { LogConsole } from "@/components/dashboard/log-console";
import { getRiskAssessment } from "@/lib/utils/tasks";
import { useAuthedSWR } from "@/lib/use-authed-swr";
import { api } from "@/lib/api";

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
  low: "bg-emerald-500/10 text-emerald-600 dark:text-emerald-400 border-emerald-500/20",
  medium: "bg-warning/10 text-warning border-warning/20",
  high: "bg-danger/10 text-danger border-danger/20",
  critical: "bg-danger/20 text-danger border-danger/30 animate-pulse",
};

interface ParsedFileDiff {
  filename: string;
  diffLines: string[];
}

function parseUnifiedDiff(diffText: string): ParsedFileDiff[] {
  if (!diffText) return [];
  const lines = diffText.split("\n");
  const fileDiffs: ParsedFileDiff[] = [];
  let currentDiff: ParsedFileDiff | null = null;

  for (const line of lines) {
    if (line.startsWith("diff --git ")) {
      const parts = line.split(" b/");
      let filename = "";
      if (parts.length > 1) {
        filename = parts[1].trim();
      } else {
        const match = line.match(/b\/(.*)$/);
        filename = match ? match[1] : "unknown";
      }
      currentDiff = {
        filename,
        diffLines: [line],
      };
      fileDiffs.push(currentDiff);
    } else if (currentDiff) {
      currentDiff.diffLines.push(line);
    }
  }

  return fileDiffs;
}

export default function ProjectTaskDetailPage({
  params,
}: {
  params: Promise<{ id: string; taskID: string }>;
}) {
  const { id: projectID, taskID } = use(params);
  const [activeSpecTab, setActiveSpecTab] = useState<"summary" | "proposal" | "specs" | "design" | "tasks">("summary");
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
    startReview,
  } = useTaskWorkflow(taskID);

  // Fetch live workspace artifacts for the active workflow job
  const jobID = workflow?.job?.id;
  const { data: artifacts } = useAuthedSWR(
    jobID ? ["workflow-artifacts", jobID] : null,
    (token) => api.taskArtifacts(jobID!, token),
  );

  // Find the latest diff/patch artifact from the runner run
  const latestDiffArtifact = useMemo(() => {
    if (!artifacts) return null;
    const diffArts = artifacts.filter((art: WorkflowArtifact) => art.type === "diff" || art.type === "patch");
    return diffArts.length > 0 ? diffArts[diffArts.length - 1] : null;
  }, [artifacts]);

  const parsedDiffs = useMemo(() => {
    if (!latestDiffArtifact) return [];
    let diffText = "";
    const payload = latestDiffArtifact.payload;
    if (typeof payload === "string") {
      diffText = payload;
    } else if (payload && typeof payload === "object") {
      const obj = payload as Record<string, unknown>;
      diffText = (typeof obj.diff === "string" ? obj.diff : "") || 
                 (typeof obj.patch === "string" ? obj.patch : "") || 
                 JSON.stringify(payload);
    }
    return parseUnifiedDiff(diffText);
  }, [latestDiffArtifact]);

  const parsedDiffFiles = useMemo(() => {
    return parsedDiffs.map((d) => d.filename);
  }, [parsedDiffs]);

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
      risk_domains?: string[];
      proposal_md?: string;
      specs_md?: string;
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

  const prSummaries = useMemo(() => {
    if (!task?.pr_metadata) return [];
    try {
      const metadata = typeof task.pr_metadata === "string" ? JSON.parse(task.pr_metadata) : task.pr_metadata;
      if (Array.isArray(metadata)) {
        return metadata;
      }
    } catch {}
    return [];
  }, [task?.pr_metadata]);

  // Prefer actual parsed diff files from git, fallback to analysis estimation
  const displayFiles = useMemo<string[]>(() => {
    if (prSummaries.length > 0 && prSummaries[0].changed_files) {
      return prSummaries[0].changed_files as string[];
    }
    const affectedFiles = analysisData.affected_files || [];
    return parsedDiffFiles.length > 0 ? parsedDiffFiles : affectedFiles;
  }, [prSummaries, parsedDiffFiles, analysisData.affected_files]);

  const selectedFile = rawSelectedFile || displayFiles[0] || null;

  const riskAssessment = useMemo(() => {
    if (prSummaries.length > 0 && prSummaries[0].risk_level) {
      return {
        level: prSummaries[0].risk_level,
        reason: prSummaries[0].risk_reason || "",
      };
    }
    return getRiskAssessment(task?.complexity ?? "easy", displayFiles, analysisData.risk_domains);
  }, [prSummaries, task?.complexity, displayFiles, analysisData.risk_domains]);

  const activeFileDiff = useMemo(() => {
    return parsedDiffs.find((d) => d.filename === selectedFile);
  }, [parsedDiffs, selectedFile]);

  const isReviewWaiting = task?.status === "human_review";
  const isPRMerged = task?.status === "merged";

  return (
    <main className="min-h-screen bg-background text-content font-sans p-6 md:p-8 max-w-7xl mx-auto space-y-6">
      <header className="flex flex-col justify-between gap-4 border-b border-stroke pb-5 md:flex-row md:items-end">
        <div>
          <Link
            href={`/projects/${projectID}`}
            className="mb-3 inline-flex items-center gap-1.5 text-xs font-semibold text-content-muted transition hover:text-foreground"
          >
            <ArrowLeft size={14} />
            Back to Project
          </Link>
          <h1 className="font-heading text-2xl md:text-3xl font-bold tracking-tight text-foreground">
            {task?.title ?? "Task workflow"}
          </h1>
          <p className="mt-1.5 max-w-4xl text-sm text-content-muted leading-relaxed">
            {task?.description ?? "Loading task details..."}
          </p>
        </div>
        <div className="flex flex-wrap gap-2">
          {task && task.status === "pr_ready" && (
            <button
              className="inline-flex items-center gap-2 rounded-md border border-brand-primary bg-transparent px-4 py-2 text-sm font-semibold text-brand-primary transition hover:bg-brand-primary/10 shadow-sm cursor-pointer"
              onClick={startReview}
              type="button"
              disabled={submittingPR}
            >
              <Sparkles size={15} />
              Start Review
            </button>
          )}
          {task &&
            (task.spec_status === "approved" ||
              task.spec_status === "auto_approved" ||
              task.status === "todo" ||
              task.status === "approved") && (
              <button
                className="inline-flex items-center gap-2 rounded-md bg-brand-primary px-4 py-2 text-sm font-semibold text-slate-950 transition hover:opacity-90 shadow-sm cursor-pointer"
                onClick={execute}
                type="button"
              >
                <Play size={15} fill="currentColor" />
                Execute DAG
              </button>
            )}
        </div>
      </header>

      {error && (
        <div className="rounded-lg border border-red-500/20 bg-red-500/10 p-4 text-sm text-red-700 dark:text-red-300 flex items-center gap-2" role="alert">
          <AlertCircle size={16} className="shrink-0" />
          {error}
        </div>
      )}

      {workflow?.job?.status === "failed" && workflow?.job?.last_error && (
        <div className="rounded-lg border border-rose-500/20 bg-rose-500/10 p-4 text-sm text-rose-700 dark:text-rose-300 flex flex-col gap-1.5" role="alert">
          <div className="flex items-center gap-2 font-semibold">
            <AlertCircle size={16} className="shrink-0 text-rose-500" />
            Task Execution Failed
          </div>
          <p className="text-xs font-mono bg-black/40 border border-stroke/50 rounded-lg p-3 break-all whitespace-pre-wrap">
            {workflow.job.last_error}
          </p>
        </div>
      )}

      <SpecReviewSection
        specStatus={task?.spec_status}
        onRequestChanges={requestSpecChanges}
        onApproveSpec={approveSpec}
      />

      {/* Pull Request & Review Center */}
      {(task?.status === "pr_ready" || isReviewWaiting || isPRMerged) && (
        <section className="rounded-xl border border-stroke bg-card overflow-hidden shadow-sm">
          {/* PR Header Banner */}
          <div className="border-b border-stroke bg-surface/40 p-5 flex flex-wrap items-center justify-between gap-4">
            <div className="flex items-center gap-3">
              <div className="grid size-9 place-items-center rounded-lg bg-brand-primary/10 text-brand-primary border border-brand-primary/20">
                <GitPullRequest size={18} />
              </div>
              <div>
                <div className="font-mono text-[10px] font-bold uppercase tracking-wider text-content-muted">Pull Request Details</div>
                <h2 className="font-sans font-bold text-base text-foreground mt-0.5">
                  {prSummaries[0]?.title || `AutoCodeOS: ${task?.title}`}
                </h2>
              </div>
            </div>
            <span
              className={`inline-flex rounded-full px-2.5 py-0.5 text-[10px] font-bold font-sans uppercase border ${
                isPRMerged
                  ? "bg-emerald-500/10 text-emerald-600 dark:text-emerald-400 border-emerald-500/25"
                  : task?.status === "pr_ready"
                  ? "bg-purple-500/10 text-purple-600 dark:text-purple-400 border-purple-500/25 animate-pulse"
                  : "bg-yellow-500/10 text-yellow-600 dark:text-yellow-400 border-yellow-500/25 animate-pulse"
              }`}
            >
              {isPRMerged ? "Merged" : task?.status === "pr_ready" ? "PR Ready" : "Awaiting Review"}
            </span>
          </div>

          {prSummaries[0]?.review_limit_exceeded && (
            <div className="border-b border-amber-500/25 bg-amber-500/5 px-5 py-3 text-xs text-amber-700 dark:text-amber-300 flex items-center gap-2">
              <AlertCircle size={15} className="text-amber-500 shrink-0" />
              <span>Review carefully before approving. This task has reached the maximum review-fix cycles limit. Next rejection will mark the task as failed.</span>
            </div>
          )}

          <div className="grid lg:grid-cols-[340px_1fr] border-b border-stroke min-h-[420px]">
            {/* PR Summary Sidebar */}
            <div className="border-r border-stroke p-5 space-y-5 bg-surface/10">
              <div>
                <h3 className="font-mono text-[10px] font-bold uppercase tracking-wider text-content-muted mb-2 flex items-center gap-1">
                  <Sparkles size={12} className="text-brand-primary" /> AI PR Summary
                </h3>
                {prSummaries.length > 0 && prSummaries[0].body ? (
                  <div className="text-xs leading-relaxed text-content-muted prose dark:prose-invert max-h-60 overflow-y-auto pr-1">
                    <Markdown content={prSummaries[0].body} />
                  </div>
                ) : (
                  <p className="text-xs leading-relaxed text-content-muted">
                    Automated changes generated for this execution run. The agent completed the code-backend,
                    code-frontend, and successfully compiled all builds.
                  </p>
                )}
              </div>

              <div>
                <h3 className="font-mono text-[10px] font-bold uppercase tracking-wider text-content-muted mb-2">
                  Risk Assessment
                </h3>
                <div className={`rounded-lg border p-3.5 ${RISK_BADGES[riskAssessment.level]} space-y-2`}>
                  <div className="flex items-center justify-between mb-1">
                    <span className="font-sans text-[10px] font-bold uppercase tracking-wider">
                      Level: {riskAssessment.level}
                    </span>
                  </div>
                  <p className="text-[11px] leading-relaxed opacity-90">{riskAssessment.reason}</p>
                  <div className="pt-1.5 border-t border-current/10">
                    <span className="font-mono text-[9px] font-bold uppercase tracking-wider opacity-85">
                      Risk Domains: {analysisData.risk_domains?.join(", ") || "none"}
                    </span>
                  </div>
                </div>
              </div>

              <div>
                <h3 className="font-mono text-[10px] font-bold uppercase tracking-wider text-content-muted mb-2">
                  Changed Files ({displayFiles.length})
                </h3>
                <div className="space-y-1 max-h-52 overflow-y-auto pr-1">
                  {displayFiles.map((file) => (
                    <button
                      key={file}
                      onClick={() => setSelectedFile(file)}
                      className={`flex w-full items-center justify-between rounded-md px-2.5 py-1.5 text-left text-xs font-mono transition-all cursor-pointer border ${
                        selectedFile === file
                          ? "bg-brand-primary/10 border-brand-primary/20 text-brand-primary"
                          : "border-transparent text-content-muted hover:bg-surface hover:text-foreground"
                      }`}
                    >
                      <span className="truncate">{file.split("/").pop()}</span>
                      <span className="text-[9px] opacity-65 truncate max-w-[100px]">{file}</span>
                    </button>
                  ))}
                  {displayFiles.length === 0 && (
                    <p className="text-xs text-content-muted italic">No file modifications detected.</p>
                  )}
                </div>
              </div>
            </div>

            {/* Interactive Git Diff Review Canvas */}
            <div className="p-5 flex flex-col min-w-0 bg-surface/5">
              <div className="mb-3 flex items-center justify-between">
                <span className="font-mono text-[11px] text-content-muted truncate max-w-[80%]">
                  Diff Review &mdash;{" "}
                  <span className="text-foreground font-semibold">{selectedFile || "Select a file"}</span>
                </span>
                <span className="text-[9px] bg-surface border border-stroke text-content-muted px-2 py-0.5 rounded font-mono uppercase">
                  Git Diff
                </span>
              </div>

              <div className="flex-1 min-h-[350px] overflow-auto rounded-lg border border-stroke bg-slate-950 dark:bg-black p-4 font-mono text-xs leading-relaxed shadow-inner">
                {selectedFile ? (
                  activeFileDiff ? (
                    <div className="space-y-0.5 font-mono text-[11px] text-foreground select-text">
                      {activeFileDiff.diffLines.map((line, idx) => {
                        let lineClass = "text-slate-400";
                        if (line.startsWith("+") && !line.startsWith("+++")) {
                          lineClass = "bg-emerald-500/15 text-emerald-400 px-1 border-l-2 border-emerald-500";
                        } else if (line.startsWith("-") && !line.startsWith("---")) {
                          lineClass = "bg-rose-500/15 text-rose-400 px-1 border-l-2 border-rose-500";
                        } else if (line.startsWith("@@")) {
                          lineClass = "text-purple-400 bg-purple-500/10 font-semibold py-0.5";
                        } else if (line.startsWith("diff ") || line.startsWith("index ")) {
                          lineClass = "text-slate-500/80 italic";
                        } else if (line.startsWith("--- ") || line.startsWith("+++ ")) {
                          lineClass = "text-slate-400 font-semibold";
                        } else {
                          lineClass = "text-slate-300 pl-1.5";
                        }
                        return (
                          <div key={idx} className={`font-mono whitespace-pre-wrap ${lineClass}`}>
                            {line}
                          </div>
                        );
                      })}
                    </div>
                  ) : (
                    <div className="h-full flex flex-col items-center justify-center text-content-muted py-12">
                      <p className="text-sm">Diff details not available inside sandbox state.</p>
                      <p className="text-[10px] mt-1">Review live branch changes via your Git provider.</p>
                    </div>
                  )
                ) : (
                  <div className="h-full flex items-center justify-center text-content-muted py-12">
                    Select a file on the left to inspect git changes.
                  </div>
                )}
              </div>
            </div>
          </div>

          {/* Action Footer */}
          {(isReviewWaiting || task?.status === "pr_ready") && (
            <div className="p-5 bg-surface/40 flex flex-col gap-4">
              {task?.status === "pr_ready" ? (
                <div className="flex justify-end">
                  <button
                    onClick={startReview}
                    disabled={submittingPR}
                    className="inline-flex items-center gap-1.5 rounded-md border border-brand-primary bg-transparent px-4 py-2 text-sm font-semibold text-brand-primary hover:bg-brand-primary/10 transition cursor-pointer disabled:opacity-50"
                  >
                    <Sparkles size={15} />
                    Start Review
                  </button>
                </div>
              ) : (
                <>
                  <div className="flex items-start gap-3">
                    <MessageSquare size={16} className="text-content-muted mt-2.5 shrink-0" />
                    <textarea
                      value={feedback}
                      onChange={(e) => setFeedback(e.target.value)}
                      placeholder="Leave rejection feedback to trigger a fix cycle..."
                      className="flex-1 rounded-lg border border-stroke bg-surface p-3 text-sm text-foreground placeholder-content-muted/50 focus:border-brand-primary focus:outline-none min-h-[90px] transition-colors"
                    />
                  </div>
                  <div className="flex justify-end gap-2.5">
                    <button
                      onClick={rejectPR}
                      disabled={submittingPR || !feedback.trim()}
                      className="inline-flex items-center gap-1.5 rounded-md border border-orange-500/20 bg-orange-500/5 px-4 py-2 text-sm font-semibold text-orange-600 hover:bg-orange-500/10 transition cursor-pointer disabled:opacity-50"
                    >
                      <AlertCircle size={15} />
                      Reject &amp; Request Fixes
                    </button>
                    <button
                      onClick={approvePR}
                      disabled={submittingPR}
                      className="inline-flex items-center gap-1.5 rounded-md bg-brand-primary px-4 py-2 text-sm font-semibold text-slate-950 transition hover:opacity-90 disabled:opacity-50 cursor-pointer shadow-sm"
                    >
                      <Check size={15} />
                      Approve &amp; Merge
                    </button>
                  </div>
                </>
              )}
            </div>
          )}
        </section>
      )}

      <div className="grid gap-6 xl:grid-cols-[1fr_380px]">
        {/* Main Details and Spec Section */}
        <section className="space-y-6">
          {task?.analysis && (
            <div className="rounded-xl border border-stroke bg-card p-5 shadow-sm">
              <div className="mb-4 flex flex-wrap items-center justify-between gap-4 border-b border-stroke pb-3">
                <div className="flex items-center gap-2">
                  <Sparkles size={18} className="text-brand-primary" />
                  <h2 className="font-heading text-base font-bold text-foreground">
                    Proposed Task Specification
                  </h2>
                </div>
                {(analysisData.proposal_md || analysisData.specs_md || analysisData.design_md || analysisData.tasks_md) && (
                  <div className="flex gap-1 bg-surface p-1 rounded-md border border-stroke text-xs font-mono">
                    <button
                      onClick={() => setActiveSpecTab("summary")}
                      className={`px-2.5 py-1 rounded transition-colors cursor-pointer ${
                        activeSpecTab === "summary" ? "bg-brand-primary text-slate-950 font-semibold" : "text-content-muted hover:text-foreground"
                      }`}
                    >
                      Summary
                    </button>
                    {analysisData.proposal_md && (
                      <button
                        onClick={() => setActiveSpecTab("proposal")}
                        className={`px-2.5 py-1 rounded transition-colors cursor-pointer ${
                          activeSpecTab === "proposal" ? "bg-brand-primary text-slate-950 font-semibold" : "text-content-muted hover:text-foreground"
                        }`}
                      >
                        proposal.md
                      </button>
                    )}
                    {analysisData.specs_md && (
                      <button
                        onClick={() => setActiveSpecTab("specs")}
                        className={`px-2.5 py-1 rounded transition-colors cursor-pointer ${
                          activeSpecTab === "specs" ? "bg-brand-primary text-slate-950 font-semibold" : "text-content-muted hover:text-foreground"
                        }`}
                      >
                        specs.md
                      </button>
                    )}
                    {analysisData.design_md && (
                      <button
                        onClick={() => setActiveSpecTab("design")}
                        className={`px-2.5 py-1 rounded transition-colors cursor-pointer ${
                          activeSpecTab === "design" ? "bg-brand-primary text-slate-950 font-semibold" : "text-content-muted hover:text-foreground"
                        }`}
                      >
                        design.md
                      </button>
                    )}
                    {analysisData.tasks_md && (
                      <button
                        onClick={() => setActiveSpecTab("tasks")}
                        className={`px-2.5 py-1 rounded transition-colors cursor-pointer ${
                          activeSpecTab === "tasks" ? "bg-brand-primary text-slate-950 font-semibold" : "text-content-muted hover:text-foreground"
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
                      <h3 className="font-mono text-[10px] font-bold uppercase tracking-wider text-content-muted mb-1">
                        Scope
                      </h3>
                      <p className="text-sm leading-relaxed text-foreground">{analysisData.scope}</p>
                    </div>
                  )}

                  {analysisData.risk_domains && analysisData.risk_domains.length > 0 && (
                    <div>
                      <h3 className="font-mono text-[10px] font-bold uppercase tracking-wider text-content-muted mb-2">
                        Risk Domains
                      </h3>
                      <div className="flex flex-wrap gap-1.5">
                        {analysisData.risk_domains.map((domain) => (
                          <span
                            key={domain}
                            className="rounded-full border border-amber-500/20 bg-amber-500/10 px-2.5 py-0.5 text-[10px] font-semibold text-amber-600 dark:text-amber-400"
                          >
                            {domain}
                          </span>
                        ))}
                      </div>
                    </div>
                  )}

                  {analysisData.clarification_questions && analysisData.clarification_questions.length > 0 && (
                    <div className="rounded-lg border border-amber-500/20 bg-amber-500/5 p-4 space-y-1.5">
                      <h3 className="font-sans text-xs font-bold text-amber-700 dark:text-amber-400 flex items-center gap-1.5">
                        <AlertCircle size={14} />
                        Questions / Clarifications Required
                      </h3>
                      <ul className="list-disc list-inside space-y-1 text-xs text-amber-800 dark:text-amber-300">
                        {analysisData.clarification_questions.map((q, idx) => (
                          <li key={idx}>{q}</li>
                        ))}
                      </ul>
                    </div>
                  )}

                  <div className="grid md:grid-cols-2 gap-5 pt-2">
                    {analysisData.execution_plan && analysisData.execution_plan.length > 0 && (
                      <div>
                        <h3 className="font-mono text-[10px] font-bold uppercase tracking-wider text-content-muted mb-2.5 flex items-center gap-1.5">
                          <Check size={14} className="text-brand-primary" />
                          Interactive Execution Plan
                        </h3>
                        <div className="space-y-1.5 max-h-[200px] overflow-y-auto pr-1">
                          {analysisData.execution_plan.map((step, idx) => {
                            const isDone = !!completedPlanSteps[idx];
                            return (
                              <label
                                key={idx}
                                className={`flex items-start gap-2.5 rounded-lg border p-2.5 transition cursor-pointer select-none ${
                                  isDone
                                    ? "border-emerald-500/20 bg-emerald-500/5 text-content-muted line-through"
                                    : "border-stroke bg-card hover:border-brand-primary/50 text-content"
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

                    <div className="space-y-4">
                      {analysisData.risks && analysisData.risks.length > 0 && (
                        <div>
                          <h3 className="font-mono text-[10px] font-bold uppercase tracking-wider text-content-muted mb-2">
                            Risks
                          </h3>
                          <ul className="space-y-1.5">
                            {analysisData.risks.map((risk, idx) => (
                              <li key={idx} className="flex items-start gap-2 text-xs text-content-muted">
                                <span className="mt-1.5 size-1.5 shrink-0 rounded-full bg-amber-500" />
                                <span className="leading-5">{risk}</span>
                              </li>
                            ))}
                          </ul>
                        </div>
                      )}

                      {analysisData.affected_files && analysisData.affected_files.length > 0 && (
                        <div>
                          <h3 className="font-mono text-[10px] font-bold uppercase tracking-wider text-content-muted mb-2">
                            Estimated Affected Files
                          </h3>
                          <div className="flex flex-wrap gap-1.5">
                            {analysisData.affected_files.map((file) => (
                              <span
                                key={file}
                                className="rounded border border-stroke bg-surface px-2 py-0.5 font-mono text-[10px] text-content-muted"
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
                <div className="rounded-lg border border-stroke bg-card p-5 overflow-auto max-h-[500px] leading-relaxed animate-fade-in shadow-inner text-sm">
                  {activeSpecTab === "proposal" && <Markdown content={analysisData.proposal_md || ""} />}
                  {activeSpecTab === "specs" && <Markdown content={analysisData.specs_md || ""} />}
                  {activeSpecTab === "design" && <Markdown content={analysisData.design_md || ""} />}
                  {activeSpecTab === "tasks" && <Markdown content={analysisData.tasks_md || ""} />}
                </div>
              )}
            </div>
          )}

          {/* Workflow Progress Panel */}
          <div className="rounded-xl border border-stroke bg-card p-5 shadow-sm">
            <div className="mb-4 flex flex-wrap items-center gap-2">
              <h2 className="font-heading text-base font-bold text-foreground">Workflow Progress</h2>
              {task && <Badge value={task.status} />}
              {task && <Badge value={task.spec_status} />}
              {workflow?.job && <Badge value={workflow.job.status} />}
            </div>
            <div className="grid gap-3 sm:grid-cols-2 md:grid-cols-3">
              {workflowSteps.map((step) => {
                const status = latest.get(step) ?? "pending";
                return (
                  <div key={step} className="rounded-lg border border-stroke bg-surface/30 p-3">
                    <div className="mb-1.5 flex items-center justify-between">
                      <span className="font-mono text-xs font-semibold capitalize text-foreground">{step.replace("_", " ")}</span>
                      <WorkflowDot status={status} />
                    </div>
                    <div className="text-[10px] font-bold uppercase tracking-wide text-content-muted">{status}</div>
                  </div>
                );
              })}
            </div>
          </div>

          <LogConsole logs={logs} />
        </section>

        {/* Sidebar Status Info */}
        <aside className="space-y-6">
          <div className="rounded-xl border border-stroke bg-card p-5 shadow-sm">
            <div className="mb-4 flex items-center gap-2 border-b border-stroke pb-3">
              <Bot size={16} className="text-brand-primary" />
              <h2 className="font-heading text-base font-bold text-foreground">Agent Activity</h2>
            </div>
            <dl className="space-y-3.5 text-xs">
              <InfoRow label="Assigned agent" value={workflow?.job?.agent_id ?? task?.agent_id ?? "Unassigned"} />
              <InfoRow label="Current step" value={workflow?.job?.step ?? "none"} />
              <InfoRow label="Attempts" value={String(workflow?.job?.attempts ?? 0)} />
              <InfoRow label="Last error" value={workflow?.job?.last_error || "none"} />
            </dl>
          </div>

          <div className="rounded-xl border border-stroke bg-card p-5 shadow-sm">
            <div className="mb-4 flex items-center gap-2 border-b border-stroke pb-3">
              <Clock size={16} className="text-brand-primary" />
              <h2 className="font-heading text-base font-bold text-foreground">Checkpoints</h2>
            </div>
            <div className="space-y-2 max-h-[300px] overflow-y-auto pr-1">
              {(workflow?.checkpoints ?? [])
                .slice()
                .reverse()
                .map((checkpoint) => (
                  <div key={checkpoint.id} className="rounded-lg border border-stroke bg-surface/20 p-3">
                    <div className="font-mono text-xs font-bold text-brand-primary capitalize">{checkpoint.step.replace("_", " ")}</div>
                    <div className="text-[10px] text-content-muted mt-0.5">{new Date(checkpoint.created_at).toLocaleString()}</div>
                  </div>
                ))}
              {(workflow?.checkpoints ?? []).length === 0 && (
                <p className="text-xs text-content-muted italic">No checkpoints recorded.</p>
              )}
            </div>
          </div>
        </aside>
      </div>

      {isRequestingChanges && (
        <div className="fixed inset-0 z-50 flex items-center justify-center bg-slate-950/80 backdrop-blur-sm p-4 animate-fade-in">
          <div className="relative w-full max-w-lg rounded-xl border border-stroke bg-card p-6 shadow-2xl animate-scale-up space-y-4">
            <div className="flex items-center gap-2.5 text-amber-500 border-b border-stroke pb-3">
              <MessageSquare size={20} />
              <h2 className="font-heading text-lg font-bold text-foreground">Request Specification Changes</h2>
            </div>

            <p className="text-sm text-content-muted leading-relaxed">
              Describe what adjustments are needed for this task&apos;s specification, plan, or scope before starting execution.
            </p>

            <textarea
              className="w-full h-32 rounded-lg border border-stroke bg-surface p-3 font-mono text-sm text-foreground placeholder-content-muted/50 outline-none focus:border-brand-primary transition-all duration-200 resize-none"
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
                className="rounded-md border border-stroke bg-transparent px-4 py-2 text-sm font-semibold text-foreground hover:bg-surface transition cursor-pointer disabled:opacity-50"
              >
                Cancel
              </button>
              <button
                type="button"
                onClick={submitSpecChanges}
                disabled={submittingPR || !specFeedbackText.trim()}
                className="rounded-md bg-brand-primary px-4 py-2 text-sm font-semibold text-slate-950 hover:opacity-90 transition cursor-pointer disabled:opacity-50 flex items-center gap-1.5"
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
      ? "bg-emerald-500 shadow-sm shadow-emerald-500/30"
      : status === "running"
      ? "bg-sky-500 animate-pulse"
      : status === "paused"
      ? "bg-amber-500"
      : status === "failed"
      ? "bg-rose-500 shadow-sm shadow-rose-500/30"
      : "bg-slate-400";
  return <span className={`size-2.5 rounded-full ${color}`} />;
}

function InfoRow({ label, value }: { label: string; value: string }) {
  return (
    <div>
      <dt className="text-[10px] font-bold uppercase tracking-wider text-content-muted">{label}</dt>
      <dd className="mt-1 break-all font-mono text-[11px] text-foreground bg-surface/30 border border-stroke/60 rounded px-2 py-1">{value}</dd>
    </div>
  );
}
