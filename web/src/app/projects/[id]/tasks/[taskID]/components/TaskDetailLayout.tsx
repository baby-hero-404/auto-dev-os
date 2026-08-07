"use client";

import { useState, useCallback, useEffect } from "react";
import { AlertCircle } from "lucide-react";
import { Skeleton } from "@/components/ui/skeleton";
import { useTaskDetail } from "./TaskDetailContext";
import { isActiveStatus } from "@/lib/status";
import { TaskHeader } from "./TaskHeader";
import { TaskSummaryHeader } from "./TaskSummaryHeader";
import { HumanDecisionSurface } from "./HumanDecisionSurface";
import { AgentTimeline } from "./AgentTimeline";
import { StatusViewRegistry } from "@/lib/status/registry";

import { SupportingAccordion } from "./SupportingAccordion";

// The status-driven UI (Phase 3 + 4) is now the default layout.
export function TaskDetailLayout() {
  const { task, workflow, updateTask, execute, retry, setError, isTaskLoading, workflowError } = useTaskDetail();



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
            <button onClick={() => window.location.reload()} className="rounded-md bg-brand-primary px-4 py-2 text-sm font-semibold text-background hover:opacity-90 transition">
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
        <TaskSummaryHeader />

        <SplitScreenWorkspace />
      </div>
    </div>
  );
}

// The split-screen workspace: Human Decision Surface (control) on the left,
// Agent Timeline (activity) on the right. Desktop shows both side-by-side;
// mobile collapses to tabs, defaulting per StatusViewRegistry[status].defaultTab
// since e.g. "coding" wants Activity up front while "spec_review" wants Control.
function SplitScreenWorkspace() {
  const { task, workflow, updateTask, execute, retry, setError, isTaskLoading, workflowError } = useTaskDetail();
  const defaultTab = task ? StatusViewRegistry[task.status]?.defaultTab ?? "control" : "control";
  const [mobileTab, setMobileTab] = useState<"control" | "activity">(defaultTab);
  const [lastStatus, setLastStatus] = useState(task?.status);

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
    if (task && isActiveStatus(task.status)) {
      setOpenSections((prev) => {
        if (!prev.logs || !prev.checkpoints) {
          return { ...prev, logs: true, checkpoints: true };
        }
        return prev;
      });
    }
  }, [task?.status]);

  // Reset the mobile tab to the new status's default whenever status changes
  // (adjusted during render, not an effect, per React's "storing information
  // from previous renders" pattern — avoids a redundant extra render).
  if (task?.status !== lastStatus) {
    setLastStatus(task?.status);
    setMobileTab(defaultTab);
  }

  return (
    <div className="mb-8">
      <div className="flex gap-1 border-b border-stroke [@media(min-width:1200px)]:hidden">
        <button
          type="button"
          onClick={() => setMobileTab("control")}
          className={`px-3 py-2 text-sm ${mobileTab === "control" ? "border-b-2 border-brand-primary text-content" : "text-content-secondary"}`}
        >
          Control
        </button>
        <button
          type="button"
          onClick={() => setMobileTab("activity")}
          className={`px-3 py-2 text-sm ${mobileTab === "activity" ? "border-b-2 border-brand-primary text-content" : "text-content-secondary"}`}
        >
          Activity
        </button>
      </div>

      <div className="[@media(min-width:1200px)]:grid [@media(min-width:1200px)]:grid-cols-[70%_30%] [@media(min-width:1200px)]:gap-8 [@media(min-width:1200px)]:items-start">
        <div className={`${mobileTab === "control" ? "block" : "hidden"} [@media(min-width:1200px)]:block [@media(min-width:1200px)]:max-h-[calc(100vh-280px)] [@media(min-width:1200px)]:overflow-y-auto [@media(min-width:1200px)]:sticky [@media(min-width:1200px)]:top-4 pr-2`}>
          <HumanDecisionSurface />
          <SupportingAccordion openSections={openSections} onToggleSection={toggleSection} />
        </div>
        <div className={`${mobileTab === "activity" ? "block" : "hidden"} [@media(min-width:1200px)]:block [@media(min-width:1200px)]:max-h-[calc(100vh-280px)] [@media(min-width:1200px)]:overflow-y-auto [@media(min-width:1200px)]:sticky [@media(min-width:1200px)]:top-4 bg-surface/30 rounded-2xl border border-stroke/10 p-2 shadow-inner`}>
          <AgentTimeline />
        </div>
      </div>
    </div>
  );
}
