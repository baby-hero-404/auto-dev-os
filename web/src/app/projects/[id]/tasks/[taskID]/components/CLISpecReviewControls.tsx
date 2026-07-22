"use client";

import { useState } from "react";
import { Loader2, Check, Send } from "lucide-react";
import { tasks } from "@/lib/api/projects";

export interface CLISpecReviewControlsProps {
  taskID: string;
  token: string;
  onReviewed: () => Promise<void>;
  setError: (err: string) => void;
}

export function CLISpecReviewControls({ taskID, token, onReviewed, setError }: CLISpecReviewControlsProps) {
  const [comment, setComment] = useState("");
  const [submitting, setSubmitting] = useState<"approve" | "request_changes" | null>(null);

  const handleApprove = async () => {
    setSubmitting("approve");
    try {
      await tasks.specReview(taskID, token, "approve");
      await onReviewed();
    } catch (err) {
      setError(err instanceof Error ? err.message : "Failed to approve spec");
    } finally {
      setSubmitting(null);
    }
  };

  const handleRequestChanges = async () => {
    if (!comment.trim()) return;
    setSubmitting("request_changes");
    try {
      await tasks.specReview(taskID, token, "request_changes", comment.trim());
      setComment("");
      await onReviewed();
    } catch (err) {
      setError(err instanceof Error ? err.message : "Failed to request spec changes");
    } finally {
      setSubmitting(null);
    }
  };

  return (
    <div className="mt-3 flex flex-col gap-3.5 sm:flex-row sm:items-stretch border-t border-amber-500/15 pt-4 text-slate-800 dark:text-slate-100">
      <div className="flex-1 flex flex-col justify-between rounded-xl border border-amber-500/10 bg-amber-500/5 p-4 shadow-sm">
        <div className="mb-2">
          <h4 className="text-xs font-bold text-amber-800 dark:text-amber-400">Approve Spec</h4>
          <p className="text-xs text-amber-900/75 dark:text-amber-200/75 leading-normal mt-1">
            Accept the proposal/specs/design/tasks as authored and let cli_implement proceed.
          </p>
        </div>
        <button
          onClick={handleApprove}
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
            onClick={handleRequestChanges}
            disabled={submitting !== null || !comment.trim()}
            className="inline-flex items-center justify-center gap-1.5 rounded-lg bg-gradient-to-r from-amber-700 to-amber-800 px-3.5 py-2 text-xs font-semibold text-white transition hover:from-amber-600 hover:to-amber-700 disabled:opacity-50 cursor-pointer shadow-md active:scale-[0.98] ml-auto mt-1"
          >
            {submitting === "request_changes" ? <Loader2 size={13} className="animate-spin" /> : <Send size={13} />}
            Send Feedback & Retry
          </button>
        </div>
      </div>
    </div>
  );
}
