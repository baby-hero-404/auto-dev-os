"use client";

import { useState, useCallback, useEffect } from "react";
import { AlertCircle, Loader2 } from "lucide-react";
import { Skeleton } from "@/components/ui/skeleton";
import { useTaskDetail } from "./TaskDetailContext";
import { TaskHeader } from "./TaskHeader";
import { TaskTitleBlock } from "./TaskTitleBlock";
import { TaskHeroCards } from "./TaskHeroCards";
import { TaskSubtasks } from "./TaskSubtasks";
import { TaskSidebar } from "./TaskSidebar";
import { BoundaryResolutionControls } from "./BoundaryResolutionControls";
import { CLISpecReviewControls } from "./CLISpecReviewControls";
import { SupportingAccordion } from "./SupportingAccordion";

export function TaskDetailLayout() {
  const { task, taskID, token, workflow, updateTask, execute, mutateWorkflow, setError, isTaskLoading, workflowError } = useTaskDetail();

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
      <div className="min-h-screen bg-background text-content font-sans">
        {/* Header Skeleton */}
        <div className="flex items-center justify-between gap-4 px-8 py-3.5 bg-card border-b border-stroke">
          <Skeleton className="h-4 w-24" />
          <Skeleton className="h-8 w-28 rounded-lg" />
        </div>

        <div className="max-w-295 mx-auto px-8 pt-7 pb-12 animate-fade-in">
          {/* Title Block Skeleton */}
          <div className="flex flex-col md:flex-row md:items-center justify-between gap-4 mb-6 pb-5 border-b border-stroke/10">
            <div className="flex-1 space-y-4 w-full">
              <Skeleton className="h-9 w-full max-w-md" />
              <div className="flex gap-2">
                <Skeleton className="h-6 w-20 rounded-full" />
                <Skeleton className="h-6 w-24 rounded-full" />
                <Skeleton className="h-6 w-16 rounded-full" />
              </div>
              <Skeleton className="h-28 w-full rounded-2xl" />
            </div>
            <Skeleton className="h-16 w-32 shrink-0 rounded-xl hidden md:block" />
          </div>

          {/* Main Grid Skeleton */}
          <div className="grid grid-cols-1 lg:grid-cols-[1fr_300px] gap-5 items-start">
            <div className="flex flex-col gap-4">
              <Skeleton className="h-48 w-full rounded-2xl" />
              <Skeleton className="h-64 w-full rounded-2xl" />
            </div>
            <div className="flex flex-col gap-4">
              <Skeleton className="h-96 w-full rounded-2xl" />
            </div>
          </div>
        </div>
      </div>
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

        {workflow?.job?.status === "paused" &&
          workflow?.job?.last_error &&
          !workflow.job.last_error.includes("workflow paused for human task clarification") &&
          task?.status !== "pr_ready" &&
          task?.status !== "human_review" &&
          task?.status !== "merged" && (
            <div className="mb-6 rounded-2xl border border-amber-500/30 bg-gradient-to-br from-amber-500/10 via-amber-500/5 to-orange-500/5 backdrop-blur-md shadow-lg shadow-amber-500/5 p-5 text-sm flex flex-col gap-3 relative overflow-hidden transition-all duration-300 hover:shadow-amber-500/10">
              <div className="absolute -top-12 -right-12 w-32 h-32 bg-amber-500/10 rounded-full blur-2xl pointer-events-none" />
              <div className="flex items-center gap-2.5 font-bold text-amber-800 dark:text-amber-400 text-sm tracking-wide z-10">
                <span className="relative flex h-2 w-2">
                  <span className="animate-ping absolute inline-flex h-full w-full rounded-full bg-amber-400 opacity-75"></span>
                  <span className="relative inline-flex rounded-full h-2 w-2 bg-amber-500"></span>
                </span>
                Task Execution Paused (Action Required)
              </div>
              <p className="text-xs font-mono bg-amber-500/[0.03] dark:bg-amber-950/20 border border-amber-500/10 dark:border-amber-900/20 rounded-xl p-3.5 break-all whitespace-pre-wrap text-amber-900/90 dark:text-amber-200/95 leading-relaxed shadow-inner z-10">
                {workflow.job.last_error}
              </p>
              <div className="z-10">
                {workflow.job.last_error === "workflow paused for human spec review" && task?.execution_engine === "cli" ? (
                  <CLISpecReviewControls
                    taskID={taskID}
                    token={token}
                    onReviewed={async () => {
                      await mutateWorkflow();
                    }}
                    setError={setError}
                  />
                ) : (
                  <BoundaryResolutionControls
                    errorMsg={workflow.job.last_error}
                    task={task}
                    updateTask={updateTask}
                    execute={execute}
                    setError={setError}
                  />
                )}
              </div>
            </div>
          )}

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
    </div>
  );
}
