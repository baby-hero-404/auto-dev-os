"use client";

import { useState, useCallback } from "react";
import { useRouter } from "next/navigation";
import { Sparkles, Play, Trash2, Settings, Check, MessageSquare } from "lucide-react";
import { ConfirmDialog } from "@/components/ui/confirm-dialog";
import { useTaskDetail } from "./TaskDetailContext";

export function WorkflowProgress() {
  const {
    workflow,
    workflowCompletion,
    displayFiles,
  } = useTaskDetail();

  const checkpointsCount = workflow?.checkpoints?.length ?? 0;
  const attemptsCount = workflow?.job?.attempts ?? 0;
  const filesCount = displayFiles.length;

  return (
    <div className="rounded-xl border border-stroke bg-card p-5 shadow-sm flex flex-col gap-4">
      <div className="flex items-center justify-between gap-3">
        <span className="text-xs font-medium text-content-muted">Workflow progress</span>
        <span className="font-mono text-sm font-semibold text-foreground">{workflowCompletion}%</span>
      </div>
      <div className="h-2 overflow-hidden rounded-full bg-surface">
        <div className="h-full rounded-full bg-brand-primary transition-all" style={{ width: `${workflowCompletion}%` }} />
      </div>
      <div className="grid grid-cols-3 gap-2 text-center text-[11px] text-content-muted">
        <div className="rounded border border-stroke bg-card px-2 py-1">
          <div className="font-mono text-foreground font-bold">{checkpointsCount}</div>
          checkpoints
        </div>
        <div className="rounded border border-stroke bg-card px-2 py-1">
          <div className="font-mono text-foreground font-bold">{attemptsCount}</div>
          attempts
        </div>
        <div className="rounded border border-stroke bg-card px-2 py-1">
          <div className="font-mono text-foreground font-bold">{filesCount}</div>
          files
        </div>
      </div>
    </div>
  );
}

