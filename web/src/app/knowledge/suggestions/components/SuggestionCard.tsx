"use client";

import { Info, Loader2, ThumbsDown, ThumbsUp, FileCode } from "lucide-react";
import type { LearningSuggestion } from "@/lib/types";

interface SuggestionCardProps {
  suggestion: LearningSuggestion;
  actioningID: string | null;
  rejectionID: string | null;
  setRejectionID: (id: string | null) => void;
  rejectionFeedback: string;
  setRejectionFeedback: (val: string) => void;
  onRejectSubmit: (e: React.FormEvent) => void;
  onApproveClick: (id: string) => void;
}

export function SuggestionCard({
  suggestion,
  actioningID,
  rejectionID,
  setRejectionID,
  rejectionFeedback,
  setRejectionFeedback,
  onRejectSubmit,
  onApproveClick,
}: SuggestionCardProps) {
  const confidencePct = Math.round(suggestion.confidence * 100);
  const isPending = suggestion.status === "pending";
  const feedback = suggestionFeedback(suggestion);

  return (
    <article
      className="rounded-lg border border-stroke bg-panel p-5 flex flex-col gap-4 transition hover:border-brand-primary/30"
    >
      <div className="flex flex-col justify-between gap-3 sm:flex-row sm:items-start">
        <div>
          <div className="flex flex-wrap items-center gap-2 mb-2">
            <span className="rounded bg-indigo-500/10 border border-indigo-500/20 px-2 py-0.5 text-[10px] font-mono text-indigo-300 font-bold uppercase tracking-wider">
              {suggestion.suggestion_type}
            </span>
            <span className="text-xs text-content-muted font-mono">
              ID: {suggestion.id.slice(0, 8)}...
            </span>
          </div>
          <h3 className="font-mono text-sm font-bold text-white">{suggestion.title}</h3>
          <p className="mt-1.5 text-xs text-content-muted leading-relaxed">
            {suggestion.description}
          </p>
        </div>

        {/* Confidence Score Gauge */}
        <div className="flex flex-col items-center sm:items-end justify-center min-w-[100px]">
          <span className="text-[10px] font-mono text-content-muted uppercase">Confidence</span>
          <div className="mt-1 flex items-center gap-2">
            <div className="h-2 w-16 bg-slate-900 border border-stroke rounded-full overflow-hidden">
              <div
                className="h-full bg-emerald-400 rounded-full"
                style={{ width: `${confidencePct}%` }}
              ></div>
            </div>
            <span className="font-mono text-xs font-bold text-emerald-400">{confidencePct}%</span>
          </div>
        </div>
      </div>

      {/* Content Section (Rule/Prompt Patch body) */}
      <div>
        <div className="mb-1 flex items-center gap-1.5 text-[10px] font-mono font-bold uppercase tracking-wider text-content-muted">
          <FileCode size={12} />
          Proposed Content
        </div>
        <pre className="rounded bg-slate-950 border border-stroke p-3.5 font-mono text-xs text-slate-300 overflow-x-auto whitespace-pre-wrap">
          {suggestion.content}
        </pre>
      </div>

      {/* Applied Metadata or Feedback Info */}
      {suggestion.status === "rejected" && feedback && (
        <div className="rounded border border-red-500/20 bg-red-950/20 p-3 text-xs text-red-200 flex items-start gap-2">
          <Info size={16} className="text-red-400 mt-0.5 shrink-0" />
          <div>
            <span className="font-bold">Rejection Feedback:</span> {feedback}
          </div>
        </div>
      )}

      {suggestion.status === "applied" && feedback && (
        <div className="rounded border border-emerald-500/20 bg-emerald-950/20 p-3 text-xs text-emerald-200 flex items-start gap-2">
          <Info size={16} className="text-emerald-400 mt-0.5 shrink-0" />
          <div>
            <span className="font-bold">Execution details:</span> {feedback}
          </div>
        </div>
      )}

      {/* Pending Action Buttons */}
      {isPending && rejectionID !== suggestion.id && (
        <div className="flex gap-2 justify-end border-t border-stroke/60 pt-4 mt-1">
          <button
            onClick={() => setRejectionID(suggestion.id)}
            className="rounded-md border border-red-500/20 bg-red-950/10 px-4 py-2 text-xs font-semibold text-red-300 hover:bg-red-950/30 transition cursor-pointer flex items-center gap-1"
            disabled={actioningID === suggestion.id}
          >
            <ThumbsDown size={14} />
            Reject Suggestion
          </button>
          <button
            onClick={() => onApproveClick(suggestion.id)}
            className="rounded-md bg-emerald-400 px-4 py-2 text-xs font-semibold text-slate-950 hover:bg-emerald-300 transition cursor-pointer flex items-center gap-1"
            disabled={actioningID === suggestion.id}
          >
            {actioningID === suggestion.id ? (
              <Loader2 size={14} className="animate-spin" />
            ) : (
              <ThumbsUp size={14} />
            )}
            Approve & Apply
          </button>
        </div>
      )}

      {/* Rejection Form Overlay */}
      {rejectionID === suggestion.id && (
        <form
          onSubmit={onRejectSubmit}
          className="border-t border-stroke/60 pt-4 mt-1 flex flex-col gap-3"
        >
          <div className="flex flex-col gap-1.5">
            <label className="text-xs text-content-muted font-mono">
              Provide Feedback (Reason for rejection)
            </label>
            <textarea
              value={rejectionFeedback}
              onChange={(e) => setRejectionFeedback(e.target.value)}
              placeholder="Why is this suggestion invalid? (e.g. incorrect pattern, conflicts with rule X)"
              className="rounded border border-stroke bg-slate-950 px-3 py-2 text-xs text-white h-20 focus:outline-none focus:border-red-500"
              required
            />
          </div>
          <div className="flex justify-end gap-2">
            <button
              type="button"
              onClick={() => {
                setRejectionID(null);
                setRejectionFeedback("");
              }}
              className="rounded border border-stroke bg-slate-900 px-3 py-1.5 text-xs text-slate-300 hover:text-white cursor-pointer"
            >
              Cancel
            </button>
            <button
              type="submit"
              className="rounded bg-red-500 px-3 py-1.5 text-xs font-semibold text-white hover:bg-red-400 transition cursor-pointer"
            >
              {actioningID === suggestion.id ? (
                <Loader2 size={12} className="animate-spin" />
              ) : (
                "Submit Rejection"
              )}
            </button>
          </div>
        </form>
      )}
    </article>
  );
}

function suggestionFeedback(suggestion: LearningSuggestion) {
  const feedback = suggestion.metadata?.review_feedback;
  if (typeof feedback === "string") return feedback;
  return "";
}
