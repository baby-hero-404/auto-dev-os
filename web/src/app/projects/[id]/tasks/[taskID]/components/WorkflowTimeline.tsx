"use client";

import { useMemo } from "react";
import {
  Compass,
  Loader2,
  Check,
  AlertCircle,
  FileText,
  Search,
  ClipboardList,
  Code,
  GitMerge,
  Eye,
  Sparkles,
  Terminal,
  GitPullRequest,
  Play,
} from "lucide-react";
import {
  useTaskDetail,
  formatStepName,
  getStepDescription,
  WORKFLOW_STEPS,
  getStepTasks
} from "./TaskDetailContext";

function getStepIcon(step: string) {
  if (step === WORKFLOW_STEPS.CONTEXT_LOAD) return <FileText size={13} />;
  if (step === WORKFLOW_STEPS.ANALYZE) return <Search size={13} />;
  if (step === WORKFLOW_STEPS.PLAN) return <ClipboardList size={13} />;
  if (step.startsWith("code_backend")) return <Code size={13} className="text-emerald-500" />;
  if (step.startsWith("code_frontend")) return <Code size={13} className="text-sky-500" />;
  if (step === WORKFLOW_STEPS.MERGE) return <GitMerge size={13} />;
  if (step === WORKFLOW_STEPS.REVIEW) return <Eye size={13} />;
  if (step === WORKFLOW_STEPS.FIX) return <Sparkles size={13} />;
  if (step === WORKFLOW_STEPS.TEST) return <Terminal size={13} />;
  if (step === WORKFLOW_STEPS.PR) return <GitPullRequest size={13} />;
  return <Play size={13} />;
}

interface WorkflowTimelineNode {
  id: string;
  title: string;
  description: string;
  icon: React.ReactNode;
  steps: string[];
  isGroup: boolean;
  isExecutionPhase?: boolean;
}

