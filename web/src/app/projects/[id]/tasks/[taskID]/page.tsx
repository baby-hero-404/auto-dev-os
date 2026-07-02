"use client";

import Link from "next/link";
import { useRouter } from "next/navigation";
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
  Trash2,
  Edit2,
  X,
  Loader2,
} from "lucide-react";
import { Badge } from "@/components/ui/badge";
import { ConfirmDialog } from "@/components/ui/confirm-dialog";
import { useTaskWorkflow } from "@/lib/hooks/use-task-workflow";
import { Markdown } from "@/components/ui/markdown";
import { WorkflowArtifact } from "@/lib/types";
import { SpecReviewSection } from "@/components/projects/spec-review-section";
import { LogConsole } from "@/components/dashboard/log-console";
import { getRiskAssessment, splitTaskDescription } from "@/lib/utils/task-utils";
import { TaskDiffViewer, parseUnifiedDiff } from "@/components/projects/task-diff-viewer";
import { TaskPrReview } from "@/components/projects/task-pr-review";
import { TaskClarificationForm } from "@/components/projects/task-clarification-form";
import { useAuthedSWR } from "@/lib/use-authed-swr";
import { api } from "@/lib/api";
import { useSession } from "@/lib/session";

const EASY_STEPS = [
  "context_load",
  "analyze",
  "code_backend",
  "test",
  "pr",
];

