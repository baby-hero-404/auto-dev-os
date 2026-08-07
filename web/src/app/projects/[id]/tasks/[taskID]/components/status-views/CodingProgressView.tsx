"use client";

import { useTaskDetail } from "../TaskDetailContext";
import { LogConsole } from "@/components/dashboard/log-console";
import { Pause } from "lucide-react";
import { SplitProposalView } from "./SplitProposalView";
import { TaskSubtasks } from "../TaskSubtasks";

/**
 * Shared visual for the "actively executing" statuses (coding, testing, fixing) —
 * isExecutionStatus() in lib/status groups these three identically today, so
 * TestProgressView/FixProgressView re-export this component rather than
 * duplicating markup for a distinction the UI doesn't actually make.
 */
export function CodingProgressView() {
  const { task, logs, droppedLogCount, reloadFullLogs, isReloadingLogs, isCliFlow, workflow } = useTaskDetail();
  const st = task?.status || "todo";
  const isPaused = workflow?.job?.status === "paused";

  return (
    <div className="flex flex-col gap-4">
      <SplitProposalView />

      {isCliFlow ? (
        <div className={`bg-linear-to-br border rounded-2xl p-5.5 shadow-sm relative overflow-hidden animate-fade-in ${isPaused ? "from-amber-500/10 via-amber-500/5 to-slate-500/5 border-amber-500/20" : "from-blue-500/10 via-blue-500/5 to-slate-500/5 border-blue-500/20"}`}>
          <div className={`absolute -top-10 -right-10 w-24 h-24 rounded-full blur-2xl pointer-events-none ${isPaused ? "bg-amber-500/10" : "bg-blue-500/10"}`} />
          <div className="flex items-center gap-3 mb-2 z-10 relative">
            {isPaused ? (
              <span className="w-5 h-5 flex items-center justify-center rounded-full bg-amber-500/20 shrink-0 text-amber-700 dark:text-amber-400">
                <Pause className="h-3 w-3" />
              </span>
            ) : (
              <span className="w-5 h-5 rounded-full border-2 border-blue-500/20 border-t-blue-600 dark:border-t-blue-400 animate-spin shrink-0"></span>
            )}
            <span className={`text-sm font-bold tracking-wide capitalize ${isPaused ? "text-amber-700 dark:text-amber-400" : "text-blue-700 dark:text-blue-400"}`}>
              {isPaused ? "Execution Paused" : "CLI Engine Executing..."}
            </span>
          </div>
          <div className="text-xs text-content-muted z-10 pl-8 relative">
            {isPaused ? "The agent has been paused. Terminal output is suspended." : "The agent is running locally in the background. Terminal output will be captured and displayed once the command completes."}
          </div>
        </div>
      ) : (
        <div className="rounded-2xl border border-stroke/10 bg-background shadow-lg overflow-hidden transition-all duration-300">
          <div className="flex items-center justify-between gap-3 px-5 py-4 border-b border-stroke/10 bg-surface/20">
            <div className="flex items-center gap-2.5">
              <span className="relative flex h-2 w-2">
                {!isPaused && <span className={`animate-ping absolute inline-flex h-full w-full rounded-full opacity-75 ${st === "fixing" ? "bg-amber-400" : "bg-blue-400"}`}></span>}
                <span className={`relative inline-flex rounded-full h-2 w-2 ${isPaused ? "bg-amber-500" : st === "fixing" ? "bg-amber-500" : "bg-blue-500"}`}></span>
              </span>
              <span className="text-xs uppercase font-extrabold tracking-wider text-content-muted capitalize">{st} {isPaused ? "paused" : "in progress"}</span>
            </div>
          </div>
          <LogConsole logs={logs} isExpanded={true} hideHeader={true} droppedLogCount={droppedLogCount} onReloadFullHistory={reloadFullLogs} isReloadingHistory={isReloadingLogs} isCliFlow={isCliFlow} />
        </div>
      )}

      <TaskSubtasks />
    </div>
  );
}
