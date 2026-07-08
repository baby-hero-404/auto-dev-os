"use client";

import { useCallback } from "react";
import { MessageSquare, Check } from "lucide-react";
import { useTaskDetail } from "./TaskDetailContext";

export function RequestChangesModal() {
  const {
    isRequestingChanges,
    setIsRequestingChanges,
    specFeedbackText,
    setSpecFeedbackText,
    submittingPR,
    submitSpecChanges,
  } = useTaskDetail();

  const handleCancel = useCallback(() => {
    setIsRequestingChanges(false);
    setSpecFeedbackText("");
  }, [setIsRequestingChanges, setSpecFeedbackText]);

  const handleTextChange = useCallback((e: React.ChangeEvent<HTMLTextAreaElement>) => {
    setSpecFeedbackText(e.target.value);
  }, [setSpecFeedbackText]);

  const handleSubmit = useCallback(async () => {
    await submitSpecChanges();
  }, [submitSpecChanges]);

  if (!isRequestingChanges) return null;

  const isSubmitDisabled = submittingPR || !specFeedbackText.trim();

  return (
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
          onChange={handleTextChange}
          disabled={submittingPR}
          autoFocus
        />

        <div className="flex justify-end gap-2 pt-2 border-t border-stroke">
          <button
            type="button"
            onClick={handleCancel}
            disabled={submittingPR}
            className="rounded-md border border-stroke bg-transparent px-4 py-2 text-sm font-semibold text-foreground hover:bg-surface transition cursor-pointer disabled:opacity-50"
          >
            Cancel
          </button>
          <button
            type="button"
            onClick={handleSubmit}
            disabled={isSubmitDisabled}
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
  );
}
