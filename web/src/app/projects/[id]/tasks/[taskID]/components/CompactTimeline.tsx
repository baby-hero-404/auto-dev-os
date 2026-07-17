"use client";

import { useMemo } from "react";
import { Loader2, Check, AlertCircle, Play } from "lucide-react";
import { useTaskDetail, formatStepName, getSemanticStatusColor } from "./TaskDetailContext";

interface TimelineNode {
  id: string;
  title: string;
  steps: string[];
  isGroup: boolean;
}

export function CompactTimeline() {
  const {
    workflowSteps,
    stepDurations,
    analysisData,
    latest,
  } = useTaskDetail();

  const timelineNodes = useMemo(() => {
    const nodes: TimelineNode[] = [];
    let codeStepsAdded = false;
    const codeSteps = workflowSteps.filter((s) => s.startsWith("code_"));

    for (const step of workflowSteps) {
      if (step.startsWith("code_")) {
        if (!codeStepsAdded) {
          nodes.push({
            id: "implementation",
            title: "Implementation",
            steps: codeSteps,
            isGroup: true,
          });
          codeStepsAdded = true;
        }
      } else {
        nodes.push({
          id: step,
          title: formatStepName(step, analysisData),
          steps: [step],
          isGroup: false,
        });
      }
    }
    return nodes;
  }, [workflowSteps, analysisData]);

  return (
    <section className="rounded-xl border border-stroke/30 bg-card p-5 shadow-sm text-foreground">
      <h3 className="font-heading text-sm font-bold mb-4">Workflow Phases</h3>
      <div className="flex flex-col gap-1.5 max-h-[300px] overflow-y-auto pr-1">
        {timelineNodes.map((node) => {
          let nodeStatus = "pending";
          let allDone = true;
          let anyRunning = false;
          let anyFailed = false;
          let anyStarted = false;

          for (const step of node.steps) {
            const status = latest.get(step);
            if (status === "running") anyRunning = true;
            if (status === "failed") anyFailed = true;
            if (status) anyStarted = true;
            if (status !== "success" && status !== "recorded" && status !== "skipped") {
              allDone = false;
            }
          }

          if (anyFailed) {
            nodeStatus = "failed";
          } else if (anyRunning) {
            nodeStatus = "running";
          } else if (allDone && node.steps.length > 0) {
            nodeStatus = "success";
          } else if (anyStarted) {
            nodeStatus = "running";
          }

          const isCompleted = nodeStatus === "success";
          const isRunning = nodeStatus === "running";
          const isFailed = nodeStatus === "failed";

          // Duration selection
          let duration = "";
          if (node.isGroup) {
            // Find the duration of the latest step that has one
            for (const step of [...node.steps].reverse()) {
              const dur = stepDurations.get(step);
              if (dur) {
                duration = dur;
                break;
              }
            }
          } else {
            duration = stepDurations.get(node.steps[0]) || "";
          }

          const semantic = getSemanticStatusColor(nodeStatus);

          // Dot classes
          let dotClass = "bg-slate-400";
          if (isCompleted) dotClass = "bg-emerald-500";
          else if (isRunning) dotClass = "bg-sky-500 animate-pulse ring-4 ring-sky-500/20";
          else if (isFailed) dotClass = "bg-rose-500";

          return (
            <div key={node.id} className="flex items-center gap-3 py-1 text-xs">
              {/* 8px Status Dot */}
              <div className={`h-2 w-2 rounded-full shrink-0 ${dotClass}`} />
              
              {/* Step Name */}
              <span className={`font-medium ${isRunning ? "text-sky-500 font-semibold" : isCompleted ? "text-foreground" : "text-content-muted"}`}>
                {node.title}
              </span>

              {/* Flex Dotted Leader */}
              <div className="flex-1 border-b border-dotted border-stroke/40 mx-2" />

              {/* Duration */}
              {duration && (
                <span className="font-mono text-[10px] text-content-muted/80 mr-2 shrink-0">
                  {duration}
                </span>
              )}

              {/* Status Icon */}
              <div className="w-4 h-4 flex items-center justify-center shrink-0">
                {isCompleted && <Check size={13} className="text-emerald-500" />}
                {isRunning && <Loader2 size={13} className="text-sky-500 animate-spin" />}
                {isFailed && <AlertCircle size={13} className="text-rose-500" />}
                {!isCompleted && !isRunning && !isFailed && <div className="h-1.5 w-1.5 rounded-full bg-slate-300 dark:bg-slate-700" />}
              </div>
            </div>
          );
        })}
      </div>
    </section>
  );
}
