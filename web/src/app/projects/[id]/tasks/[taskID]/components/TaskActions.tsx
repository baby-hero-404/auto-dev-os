"use client";

import { useState, useCallback } from "react";
import { useRouter } from "next/navigation";
import { Sparkles, Play, Trash2, Pause, Ban } from "lucide-react";
import { ConfirmDialog } from "@/components/ui/confirm-dialog";
import { useTaskDetail } from "./TaskDetailContext";

export function TaskActions() {
  const router = useRouter();
  const {
    projectID,
    task,
    workflow,
    workflowCompletion,
    displayFiles,
    startReview,
    submittingPR,
    isExecutionReady,
    isPaused,
    retry,
    analyze,
    execute,
    pause,
    cancel,
    deleteTask,
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

  const checkpointsCount = workflow?.checkpoints?.length ?? 0;
  const attemptsCount = workflow?.job?.attempts ?? 0;
  const filesCount = displayFiles.length;

  const jobStatus = workflow?.job?.status?.toLowerCase();
  const canPause = jobStatus === "running";
  const canCancel = jobStatus === "running" || jobStatus === "paused" || jobStatus === "queued";

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
          <div className="font-mono text-foreground">{checkpointsCount}</div>
          checkpoints
        </div>
        <div className="rounded border border-stroke bg-card px-2 py-1">
          <div className="font-mono text-foreground">{attemptsCount}</div>
          attempts
        </div>
        <div className="rounded border border-stroke bg-card px-2 py-1">
          <div className="font-mono text-foreground">{filesCount}</div>
          files
        </div>
      </div>
      <div className="flex flex-wrap gap-2">
        {task && task.status === "pr_ready" && (
          <button
            className="inline-flex flex-1 items-center justify-center gap-2 rounded-md border border-brand-primary bg-transparent px-3 py-2 text-sm font-semibold text-brand-primary transition hover:bg-brand-primary/10 shadow-sm cursor-pointer"
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
            className="inline-flex flex-1 items-center justify-center gap-2 rounded-md bg-brand-primary px-3 py-2 text-sm font-semibold text-slate-950 transition hover:opacity-90 shadow-sm cursor-pointer"
            onClick={handleTriggerAction}
            type="button"
          >
            <Play size={15} />
            {task.status === "failed" ? "Retry Analyze" : "Analyze"}
          </button>
        )}
        {task && isExecutionReady && (
          <button
            className="inline-flex flex-1 items-center justify-center gap-2 rounded-md bg-brand-primary px-3 py-2 text-sm font-semibold text-slate-950 transition hover:opacity-90 shadow-sm cursor-pointer"
            onClick={handleTriggerExecute}
            type="button"
          >
            <Play size={15} fill="currentColor" />
            {task.status === "failed" ? "Retry Execute" : "Execute"}
          </button>
        )}
        {task && isPaused && (
          <button
            className="inline-flex flex-1 items-center justify-center gap-2 rounded-md bg-brand-primary px-3 py-2 text-sm font-semibold text-slate-950 transition hover:opacity-90 shadow-sm cursor-pointer"
            onClick={execute}
            type="button"
          >
            <Play size={15} fill="currentColor" />
            Resume
          </button>
        )}
        {task && canPause && (
          <button
            className="inline-flex flex-1 items-center justify-center gap-2 rounded-md border border-stroke bg-card px-3 py-2 text-sm font-semibold text-foreground transition hover:bg-surface shadow-sm cursor-pointer"
            onClick={pause}
            type="button"
          >
            <Pause size={15} />
            Pause
          </button>
        )}
        {task && canCancel && (
          <button
            className="inline-flex flex-1 items-center justify-center gap-2 rounded-md border border-danger/40 bg-danger/10 px-3 py-2 text-sm font-semibold text-danger transition hover:bg-danger/20 cursor-pointer shadow-sm"
            onClick={cancel}
            type="button"
          >
            <Ban size={15} />
            Close Task
          </button>
        )}
        {task && (
          <button
            className="inline-flex flex-1 items-center justify-center gap-2 rounded-md border border-danger/40 bg-danger/10 px-3 py-2 text-sm font-semibold text-danger transition hover:bg-danger/20 disabled:opacity-50 cursor-pointer shadow-sm"
            onClick={handleDeleteTrigger}
            type="button"
            disabled={isDeleting}
          >
            <Trash2 size={15} />
            Delete Task
          </button>
        )}
      </div>

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
