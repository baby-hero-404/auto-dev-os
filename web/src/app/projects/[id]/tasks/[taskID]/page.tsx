"use client";

import { use, useState } from "react";
import Link from "next/link";
import { AlertCircle, Loader2, Check, Send } from "lucide-react";
import { TaskDetailProvider, useTaskDetail } from "./components/TaskDetailContext";
import { TaskHeader } from "./components/TaskHeader";
import { WorkflowTimeline } from "./components/WorkflowTimeline";
import { SpecPanel } from "./components/SpecPanel";
import { PRPanel } from "./components/PRPanel";
import { WorkflowSidebar } from "./components/WorkflowSidebar";
import { TaskActions } from "./components/TaskActions";
import { RequestChangesModal } from "./components/RequestChangesModal";
import { SpecReviewSection } from "@/components/projects/spec-review-section";
import { LogConsole } from "@/components/dashboard/log-console";
import { useSession } from "@/lib/session";

function TaskDetailContent() {
  const {
    task,
    workflow,
    logs,
    error,
    setError,
    token,
    updateTask,
    execute,
    clarificationQuestions,
    requestSpecChanges,
    approveSpec,
    isTaskLoading,
    workflowError,
  } = useTaskDetail();

  if (isTaskLoading) {
    return (
      <main className="min-h-screen bg-slate-950 p-6 flex flex-col justify-center items-center gap-4">
        <Loader2 className="h-8 w-8 animate-spin text-brand-primary" />
        <p className="text-sm font-mono text-content-muted animate-pulse">Loading task workspace...</p>
      </main>
    );
  }

  if (workflowError) {
    return (
      <main className="grid min-h-screen place-items-center p-6">
        <div className="rounded-lg border border-red-500/20 bg-red-500/5 p-6 max-w-lg text-center">
          <AlertCircle className="h-10 w-10 text-red-500 mx-auto mb-4" />
          <p className="font-sans text-base font-semibold text-red-600 dark:text-red-400">Failed to load task workflow.</p>
          <p className="mt-1 text-xs text-content-muted mb-4">{workflowError.message || "An unexpected error occurred."}</p>
          <div className="flex justify-center gap-3">
            <button onClick={() => window.location.reload()} className="rounded-md bg-brand-primary px-4 py-2 text-sm font-semibold text-slate-950 hover:opacity-90 transition">
              Retry Load
            </button>
          </div>
        </div>
      </main>
    );
  }

  return (
    <main className="min-h-screen bg-background px-4 py-5 font-sans text-content md:px-8 md:py-7">
      <div className="mx-auto max-w-7xl space-y-6">
        <TaskHeader />

        {error && (
          <div className="rounded-lg border border-red-500/20 bg-red-500/10 p-4 text-sm text-red-700 dark:text-red-300 flex items-center gap-2" role="alert">
            <AlertCircle size={16} className="shrink-0" />
            {error}
          </div>
        )}

        {workflow?.job?.status === "failed" && workflow?.job?.last_error && (
          <div className="rounded-lg border border-rose-500/20 bg-rose-500/10 p-4 text-sm text-rose-700 dark:text-rose-300 flex flex-col gap-1.5" role="alert">
            <div className="flex items-center gap-2 font-semibold">
              <AlertCircle size={16} className="shrink-0 text-rose-500" />
              Task Execution Failed
            </div>
            <p className="text-xs font-mono bg-black/40 border border-stroke/50 rounded-lg p-3 break-all whitespace-pre-wrap">
              {workflow.job.last_error}
            </p>
          </div>
        )}

        {workflow?.job?.status === "paused" && 
         workflow?.job?.last_error && 
         !workflow.job.last_error.includes("workflow paused for human spec review") &&
         !workflow.job.last_error.includes("workflow paused for human task clarification") && (
          <div className="rounded-lg border border-amber-500/20 bg-amber-500/10 p-4 text-sm text-amber-700 dark:text-amber-300 flex flex-col gap-2" role="alert">
            <div className="flex items-center gap-2 font-semibold text-amber-600 dark:text-amber-400">
              <AlertCircle size={16} className="shrink-0 text-amber-500" />
              Task Execution Paused (Human Action Required)
            </div>
            <p className="text-xs font-mono bg-black/40 border border-stroke/50 rounded-lg p-3 break-all whitespace-pre-wrap text-amber-600 dark:text-amber-200">
              {workflow.job.last_error}
            </p>
            <BoundaryResolutionControls
              errorMsg={workflow.job.last_error}
              task={task}
              token={token}
              updateTask={updateTask}
              execute={execute}
              setError={setError}
            />
          </div>
        )}

        <SpecReviewSection
          specStatus={task?.spec_status}
          hasUnansweredQuestions={clarificationQuestions.length > 0}
          onRequestChanges={requestSpecChanges}
          onApproveSpec={approveSpec}
        />

        <PRPanel />

        <WorkflowTimeline />

        <div className="grid gap-6 xl:grid-cols-[1fr_380px]">
          <section className="space-y-6">
            <SpecPanel />
            <LogConsole logs={logs} />
          </section>

          <WorkflowSidebar />
        </div>

        <RequestChangesModal />
      </div>
    </main>
  );
}

export default function ProjectTaskDetailPage({
  params,
}: {
  params: Promise<{ id: string; taskID: string }>;
}) {
  const { id: projectID, taskID } = use(params);
  const session = useSession();

  if (!session) {
    return (
      <main className="grid min-h-screen place-items-center p-6">
        <div className="rounded-lg border border-stroke bg-card p-6">
          <p className="mb-4 text-sm text-content-muted">Login from the dashboard before opening a task.</p>
          <Link className="rounded-md bg-brand-primary px-4 py-2 font-semibold text-slate-950" href="/">Back to login</Link>
        </div>
      </main>
    );
  }

  return (
    <TaskDetailProvider projectID={projectID} taskID={taskID}>
      <TaskDetailContent />
    </TaskDetailProvider>
  );
}

