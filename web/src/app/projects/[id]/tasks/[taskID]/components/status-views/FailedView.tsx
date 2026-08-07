"use client";

import { useTaskDetail } from "../TaskDetailContext";
import { LogConsole } from "@/components/dashboard/log-console";
import { X, RotateCcw } from "lucide-react";

/** failed — recovery, not approval */
export function FailedView() {
  const { workflow, retry, logs, droppedLogCount, reloadFullLogs, isReloadingLogs, isCliFlow } = useTaskDetail();

  return (
    <div className="rounded-2xl border border-rose-500/25 bg-background shadow-lg overflow-hidden flex flex-col transition-all duration-300 animate-fade-in">
      <div className="flex flex-col md:flex-row md:items-center md:justify-between gap-4 px-5.5 py-5 bg-linear-to-br from-rose-500/10 via-rose-500/[0.02] to-red-500/5 border-b border-stroke/10">
        <div className="flex gap-3.5 items-start">
          <span className="inline-flex items-center justify-center w-9 h-9 rounded-xl bg-rose-500 text-white shrink-0 shadow-md shadow-rose-500/20">
            <X className="h-5 w-5" />
          </span>
          <div>
            <div className="text-sm font-bold text-rose-600 dark:text-rose-400 mb-1">Task execution failed</div>
            <div className="text-xs text-content-muted leading-relaxed max-w-2xl break-all font-mono">
              {workflow?.job?.last_error || "Unrecoverable error. Restart the task."}
            </div>
          </div>
        </div>
        <button onClick={retry} className="px-4.5 py-2.5 rounded-xl border-none bg-linear-to-r from-rose-600 to-red-600 hover:from-rose-500 hover:to-red-500 text-white text-xs font-bold transition-all duration-150 hover:shadow-md hover:shadow-rose-500/20 active:scale-95 cursor-pointer whitespace-nowrap flex items-center gap-1.5 self-end md:self-center">
          <RotateCcw className="h-3.5 w-3.5" /> Restart Task
        </button>
      </div>
      {logs.length > 0 && (
        <LogConsole logs={logs} isExpanded={true} hideHeader={true} droppedLogCount={droppedLogCount} onReloadFullHistory={reloadFullLogs} isReloadingHistory={isReloadingLogs} isCliFlow={isCliFlow} />
      )}
    </div>
  );
}
