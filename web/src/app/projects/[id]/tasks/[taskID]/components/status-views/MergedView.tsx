"use client";

import { useTaskDetail } from "../TaskDetailContext";
import { LogConsole } from "@/components/dashboard/log-console";
import { Check } from "lucide-react";
import { PRMetadataView } from "./PRMetadataView";

/** merged — terminal, informational */
export function MergedView() {
  const { logs, droppedLogCount, reloadFullLogs, isReloadingLogs, isCliFlow } = useTaskDetail();

  return (
    <div className="flex flex-col gap-4 animate-fade-in">
      <div className="bg-linear-to-br from-emerald-500/10 via-emerald-500/[0.02] to-teal-500/5 border border-emerald-500/25 rounded-2xl p-5.5 flex gap-3.5 items-start shadow-md relative overflow-hidden">
        <div className="absolute -top-10 -right-10 w-24 h-24 bg-emerald-500/10 rounded-full blur-2xl pointer-events-none" />
        <span className="inline-flex items-center justify-center w-9 h-9 rounded-xl bg-emerald-500 text-white shrink-0 shadow-md shadow-emerald-500/20 z-10">
          <Check className="h-5 w-5" />
        </span>
        <div className="z-10">
          <div className="text-sm font-bold text-emerald-600 dark:text-emerald-400 mb-1">Merged into main</div>
          <div className="text-xs text-content-muted leading-relaxed">
            Task completed successfully and code is now integrated into the production branch.
          </div>
        </div>
      </div>

      <div className="bg-card border border-stroke/10 rounded-2xl shadow-md overflow-hidden">
        <PRMetadataView />
      </div>

      {logs.length > 0 && (
        <LogConsole logs={logs} isExpanded={true} droppedLogCount={droppedLogCount} onReloadFullHistory={reloadFullLogs} isReloadingHistory={isReloadingLogs} isCliFlow={isCliFlow} />
      )}
    </div>
  );
}
