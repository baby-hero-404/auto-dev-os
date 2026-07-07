import { AlertTriangle } from "lucide-react";

interface SpecReviewSectionProps {
  specStatus?: string;
  hasUnansweredQuestions?: boolean;
  onRequestChanges: () => void;
  onApproveSpec: () => void;
}

export function SpecReviewSection({
  specStatus,
  hasUnansweredQuestions,
  onRequestChanges,
  onApproveSpec,
}: SpecReviewSectionProps) {
  if (specStatus !== "pending_review" && specStatus !== "changes_requested") {
    return null;
  }

  return (
    <div className="mb-5 flex flex-col sm:flex-row items-start sm:items-center justify-between gap-4 rounded-lg border border-amber-400/30 bg-amber-950/30 p-4 text-amber-100">
      <div className="flex items-start gap-3">
        <AlertTriangle className="mt-0.5 shrink-0" size={18} />
        <div>
          <div className="font-mono font-semibold">Spec review required</div>
          <p className="text-sm text-amber-100/80">
            This task is paused until the analysis is approved or clarified.
          </p>
          {hasUnansweredQuestions && (
            <p className="text-xs font-semibold text-rose-400 mt-1.5 animate-pulse">
              * Please answer all clarification questions below to unlock approval.
            </p>
          )}
        </div>
      </div>
      <div className="flex shrink-0 gap-2 w-full sm:w-auto mt-2 sm:mt-0 items-start">
        <button
          onClick={onRequestChanges}
          className="flex-1 sm:flex-none cursor-pointer rounded bg-amber-950 px-3 py-1.5 text-sm font-semibold border border-amber-500/30 transition hover:bg-amber-900"
        >
          Request Changes
        </button>
        <div className="flex-1 sm:flex-none flex flex-col">
          <button
            onClick={onApproveSpec}
            disabled={hasUnansweredQuestions}
            className="cursor-pointer rounded bg-amber-500 text-slate-950 px-3 py-1.5 text-sm font-semibold transition hover:bg-amber-400 disabled:opacity-50 disabled:cursor-not-allowed"
          >
            Approve Spec
          </button>
        </div>
      </div>
    </div>
  );
}
