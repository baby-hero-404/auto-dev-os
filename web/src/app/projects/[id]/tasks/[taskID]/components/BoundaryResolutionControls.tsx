"use client";

import { useState } from "react";
import { Loader2, Check, Send } from "lucide-react";
import type { Task, TaskAnalysis } from "@/lib/types";

export interface BoundaryResolutionControlsProps {
  errorMsg: string;
  task: Task | undefined;
  updateTask: (fields: Partial<Task>) => Promise<boolean>;
  execute: () => Promise<void>;
  setError: (err: string) => void;
}

export function BoundaryResolutionControls({
  errorMsg,
  task,
  updateTask,
  execute,
  setError,
}: BoundaryResolutionControlsProps) {
  const [feedback, setFeedback] = useState("");
  const [submitting, setSubmitting] = useState(false);

  let violatedFiles: string[] = [];
  const matchUnauthorized = errorMsg.match(/unauthorized file modifications:\s*\[(.*?)\]/);
  const matchCritical = errorMsg.match(/modification to infrastructure\/security-sensitive file:\s*"(.*?)"/);
  const matchRepeated = errorMsg.match(/repeated boundary violations:\s*(.*)/);

  if (matchUnauthorized && matchUnauthorized[1]) {
    violatedFiles = matchUnauthorized[1].split(/\s+/).filter(Boolean);
  } else if (matchCritical && matchCritical[1]) {
    violatedFiles = [matchCritical[1]];
  } else if (matchRepeated && matchRepeated[1]) {
    const inner = matchRepeated[1];
    const innerMatch = inner.match(/unauthorized file modifications:\s*\[(.*?)\]/);
    if (innerMatch && innerMatch[1]) {
      violatedFiles = innerMatch[1].split(/\s+/).filter(Boolean);
    } else {
      const innerCritical = inner.match(/modification to infrastructure\/security-sensitive file:\s*"(.*?)"/);
      if (innerCritical && innerCritical[1]) {
        violatedFiles = [innerCritical[1]];
      }
    }
  }

  const handleApprove = async () => {
    if (violatedFiles.length === 0) return;
    setSubmitting(true);
    try {
      const newBoundaries = violatedFiles.map((file) => {
        const parts = file.split("/");
        let repoName = "";
        let relativePath = file;
        if (parts.length > 1) {
          repoName = parts[0];
          relativePath = parts.slice(1).join("/");
        }
        const lastSlashIndex = relativePath.lastIndexOf("/");
        const rootDir = lastSlashIndex !== -1 ? relativePath.substring(0, lastSlashIndex) : ".";
        const moduleName = rootDir !== "." ? rootDir.substring(rootDir.lastIndexOf("/") + 1) : "root";
        return {
          module: moduleName,
          root: rootDir,
          repo_name: repoName,
          capabilities: ["modify_existing", "create_test", "create_helper"],
        };
      });

      const currentAnalysis = task?.analysis || ({} as Partial<TaskAnalysis>);
      const currentBoundaries = currentAnalysis.execution_boundaries || [];
      const mergedBoundaries = [...currentBoundaries];
      
      for (const nb of newBoundaries) {
        const exists = mergedBoundaries.some(
          (eb) => eb.root === nb.root && eb.repo_name === nb.repo_name
        );
        if (!exists) mergedBoundaries.push(nb);
      }

      const updatedAnalysis = {
        ...currentAnalysis,
        execution_boundaries: mergedBoundaries,
      } as TaskAnalysis;

      const ok = await updateTask({ analysis: updatedAnalysis });
      if (ok) await execute();
    } catch (err) {
      setError(err instanceof Error ? err.message : "Failed to expand boundaries");
    } finally {
      setSubmitting(false);
    }
  };

  const handleSendFeedback = async () => {
    if (!feedback.trim()) return;
    setSubmitting(true);
    try {
      const currentAnalysis = task?.analysis || ({} as Partial<TaskAnalysis>);
      const currentRules = currentAnalysis.task_rules || [];
      const feedbackLine = violatedFiles.length > 0
        ? `Do not modify these files: ${violatedFiles.join(", ")}. Guidance: ${feedback.trim()}`
        : `Guidance: ${feedback.trim()}`;
      
      const updatedAnalysis = {
        ...currentAnalysis,
        task_rules: [...currentRules, feedbackLine],
      } as TaskAnalysis;

      const ok = await updateTask({ analysis: updatedAnalysis });
      if (ok) {
        setFeedback("");
        await execute();
      }
    } catch (err) {
      setError(err instanceof Error ? err.message : "Failed to submit feedback");
    } finally {
      setSubmitting(false);
    }
  };

  return (
    <div className="mt-3 flex flex-col gap-4 border-t border-amber-500/25 pt-3 text-slate-800 dark:text-slate-100">
      {violatedFiles.length > 0 && (
        <div>
          <div className="text-xs font-semibold uppercase tracking-wider text-amber-800 dark:text-amber-400 mb-1">
            Violating Files:
          </div>
          <ul className="list-inside list-disc pl-1 text-xs font-mono space-y-0.5 text-amber-900 dark:text-amber-100">
            {violatedFiles.map((f) => (
              <li key={f}>{f}</li>
            ))}
          </ul>
        </div>
      )}

      <div className="flex flex-col gap-3 sm:flex-row sm:items-stretch">
        {violatedFiles.length > 0 && (
          <div className="flex-1 flex flex-col justify-between rounded-lg border border-amber-500/10 bg-amber-500/5 p-3">
            <div className="mb-2">
              <h4 className="text-xs font-bold text-amber-800 dark:text-amber-400">Option A: Approve Edits</h4>
              <p className="text-xs text-amber-900/80 dark:text-amber-100/80 leading-normal mt-0.5">
                Authorize the agent to edit these directories by automatically appending them to the task&apos;s execution boundaries.
              </p>
            </div>
            <button onClick={handleApprove} disabled={submitting} className="w-full inline-flex items-center justify-center gap-1.5 rounded-md bg-amber-600 px-3 py-1.5 text-xs font-semibold text-white transition hover:bg-amber-700 disabled:opacity-50 cursor-pointer shadow-sm mt-1">
              {submitting ? <Loader2 size={13} className="animate-spin" /> : <Check size={13} />}
              Approve & Expand Boundaries
            </button>
          </div>
        )}

        <div className={violatedFiles.length > 0 ? "flex-[1.5] flex flex-col rounded-lg border border-amber-500/10 bg-amber-500/5 p-3" : "w-full flex flex-col rounded-lg border border-amber-500/10 bg-amber-500/5 p-3"}>
          <div className="mb-2">
            <h4 className="text-xs font-bold text-amber-800 dark:text-amber-400">{violatedFiles.length > 0 ? "Option B: Block & Provide Guidance" : "Provide Guidance & Retry"}</h4>
            <p className="text-xs text-amber-900/80 dark:text-amber-100/80 leading-normal mt-0.5">
              {violatedFiles.length > 0
                ? "Prevent changes to these files. Instruct the agent on what to do instead (e.g., use mock data or existing functions)."
                : "Instruct the agent on how to adjust its strategy."}
            </p>
          </div>
          <div className="flex flex-col gap-2 mt-1">
            <textarea
              value={feedback}
              onChange={(e) => setFeedback(e.target.value)}
              placeholder="Guidance..."
              rows={2}
              className="w-full rounded border border-amber-500/20 bg-background/50 p-1.5 text-xs font-sans placeholder:opacity-60 focus:outline-none focus:ring-1 focus:ring-amber-500"
            />
            <button onClick={handleSendFeedback} disabled={submitting || !feedback.trim()} className="inline-flex items-center justify-center gap-1.5 rounded-md bg-amber-700 px-3 py-1.5 text-xs font-semibold text-white transition hover:bg-amber-800 disabled:opacity-50 cursor-pointer shadow-sm ml-auto">
              {submitting ? <Loader2 size={13} className="animate-spin" /> : <Send size={13} />}
              Send Guidance & Retry
            </button>
          </div>
        </div>
      </div>
    </div>
  );
}
