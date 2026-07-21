"use client";

import { useState, useCallback, useEffect } from "react";
import { AlertCircle, Loader2 } from "lucide-react";
import { useTaskDetail } from "./TaskDetailContext";
import { TaskHeader } from "./TaskHeader";
import { TaskTitleBlock } from "./TaskTitleBlock";
import { TaskHeroCards } from "./TaskHeroCards";
import { TaskSubtasks } from "./TaskSubtasks";
import { TaskSidebar } from "./TaskSidebar";
import { BoundaryResolutionControls } from "./BoundaryResolutionControls";
import { SupportingAccordion } from "./SupportingAccordion";

export function TaskDetailLayout() {
  const { task, workflow, updateTask, execute, setError, isTaskLoading, workflowError } = useTaskDetail();

  const [openSections, setOpenSections] = useState<Record<string, boolean>>({
    specification: false,
    logs: false,
    description: false,
    checkpoints: false,
  });

  const toggleSection = useCallback((key: string) => {
    setOpenSections((prev) => ({ ...prev, [key]: !prev[key] }));
  }, []);

  useEffect(() => {
    const runningStatuses = ["context_loading", "analyzing", "planning", "coding", "testing", "reviewing", "fixing"];
    if (task && runningStatuses.includes(task.status)) {
      // eslint-disable-next-line react-hooks/set-state-in-effect
      setOpenSections((prev) => {
        if (!prev.logs || !prev.checkpoints) {
          return { ...prev, logs: true, checkpoints: true };
        }
        return prev;
      });
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [task?.status]);


  if (isTaskLoading) {
    return (
      <main className="min-h-screen bg-background p-6 flex flex-col justify-center items-center gap-4">
        <Loader2 className="h-8 w-8 animate-spin text-brand-primary" />
        <p className="text-sm font-mono text-content-muted animate-pulse">Loading task workspace...</p>
      </main>
    );
  }

  if (workflowError) {
    return (
      <main className="grid min-h-screen place-items-center p-6 bg-background">
        <div className="rounded-lg border border-danger/20 bg-danger/5 p-6 max-w-lg text-center">
          <AlertCircle className="h-10 w-10 text-danger mx-auto mb-4" />
          <p className="font-sans text-base font-semibold text-danger">Failed to load task workflow.</p>
          <div className="flex justify-center gap-3 mt-4">
            <button onClick={() => window.location.reload()} className="rounded-md bg-brand-primary px-4 py-2 text-sm font-semibold text-slate-950 hover:opacity-90 transition">
              Retry Load
            </button>
          </div>
        </div>
      </main>
    );
  }

  return (
    <div className="min-h-screen bg-background text-content font-sans">
      <TaskHeader />

      <div className="max-w-295 mx-auto px-8 pt-7 pb-12">
        <TaskTitleBlock />

        <div className="grid grid-cols-1 lg:grid-cols-[1fr_300px] gap-5 items-start mb-8">
          <div id="hero-cards-section" className="flex flex-col gap-4">
            <TaskHeroCards />
            <TaskSubtasks />
          </div>

          <div className="flex flex-col gap-4">
            <TaskSidebar />
          </div>
        </div>

        <SupportingAccordion openSections={openSections} onToggleSection={toggleSection} />
      </div>

      {workflow?.job?.status === "paused" &&
        workflow?.job?.last_error &&
        !workflow.job.last_error.includes("workflow paused for human task clarification") &&
        task?.status !== "pr_ready" &&
        task?.status !== "human_review" &&
        task?.status !== "merged" && (
          <div className="max-w-295 mx-auto px-8 pb-12">
            <div className="rounded-lg border border-amber-500/20 bg-amber-500/10 p-4 text-sm text-amber-700 dark:text-amber-300 flex flex-col gap-2">
              <div className="flex items-center gap-2 font-semibold text-amber-600 dark:text-amber-400">
                <AlertCircle size={16} className="shrink-0 text-amber-500" />
                Task Execution Paused (Human Action Required)
              </div>
              <p className="text-xs font-mono bg-amber-500/5 dark:bg-amber-950/20 border border-amber-500/10 dark:border-amber-900/30 rounded-lg p-3 break-all whitespace-pre-wrap text-amber-800 dark:text-amber-200">
                {workflow.job.last_error}
              </p>
              <BoundaryResolutionControls
                errorMsg={workflow.job.last_error}
                task={task}
                updateTask={updateTask}
                execute={execute}
                setError={setError}
              />
            </div>
          </div>
        )}
    </div>
  );
}
