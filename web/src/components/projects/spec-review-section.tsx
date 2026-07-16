import { AlertTriangle } from "lucide-react";

interface SpecReviewSectionProps {
  specStatus?: string;
  hasUnansweredQuestions?: boolean;
}

export function SpecReviewSection({
  specStatus,
  hasUnansweredQuestions,
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
    </div>
  );
}
