"use client";

import { Check, MessageSquare, Sparkles } from "lucide-react";
import { useTaskDetail } from "./TaskDetailContext";

/**
 * ReviewActionBar (REQ-001)
 *
 * A sticky, above-the-fold bar surfacing the single human decision the task is
 * currently blocked on, so a reviewer never has to scroll to act. It renders ONLY
 * when a decision is pending:
 *   - spec review  (spec_status pending_review | changes_requested) → Approve / Request Changes
 *   - pr_ready     (status === "pr_ready")                          → Start Review
 * Otherwise it returns null (no empty/disabled bar).
 *
 * Handlers are the same references the rest of the page uses via useTaskDetail() —
 * this is a second render site, not a second source of truth.
 */
export function ReviewActionBar() {
  const {
    task,
    approveSpec,
    requestSpecChanges,
    startReview,
    submittingPR,
    clarificationQuestions,
  } = useTaskDetail();

  const specReview =
    !!task &&
    (task.spec_status === "pending_review" || task.spec_status === "changes_requested");
  const prReady = !!task && task.status === "pr_ready";

  if (!specReview && !prReady) {
    return null;
  }

  const blockedByClarifications = clarificationQuestions.length > 0;

  return (
    <div className="sticky top-0 z-30 -mx-4 border-b border-stroke bg-card/80 px-4 py-3 shadow-sm backdrop-blur-xl md:-mx-8 md:px-8">
      <div className="mx-auto flex max-w-7xl flex-col gap-3 sm:flex-row sm:items-center sm:justify-between">
        <div className="flex items-center gap-2.5">
          <span className="relative flex h-2.5 w-2.5 shrink-0">
            <span className="absolute inline-flex h-full w-full animate-ping rounded-full bg-amber-400 opacity-75" />
            <span className="relative inline-flex h-2.5 w-2.5 rounded-full bg-amber-500 shadow-[0_0_6px_currentColor]" />
          </span>
          <span className="text-sm font-semibold text-foreground">
            Waiting for your review
          </span>
          <span className="hidden text-xs text-content-muted sm:inline">
            {specReview
              ? "Approve the specification or request changes to continue."
              : "The PR is ready — start the review."}
          </span>
        </div>

        <div className="flex items-center gap-2">
          {specReview && (
            <>
              <button
                type="button"
                onClick={requestSpecChanges}
                className="inline-flex items-center justify-center gap-2 rounded-md border border-amber-500/40 bg-amber-500/10 px-3.5 py-2 text-sm font-semibold text-amber-600 shadow-sm transition hover:bg-amber-500/20 dark:text-amber-400 cursor-pointer"
              >
                <MessageSquare size={15} />
                Request Changes
              </button>
              <button
                type="button"
                onClick={approveSpec}
                disabled={blockedByClarifications}
                title={
                  blockedByClarifications
                    ? "Please answer all clarification questions before approving"
                    : undefined
                }
                className="inline-flex items-center justify-center gap-2 rounded-md bg-amber-500 px-3.5 py-2 text-sm font-semibold text-slate-950 shadow-sm transition hover:opacity-90 disabled:cursor-not-allowed disabled:opacity-50 cursor-pointer"
              >
                <Check size={15} />
                Approve Spec
              </button>
            </>
          )}

          {prReady && (
            <button
              type="button"
              onClick={startReview}
              disabled={submittingPR}
              className="inline-flex items-center justify-center gap-2 rounded-md bg-brand-primary px-3.5 py-2 text-sm font-semibold text-slate-950 shadow-sm transition hover:opacity-90 disabled:cursor-not-allowed disabled:opacity-50 cursor-pointer"
            >
              <Sparkles size={15} />
              Start Review
            </button>
          )}
        </div>
      </div>
    </div>
  );
}
