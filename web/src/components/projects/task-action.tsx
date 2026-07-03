import { useState } from "react";
import Link from "next/link";
import { Play, Search, CheckCircle2, ShieldAlert, Activity, Loader2, Eye, Trash2 } from "lucide-react";
import type { Task } from "@/lib/types";
import { ConfirmDialog } from "@/components/ui/confirm-dialog";

interface TaskActionProps {
  task: Task;
  projectID: string;
  isLoading?: boolean;
  onAction?: (action: "analyze" | "execute" | "delete") => Promise<boolean> | void;
}

export function TaskAction({ task, projectID, isLoading, onAction }: TaskActionProps) {
  const [isDeleteConfirmOpen, setIsDeleteConfirmOpen] = useState(false);

  const isPendingSpecReview = task.status === "spec_review" || task.spec_status === "pending_review";
  const isExecutionReady =
    (task.spec_status === "auto_approved" || task.spec_status === "approved") &&
    task.status === "todo";

  const showMonitor = [
    "context_loading",
    "analyzing",
    "coding",
    "reviewing",
    "fixing",
    "testing",
  ].includes(task.status);

  return (
    <div className="flex shrink-0 flex-wrap gap-2">
      {task.status === "todo" && !isExecutionReady && (
        <button
          className="inline-flex items-center gap-2 rounded-md bg-brand-primary px-3 py-2 text-sm font-medium text-slate-950 transition hover:opacity-90 disabled:opacity-50 cursor-pointer"
          disabled={isLoading}
          onClick={() => onAction?.("analyze")}
        >
          {isLoading ? <Loader2 size={15} className="animate-spin" /> : <Play size={15} />} Analyze
        </button>
      )}

      {isPendingSpecReview && (
        <Link
          href={`/projects/${projectID}/tasks/${task.id}`}
          className="inline-flex items-center gap-2 rounded-md border border-warning/40 bg-warning/10 px-3 py-2 text-sm font-medium text-warning transition hover:bg-warning/20"
        >
          <Search size={15} /> Review Spec
        </Link>
      )}

      {isExecutionReady && (
        <button
          className="inline-flex items-center gap-2 rounded-md bg-brand-primary px-3 py-2 text-sm font-medium text-slate-950 transition hover:opacity-90 disabled:opacity-50 cursor-pointer"
          disabled={isLoading}
          onClick={() => onAction?.("execute")}
        >
          {isLoading ? <Loader2 size={15} className="animate-spin" /> : <Play size={15} />} Execute
        </button>
      )}

      {task.status === "human_review" && (
        <Link
          href={`/projects/${projectID}/tasks/${task.id}`}
          className="inline-flex items-center gap-2 rounded-md border border-success/40 bg-success/10 px-3 py-2 text-sm font-medium text-success transition hover:bg-success/20"
        >
          <CheckCircle2 size={15} /> Review PR
        </Link>
      )}

      {task.status === "failed" && (
        <Link
          href={`/projects/${projectID}/tasks/${task.id}`}
          className="inline-flex items-center gap-2 rounded-md border border-danger/40 bg-danger/10 px-3 py-2 text-sm font-medium text-danger transition hover:bg-danger/20"
        >
          <ShieldAlert size={15} /> Inspect
        </Link>
      )}

      {showMonitor && (
        <Link
          href={`/projects/${projectID}/tasks/${task.id}`}
          className="inline-flex items-center gap-2 rounded-md border border-info/40 bg-info/10 px-3 py-2 text-sm font-medium text-info transition hover:bg-info/20"
        >
          <Activity size={15} /> Monitor
        </Link>
      )}

      {/* Secondary action: Details is only shown if no other primary link to the same page exists */}
      {!isPendingSpecReview && task.status !== "human_review" && task.status !== "failed" && !showMonitor && (
        <Link
          className="inline-flex items-center gap-2 rounded-md border border-stroke bg-panel text-foreground px-3 py-2 text-sm transition hover:bg-surface"
          href={`/projects/${projectID}/tasks/${task.id}`}
        >
          <Eye size={15} /> Details
        </Link>
      )}

      <button
        className="inline-flex h-9 w-9 items-center justify-center rounded-md border border-stroke bg-panel text-content-muted transition hover:border-danger/30 hover:bg-danger/10 hover:text-danger disabled:opacity-50 cursor-pointer"
        disabled={isLoading}
        title="Delete Task"
        aria-label="Delete Task"
        onClick={() => setIsDeleteConfirmOpen(true)}
      >
        <Trash2 size={15} />
      </button>

      <ConfirmDialog
        isOpen={isDeleteConfirmOpen}
        title="Delete Task"
        description="Are you sure you want to delete this task? This action cannot be undone."
        confirmText="Delete"
        variant="danger"
        isLoading={isLoading}
        onConfirm={async () => {
          if (onAction) {
            await onAction("delete");
            setIsDeleteConfirmOpen(false);
          }
        }}
        onClose={() => setIsDeleteConfirmOpen(false)}
      />
    </div>
  );
}
