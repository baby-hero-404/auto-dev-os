"use client";

import { useState } from "react";
import { toast } from "sonner";
import { Loader2 } from "lucide-react";
import { useTaskDetail } from "./TaskDetailContext";
import { tasks as tasksApi } from "@/lib/api/projects";
import type { AvailableAction } from "@/lib/types";

const STYLE_CLASSES: Record<string, string> = {
  primary: "bg-brand-primary hover:opacity-90 text-background",
  warning: "bg-amber-600 hover:bg-amber-700 text-white",
  danger: "bg-rose-600 hover:bg-rose-500 text-white",
  default: "bg-card border border-stroke hover:bg-stroke/20 text-content",
};

/** Renders task.available_actions and dispatches via the single /actions endpoint. */
export function DynamicActionBar() {
  const { task, taskID, token, mutateWorkflow, setError } = useTaskDetail();
  const [pendingActionID, setPendingActionID] = useState<string | null>(null);

  const actions = task?.available_actions ?? [];
  if (actions.length === 0) return null;

  const handleDispatch = async (action: AvailableAction) => {
    if (!token || pendingActionID) return;
    if (action.confirmation_required && !window.confirm(`${action.label}?`)) return;

    setPendingActionID(action.id);
    try {
      await tasksApi.dispatchAction(taskID, token, action.id, crypto.randomUUID());
      await mutateWorkflow();
    } catch (err) {
      const message = err instanceof Error ? err.message : `Failed to dispatch "${action.label}"`;
      toast.error(message);
      setError(message);
    } finally {
      setPendingActionID(null);
    }
  };

  return (
    <div className="flex flex-wrap items-center gap-2.5 mb-6">
      {actions.map((action) => (
        <button
          key={action.id}
          onClick={() => handleDispatch(action)}
          disabled={!!action.disabled_reason || pendingActionID !== null}
          title={action.disabled_reason}
          className={`px-4 py-2 rounded-xl border-none text-xs font-bold transition-all duration-150 shadow-sm cursor-pointer disabled:opacity-50 disabled:cursor-not-allowed flex items-center gap-2 ${STYLE_CLASSES[action.style] ?? STYLE_CLASSES.default}`}
        >
          {pendingActionID === action.id && <Loader2 className="w-3.5 h-3.5 animate-spin" />}
          {action.label}
        </button>
      ))}
    </div>
  );
}
