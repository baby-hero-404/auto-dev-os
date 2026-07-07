import { useState } from "react";
import { AlertCircle, Check } from "lucide-react";
import { api } from "@/lib/api";

interface TaskClarificationFormProps {
  taskID: string;
  specStatus?: string;
  token: string;
  clarificationQuestions: string[];
  onAnswersSubmitted: () => Promise<void>;
}

export function TaskClarificationForm({
  taskID,
  specStatus,
  token,
  clarificationQuestions,
  onAnswersSubmitted,
}: TaskClarificationFormProps) {
  const [answers, setAnswers] = useState<Record<number, string>>({});
  const [submittingAnswers, setSubmittingAnswers] = useState(false);
  const [submitted, setSubmitted] = useState(false);
  const [error, setError] = useState("");

  if (submitted) {
    return (
      <div className="rounded-lg border border-emerald-500/20 bg-emerald-500/5 p-4 flex items-center gap-2 animate-fade-in">
        <Check size={16} className="text-emerald-500" />
        <span className="text-xs text-emerald-700 dark:text-emerald-400 font-semibold">
          Clarification answers submitted successfully!
        </span>
      </div>
    );
  }

  if (!(clarificationQuestions.length > 0 && (specStatus === "pending_review" || specStatus === "changes_requested"))) {
    return null;
  }

  const handleAnswerSubmit = async () => {
    if (!token || clarificationQuestions.length === 0) return;
    setSubmittingAnswers(true);
    setError("");

    let formattedText = "### Answers to Clarification Questions:\n";
    clarificationQuestions.forEach((q, idx) => {
      const ans = (answers[idx] || "").trim();
      formattedText += `- **Q**: ${q}\n  **A**: ${ans || "No answer provided"}\n\n`;
    });

    // Optimistically update UI
    setSubmittingAnswers(true);
    setSubmitted(true);
    
    api.clarifyTask(taskID, token, formattedText.trim())
      .then(() => api.retryTask(taskID, token))
      .then(() => {
        setAnswers({});
        onAnswersSubmitted();
      })
      .catch((err) => {
        setError((err as Error)?.message || "Failed to submit answers");
        setSubmitted(false);
      })
      .finally(() => {
        setSubmittingAnswers(false);
      });
  };

  return (
    <div className="rounded-lg border border-amber-500/20 bg-amber-500/5 p-4 space-y-4">
      <h3 className="font-sans text-xs font-bold text-amber-700 dark:text-amber-400 flex items-center gap-1.5 border-b border-amber-500/10 pb-2">
        <AlertCircle size={14} className="text-amber-500" />
        Questions / Clarifications Required
      </h3>
      {error && (
        <div className="text-xs text-rose-500 bg-rose-500/10 p-2 rounded">
          {error}
        </div>
      )}
      <div className="space-y-4">
        {clarificationQuestions.map((q, idx) => (
          <div key={idx} className="space-y-2">
            <label className="block text-xs font-semibold text-amber-800 dark:text-amber-300">
              {idx + 1}. {q}
            </label>
            <textarea
              className="w-full rounded-lg border border-stroke bg-card p-3 font-sans text-xs text-foreground placeholder-content-muted/40 focus:border-brand-primary outline-none resize-none h-20 transition-all"
              placeholder="Describe your answer for this question..."
              value={answers[idx] || ""}
              onChange={(e) => setAnswers((prev) => ({ ...prev, [idx]: e.target.value }))}
              disabled={submittingAnswers}
            />
          </div>
        ))}
      </div>
      <div className="flex justify-end pt-2">
        <button
          type="button"
          onClick={handleAnswerSubmit}
          disabled={submittingAnswers || Object.values(answers).every((a) => !a.trim())}
          className="rounded-md bg-amber-500 hover:bg-amber-600 text-slate-950 px-4 py-2 text-xs font-bold disabled:opacity-50 transition cursor-pointer flex items-center gap-1.5 shadow-sm"
        >
          {submittingAnswers ? (
            <span className="size-3.5 animate-spin rounded-full border-2 border-slate-950 border-t-transparent" />
          ) : (
            <Check size={14} />
          )}
          Submit Answers
        </button>
      </div>
    </div>
  );
}
