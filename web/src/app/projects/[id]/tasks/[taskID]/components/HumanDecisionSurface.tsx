"use client";

import { useTaskDetail } from "./TaskDetailContext";
import { BoundaryResolutionControls } from "./BoundaryResolutionControls";
import { DynamicActionBar } from "./DynamicActionBar";
import { StatusViewRegistry } from "@/lib/status/registry";

/**
 * Top-level status-view dispatcher: reads task.status, looks up
 * StatusViewRegistry, and renders the matched view + DynamicActionBar.
 * Also owns the paused-job recovery banner (workflow engine pause,
 * distinct from a `blocked` task status), shown above the per-status
 * view since a workflow can pause mid-execution on any of them.
 */
export function HumanDecisionSurface() {
  const { task, workflow, execute, retry, updateTask, setError } = useTaskDetail();

  const isPausedWithError =
    workflow?.job?.status === "paused" &&
    workflow?.job?.last_error &&
    !workflow.job.last_error.includes("workflow paused for human task clarification") &&
    task?.status !== "spec_review" &&
    task?.status !== "pr_ready" &&
    task?.status !== "human_review" &&
    task?.status !== "merged";

  if (!task) return null;

  const StatusView = StatusViewRegistry[task.status]?.component;

  return (
    <>
      {isPausedWithError && workflow?.job?.last_error && (
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
          <div className="z-10 flex items-center gap-3 mt-1">
            {workflow.job.last_error.includes("workflow paused by user") ? (
              <button onClick={execute} className="px-4 py-2 rounded-xl border-none bg-amber-600 hover:bg-amber-700 text-white text-xs font-bold transition-all duration-150 shadow-sm cursor-pointer">
                ▶ Resume Task
              </button>
            ) : (
              <>
                <BoundaryResolutionControls
                  errorMsg={workflow.job.last_error}
                  task={task}
                  updateTask={updateTask}
                  execute={execute}
                  setError={setError}
                />
                {!workflow.job.last_error.includes("boundary") && (
                  <button onClick={retry} className="px-4 py-2 rounded-xl border-none bg-amber-600 hover:bg-amber-700 text-white text-xs font-bold transition-all duration-150 shadow-sm cursor-pointer">
                    Retry Step
                  </button>
                )}
              </>
            )}
          </div>
        </div>
      )}

      <DynamicActionBar />
      {StatusView && <StatusView />}
    </>
  );
}
