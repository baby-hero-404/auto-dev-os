"use client";

import { useState, useCallback } from "react";
import Link from "next/link";
import { useRouter } from "next/navigation";
import { ArrowLeft, Edit2, Check, X, Play, Pause, Ban, Trash2 } from "lucide-react";
import { Badge, taskStatusBadge } from "@/components/ui/badge";
import { ConfirmDialog } from "@/components/ui/confirm-dialog";
import { useTaskDetail } from "./TaskDetailContext";

export function TaskHeader() {
  const router = useRouter();
  const {
    projectID,
    task,
    workflow,
    updateTask,
    execute,
    analyze,
    retry,
    pause,
    cancel,
    deleteTask,
    isPaused,
    isExecutionReady,
  } = useTaskDetail();

  const jobStatus = workflow?.job?.status?.toLowerCase();
  const canPause = jobStatus === "running";
  const canCancel = jobStatus === "running" || jobStatus === "paused" || jobStatus === "queued";
  const canResume = !!(
    task &&
    isPaused &&
    task.spec_status !== "pending_review" &&
    task.spec_status !== "changes_requested" &&
    task.spec_status !== "clarification_required"
  );

  const showAnalyze = !!(task && (task.status === "todo" || task.status === "failed") && !isExecutionReady);
  const showExecute = !!(task && isExecutionReady);

  const [isDeleting, setIsDeleting] = useState(false);
  const [isDeleteConfirmOpen, setIsDeleteConfirmOpen] = useState(false);

  const handleAnalyze = useCallback(async () => {
    if (!task) return;
    if (task.status === "failed") await retry();
    else await analyze();
  }, [task, retry, analyze]);

  const handleExecute = useCallback(async () => {
    if (!task) return;
    if (task.status === "failed") await retry();
    else await execute();
  }, [task, retry, execute]);

  const handleDeleteConfirm = useCallback(async () => {
    setIsDeleting(true);
    const success = await deleteTask();
    if (success) {
      router.push(`/projects/${projectID}`);
    } else {
      setIsDeleting(false);
      setIsDeleteConfirmOpen(false);
    }
  }, [deleteTask, router, projectID]);

  const [isEditingTitle, setIsEditingTitle] = useState(false);
  const [editedTitle, setEditedTitle] = useState("");
  const [isSaving, setIsSaving] = useState(false);

  const [prevTaskTitle, setPrevTaskTitle] = useState(task?.title ?? "");
  if (task?.title !== prevTaskTitle) {
    setPrevTaskTitle(task?.title ?? "");
    if (!isEditingTitle && task?.title) {
      setEditedTitle(task.title);
    }
  }

  const handleStartEditTitle = useCallback(() => {
    setEditedTitle(task?.title ?? "");
    setIsEditingTitle(true);
  }, [task?.title]);

  const handleSaveTitle = useCallback(async () => {
    if (!editedTitle.trim()) return;
    setIsSaving(true);
    await updateTask({ title: editedTitle.trim() });
    setIsEditingTitle(false);
    setIsSaving(false);
  }, [editedTitle, updateTask]);

  const handleCancelTitle = useCallback(() => {
    setIsEditingTitle(false);
  }, []);

  return (
    <header className="relative overflow-hidden rounded-xl border border-stroke/50 bg-card/60 backdrop-blur-xl p-5 shadow-lg transition-all hover:shadow-xl hover:border-stroke/80 group">
      <div className="absolute inset-0 bg-gradient-to-br from-brand-primary/5 via-transparent to-transparent opacity-0 group-hover:opacity-100 transition-opacity duration-500 pointer-events-none" />
      <div className="relative flex flex-col justify-between gap-5 xl:flex-row xl:items-start z-10">
        <div className="min-w-0 flex-1">
          <Link
            href={`/projects/${projectID}`}
            className="mb-3 inline-flex items-center gap-1.5 text-xs font-semibold text-content-muted transition hover:text-foreground"
          >
            <ArrowLeft size={14} />
            Back to Project
          </Link>

          {isEditingTitle ? (
            <div className="flex items-center gap-2 mt-1">
              <input
                type="text"
                value={editedTitle}
                onChange={(e) => setEditedTitle(e.target.value)}
                className="font-heading text-2xl md:text-3xl font-bold tracking-tight text-foreground bg-surface border border-stroke rounded px-3 py-1 focus:outline-none focus:border-brand-primary min-w-[300px] max-w-xl"
                disabled={isSaving}
                autoFocus
              />
              <button
                onClick={handleSaveTitle}
                disabled={isSaving}
                className="p-2 bg-success/10 hover:bg-success/20 text-success rounded border border-success/20 transition cursor-pointer"
                title="Save Title"
              >
                <Check size={16} />
              </button>
              <button
                onClick={handleCancelTitle}
                disabled={isSaving}
                className="p-2 bg-danger/10 hover:bg-danger/20 text-danger rounded border border-danger/20 transition cursor-pointer"
                title="Cancel"
              >
                <X size={16} />
              </button>
            </div>
          ) : (
            <h1 className="group flex items-center gap-2 font-heading text-2xl md:text-3xl font-bold tracking-tight text-foreground">
              <span className="min-w-0 truncate">{task?.title ?? "Task workflow"}</span>
              {task && (
                <button
                  onClick={handleStartEditTitle}
                  className="opacity-40 hover:opacity-100 focus:opacity-100 group-hover:opacity-100 focus-within:opacity-100 p-1 text-content-muted hover:text-foreground hover:bg-surface rounded transition cursor-pointer"
                  title="Edit Title"
                >
                  <Edit2 size={16} />
                </button>
              )}
            </h1>
          )}

          <div className="mt-3 flex flex-wrap items-center gap-2">
            {task && (() => {
              const statusInfo = taskStatusBadge(task.status);
              return <Badge variant={statusInfo.variant} value={statusInfo.label} />;
            })()}
            {task?.spec_status && task.spec_status !== "none" && <Badge value={task.spec_status} />}
            {workflow?.job && (() => {
              const jobStatusInfo = taskStatusBadge(workflow.job.status);
              return <Badge variant={jobStatusInfo.variant} value={jobStatusInfo.label} />;
            })()}
            {task && (
              <span className="rounded border border-stroke bg-surface px-2 py-0.5 text-xs font-medium text-content-muted">
                Priority {task.priority}
              </span>
            )}
          </div>
        </div>

        {task && (
          <div className="flex items-center gap-2 xl:mt-6">
            {showAnalyze && (
              <button
                onClick={handleAnalyze}
                className="inline-flex items-center gap-1.5 rounded-md bg-brand-primary px-4 py-1.5 text-xs font-semibold text-slate-950 transition-all duration-300 hover:opacity-90 hover:shadow-[0_0_15px_rgba(var(--brand-primary),0.4)] hover:scale-[1.02] active:scale-95 cursor-pointer shadow-sm"
              >
                <Play size={13} />
                {task.status === "failed" ? "Retry Analyze" : "Analyze"}
              </button>
            )}
            {showExecute && (
              <button
                onClick={handleExecute}
                className="inline-flex items-center gap-1.5 rounded-md bg-brand-primary px-4 py-1.5 text-xs font-semibold text-slate-950 transition-all duration-300 hover:opacity-90 hover:shadow-[0_0_15px_rgba(var(--brand-primary),0.4)] hover:scale-[1.02] active:scale-95 cursor-pointer shadow-sm"
              >
                <Play size={13} fill="currentColor" />
                {task.status === "failed" ? "Retry Execute" : "Execute"}
              </button>
            )}
            {canResume && (
              <button
                onClick={execute}
                className="inline-flex items-center gap-1.5 rounded-md bg-brand-primary px-4 py-1.5 text-xs font-semibold text-slate-950 transition-all duration-300 hover:opacity-90 hover:shadow-[0_0_15px_rgba(var(--brand-primary),0.4)] hover:scale-[1.02] active:scale-95 cursor-pointer shadow-sm"
              >
                <Play size={13} fill="currentColor" />
                Resume
              </button>
            )}
            {canPause && (
              <button
                onClick={pause}
                className="inline-flex items-center gap-1.5 rounded-md border border-stroke/50 bg-card/80 backdrop-blur px-4 py-1.5 text-xs font-semibold text-foreground transition-all duration-300 hover:bg-surface hover:border-stroke hover:shadow-md hover:scale-[1.02] active:scale-95 cursor-pointer shadow-sm"
              >
                <Pause size={13} />
                Pause
              </button>
            )}
            {canCancel && (
              <button
                onClick={cancel}
                className="inline-flex items-center gap-1.5 rounded-md border border-danger/40 bg-danger/10 px-4 py-1.5 text-xs font-semibold text-danger transition-all duration-300 hover:bg-danger/20 hover:shadow-[0_0_12px_rgba(var(--danger),0.3)] hover:border-danger/60 hover:scale-[1.02] active:scale-95 cursor-pointer shadow-sm"
              >
                <Ban size={13} />
                Close Task
              </button>
            )}
            <button
              onClick={() => setIsDeleteConfirmOpen(true)}
              disabled={isDeleting}
              title="Delete Task"
              className="inline-flex items-center gap-1.5 rounded-md border border-danger/30 bg-transparent px-2.5 py-1.5 text-xs font-semibold text-danger transition hover:bg-danger/5 disabled:opacity-50 cursor-pointer"
            >
              <Trash2 size={13} />
            </button>
          </div>
        )}
      </div>

      <ConfirmDialog
        isOpen={isDeleteConfirmOpen}
        title="Delete Task"
        description="Are you sure you want to delete this task? This action cannot be undone."
        confirmText="Delete"
        variant="danger"
        onConfirm={handleDeleteConfirm}
        onClose={() => setIsDeleteConfirmOpen(false)}
      />
    </header>
  );
}