interface BoundaryResolutionControlsProps {
  errorMsg: string;
  task: any;
  token: string;
  updateTask: (fields: any) => Promise<boolean>;
  execute: () => Promise<void>;
  setError: (err: string) => void;
}

function BoundaryResolutionControls({
  errorMsg,
  task,
  token,
  updateTask,
  execute,
  setError,
}: BoundaryResolutionControlsProps) {
  const [feedback, setFeedback] = useState("");
  const [submitting, setSubmitting] = useState(false);

  // Parse violated files
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

      const currentAnalysis = task?.analysis || {};
      const currentBoundaries = currentAnalysis.execution_boundaries || [];
      
      // Deduplicate boundaries by root and repo_name
      const mergedBoundaries = [...currentBoundaries];
      for (const nb of newBoundaries) {
        const exists = mergedBoundaries.some(
          (eb) => eb.root === nb.root && eb.repo_name === nb.repo_name
        );
        if (!exists) {
          mergedBoundaries.push(nb);
        }
      }

      const updatedAnalysis = {
        ...currentAnalysis,
        execution_boundaries: mergedBoundaries,
      };

      const ok = await updateTask({ analysis: updatedAnalysis });
      if (ok) {
        await execute();
      }
    } catch (err: any) {
      setError(err?.message || "Failed to expand boundaries");
    } finally {
      setSubmitting(false);
    }
  };

  const handleSendFeedback = async () => {
    if (!feedback.trim()) return;
    setSubmitting(true);
    try {
      const currentAnalysis = task?.analysis || {};
      const currentRules = currentAnalysis.task_rules || [];
      const feedbackLine = violatedFiles.length > 0 
        ? `Do not modify these files: ${violatedFiles.join(", ")}. Guidance: ${feedback.trim()}`
        : `Guidance: ${feedback.trim()}`;
      const updatedRules = [...currentRules, feedbackLine];

      const updatedAnalysis = {
        ...currentAnalysis,
        task_rules: updatedRules,
      };

      const ok = await updateTask({ analysis: updatedAnalysis });
      if (ok) {
        setFeedback("");
        await execute();
      }
    } catch (err: any) {
      setError(err?.message || "Failed to submit feedback");
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
        {/* Option 1: Expand Boundaries */}
        {violatedFiles.length > 0 && (
          <div className="flex-1 flex flex-col justify-between rounded-lg border border-amber-500/10 bg-amber-500/5 p-3">
            <div className="mb-2">
              <h4 className="text-xs font-bold text-amber-800 dark:text-amber-400">
                Option A: Approve Edits
              </h4>
              <p className="text-xs text-amber-900/80 dark:text-amber-100/80 leading-normal mt-0.5">
                Authorize the agent to edit these directories by automatically appending them to the task's execution boundaries.
              </p>
            </div>
            <button
              onClick={handleApprove}
              disabled={submitting}
              className="w-full inline-flex items-center justify-center gap-1.5 rounded-md bg-amber-600 px-3 py-1.5 text-xs font-semibold text-white transition hover:bg-amber-700 disabled:opacity-50 cursor-pointer shadow-sm mt-1"
            >
              {submitting ? (
                <Loader2 size={13} className="animate-spin" />
              ) : (
                <Check size={13} />
              )}
              Approve & Expand Boundaries
            </button>
          </div>
        )}

        {/* Option 2: Block & Feedback */}
        <div className={violatedFiles.length > 0 ? "flex-[1.5] flex flex-col rounded-lg border border-amber-500/10 bg-amber-500/5 p-3" : "w-full flex flex-col rounded-lg border border-amber-500/10 bg-amber-500/5 p-3"}>
          <div className="mb-2">
            <h4 className="text-xs font-bold text-amber-800 dark:text-amber-400">
              {violatedFiles.length > 0 ? "Option B: Block & Provide Guidance" : "Provide Guidance & Retry"}
            </h4>
            <p className="text-xs text-amber-900/80 dark:text-amber-100/80 leading-normal mt-0.5">
              {violatedFiles.length > 0 
                ? "Prevent changes to these files. Instruct the agent on what to do instead (e.g., use mock data or existing functions)."
                : "Instruct the agent on how to adjust its strategy (e.g., focus on a specific module or avoid a file path)."}
            </p>
          </div>
          <div className="flex flex-col gap-2 mt-1">
            <textarea
              value={feedback}
              onChange={(e) => setFeedback(e.target.value)}
              placeholder={violatedFiles.length > 0 
                ? "e.g., Do not create sqlite/repository.go, use existing database functions instead."
                : "e.g., Focus on creating the test file first, do not touch the main config files."}
              rows={2}
              className="w-full rounded border border-amber-500/20 bg-background/50 p-1.5 text-xs font-sans placeholder:opacity-60 focus:outline-none focus:ring-1 focus:ring-amber-500"
            />
            <button
              onClick={handleSendFeedback}
              disabled={submitting || !feedback.trim()}
              className="inline-flex items-center justify-center gap-1.5 rounded-md bg-amber-700 px-3 py-1.5 text-xs font-semibold text-white transition hover:bg-amber-800 disabled:opacity-50 cursor-pointer shadow-sm ml-auto"
            >
              {submitting ? (
                <Loader2 size={13} className="animate-spin" />
              ) : (
                <Send size={13} />
              )}
              Send Guidance & Retry
            </button>
          </div>
        </div>
      </div>
    </div>
  );
}
