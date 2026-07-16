"use client";

import { useState, useCallback } from "react";
import Link from "next/link";
import { ArrowLeft, Edit2, Check, X, Play, Pause, Ban } from "lucide-react";
import { Badge, taskStatusBadge } from "@/components/ui/badge";
import { Markdown } from "@/components/ui/markdown";
import { useTaskDetail, formatStepName } from "./TaskDetailContext";

export function TaskHeader() {
  const {
    projectID,
    task,
    workflow,
    descriptionParts,
    updateTask,
    workflowCompletion,
    analysisData,
    execute,
    pause,
    cancel,
    isPaused,
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

  const [isEditingTitle, setIsEditingTitle] = useState(false);
  const [isEditingDesc, setIsEditingDesc] = useState(false);
  const [editedTitle, setEditedTitle] = useState("");
  const [editedDesc, setEditedDesc] = useState("");
  const [isSaving, setIsSaving] = useState(false);

  const [prevTaskTitle, setPrevTaskTitle] = useState(task?.title ?? "");
  if (task?.title !== prevTaskTitle) {
    setPrevTaskTitle(task?.title ?? "");
    if (!isEditingTitle && task?.title) {
      setEditedTitle(task.title);
    }
  }

  const [prevTaskDesc, setPrevTaskDesc] = useState(task?.description ?? "");
  if (task?.description !== prevTaskDesc) {
    setPrevTaskDesc(task?.description ?? "");
    if (!isEditingDesc && task?.description) {
      setEditedDesc(task.description);
    }
  }

  const handleStartEditTitle = useCallback(() => {
    setEditedTitle(task?.title ?? "");
    setIsEditingTitle(true);
  }, [task?.title]);

  const handleStartEditDesc = useCallback(() => {
    setEditedDesc(task?.description ?? "");
    setIsEditingDesc(true);
  }, [task?.description]);

  const handleSaveTitle = useCallback(async () => {
    if (!editedTitle.trim()) return;
    setIsSaving(true);
    await updateTask({ title: editedTitle.trim() });
    setIsEditingTitle(false);
    setIsSaving(false);
  }, [editedTitle, updateTask]);

  const handleSaveDesc = useCallback(async () => {
    setIsSaving(true);
    await updateTask({ description: editedDesc.trim() });
    setIsEditingDesc(false);
    setIsSaving(false);
  }, [editedDesc, updateTask]);

  const handleCancelTitle = useCallback(() => {
    setIsEditingTitle(false);
  }, []);

  const handleCancelDesc = useCallback(() => {
    setIsEditingDesc(false);
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

          {isEditingDesc ? (
            <div className="flex flex-col gap-2 mt-3 max-w-4xl">
              <textarea
                value={editedDesc}
                onChange={(e) => setEditedDesc(e.target.value)}
                className="text-sm text-foreground bg-surface border border-stroke rounded px-3 py-2 focus:outline-none focus:border-brand-primary min-h-[100px] resize-y"
                disabled={isSaving}
                autoFocus
                placeholder="Detail the target objective, files to modify, or technical requirements."
              />
              <div className="flex gap-2 justify-end">
                <button
                  onClick={handleCancelDesc}
                  disabled={isSaving}
                  className="px-3 py-1.5 text-xs font-semibold border border-stroke hover:bg-surface rounded transition cursor-pointer disabled:opacity-50"
                >
                  Cancel
                </button>
                <button
                  onClick={handleSaveDesc}
                  disabled={isSaving}
                  className="px-3 py-1.5 text-xs font-semibold bg-brand-primary text-slate-950 hover:opacity-90 rounded transition cursor-pointer disabled:opacity-50"
                >
                  {isSaving ? "Saving..." : "Save Description"}
                </button>
              </div>
            </div>
          ) : (
            <div className="group mt-1.5 max-w-4xl space-y-3">
              <div className="flex items-start gap-2">
                <div className="min-w-0 flex-1 rounded-lg border border-stroke bg-surface/20 p-3">
                  {descriptionParts.body ? (
                    <div className="prose prose-sm max-w-none text-content-muted dark:prose-invert prose-headings:text-foreground prose-strong:text-foreground prose-p:leading-relaxed prose-li:leading-relaxed">
                      <Markdown content={descriptionParts.body} />
                    </div>
                  ) : (
                    <p className="text-sm text-content-muted/70 italic">No description provided. Click the edit icon to add one.</p>
                  )}
                </div>
                {task && (
                  <button
                    onClick={handleStartEditDesc}
                    className="opacity-40 hover:opacity-100 focus:opacity-100 group-hover:opacity-100 focus-within:opacity-100 p-1 text-content-muted hover:text-foreground hover:bg-surface rounded transition shrink-0 cursor-pointer"
                    title="Edit Description"
                  >
                    <Edit2 size={14} />
                  </button>
                )}
              </div>
              {descriptionParts.context && (
                <div className="rounded-lg border border-warning/20 bg-warning/5 p-3 text-xs text-content-muted">
                  <div className="mb-2 font-mono text-[10px] font-bold uppercase tracking-wider text-warning">
                    Request history (Legacy)
                  </div>
                  <div className="prose prose-sm max-w-none text-content-muted dark:prose-invert prose-headings:text-foreground prose-strong:text-foreground prose-p:leading-relaxed prose-li:leading-relaxed">
                    <Markdown content={descriptionParts.context} />
                  </div>
                </div>
              )}

              {task?.clarifications && task.clarifications.length > 0 && (
                <div className="rounded-lg border border-stroke bg-surface/20 p-3 text-xs text-content-muted">
                  <div className="mb-2 font-mono text-[10px] font-bold uppercase tracking-wider text-warning">
                    Clarification History
                  </div>
                  <div className="space-y-3">
                    {task.clarifications.map((round) => (
                      <div key={round.round} className="border-t border-stroke/50 pt-2 first:border-0 first:pt-0">
                        <div className="text-[10px] font-semibold text-content-muted mb-1.5 flex justify-between items-center">
                          <span>Round {round.round}</span>
                          <span className="opacity-70">{new Date(round.timestamp).toLocaleString()}</span>
                        </div>
                        <div className="pl-3 border-l-2 border-warning/30 space-y-1.5 text-xs text-foreground/90 bg-warning/[0.02] p-2 rounded-r">
                          <div className="prose prose-sm max-w-none text-content-muted dark:prose-invert">
                            <Markdown content={round.response} />
                          </div>
                        </div>
                      </div>
                    ))}
                  </div>
                </div>
              )}
            </div>
          )}
        </div>
      </div>

      {task && (
        <div className="mt-5 pt-4 border-t border-stroke/30 flex flex-wrap items-center justify-between gap-4">
          <div className="flex flex-wrap items-center gap-6 text-xs text-content-muted">
            <div className="flex items-center gap-1.5">
              <span className="font-semibold text-foreground">Current Step:</span>
              <span className="font-mono bg-surface/80 px-2 py-0.5 rounded border border-stroke/40 font-bold text-foreground">
                {workflow?.job?.step ? formatStepName(workflow.job.step, analysisData) : "None"}
              </span>
            </div>
            <div className="flex items-center gap-1.5">
              <span className="font-semibold text-foreground">Progress:</span>
              <span className="font-mono bg-surface/80 px-2 py-0.5 rounded border border-stroke/40 font-bold text-foreground">
                {workflowCompletion}%
              </span>
            </div>
            <div className="flex items-center gap-1.5">
              <span className="font-semibold text-foreground">Assigned Agent:</span>
              <span className="font-mono bg-surface/80 px-2 py-0.5 rounded border border-stroke/40 font-bold text-foreground">
                {task.agent_id || "Unassigned"}
              </span>
            </div>
          </div>

          <div className="flex items-center gap-2">
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
          </div>
        </div>
      )}
    </header>
  );
}