const STANDARD_STEPS = [
  "context_load",
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



export default function ProjectTaskDetailPage({
  params,
}: {
  params: Promise<{ id: string; taskID: string }>;
}) {
  const { id: projectID, taskID } = use(params);
  const [activeSpecTab, setActiveSpecTab] = useState<"summary" | "proposal" | "specs" | "design" | "tasks">("summary");
  const [completedPlanSteps, setCompletedPlanSteps] = useState<Record<number, boolean>>({});
  const session = useSession();
  const token = session?.token ?? "";

  const router = useRouter();
  const [isDeleting, setIsDeleting] = useState(false);

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
    analyze,
    retry,
    approveSpec,
    requestSpecChanges,
    submitSpecChanges,
    approvePR,
    rejectPR,
    startReview,
    deleteTask,
    updateTask,
    mutateWorkflow,
    isLoading: isTaskLoading,
    workflowError,
  } = useTaskWorkflow(taskID);

  const [isEditingTitle, setIsEditingTitle] = useState(false);
  const [isEditingDesc, setIsEditingDesc] = useState(false);
  const [editedTitle, setEditedTitle] = useState("");
  const [editedDesc, setEditedDesc] = useState("");
  const [isSaving, setIsSaving] = useState(false);
  const [isDeleteConfirmOpen, setIsDeleteConfirmOpen] = useState(false);

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

  const diffText = useMemo(() => {
    if (!latestDiffArtifact) return "";
    const payload = latestDiffArtifact.payload;
    if (typeof payload === "string") return payload;
    if (payload && typeof payload === "object") {
      const obj = payload as Record<string, unknown>;
      return (typeof obj.diff === "string" ? obj.diff : "") || 
             (typeof obj.patch === "string" ? obj.patch : "") || 
             JSON.stringify(payload);
    }
    return "";
  }, [latestDiffArtifact]);

  const parsedDiffs = useMemo(() => {
    return parseUnifiedDiff(diffText);
  }, [diffText]);

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

  const workflowSteps = useMemo(() => {
    if (task?.complexity === "easy") {
      return EASY_STEPS;
    }
    if (task?.complexity === "medium" || task?.complexity === "hard") {
      return STANDARD_STEPS;
    }
    return ["context_load", "analyze"];
  }, [task?.complexity]);

  const workflowCompletion = useMemo(() => {
    const finished = workflowSteps.filter((step) => {
      const status = latest.get(step);
      return status === "success" || status === "recorded";
    }).length;
    return Math.round((finished / workflowSteps.length) * 100);
  }, [latest, workflowSteps]);

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

  const clarificationQuestions = useMemo(() => {
    return Array.isArray(analysisData.clarification_questions)
      ? analysisData.clarification_questions.filter(
          (question): question is string => typeof question === "string" && question.trim().length > 0,
        )
      : [];
  }, [analysisData.clarification_questions]);

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

  const riskAssessment = useMemo(() => {
    if (prSummaries.length > 0 && prSummaries[0].risk_level) {
      return {
        level: prSummaries[0].risk_level,
        reason: prSummaries[0].risk_reason || "",
      };
    }
    return getRiskAssessment(task?.complexity ?? "easy", displayFiles, analysisData.risk_domains);
  }, [prSummaries, task?.complexity, displayFiles, analysisData.risk_domains]);

  const descriptionParts = useMemo(
    () => splitTaskDescription(task?.description ?? ""),
    [task?.description],
  );

  const isReviewWaiting = task?.status === "human_review";
  const isPRMerged = task?.status === "merged";
  const hasPR = !!(task?.pr_urls && task.pr_urls.length > 0);
  const isExecutionReady = !!(
    task &&
    (task.spec_status === "auto_approved" || task.spec_status === "approved") &&
    (task.status === "todo" || task.status === "failed")
  );

  // Early returns can now happen safely AFTER all hook calls
  if (!session) {
    return (
      <main className="grid min-h-screen place-items-center p-6">
        <div className="rounded-lg border border-stroke bg-card p-6">
          <p className="mb-4 text-sm text-content-muted">Login from the dashboard before opening a task.</p>
          <Link className="rounded-md bg-brand-primary px-4 py-2 font-semibold text-slate-950" href="/">Back to login</Link>
        </div>
      </main>
    );
  }

  if (isTaskLoading) {
    return (
      <main className="min-h-screen bg-slate-950 p-6 flex flex-col justify-center items-center gap-4">
        <Loader2 className="h-8 w-8 animate-spin text-brand-primary" />
        <p className="text-sm font-mono text-content-muted animate-pulse">Loading task workspace...</p>
      </main>
    );
  }

  if (workflowError) {
    return (
      <main className="grid min-h-screen place-items-center p-6">
        <div className="rounded-lg border border-red-500/20 bg-red-500/5 p-6 max-w-lg text-center">
          <AlertCircle className="h-10 w-10 text-red-500 mx-auto mb-4" />
          <p className="font-sans text-base font-semibold text-red-600 dark:text-red-400">Failed to load task workflow.</p>
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

  const handleDeleteTask = async () => {
    setIsDeleting(true);
    const success = await deleteTask();
    if (success) {
      router.push(`/projects/${projectID}`);
    } else {
      setIsDeleting(false);
    }
  };

  return (
    <main className="min-h-screen bg-background px-4 py-5 font-sans text-content md:px-8 md:py-7">
      <div className="mx-auto max-w-7xl space-y-6">
        <header className="rounded-xl border border-stroke bg-card p-5 shadow-sm">
          <div className="flex flex-col justify-between gap-5 xl:flex-row xl:items-start">
            {/* Header Content Block */}
            <div className="min-w-0 flex-1">
              <Link
                href={`/projects/${projectID}`}
                className="mb-3 inline-flex items-center gap-1.5 text-xs font-semibold text-content-muted transition hover:text-foreground"
              >
                <ArrowLeft size={14} />
                Back to Project
              </Link>

              {isEditingTitle ? (
                <div className="flex items-center gap-2 mt-1">
                  <input
                    type="text"
                    value={editedTitle}
                    onChange={(e) => setEditedTitle(e.target.value)}
                    className="font-heading text-2xl md:text-3xl font-bold tracking-tight text-foreground bg-surface border border-stroke rounded px-3 py-1 focus:outline-none focus:border-brand-primary min-w-[300px] max-w-xl"
                    disabled={isSaving}
                    autoFocus
                  />
                  <button
                    onClick={async () => {
                      if (!editedTitle.trim()) return;
                      setIsSaving(true);
                      await updateTask({ title: editedTitle.trim() });
                      setIsEditingTitle(false);
                      setIsSaving(false);
                    }}
                    disabled={isSaving}
                    className="p-2 bg-emerald-500/10 hover:bg-emerald-500/20 text-emerald-500 rounded border border-emerald-500/20 transition cursor-pointer"
                    title="Save Title"
                  >
                    <Check size={16} />
                  </button>
                  <button
                    onClick={() => setIsEditingTitle(false)}
                    disabled={isSaving}
                    className="p-2 bg-danger/10 hover:bg-danger/20 text-danger rounded border border-danger/20 transition cursor-pointer"
                    title="Cancel"
                  >
                    <X size={16} />
                  </button>
                </div>
              ) : (
                <h1 className="group flex items-center gap-2 font-heading text-2xl md:text-3xl font-bold tracking-tight text-foreground">
                  <span className="min-w-0 truncate">{task?.title ?? "Task workflow"}</span>
                  {task && (
                    <button
                      onClick={() => {
                        setEditedTitle(task.title);
                        setIsEditingTitle(true);
                      }}
                      className="opacity-40 hover:opacity-100 focus:opacity-100 group-hover:opacity-100 focus-within:opacity-100 p-1 text-content-muted hover:text-foreground hover:bg-surface rounded transition cursor-pointer"
                      title="Edit Title"
                    >
                      <Edit2 size={16} />
                    </button>
                  )}
                </h1>
              )}

              <div className="mt-3 flex flex-wrap items-center gap-2">
                {task && <Badge value={task.status} />}
                {task?.spec_status && <Badge value={task.spec_status} />}
                {workflow?.job && <Badge value={workflow.job.status} />}
                {task && (
                  <span className="rounded border border-stroke bg-surface px-2 py-0.5 text-xs font-medium text-content-muted">
                    Priority {task.priority}
                  </span>
                )}
              </div>

              {isEditingDesc ? (
                <div className="flex flex-col gap-2 mt-3 max-w-4xl">
                  <textarea
                    value={editedDesc}
                    onChange={(e) => setEditedDesc(e.target.value)}
                    className="text-sm text-foreground bg-surface border border-stroke rounded px-3 py-2 focus:outline-none focus:border-brand-primary min-h-[100px] resize-y"
                    disabled={isSaving}
                    autoFocus
                    placeholder="Detail the target objective, files to modify, or technical requirements."
                  />
                  <div className="flex gap-2 justify-end">
                    <button
                      onClick={() => setIsEditingDesc(false)}
                      disabled={isSaving}
                      className="px-3 py-1.5 text-xs font-semibold border border-stroke hover:bg-surface rounded transition cursor-pointer disabled:opacity-50"
                    >
                      Cancel
                    </button>
                    <button
                      onClick={async () => {
                        setIsSaving(true);
                        await updateTask({ description: editedDesc.trim() });
                        setIsEditingDesc(false);
                        setIsSaving(false);
                      }}
                      disabled={isSaving}
                      className="px-3 py-1.5 text-xs font-semibold bg-brand-primary text-slate-950 hover:opacity-90 rounded transition cursor-pointer disabled:opacity-50"
                    >
                      {isSaving ? "Saving..." : "Save Description"}
                    </button>
                  </div>
                </div>
              ) : (
                <div className="group mt-1.5 max-w-4xl space-y-3">
                  <div className="flex items-start gap-2">
                    <div className="min-w-0 flex-1 rounded-lg border border-stroke bg-surface/20 p-3">
                      {descriptionParts.body ? (
                        <div className="prose prose-sm max-w-none text-content-muted dark:prose-invert prose-headings:text-foreground prose-strong:text-foreground prose-p:leading-relaxed prose-li:leading-relaxed">
                          <Markdown content={descriptionParts.body} />
                        </div>
                      ) : (
                        <p className="text-sm text-content-muted/70 italic">No description provided. Click the edit icon to add one.</p>
                      )}
                    </div>
                    {task && (
                      <button
                        onClick={() => {
                          setEditedDesc(task.description || "");
                          setIsEditingDesc(true);
                        }}
                        className="opacity-40 hover:opacity-100 focus:opacity-100 group-hover:opacity-100 focus-within:opacity-100 p-1 text-content-muted hover:text-foreground hover:bg-surface rounded transition shrink-0 cursor-pointer"
                        title="Edit Description"
                      >
                        <Edit2 size={14} />
                      </button>
                    )}
                  </div>
                  {descriptionParts.context && (
                    <div className="rounded-lg border border-amber-500/20 bg-amber-500/5 p-3 text-xs text-content-muted">
                      <div className="mb-2 font-mono text-[10px] font-bold uppercase tracking-wider text-amber-700 dark:text-amber-400">
                        Request history
                      </div>
                      <div className="prose prose-sm max-w-none text-content-muted dark:prose-invert prose-headings:text-foreground prose-strong:text-foreground prose-p:leading-relaxed prose-li:leading-relaxed">
                        <Markdown content={descriptionParts.context} />
                      </div>
                    </div>
                  )}
                </div>
              )}
            </div>

            {/* Sidebar Action Block */}
            <div className="flex shrink-0 flex-col gap-3 rounded-lg border border-stroke bg-background p-3 xl:w-72">
              <div className="flex items-center justify-between gap-3">
                <span className="text-xs font-medium text-content-muted">Workflow progress</span>
                <span className="font-mono text-sm font-semibold text-foreground">{workflowCompletion}%</span>
              </div>
              <div className="h-2 overflow-hidden rounded-full bg-surface">
                <div className="h-full rounded-full bg-brand-primary transition-all" style={{ width: `${workflowCompletion}%` }} />
              </div>
              <div className="grid grid-cols-3 gap-2 text-center text-[11px] text-content-muted">
                <div className="rounded border border-stroke bg-card px-2 py-1">
                  <div className="font-mono text-foreground">{workflow?.checkpoints?.length ?? 0}</div>
                  checkpoints
                </div>
                <div className="rounded border border-stroke bg-card px-2 py-1">
                  <div className="font-mono text-foreground">{workflow?.job?.attempts ?? 0}</div>
                  attempts
                </div>
                <div className="rounded border border-stroke bg-card px-2 py-1">
                  <div className="font-mono text-foreground">{displayFiles.length}</div>
                  files
                </div>
              </div>
              <div className="flex flex-wrap gap-2">
                {task && task.status === "pr_ready" && (
                  <button
                    className="inline-flex flex-1 items-center justify-center gap-2 rounded-md border border-brand-primary bg-transparent px-3 py-2 text-sm font-semibold text-brand-primary transition hover:bg-brand-primary/10 shadow-sm cursor-pointer"
                    onClick={startReview}
                    type="button"
                    disabled={submittingPR}
                  >
                    <Sparkles size={15} />
                    Start Review
                  </button>
                )}
                {task && (task.status === "todo" || task.status === "failed") && !isExecutionReady && (
                  <button
                    className="inline-flex flex-1 items-center justify-center gap-2 rounded-md bg-brand-primary px-3 py-2 text-sm font-semibold text-slate-950 transition hover:opacity-90 shadow-sm cursor-pointer"
                    onClick={task.status === "failed" ? retry : analyze}
                    type="button"
                  >
                    <Play size={15} />
                    {task.status === "failed" ? "Retry Analyze" : "Analyze"}
                  </button>
                )}
                {task && isExecutionReady && (
                  <button
                    className="inline-flex flex-1 items-center justify-center gap-2 rounded-md bg-brand-primary px-3 py-2 text-sm font-semibold text-slate-950 transition hover:opacity-90 shadow-sm cursor-pointer"
                    onClick={task.status === "failed" ? retry : execute}
                    type="button"
                  >
                    <Play size={15} fill="currentColor" />
                    {task.status === "failed" ? "Retry Execute" : "Execute"}
                  </button>
                )}
                {task && (
                  <button
                    className="inline-flex flex-1 items-center justify-center gap-2 rounded-md border border-danger/40 bg-danger/10 px-3 py-2 text-sm font-semibold text-danger transition hover:bg-danger/20 disabled:opacity-50 cursor-pointer shadow-sm"
                    onClick={() => setIsDeleteConfirmOpen(true)}
                    type="button"
                    disabled={isDeleting}
                  >
                    <Trash2 size={15} />
                    Delete Task
                  </button>
                )}
              </div>
            </div>
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

          <TaskDiffViewer
            diffText={diffText}
            displayFiles={displayFiles}
            prSummaries={prSummaries}
            riskAssessment={riskAssessment}
            riskDomains={analysisData.risk_domains || []}
          />

          {/* Action Footer */}
          <TaskPrReview
            task={task}
            hasPR={hasPR}
            isReviewWaiting={isReviewWaiting}
            submittingPR={submittingPR}
            feedback={feedback}
            setFeedback={setFeedback}
            startReview={startReview}
            rejectPR={rejectPR}
            approvePR={approvePR}
          />
        </section>
      )}

      <section className="rounded-xl border border-stroke bg-card p-5 shadow-sm">
        <div className="mb-4 flex flex-wrap items-center justify-between gap-3">
          <div>
            <h2 className="font-heading text-base font-bold text-foreground">Task Flow</h2>
            <p className="mt-1 text-xs text-content-muted">Analysis, implementation, review, testing, and PR gates for this task.</p>
          </div>
          {workflow?.job?.step && (
            <span className="rounded-md border border-brand-primary/25 bg-brand-primary/10 px-2.5 py-1 text-xs font-semibold text-brand-primary">
              Current: {workflow.job.step.replace("_", " ")}
            </span>
          )}
        </div>
        <div className="relative flex w-full items-start justify-between gap-2 overflow-x-auto pb-4 pt-2 hide-scrollbar">
          {/* Connector Line Background */}
          <div className="absolute left-8 right-8 top-6 -z-10 h-[2px] -translate-y-1/2 bg-stroke/50" />
          
          {workflowSteps.map((step, index) => {
            const status = latest.get(step) ?? "pending";
            const isCompleted = status === "success" || status === "recorded";
            const isRunning = status === "running";
            const isFailed = status === "failed";
            
            return (
              <div key={step} className="group relative flex flex-col items-center justify-start gap-2.5 min-w-[70px] flex-1">
                {/* Connecting Line Progress Overlay */}
                {index > 0 && isCompleted && (
                  <div className="absolute right-[50%] top-6 -z-10 h-[2px] w-full bg-brand-primary transition-all duration-500" />
                )}
                
                <div className={`relative z-10 flex size-8 items-center justify-center rounded-full border-2 transition-all duration-300 shadow-sm ${
                  isCompleted ? "border-brand-primary bg-brand-primary/10 text-brand-primary" : 
                  isFailed ? "border-rose-500 bg-rose-500/10 text-rose-500" :
                  isRunning ? "border-sky-500 bg-sky-500/10 text-sky-500 shadow-[0_0_12px_rgba(14,165,233,0.4)] animate-pulse" : 
                  "border-stroke bg-card text-content-muted"
                }`}>
                  {isCompleted ? <Check size={14} strokeWidth={3} /> : <span className="font-mono text-[11px] font-bold">{index + 1}</span>}
                </div>
                
                <div className="text-center w-full px-1">
                  <div className={`text-[10px] font-bold uppercase tracking-wider transition-colors line-clamp-2 leading-tight ${
                    isCompleted || isRunning ? "text-foreground" : "text-content-muted"
                  }`}>
                    {step.replace("_", " ")}
                  </div>
                  <div className="mt-1 text-[9px] font-bold uppercase text-content-muted/70">{status}</div>
                </div>
              </div>
            );
          })}
        </div>
      </section>

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
                  <div className="flex gap-1.5 bg-surface/60 p-1.5 rounded-lg border border-stroke shadow-inner overflow-x-auto hide-scrollbar">
                    <button
                      onClick={() => setActiveSpecTab("summary")}
                      className={`px-3 py-1.5 rounded-md text-[11px] font-bold uppercase tracking-wider transition-all duration-200 cursor-pointer whitespace-nowrap ${
                        activeSpecTab === "summary" ? "bg-card text-brand-primary shadow-sm ring-1 ring-stroke" : "text-content-muted hover:text-foreground hover:bg-card/50"
                      }`}
                    >
                      Summary
                    </button>
                    {analysisData.proposal_md && (
                      <button
                        onClick={() => setActiveSpecTab("proposal")}
                        className={`px-3 py-1.5 rounded-md text-[11px] font-bold uppercase tracking-wider transition-all duration-200 cursor-pointer whitespace-nowrap ${
                          activeSpecTab === "proposal" ? "bg-card text-brand-primary shadow-sm ring-1 ring-stroke" : "text-content-muted hover:text-foreground hover:bg-card/50"
                        }`}
                      >
                        Proposal
                      </button>
                    )}
                    {analysisData.specs_md && (
                      <button
                        onClick={() => setActiveSpecTab("specs")}
                        className={`px-3 py-1.5 rounded-md text-[11px] font-bold uppercase tracking-wider transition-all duration-200 cursor-pointer whitespace-nowrap ${
                          activeSpecTab === "specs" ? "bg-card text-brand-primary shadow-sm ring-1 ring-stroke" : "text-content-muted hover:text-foreground hover:bg-card/50"
                        }`}
                      >
                        Specs
                      </button>
                    )}
                    {analysisData.design_md && (
                      <button
                        onClick={() => setActiveSpecTab("design")}
                        className={`px-3 py-1.5 rounded-md text-[11px] font-bold uppercase tracking-wider transition-all duration-200 cursor-pointer whitespace-nowrap ${
                          activeSpecTab === "design" ? "bg-card text-brand-primary shadow-sm ring-1 ring-stroke" : "text-content-muted hover:text-foreground hover:bg-card/50"
                        }`}
                      >
                        Design
                      </button>
                    )}
                    {analysisData.tasks_md && (
                      <button
                        onClick={() => setActiveSpecTab("tasks")}
                        className={`px-3 py-1.5 rounded-md text-[11px] font-bold uppercase tracking-wider transition-all duration-200 cursor-pointer whitespace-nowrap ${
                          activeSpecTab === "tasks" ? "bg-card text-brand-primary shadow-sm ring-1 ring-stroke" : "text-content-muted hover:text-foreground hover:bg-card/50"
                        }`}
                      >
                        Tasks
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

                  <TaskClarificationForm
                    taskID={taskID}
                    specStatus={task?.spec_status}
                    token={token}
                    clarificationQuestions={clarificationQuestions}
                    onAnswersSubmitted={async () => { await mutateWorkflow(); }}
                  />

                  {task?.spec_status === "changes_requested" && clarificationQuestions.length === 0 && (
                    <div className="rounded-lg border border-sky-500/20 bg-sky-500/5 p-4">
                      <div className="flex items-start gap-2">
                        <AlertCircle size={14} className="mt-0.5 text-sky-500" />
                        <div>
                          <h3 className="text-xs font-bold uppercase tracking-wider text-sky-700 dark:text-sky-400">
                            Spec changes requested
                          </h3>
                          <p className="mt-1 text-xs leading-relaxed text-content-muted">
                            This task was sent back for a spec update, but there were no clarification questions to answer.
                          </p>
                        </div>
                      </div>
                    </div>
                  )}

                  {clarificationQuestions.length > 0 && task?.spec_status === "changes_requested" && (
                    <div className="rounded-lg border border-emerald-500/20 bg-emerald-500/5 p-4">
                      <div className="flex items-start gap-2">
                        <Check size={14} className="mt-0.5 text-emerald-500" />
                        <div>
                          <h3 className="text-xs font-bold uppercase tracking-wider text-emerald-700 dark:text-emerald-400">
                            Clarification responses submitted
                          </h3>
                          <p className="mt-1 text-xs leading-relaxed text-content-muted">
                            Your answers were recorded as change requests. The task now waits for a new spec review decision.
                          </p>
                        </div>
                      </div>
                    </div>
                  )}

                  <div className="grid md:grid-cols-2 gap-5 pt-2">
                    {analysisData.execution_plan && analysisData.execution_plan.length > 0 && (
                      <div>
                        <h3 className="font-mono text-[10px] font-bold uppercase tracking-wider text-content-muted mb-2.5 flex items-center gap-1.5">
                          <Check size={14} className="text-brand-primary" />
                          Interactive Execution Plan
                        </h3>
                        <div className="space-y-2 max-h-[350px] overflow-y-auto pr-2 custom-scrollbar">
                          {analysisData.execution_plan.map((step, idx) => {
                            const isDone = !!completedPlanSteps[idx];
                            return (
                              <label
                                key={idx}
                                className={`group flex items-start gap-3 rounded-xl border p-3.5 transition-all duration-300 cursor-pointer select-none relative overflow-hidden ${
                                  isDone
                                    ? "border-emerald-500/30 bg-emerald-500/10 text-content-muted shadow-sm"
                                    : "border-stroke bg-surface hover:border-brand-primary/50 text-foreground hover:shadow-md hover:bg-surface/80"
                                }`}
                              >
                                <input
                                  type="checkbox"
                                  checked={isDone}
                                  onChange={() => togglePlanStep(idx)}
                                  className="hidden"
                                />
                                <div className={`mt-0.5 flex size-5 shrink-0 items-center justify-center rounded-[6px] border transition-all duration-300 ${
                                  isDone ? "bg-emerald-500 border-emerald-500 text-slate-950 scale-110" : "border-stroke/80 bg-background group-hover:border-brand-primary group-hover:bg-brand-primary/10"
                                }`}>
                                  {isDone && <Check size={14} strokeWidth={3.5} />}
                                </div>
                                <div className={`flex-1 text-sm leading-relaxed transition-all duration-300 [&_p]:mb-0 ${isDone ? "line-through opacity-70" : ""}`}>
                                  <Markdown content={step} />
                                </div>
                                {isDone && <div className="absolute inset-0 bg-gradient-to-r from-emerald-500/0 via-emerald-500/5 to-emerald-500/0 opacity-0 group-hover:opacity-100 transition-opacity pointer-events-none" />}
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
            <div className="relative space-y-4 max-h-[350px] overflow-y-auto pr-2 custom-scrollbar pl-2 mt-2">
              <div className="absolute left-3.5 top-2 bottom-2 w-[2px] bg-stroke/60 rounded-full" />
              {(workflow?.checkpoints ?? [])
                .slice()
                .reverse()
                .map((checkpoint, index) => (
                  <div key={checkpoint.id} className="relative pl-7 group">
                    <div className="absolute left-[-2px] top-2.5 size-[11px] rounded-full border-2 border-card bg-brand-primary ring-2 ring-transparent group-hover:ring-brand-primary/30 transition-all" />
                    <div className="rounded-lg border border-stroke bg-surface/40 p-2.5 hover:bg-surface/80 transition-colors shadow-sm">
                      <div className="font-mono text-[11px] font-bold text-brand-primary capitalize tracking-wide">{checkpoint.step.replace("_", " ")}</div>
                      <div className="text-[10px] text-content-muted mt-1 font-medium">{new Date(checkpoint.created_at).toLocaleString()}</div>
                    </div>
                  </div>
                ))}
              {(workflow?.checkpoints ?? []).length === 0 && (
                <p className="text-xs text-content-muted italic pl-6">No checkpoints recorded.</p>
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
      </div>

      <ConfirmDialog
        isOpen={isDeleteConfirmOpen}
        title="Delete Task"
        description="Are you sure you want to delete this task? This action cannot be undone."
        confirmText="Delete"
        variant="danger"
        onConfirm={handleDeleteTask}
        onClose={() => setIsDeleteConfirmOpen(false)}
      />
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
