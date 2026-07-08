"use client";

import { GitPullRequest, AlertCircle } from "lucide-react";
import { TaskDiffViewer } from "@/components/projects/task-diff-viewer";
import { TaskPrReview } from "@/components/projects/task-pr-review";
import { useTaskDetail } from "./TaskDetailContext";

export function PRPanel() {
  const {
    task,
    hasPR,
    isReviewWaiting,
    isPRMerged,
    submittingPR,
    feedback,
    setFeedback,
    startReview,
    rejectPR,
    approvePR,
    prSummaries,
    diffText,
    displayFiles,
    riskAssessment,
    analysisData,
  } = useTaskDetail();

  const isVisible = task?.status === "pr_ready" || isReviewWaiting || isPRMerged;
  if (!isVisible) return null;

  const prTitle = prSummaries[0]?.title || `AutoCodeOS: ${task?.title}`;
  const isLimitExceeded = !!prSummaries[0]?.review_limit_exceeded;
  const riskDomains = analysisData.risk_domains || [];

  return (
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
              {prTitle}
            </h2>
          </div>
        </div>
        <span
          className={`inline-flex rounded-full px-2.5 py-0.5 text-[10px] font-bold font-sans uppercase border ${isPRMerged
            ? "bg-emerald-500/10 text-emerald-600 dark:text-emerald-400 border-emerald-500/25"
            : task?.status === "pr_ready"
              ? "bg-purple-500/10 text-purple-600 dark:text-purple-400 border-purple-500/25 animate-pulse"
              : "bg-yellow-500/10 text-yellow-600 dark:text-yellow-400 border-yellow-500/25 animate-pulse"
            }`}
        >
          {isPRMerged ? "Merged" : task?.status === "pr_ready" ? "PR Ready" : "Awaiting Review"}
        </span>
      </div>

      {isLimitExceeded && (
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
        riskDomains={riskDomains}
      />

      {/* Action Footer */}
      <TaskPrReview
        task={task ?? null}
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
  );
}
