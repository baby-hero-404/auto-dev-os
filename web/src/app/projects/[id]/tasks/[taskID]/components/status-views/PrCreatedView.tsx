"use client";

import { useTaskDetail } from "../TaskDetailContext";
import { LogConsole } from "@/components/dashboard/log-console";
import { Sparkles, Pause } from "lucide-react";
import { PRMetadataView } from "./PRMetadataView";
import { ReviewVerdictCard } from "../ReviewVerdictCard";
import type { ReviewVerdict } from "@/lib/types";

/**
 * pr_ready, human_review — informational only. Merge/reject are no longer
 * rendered here; they're dispatched via DynamicActionBar off available_actions
 * (spec_review is the only human approval gate per design.md).
 */
export function PrCreatedView() {
  const { task, workflow, logs, droppedLogCount, reloadFullLogs, isReloadingLogs, isCliFlow } = useTaskDetail();
  const st = task?.status || "todo";

  const reviewVerdict = [...(workflow?.checkpoints ?? [])]
    .reverse()
    .map((cp) => (cp.state?.output as { review_verdict?: ReviewVerdict } | undefined)?.review_verdict)
    .find((v): v is ReviewVerdict => !!v);

  return (
    <div className="flex flex-col gap-4 animate-fade-in">
      <div className="bg-card border border-stroke/10 rounded-2xl shadow-md overflow-hidden" style={{ borderColor: st === "human_review" ? "#f59e0b" : "#10b981" }}>
        {st === "human_review" && (
          <div className="flex items-center gap-3 px-5.5 py-4.5 bg-linear-to-br from-amber-500/10 via-amber-500/[0.02] to-orange-500/5 border-b border-stroke/10">
            <span className="w-10 h-10 flex items-center justify-center rounded-xl bg-amber-500/10 border border-amber-500/20 text-amber-600 dark:text-amber-500 shrink-0">
              <Pause className="h-5 w-5" />
            </span>
            <div>
              <div className="text-sm font-bold text-foreground">Waiting for Human Review</div>
              <div className="text-xs text-content-muted mt-0.5 leading-normal">Final approval required before merging changes.</div>
            </div>
          </div>
        )}

        {st === "pr_ready" && (
          <div className="flex items-center gap-3 px-5.5 py-4.5 bg-linear-to-br from-emerald-500/10 via-emerald-500/[0.02] to-teal-500/5 border-b border-stroke/10">
            <span className="w-10 h-10 flex items-center justify-center rounded-xl bg-emerald-500/10 border border-emerald-500/20 text-emerald-600 dark:text-emerald-500 shrink-0">
              <Sparkles className="h-5 w-5" />
            </span>
            <div>
              <div className="text-sm font-bold text-foreground">Pull Request Ready</div>
              <div className="text-xs text-content-muted mt-0.5 leading-normal">Review the changes on your Git provider.</div>
            </div>
          </div>
        )}

        <PRMetadataView />
        {reviewVerdict && (
          <div className="px-5.5 pb-5">
            <ReviewVerdictCard verdict={reviewVerdict} />
          </div>
        )}
      </div>
      {logs.length > 0 && (
        <LogConsole logs={logs} isExpanded={true} droppedLogCount={droppedLogCount} onReloadFullHistory={reloadFullLogs} isReloadingHistory={isReloadingLogs} isCliFlow={isCliFlow} />
      )}
    </div>
  );
}
