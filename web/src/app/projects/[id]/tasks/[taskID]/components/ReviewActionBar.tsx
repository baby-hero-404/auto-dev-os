"use client";

import { Check, MessageSquare, X, Clock } from "lucide-react";
import { useTaskDetail } from "./TaskDetailContext";

/**
 * ReviewActionBar (REQ-001)
 *
 * A sticky, above-the-fold bar surfacing the single human decision the task is
 * currently blocked on, so a reviewer never has to scroll to act. It renders ONLY
 * when a decision is pending:
 *   - spec review  (spec_status pending_review | changes_requested) → Approve / Request Changes
 *   - pr_ready     (status === "pr_ready")                          → Start Review / Merge / Reject
 *   - human_review (status === "human_review")                      → Merge / Reject
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
    approvePR,
    submittingPR,
    clarificationQuestions,
  } = useTaskDetail();

  const specReview =
    !!task &&
    (task.spec_status === "pending_review" || task.spec_status === "changes_requested");
  const prReady = !!task && task.status === "pr_ready";
  const humanReview = !!task && task.status === "human_review";

  if (!specReview && !prReady && !humanReview) {
    return null;
  }

  const blockedByClarifications = clarificationQuestions.length > 0;

  const scrollToHero = () => {
    const el = document.getElementById("hero-cards-section");
    if (el) {
      el.scrollIntoView({ behavior: "smooth" });
      // Find the reject textarea and focus it if possible
      setTimeout(() => {
        const textarea = el.querySelector("textarea");
        if (textarea) textarea.focus();
      }, 500);
    }
  };

  return (
    <div className="sticky top-0 z-30 -mx-4 border-b border-stroke bg-card/80 px-4 py-3 shadow-sm backdrop-blur-xl md:-mx-8 md:px-8">
      <div className="mx-auto flex max-w-7xl flex-col gap-3 sm:flex-row sm:items-center sm:justify-between">
        <div className="flex items-center gap-2.5">
          <span className="relative flex h-2.5 w-2.5 shrink-0">
            <span className={`absolute inline-flex h-full w-full animate-ping rounded-full opacity-75 ${
              prReady || humanReview ? "bg-emerald-400" : "bg-amber-400"
            }`} />
            <span className={`relative inline-flex h-2.5 w-2.5 rounded-full shadow-[0_0_6px_currentColor] ${
              prReady || humanReview ? "bg-emerald-500" : "bg-amber-500"
            }`} />
          </span>
          <span className="text-sm font-semibold text-foreground">
            {specReview && "Specification Review Pending"}
            {prReady && "Pull Request Ready for Review"}
            {humanReview && "Awaiting Final Human Approval"}
          </span>
          <span className="hidden text-xs text-content-muted sm:inline">
            {specReview && "Approve the specification or request changes to continue."}
            {prReady && "The PR is ready — merge it directly, start a review, or reject."}
            {humanReview && "Final review phase — merge the changes or reject with feedback."}
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
            <>
              <button
                type="button"
                onClick={() => startReview()}
                disabled={submittingPR}
                className="inline-flex items-center justify-center gap-2 rounded-md border border-stroke bg-surface px-3.5 py-2 text-sm font-semibold text-foreground hover:bg-muted/10 transition cursor-pointer"
              >
                <Clock size={15} />
                Start Review
              </button>
              <button
                type="button"
                onClick={scrollToHero}
                disabled={submittingPR}
                className="inline-flex items-center justify-center gap-2 rounded-md border border-danger/45 bg-danger/5 px-3.5 py-2 text-sm font-semibold text-danger shadow-sm transition hover:bg-danger/10 cursor-pointer"
              >
                <X size={15} />
                Reject PR
              </button>
              <button
                type="button"
                onClick={approvePR}
                disabled={submittingPR}
                className="inline-flex items-center justify-center gap-2 rounded-md bg-success px-3.5 py-2 text-sm font-semibold text-white shadow-sm transition hover:opacity-90 disabled:cursor-not-allowed disabled:opacity-50 cursor-pointer"
              >
                <Check size={15} />
                Merge PR
              </button>
            </>
          )}

          {humanReview && (
            <>
              <button
                type="button"
                onClick={scrollToHero}
                disabled={submittingPR}
                className="inline-flex items-center justify-center gap-2 rounded-md border border-danger/45 bg-danger/5 px-3.5 py-2 text-sm font-semibold text-danger shadow-sm transition hover:bg-danger/10 cursor-pointer"
              >
                <X size={15} />
                Reject PR
              </button>
              <button
                type="button"
                onClick={approvePR}
                disabled={submittingPR}
                className="inline-flex items-center justify-center gap-2 rounded-md bg-success px-3.5 py-2 text-sm font-semibold text-white shadow-sm transition hover:opacity-90 disabled:cursor-not-allowed disabled:opacity-50 cursor-pointer"
              >
                <Check size={15} />
                Approve Merge
              </button>
            </>
          )}
        </div>
      </div>
    </div>
  );
}
