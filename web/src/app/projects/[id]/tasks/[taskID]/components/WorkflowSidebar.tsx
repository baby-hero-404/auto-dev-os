"use client";

import { useMemo, useState, useEffect, useRef } from "react";
import { Bot, Clock, ChevronDown, ChevronUp } from "lucide-react";
import { useTaskDetail, formatStepName } from "./TaskDetailContext";
import { WorkflowProgress } from "./TaskActions";

function InfoRow({ label, value }: { label: string; value: string }) {
  return (
    <div>
      <dt className="text-[10px] font-bold uppercase tracking-wider text-content-muted">{label}</dt>
      <dd className="mt-1 break-all font-mono text-[11px] text-foreground bg-surface/30 border border-stroke/60 rounded px-2 py-1">{value}</dd>
    </div>
  );
}

export function WorkflowSidebar() {
  const {
    task,
    workflow,
    analysisData,
    logs,
  } = useTaskDetail();

  const [isCheckpointsExpanded, setIsCheckpointsExpanded] = useState(false);
  const [mobileOpenSection, setMobileOpenSection] = useState<"agent" | "checkpoints" | null>(null);
  const [isMobile, setIsMobile] = useState(false);

  useEffect(() => {
    const checkIsMobile = () => {
      setIsMobile(window.innerWidth < 768);
    };
    checkIsMobile();
    window.addEventListener("resize", checkIsMobile);
    return () => window.removeEventListener("resize", checkIsMobile);
  }, []);

  const checkpointsList = useMemo(() => {
    return [...(workflow?.checkpoints ?? [])].reverse();
  }, [workflow?.checkpoints]);

  const lastToolCall = useMemo(() => {
    if (!logs) return null;
    for (let i = logs.length - 1; i >= 0; i--) {
      const msg = logs[i].message;
      
      if (msg.includes("search_replace") || msg.includes("replace_file_content") || msg.includes("multi_replace_file_content")) {
        const fileMatch = msg.match(/TargetFile:\s*([^\s,]+)|to:\s*([^\s,]+)/i) || msg.match(/(?:file|to|content of)\s*`?([^`\s]+)`?/i);
        const fileName = fileMatch ? fileMatch[1] || fileMatch[2] : "";
        const baseName = fileName.split("/").pop() || "file";
        return `Editing ${baseName}`;
      }
      
      if (msg.includes("create_file") || msg.includes("write_to_file")) {
        const fileMatch = msg.match(/TargetFile:\s*([^\s,]+)/i) || msg.match(/(?:file|to)\s*`?([^`\s]+)`?/i);
        const fileName = fileMatch ? fileMatch[1] : "";
        const baseName = fileName.split("/").pop() || "file";
        return `Creating ${baseName}`;
      }
      
      if (msg.includes("run_tests") || msg.includes("playwright test") || msg.includes("npx playwright test") || msg.includes("npx jest")) {
        return "Running tests";
      }
      
      if (msg.includes("run_build") || msg.includes("npm run build")) {
        return "Running build";
      }
      
      if (msg.includes("run_lint") || msg.includes("npx eslint") || msg.includes("tsc --noEmit")) {
        return "Running linter";
      }
      
      if (msg.includes("read_file") || msg.includes("view_file")) {
        const fileMatch = msg.match(/AbsolutePath:\s*([^\s,]+)|file\s*([^\s,]+)/i) || msg.match(/(?:file|content of)\s*`?([^`\s]+)`?/i);
        const fileName = fileMatch ? fileMatch[1] : "";
        const baseName = fileName.split("/").pop() || "file";
        return `Reading ${baseName}`;
      }
      
      if (msg.includes("list_files") || msg.includes("list_dir")) {
        return "Listing directories";
      }
      
      if (msg.includes("grep_search") || msg.includes("search_web") || msg.includes("ripgrep")) {
        return "Searching codebase";
      }
    }
    
    if (workflow?.job?.status === "running") {
      return "Thinking...";
    }
    return null;
  }, [logs, workflow?.job?.status]);

  const [throttledAction, setThrottledAction] = useState<string | null>(null);
  const lastUpdateTime = useRef(0);

  useEffect(() => {
    const now = Date.now();
    const action = lastToolCall;
    
    if (now - lastUpdateTime.current >= 1000) {
      setThrottledAction(action);
      lastUpdateTime.current = now;
    } else {
      const delay = 1000 - (now - lastUpdateTime.current);
      const timer = setTimeout(() => {
        setThrottledAction(action);
        lastUpdateTime.current = Date.now();
      }, delay);
      return () => clearTimeout(timer);
    }
  }, [lastToolCall]);

  const agentName = task?.agent_id || "Unassigned";
  const currentStep = workflow?.job?.step || "none";
  const attemptsCount = String(workflow?.job?.attempts ?? 0);
  const lastErrorText = workflow?.job?.last_error || "none";

  const isAgentOpen = isMobile ? mobileOpenSection === "agent" : true;
  const isCheckpointsOpen = isMobile ? mobileOpenSection === "checkpoints" : isCheckpointsExpanded;

  const toggleAgent = () => {
    if (isMobile) {
      setMobileOpenSection(mobileOpenSection === "agent" ? null : "agent");
    }
  };

  const toggleCheckpoints = () => {
    if (isMobile) {
      setMobileOpenSection(mobileOpenSection === "checkpoints" ? null : "checkpoints");
    } else {
      setIsCheckpointsExpanded(!isCheckpointsExpanded);
    }
  };

  return (
    <aside className="space-y-6">
      {/* Workflow Progress is always visible */}
      <WorkflowProgress />

      {/* Agent Activity Section */}
      <div className="relative overflow-hidden rounded-xl border border-stroke/50 bg-card/60 backdrop-blur-xl p-5 shadow-lg hover:shadow-xl transition-all group">
        <div className="absolute inset-0 bg-gradient-to-br from-brand-primary/5 via-transparent to-transparent opacity-0 group-hover:opacity-100 transition-opacity duration-500 pointer-events-none" />
        {isMobile ? (
          <button
            onClick={toggleAgent}
            className="flex w-full items-center justify-between font-heading text-base font-bold text-foreground cursor-pointer hover:text-brand-primary transition-colors"
          >
            <div className="flex items-center gap-2">
              <Bot size={16} className="text-brand-primary" />
              <span>Agent Activity</span>
            </div>
            {isAgentOpen ? <ChevronUp size={16} /> : <ChevronDown size={16} />}
          </button>
        ) : (
          <div className="mb-4 flex items-center gap-2 border-b border-stroke pb-3">
            <Bot size={16} className="text-brand-primary" />
            <h2 className="font-heading text-base font-bold text-foreground">Agent Activity</h2>
          </div>
        )}

        {isAgentOpen && (
          <div className="space-y-4">
            <dl className={`text-xs space-y-3.5 ${isMobile ? "mt-4 pt-3 border-t border-stroke/30" : ""}`}>
              <InfoRow label="Assigned agent" value={agentName} />
              <InfoRow label="Current step" value={formatStepName(currentStep, analysisData)} />
              <InfoRow label="Attempts" value={attemptsCount} />
              <InfoRow label="Last error" value={lastErrorText} />
            </dl>
            {workflow?.job?.status === "running" && throttledAction && (
              <div className="relative z-10 flex items-center gap-2 rounded-lg border border-sky-500/30 bg-sky-500/10 backdrop-blur px-3 py-2.5 text-sky-600 dark:text-sky-400 shadow-[0_0_15px_rgba(14,165,233,0.15)] animate-in fade-in slide-in-from-bottom-2 duration-300">
                <span className="relative flex h-2 w-2 shrink-0">
                  <span className="animate-ping absolute inline-flex h-full w-full rounded-full bg-sky-400 opacity-75"></span>
                  <span className="relative inline-flex rounded-full h-2 w-2 bg-sky-500 shadow-[0_0_5px_currentColor]"></span>
                </span>
                <span className="font-mono text-[10px] font-bold uppercase tracking-wider leading-none drop-shadow-sm">{throttledAction}</span>
              </div>
            )}
          </div>
        )}
      </div>

      {/* Checkpoints Section */}
      <div className="relative overflow-hidden rounded-xl border border-stroke/50 bg-card/60 backdrop-blur-xl p-5 shadow-lg hover:shadow-xl transition-all group">
        <div className="absolute inset-0 bg-gradient-to-br from-brand-primary/5 via-transparent to-transparent opacity-0 group-hover:opacity-100 transition-opacity duration-500 pointer-events-none" />
        <button
          onClick={toggleCheckpoints}
          className="flex w-full items-center justify-between font-heading text-base font-bold text-foreground cursor-pointer hover:text-brand-primary transition-colors"
        >
          <div className="flex items-center gap-2">
            <Clock size={16} className="text-brand-primary" />
            <span>Checkpoints ({checkpointsList.length})</span>
          </div>
          {isCheckpointsOpen ? <ChevronUp size={16} /> : <ChevronDown size={16} />}
        </button>

        {isCheckpointsOpen && (
          <div className={`relative space-y-4 max-h-[350px] overflow-y-auto pr-2 custom-scrollbar pl-2 mt-4 pt-3 border-t border-stroke/30`}>
            <div className="absolute left-3.5 top-5 bottom-2 w-[2px] bg-stroke/60 rounded-full" />
            {checkpointsList.map((checkpoint) => (
              <div key={checkpoint.id} className="relative pl-7 group">
                <div className="absolute left-[-2px] top-2.5 size-[11px] rounded-full border-2 border-card bg-brand-primary ring-2 ring-transparent group-hover:ring-brand-primary/30 transition-all" />
                <div className="rounded-lg border border-stroke bg-surface/40 p-2.5 hover:bg-surface/80 transition-colors shadow-sm">
                  <div className="font-mono text-[11px] font-bold text-brand-primary capitalize tracking-wide">
                    {formatStepName(checkpoint.step, analysisData)}
                  </div>
                  <div className="text-[10px] text-content-muted mt-1 font-medium">
                    {new Date(checkpoint.created_at).toLocaleString()}
                  </div>
                </div>
              </div>
            ))}
            {checkpointsList.length === 0 && (
              <p className="text-xs text-content-muted italic pl-6 pt-2">No checkpoints recorded.</p>
            )}
          </div>
        )}
      </div>
    </aside>
  );
}