export function WorkflowTimeline() {
  const {
    workflow,
    workflowSteps,
    stepMetadata,
    workflowStatusCounts,
    stepDurations,
    analysisData,
    latest,
  } = useTaskDetail();

  const activeStepName = useMemo(() => {
    if (!workflow?.job?.step) return null;
    return formatStepName(workflow.job.step, analysisData);
  }, [workflow, analysisData]);

  const timelineNodes = useMemo(() => {
    const nodes: WorkflowTimelineNode[] = [];
    let codeStepsAdded = false;
    const codeSteps = workflowSteps.filter(s => s.startsWith("code_"));
    const execPhases = analysisData?.execution_phases || [];

    // Helper to fairly distribute N steps among M phases
    const distributeSteps = (steps: string[], numPhases: number) => {
      const result = Array.from({ length: numPhases }, () => [] as string[]);
      if (steps.length === 0) return result;
      
      // If fewer steps than phases, map 1-to-1 until run out
      if (steps.length <= numPhases) {
        for (let i = 0; i < steps.length; i++) {
          result[i].push(steps[i]);
        }
        return result;
      }
      
      const base = Math.floor(steps.length / numPhases);
      let remainder = steps.length % numPhases;
      
      let stepIdx = 0;
      for (let i = 0; i < numPhases; i++) {
        const count = base + (remainder > 0 ? 1 : 0);
        if (remainder > 0) remainder--;
        for (let j = 0; j < count; j++) {
          if (stepIdx < steps.length) {
            result[i].push(steps[stepIdx]);
            stepIdx++;
          }
        }
      }
      return result;
    };

    for (const step of workflowSteps) {
      if (step.startsWith("code_")) {
        if (!codeStepsAdded) {
          if (execPhases.length > 0) {
            const distributed = distributeSteps(codeSteps, execPhases.length);
            
            execPhases.forEach((phase: { phase?: string; tasks?: string[] }, index: number) => {
               const phaseSteps = distributed[index] || [];
               const phaseDesc = phase.phase ? phase.phase : (phase.tasks ? phase.tasks.join(", ") : "Execution phase");
               nodes.push({
                 id: `phase_${index}`,
                 title: `Phase ${index + 1}`,
                 description: phaseDesc,
                 icon: <Code size={14} />,
                 steps: phaseSteps,
                 isGroup: true,
                 isExecutionPhase: true, // Special flag for phase rendering
               });
            });
          } else {
             // Fallback to single implementation node
             nodes.push({
               id: "implementation",
               title: "Implementation",
               description: "Executing granular execution units (code writing, models, UI).",
               icon: <Code size={14} />,
               steps: codeSteps,
               isGroup: true,
             });
          }
          codeStepsAdded = true;
        }
      } else {
        nodes.push({
          id: step,
          title: formatStepName(step, analysisData),
          description: getStepDescription(step, analysisData),
          icon: getStepIcon(step),
          steps: [step],
          isGroup: false,
        });
      }
    }
    return nodes;
  }, [workflowSteps, analysisData]);

  return (
    <section className="rounded-xl border border-stroke bg-card p-6 shadow-sm hover:shadow-md transition-shadow">
      <div className="mb-5 flex flex-wrap items-start justify-between gap-4">
        <div className="flex items-center gap-2">
          <div className="p-1.5 rounded-lg bg-brand-primary/10 border border-brand-primary/20 text-brand-primary">
            <Compass size={18} />
          </div>
          <div>
            <h2 className="font-heading text-base font-bold text-foreground">Task Flow</h2>
            <p className="mt-0.5 text-xs text-content-muted">Execution pipeline, sandboxed runs, and checkpoint state.</p>
          </div>
        </div>
        <div className="flex flex-wrap items-center gap-2">
          <span className="rounded-md border border-stroke bg-surface px-3 py-1 text-xs font-semibold text-content-muted">
            Done {workflowStatusCounts.done}
          </span>
          <span className="rounded-md border border-sky-500/25 bg-sky-500/10 px-3 py-1 text-xs font-semibold text-sky-500">
            Running {workflowStatusCounts.running}
          </span>
          <span className="rounded-md border border-rose-500/25 bg-rose-500/10 px-3 py-1 text-xs font-semibold text-rose-500">
            Failed {workflowStatusCounts.failed}
          </span>
          <span className="rounded-md border border-stroke bg-surface px-3 py-1 text-xs font-semibold text-content-muted">
            Pending {workflowStatusCounts.pending}
          </span>
          {workflow?.job?.step && (
            <span className="rounded-md border border-sky-500/25 bg-sky-500/10 px-3 py-1 text-xs font-semibold text-sky-500 flex items-center gap-1.5 animate-pulse">
              <Loader2 size={12} className="animate-spin" />
              Active: {activeStepName}
            </span>
          )}
        </div>
      </div>

      <div className="relative flex w-full items-start gap-4 overflow-x-auto pb-6 pt-4 hide-scrollbar md:gap-5">
        {/* Connector Line Background */}
        <div className="absolute left-[52px] right-[52px] top-[39px] -z-10 h-[3px] bg-stroke/30 rounded-full" />

        {timelineNodes.map((node, index) => {
          let nodeStatus = "pending";
          let allDone = true;
          let anyRunning = false;
          let anyFailed = false;
          let anyStarted = false;
          
          const activeNodeSteps = node.steps.filter((s: string) => workflowSteps.includes(s));
          if (activeNodeSteps.length === 0) {
            allDone = false;
          }

          let lastDuration = "";
          for (const step of activeNodeSteps) {
             const status = latest.get(step);
             if (status === "running") anyRunning = true;
             if (status === "failed") anyFailed = true;
             if (status) anyStarted = true;
             if (status !== "success" && status !== "recorded") allDone = false;
             
             const dur = stepDurations.get(step);
             if (dur) lastDuration = dur; // grab the latest available duration
          }

          if (anyFailed) nodeStatus = "failed";
          else if (anyRunning) nodeStatus = "running";
          else if (allDone && activeNodeSteps.length > 0) nodeStatus = "success";
          else if (anyStarted) nodeStatus = "running";

          const isCompleted = nodeStatus === "success";
          const isRunning = nodeStatus === "running";
          const isFailed = nodeStatus === "failed";
          
          // Timestamp parsing for non-group nodes
          const timestamp = !node.isGroup && activeNodeSteps.length > 0 ? stepMetadata.get(activeNodeSteps[0])?.timestamp : undefined;

          return (
            <div key={node.id} className="group relative flex min-w-[128px] flex-col items-center justify-start gap-3 shrink-0 md:min-w-[140px]">
              
              {/* Premium Hover Card / Tooltip */}
              <div className="absolute bottom-full mb-3 hidden group-hover:flex flex-col items-start w-64 p-3 rounded-xl border border-stroke bg-card/95 backdrop-blur-md shadow-xl z-30 transition-all duration-300 animate-fade-in pointer-events-none">
                <div className="flex items-center gap-2 border-b border-stroke/50 pb-1.5 w-full">
                  <div className={`p-1 rounded bg-surface border border-stroke/50 ${isRunning ? "text-sky-500" : isCompleted ? "text-brand-primary" : isFailed ? "text-rose-500" : "text-content-muted"}`}>
                    {node.icon}
                  </div>
                  <div className="font-sans font-bold text-xs text-foreground capitalize leading-none">
                    {node.title}
                  </div>
                </div>
                <p className="text-[10px] text-content-muted leading-relaxed mt-2 font-normal">
                  {node.description}
                </p>
                
                {/* Granular Task Checklist (Only show if it's a group like Implementation) */}
                {node.isGroup && activeNodeSteps.length > 0 && (
                  <div className="flex flex-col gap-2 w-full mt-3 border-t border-stroke/50 pt-2.5 max-h-[160px] overflow-y-auto custom-scrollbar">
                    {activeNodeSteps.map((step: string) => {
                      const st = latest.get(step);
                      const isDone = st === "success" || st === "recorded";
                      const isRun = st === "running";
                      const isFail = st === "failed";
                      const tasks = getStepTasks(step, analysisData);

                      return (
                        <div key={step} className="flex flex-col gap-1 w-full bg-surface/30 p-1.5 rounded-lg border border-stroke/30">
                          <div className="flex items-start gap-1.5 w-full">
                            <div className="mt-0.5 shrink-0">
                              {isDone ? <Check size={10} className="text-emerald-500" /> : 
                               isRun ? <Loader2 size={10} className="text-sky-500 animate-spin" /> : 
                               isFail ? <AlertCircle size={10} className="text-rose-500" /> :
                               <div className="w-2 h-2 rounded-full border border-stroke/80 mt-0.5" />}
                            </div>
                            <span className={`text-[10px] leading-tight font-bold line-clamp-2 ${isDone ? "text-content-muted" : "text-foreground"}`}>
                              {formatStepName(step, analysisData)}
                            </span>
                          </div>
                          {tasks && tasks.length > 0 && (
                            <div className="pl-4 flex flex-col gap-1 mt-0.5">
                              {tasks.map((t, i) => (
                                <div key={i} className="flex items-start gap-1.5">
                                  <div className={`w-1 h-1 rounded-full mt-1.5 shrink-0 ${isDone ? "bg-emerald-500/50" : "bg-stroke"}`} />
                                  <span className={`text-[9.5px] leading-snug ${isDone ? "text-content-muted/70 line-through" : "text-content-muted"}`}>{t}</span>
                                </div>
                              ))}
                            </div>
                          )}
                        </div>
                      );
                    })}
                  </div>
                )}

                <div className="flex items-center justify-between w-full mt-2.5 pt-2 border-t border-stroke/50 text-[9px] font-bold uppercase tracking-wider text-content-muted/80">
                  <span>Status</span>
                  <span className={isCompleted ? "text-emerald-500" : isRunning ? "text-sky-500 animate-pulse" : isFailed ? "text-rose-500" : "text-content-muted"}>
                    {nodeStatus}
                  </span>
                </div>
                {!node.isGroup && lastDuration && (
                  <div className="flex items-center justify-between w-full mt-1 text-[9px] text-content-muted/80">
                    <span>Duration</span>
                    <span className="font-mono">{lastDuration}</span>
                  </div>
                )}
              </div>

              {/* Connecting Line Progress Overlay */}
              {index > 0 && (
                <div className={`absolute right-[50%] top-[38px] -z-10 h-[3px] w-full transition-all duration-500 ${isCompleted ? "bg-brand-primary" :
                    isRunning ? "bg-gradient-to-r from-brand-primary to-sky-500/30 animate-pulse" :
                      "bg-transparent"
                  }`} />
              )}

              {/* Node Circle */}
              <div className={`relative z-10 flex size-11 items-center justify-center rounded-full border-2 transition-all duration-300 shadow-sm cursor-pointer ${isCompleted ? "border-brand-primary bg-brand-primary/5 text-brand-primary hover:bg-brand-primary/10" :
                  isFailed ? "border-rose-500 bg-rose-500/10 text-rose-500 hover:bg-rose-500/20" :
                    isRunning ? "border-sky-500 bg-sky-500/10 text-sky-500 shadow-[0_0_12px_rgba(14,165,233,0.3)] animate-pulse" :
                      "border-stroke bg-card text-content-muted/80 hover:border-content-muted hover:text-foreground"
                }`}>
                {node.icon}

                {/* Tiny Status Badges on the Circle */}
                {isCompleted && (
                  <div className="absolute -bottom-0.5 -right-0.5 flex size-4 items-center justify-center rounded-full bg-brand-primary text-card border border-card shadow-sm">
                    <Check size={9} strokeWidth={4} />
                  </div>
                )}
                {isFailed && (
                  <div className="absolute -bottom-0.5 -right-0.5 flex size-4 items-center justify-center rounded-full bg-rose-500 text-card border border-card shadow-sm">
                    <AlertCircle size={9} strokeWidth={4} />
                  </div>
                )}
              </div>

              {/* Label Details */}
              <div className="w-full px-1 flex flex-col items-center text-center">
                <div className={`text-[10px] font-bold uppercase tracking-wider transition-colors line-clamp-1 leading-tight ${isCompleted || isRunning ? "text-foreground font-semibold" : "text-content-muted/80"
                  }`}>
                  {node.title}
                </div>
                {isRunning ? (
                  <span className="mt-1 text-[8px] font-bold uppercase text-sky-500 bg-sky-500/10 px-1.5 py-0.5 rounded-full border border-sky-500/20 animate-pulse">
                    {node.isGroup ? "in progress" : "running"}
                  </span>
                ) : (
                  <div className="mt-1 text-[9px] font-medium uppercase text-content-muted/60">{nodeStatus}</div>
                )}
                {!node.isGroup && timestamp && (
                  <div className="mt-1 text-[8px] text-content-muted/80 whitespace-nowrap font-mono">
                    {new Date(timestamp).toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' })}
                    {lastDuration && ` (${lastDuration})`}
                  </div>
                )}
              </div>
            </div>
          );
        })}
      </div>
    </section>
  );
}