export function TaskActions() {
  const router = useRouter();
  const {
    projectID,
    task,
    workflow,
    startReview,
    submittingPR,
    isExecutionReady,
    isPaused,
    retry,
    analyze,
    execute,
    deleteTask,
    requestSpecChanges,
    approveSpec,
    clarificationQuestions,
  } = useTaskDetail();

  const [isDeleting, setIsDeleting] = useState(false);
  const [isDeleteConfirmOpen, setIsDeleteConfirmOpen] = useState(false);

  const handleStartReview = useCallback(async () => {
    await startReview();
  }, [startReview]);

  const handleTriggerAction = useCallback(async () => {
    if (!task) return;
    if (task.status === "failed") {
      await retry();
    } else {
      await analyze();
    }
  }, [task, retry, analyze]);

  const handleTriggerExecute = useCallback(async () => {
    if (!task) return;
    if (task.status === "failed") {
      await retry();
    } else {
      await execute();
    }
  }, [task, retry, execute]);

  const handleDeleteTrigger = useCallback(() => {
    setIsDeleteConfirmOpen(true);
  }, []);

  const handleDeleteConfirm = useCallback(async () => {
    setIsDeleting(true);
    const success = await deleteTask();
    if (success) {
      router.push(`/projects/${projectID}`);
    } else {
      setIsDeleting(false);
    }
  }, [deleteTask, router, projectID]);

  const handleDeleteClose = useCallback(() => {
    setIsDeleteConfirmOpen(false);
  }, []);

  return (
    <div className="rounded-xl border border-stroke bg-card p-5 shadow-sm flex flex-col gap-3">
      <div className="flex items-center gap-2 border-b border-stroke pb-2">
        <Settings size={14} className="text-brand-primary" />
        <span className="text-[10px] font-bold uppercase tracking-wider text-content-muted">Task Controls</span>
      </div>
      
      {/* Primary Actions Panel */}
      <div className="flex flex-col gap-3 mt-1">
        {task && (task.spec_status === "pending_review" || task.spec_status === "changes_requested") && (
          <div className="flex flex-col sm:flex-row gap-2">
            <button
              className="inline-flex flex-1 items-center justify-center gap-2 rounded-md border border-amber-500/40 bg-amber-500/10 px-3 py-2.5 text-sm font-semibold text-amber-600 dark:text-amber-400 transition hover:bg-amber-500/20 cursor-pointer shadow-sm"
              onClick={requestSpecChanges}
              type="button"
            >
              <MessageSquare size={15} />
              Request Changes
            </button>
            <button
              className="inline-flex flex-1 items-center justify-center gap-2 rounded-md bg-amber-500 px-3 py-2.5 text-sm font-semibold text-slate-950 transition hover:opacity-90 disabled:opacity-50 disabled:cursor-not-allowed cursor-pointer shadow-sm"
              onClick={approveSpec}
              type="button"
              disabled={clarificationQuestions.length > 0}
              title={clarificationQuestions.length > 0 ? "Please answer all clarification questions before approving" : undefined}
            >
              <Check size={15} />
              Approve Spec
            </button>
          </div>
        )}

        {task && task.status === "pr_ready" && (
          <button
            className="w-full inline-flex items-center justify-center gap-2 rounded-md bg-brand-primary px-3 py-2.5 text-sm font-semibold text-slate-950 transition hover:opacity-90 shadow-sm cursor-pointer"
            onClick={handleStartReview}
            type="button"
            disabled={submittingPR}
          >
            <Sparkles size={15} />
            Start Review
          </button>
        )}

        {task && (task.status === "todo" || task.status === "failed") && !isExecutionReady && (
          <button
            className="w-full inline-flex items-center justify-center gap-2 rounded-md bg-brand-primary px-3 py-2.5 text-sm font-semibold text-slate-950 transition hover:opacity-90 shadow-sm cursor-pointer"
            onClick={handleTriggerAction}
            type="button"
          >
            <Play size={15} />
            {task.status === "failed" ? "Retry Analyze" : "Analyze"}
          </button>
        )}

        {task && isExecutionReady && (
          <button
            className="w-full inline-flex items-center justify-center gap-2 rounded-md bg-brand-primary px-3 py-2.5 text-sm font-semibold text-slate-950 transition hover:opacity-90 shadow-sm cursor-pointer"
            onClick={handleTriggerExecute}
            type="button"
          >
            <Play size={15} fill="currentColor" />
            {task.status === "failed" ? "Retry Execute" : "Execute"}
          </button>
        )}

        {task && isPaused && 
         task.spec_status !== "pending_review" && 
         task.spec_status !== "changes_requested" && 
         task.spec_status !== "clarification_required" && (
          <button
            className="w-full inline-flex items-center justify-center gap-2 rounded-md bg-brand-primary px-3 py-2.5 text-sm font-semibold text-slate-950 transition hover:opacity-90 shadow-sm cursor-pointer"
            onClick={execute}
            type="button"
          >
            <Play size={15} fill="currentColor" />
            Resume
          </button>
        )}

        {task && workflow?.job?.status === "running" && !isPaused && (
          <div className="flex flex-col items-center justify-center p-4 rounded-lg bg-surface/30 border border-stroke/40 text-center gap-1.5">
            <span className="text-xs font-semibold text-content-muted">Workflow Active</span>
            <span className="text-[11px] text-content-muted/80 leading-normal">
              AI agent is actively executing the workflow. Controls are available in the page header.
            </span>
          </div>
        )}
      </div>

      {/* Secondary Actions Panel */}
      {task && (
        <div className="border-t border-stroke/30 pt-3 mt-1 flex justify-end">
          <button
            className="inline-flex items-center gap-1.5 rounded-md border border-danger/30 bg-transparent px-2.5 py-1.5 text-xs font-semibold text-danger transition hover:bg-danger/5 disabled:opacity-50 cursor-pointer shadow-sm"
            onClick={handleDeleteTrigger}
            type="button"
            disabled={isDeleting}
          >
            <Trash2 size={13} />
            Delete Task
          </button>
        </div>
      )}

      <ConfirmDialog
        isOpen={isDeleteConfirmOpen}
        title="Delete Task"
        description="Are you sure you want to delete this task? This action cannot be undone."
        confirmText="Delete"
        variant="danger"
        onConfirm={handleDeleteConfirm}
        onClose={handleDeleteClose}
      />
    </div>
  );
}
