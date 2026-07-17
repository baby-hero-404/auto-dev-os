"use client";

import { useTaskDetail } from "./TaskDetailContext";
import { Markdown } from "@/components/ui/markdown";
import { LogConsole } from "@/components/dashboard/log-console";
import { SpecPanel } from "./SpecPanel";

export function TaskHeroCards() {
  const { task, workflow, analysisData, logs, requestSpecChanges, approveSpec, rejectPR, approvePR, retry } = useTaskDetail();
  const st = task?.status || "todo";
  const isRunning = workflow?.job?.status === "running";
  
  const heroLoad = st === 'context_loading' || st === 'analyzing';
  const heroSpec = st === 'spec_review';
  const heroExec = ['coding','testing','fixing'].includes(st);
  const heroReview = st === 'reviewing';
  const heroPr = st === 'pr_ready' || st === 'human_review';
  const heroMerged = st === 'merged';
  const heroFailed = st === 'failed';

  return (
    <>
      {heroLoad && (
        <div className="bg-card border border-[#90c5ff] rounded-xl p-5.5">
          <div className="flex items-center gap-2.5 mb-3.5">
            <span className="w-4 h-4 rounded-full border-2 border-[#90c5ff] border-t-[#005bb8] animate-spin"></span>
            <span className="text-sm font-semibold text-[#005bb8]">
              {st === 'context_loading' ? 'Loading context...' : 'Analyzing requirements...'}
            </span>
          </div>
          {workflow?.checkpoints?.map((cp, idx) => (
            <div key={idx} className="flex items-center gap-2.5 py-1 text-[13px]">
              <span className="w-4 text-center text-[#00590e]">✓</span>
              <span className="text-[#00590e]">{cp.step.replace(/_/g, " ")}</span>
            </div>
          ))}
        </div>
      )}

      {heroSpec && (
        <div className="flex flex-col gap-4">
          <div className="bg-[#fef3c6] border border-[#ffd236] rounded-xl px-5.5 py-4 flex items-center justify-between">
            <div className="flex items-center gap-3">
              <span className="text-[18px]">⏸</span>
              <div>
                <div className="text-[14px] font-bold text-[#795800]">Definition-of-Ready Gate</div>
                <div className="text-[13px] text-[#795800]">Review the specification below and approve it before coding starts.</div>
              </div>
            </div>
            <div className="flex gap-2">
              <button onClick={requestSpecChanges} className="px-3.5 py-1.5 rounded-lg border border-[#ffd236] bg-[#fef3c6] text-[#795800] text-[13px] font-medium hover:bg-[#ffeaa7] cursor-pointer">
                Request Changes
              </button>
              <button onClick={approveSpec} className="px-4 py-1.5 rounded-lg border-none bg-[#00590e] text-white text-[13px] font-semibold hover:bg-[#007a13] cursor-pointer shadow-sm">
                ✓ Approve Spec
              </button>
            </div>
          </div>
          <SpecPanel isExpanded={true} />
        </div>
      )}

      {heroExec && (
        <div className="bg-card border border-stroke rounded-xl overflow-hidden">
          <div className="flex items-center gap-2.5 px-5 py-3.5 border-b border-stroke bg-card">
            <span className="w-2 h-2 rounded-full animate-pulse" style={{ background: st === 'fixing' ? '#b75000' : '#005bb8' }}></span>
            <span className="text-sm font-semibold flex-1 capitalize">{st} in progress</span>
          </div>
          <LogConsole logs={logs} isWorkflowRunning={isRunning} isExpanded={true} hideHeader={true} />
        </div>
      )}

      {heroReview && (
        <div className="bg-card border border-[#c07eff] rounded-xl px-5.5 py-5">
          <div className="flex items-center gap-2.5 mb-3.5">
            <span className="w-2 h-2 rounded-full bg-[#9810fa] animate-pulse"></span>
            <span className="text-sm font-semibold text-[#7f22fe]">AI review in progress</span>
          </div>
        </div>
      )}

      {heroPr && (
        <div className="flex flex-col gap-4">
          <div className="bg-card border rounded-xl overflow-hidden" style={{ borderColor: st === 'human_review' ? '#ffd236' : '#a4f4cf' }}>
            {st === 'human_review' && (
              <div className="flex items-center justify-between px-5.5 py-3.5 bg-[#fef3c6]">
                <div className="flex items-center gap-3">
                  <span className="text-[18px]">⏸</span>
                  <div>
                    <div className="text-[14px] font-bold text-[#795800]">Waiting for human review</div>
                    <div className="text-[13px] text-[#795800]">Final approval required before merge.</div>
                  </div>
                </div>
                <div className="flex gap-2">
                  <button onClick={rejectPR} className="px-3.5 py-1.5 rounded-lg border border-[#ffd236] bg-[#fef3c6] text-[#795800] text-[13px] font-medium hover:bg-[#ffeaa7] cursor-pointer">
                    Reject PR
                  </button>
                  <button onClick={approvePR} className="px-4 py-1.5 rounded-lg border-none bg-[#00590e] text-white text-[13px] font-semibold hover:bg-[#007a13] cursor-pointer shadow-sm">
                    ✓ Approve Merge
                  </button>
                </div>
              </div>
            )}
            <div className="px-5.5 py-5">
              <div className="flex items-center gap-2.5 mb-3">
                <span className="text-sm font-semibold flex-1">Pull Request Ready</span>
              </div>
            </div>
          </div>
          {logs.length > 0 && (
            <LogConsole logs={logs} isWorkflowRunning={false} isExpanded={true} />
          )}
        </div>
      )}

      {heroFailed && (
        <div className="bg-card border border-[#ffa3a3] rounded-xl overflow-hidden flex flex-col">
          <div className="flex items-start justify-between px-5.5 py-5 bg-[#ffe2e2] border-b border-[#ffa3a3]/30">
            <div className="flex gap-3.5 items-start">
              <span className="inline-flex items-center justify-center w-8 h-8 rounded-full bg-[#bf000f] text-white text-base font-bold shrink-0">✕</span>
              <div>
                <div className="text-[15px] font-bold text-[#bf000f] mb-1">Task failed</div>
                <div className="text-[13px] text-[#8b0836] leading-relaxed max-w-2xl break-words">
                  {workflow?.job?.last_error || "Unrecoverable error. Restart the task."}
                </div>
              </div>
            </div>
            <button onClick={retry} className="px-4 py-1.5 rounded-lg border-none bg-[#bf000f] text-white text-[13px] font-semibold hover:bg-[#e40014] cursor-pointer shadow-sm whitespace-nowrap">
              ↻ Restart Task
            </button>
          </div>
          {logs.length > 0 && (
            <LogConsole logs={logs} isWorkflowRunning={false} isExpanded={true} hideHeader={true} />
          )}
        </div>
      )}

      {heroMerged && (
        <div className="flex flex-col gap-4">
          <div className="bg-[#e6f4ea] border border-[#a4f4cf] rounded-xl p-5.5 flex gap-3.5 items-start">
            <span className="inline-flex items-center justify-center w-8 h-8 rounded-full bg-[#00590e] text-white text-base font-bold shrink-0">✓</span>
            <div>
              <div className="text-[15px] font-bold text-[#00590e] mb-1">Merged into main</div>
              <div className="text-[13px] text-[#00590e] leading-relaxed">
                Task completed successfully.
              </div>
            </div>
          </div>
          {logs.length > 0 && (
            <LogConsole logs={logs} isWorkflowRunning={false} isExpanded={true} />
          )}
        </div>
      )}
    </>
  );
}
