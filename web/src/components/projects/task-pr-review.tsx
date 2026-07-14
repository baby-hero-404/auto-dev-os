import { Sparkles, MessageSquare, AlertCircle, Check } from "lucide-react";
import type { Task } from "@/lib/types";

interface TaskPrReviewProps {
  task: Task | null;
  hasPR: boolean;
  isReviewWaiting: boolean;
  submittingPR: boolean;
  feedback: string;
  setFeedback: (val: string) => void;
  startReview: () => void;
  rejectPR: () => void;
  approvePR: () => void;
}

export function TaskPrReview({
  task,
  hasPR,
  isReviewWaiting,
  submittingPR,
  feedback,
  setFeedback,
  startReview,
  rejectPR,
  approvePR,
}: TaskPrReviewProps) {
  if (!((isReviewWaiting || task?.status === "pr_ready") && hasPR)) {
    return null;
  }

  return (
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
              className="inline-flex items-center gap-1.5 rounded-md bg-brand-primary px-4 py-2 text-sm font-semibold text-brand-primary-fg transition hover:opacity-90 disabled:opacity-50 cursor-pointer shadow-sm"
            >
              <Check size={15} />
              Approve &amp; Merge
            </button>
          </div>
        </>
      )}
    </div>
  );
}
