"use client";

import { useState } from "react";
import { Check, Loader2, Send } from "lucide-react";
import { tasks as tasksApi } from "@/lib/api/projects";
import { useTaskDetail } from "./TaskDetailContext";
import { SpecPanel } from "./SpecPanel";
import { CLISpecPanel } from "./CLISpecPanel";

/**
 * Single spec-review gate for the "spec_review" status, covering both flows:
 * - CLI-spec-first: CLISpecPanel + Approve/Request-Changes wired to the
 *   /spec-review endpoint (tasksApi.specReview).
 * - Classic: SpecPanel + Approve/Request-Changes wired to the
 *   /analysis/approve and /analysis/request-changes endpoints via context.
 * Exactly one panel and one control pair renders per flow.
 */
export function SpecReviewGate() {
  const { taskID, token, isCliFlow, approveSpec, requestSpecChanges, mutateWorkflow, setError } = useTaskDetail();

  const [comment, setComment] = useState("");
  const [submitting, setSubmitting] = useState<"approve" | "request_changes" | null>(null);
  const [includeSpec, setIncludeSpec] = useState(false);

  const handleCliApprove = async () => {
    if (!token) return;
    setSubmitting("approve");
    try {
      await tasksApi.updateSpecConfig(taskID, token, includeSpec);
      await tasksApi.specReview(taskID, token, "approve");
      await mutateWorkflow();
    } catch (err) {
      setError(err instanceof Error ? err.message : "Failed to approve spec");
    } finally {
      setSubmitting(null);
    }
  };

  const handleClassicApprove = async () => {
    if (!token) return;
    try {
      await tasksApi.updateSpecConfig(taskID, token, includeSpec);
      approveSpec();
    } catch (err) {
      setError(err instanceof Error ? err.message : "Failed to update spec config");
    }
  };

  const handleCliRequestChanges = async () => {
    if (!token || !comment.trim()) return;
    setSubmitting("request_changes");
    try {
      await tasksApi.specReview(taskID, token, "request_changes", comment.trim());
      setComment("");
      await mutateWorkflow();
    } catch (err) {
      setError(err instanceof Error ? err.message : "Failed to request spec changes");
    } finally {
      setSubmitting(null);
    }
  };

  if (isCliFlow) {
    return (
      <div className="flex flex-col gap-4">
        <div className="bg-linear-to-br from-amber-500/10 via-amber-500/[0.02] to-orange-500/5 border border-amber-500/25 rounded-2xl p-5 shadow-md relative overflow-hidden animate-fade-in">
          <div className="absolute -top-10 -right-10 w-24 h-24 bg-amber-500/10 rounded-full blur-2xl pointer-events-none" />
          <div className="mb-1 z-10 relative">
            <div className="text-sm font-bold text-foreground">Definition-of-Ready Gate</div>
            <div className="text-xs text-content-muted mt-0.5 leading-normal">Review the CLI-authored specification below and approve it before implementation starts.</div>
          </div>
          <div className="mt-3 flex flex-col gap-3.5 sm:flex-row sm:items-stretch border-t border-amber-500/15 pt-4 text-foreground z-10 relative">
            <div className="flex-1 flex flex-col justify-between rounded-xl border border-amber-500/10 bg-amber-500/5 p-4 shadow-sm">
              <div className="mb-2">
                <h4 className="text-xs font-bold text-amber-800 dark:text-amber-400">Approve Spec</h4>
                <p className="text-xs text-amber-900/75 dark:text-amber-200/75 leading-normal mt-1">
                  Accept the proposal/specs/design/tasks as authored and let cli_implement proceed.
                </p>
                <label className="flex items-center gap-2 mt-3 text-xs text-amber-900 dark:text-amber-200 cursor-pointer">
                  <input type="checkbox" checked={includeSpec} onChange={(e) => setIncludeSpec(e.target.checked)} className="rounded border-amber-500/30 text-amber-600 focus:ring-amber-500 bg-background/50" />
                  Include specification files in the final Merge Request
                </label>
              </div>
              <button
                onClick={handleCliApprove}
                disabled={submitting !== null}
                className="w-full inline-flex items-center justify-center gap-1.5 rounded-lg bg-gradient-to-r from-amber-600 to-orange-600 px-3.5 py-2 text-xs font-semibold text-white transition hover:from-amber-500 hover:to-orange-500 disabled:opacity-50 cursor-pointer shadow-md shadow-orange-500/10 active:scale-[0.98] mt-2"
              >
                {submitting === "approve" ? <Loader2 size={13} className="animate-spin" /> : <Check size={13} />}
                Approve & Continue
              </button>
            </div>

            <div className="flex-[1.5] flex flex-col rounded-xl border border-amber-500/10 bg-amber-500/5 p-4 justify-between shadow-sm">
              <div className="mb-2">
                <h4 className="text-xs font-bold text-amber-800 dark:text-amber-400">Request Changes</h4>
                <p className="text-xs text-amber-900/75 dark:text-amber-200/75 leading-normal mt-1">
                  Send this back to cli_spec with your feedback embedded in the next prompt.
                </p>
              </div>
              <div className="flex flex-col gap-2 mt-1">
                <textarea
                  value={comment}
                  onChange={(e) => setComment(e.target.value)}
                  placeholder="What should change in the spec?"
                  rows={2}
                  className="w-full rounded-lg border border-amber-500/20 bg-background/40 p-2 text-xs font-sans placeholder:opacity-50 focus:outline-none focus:ring-1 focus:ring-amber-500 focus:bg-background/80 transition-all duration-150 resize-none"
                />
                <button
                  onClick={handleCliRequestChanges}
                  disabled={submitting !== null || !comment.trim()}
                  className="inline-flex items-center justify-center gap-1.5 rounded-lg bg-gradient-to-r from-amber-700 to-amber-800 px-3.5 py-2 text-xs font-semibold text-white transition hover:from-amber-600 hover:to-amber-700 disabled:opacity-50 cursor-pointer shadow-md active:scale-[0.98] ml-auto mt-1"
                >
                  {submitting === "request_changes" ? <Loader2 size={13} className="animate-spin" /> : <Send size={13} />}
                  Send Feedback & Retry
                </button>
              </div>
            </div>
          </div>
        </div>
        <CLISpecPanel />
      </div>
    );
  }

  return (
    <div className="flex flex-col gap-4">
      <div className="bg-linear-to-br from-amber-500/10 via-amber-500/[0.02] to-orange-500/5 border border-amber-500/25 rounded-2xl p-5 flex flex-col md:flex-row md:items-center md:justify-between gap-4 shadow-md relative overflow-hidden animate-fade-in">
        <div className="absolute -top-10 -right-10 w-24 h-24 bg-amber-500/10 rounded-full blur-2xl pointer-events-none" />
        <div className="z-10">
          <div className="text-sm font-bold text-foreground">Definition-of-Ready Gate</div>
          <div className="text-xs text-content-muted mt-0.5 leading-normal">Review the specification below and approve it before coding starts.</div>
          <label className="flex items-center gap-2 mt-2 text-xs text-content-muted cursor-pointer">
            <input type="checkbox" checked={includeSpec} onChange={(e) => setIncludeSpec(e.target.checked)} className="rounded border-stroke text-brand-primary focus:ring-brand-primary bg-background/50" />
            Include specification files in the final Merge Request
          </label>
        </div>
        <div className="flex items-center gap-2 z-10 self-end md:self-center">
          <button onClick={requestSpecChanges} className="px-4 py-2 rounded-xl border border-stroke bg-background/50 text-content text-xs font-semibold hover:bg-surface transition-all duration-150 cursor-pointer">
            Request Changes
          </button>
          <button onClick={handleClassicApprove} className="px-4.5 py-2 rounded-xl border-none bg-linear-to-r from-emerald-600 to-teal-600 hover:from-emerald-500 hover:to-teal-500 text-white text-xs font-bold transition-all duration-150 hover:shadow-md hover:shadow-emerald-500/20 active:scale-95 cursor-pointer shadow-sm flex items-center gap-1.5">
            <Check className="h-3.5 w-3.5" /> Approve Spec
          </button>
        </div>
      </div>
      <SpecPanel isExpanded={true} />
    </div>
  );
}
