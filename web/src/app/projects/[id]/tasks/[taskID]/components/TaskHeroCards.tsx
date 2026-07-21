"use client";

import { useState } from "react";
import { useTaskDetail } from "./TaskDetailContext";
import { LogConsole } from "@/components/dashboard/log-console";
import { SpecPanel } from "./SpecPanel";
import {
  Clock,
  Sparkles,
  Check,
  Pause,
  X,
  AlertCircle,
  ExternalLink,
  FileText,
  AlertTriangle,
  RotateCcw,
  GitPullRequest,
  Flame,
} from "lucide-react";

interface PRSummary {
  title?: string;
  body?: string;
  review_limit_exceeded?: boolean;
  changed_files?: string[];
  risk_level?: string;
  risk_reason?: string;
  pr_url?: string;
  self_review_fallback?: boolean;
}

export function TaskHeroCards() {
  const {
    task,
    workflow,
    logs,
    requestSpecChanges,
    approveSpec,
    rejectPR,
    approvePR,
    retry,
    analyze,
    execute,
    isExecutionReady,
    prSummaries,
    feedback,
    setFeedback,
    submittingPR,
    startReview,
  } = useTaskDetail();

  const [isRejectFormOpen, setIsRejectFormOpen] = useState(false);
  const st = task?.status || "todo";
  
  const heroTodo = st === 'todo';
  const heroLoad = st === 'context_loading' || st === 'analyzing' || st === 'planning';
  const heroSpec = st === 'spec_review';
  const heroExec = ['coding','testing','fixing'].includes(st);
  const heroReview = st === 'reviewing';
  const heroPr = st === 'pr_ready' || st === 'human_review';
  const heroMerged = st === 'merged';
  const heroFailed = st === 'failed';

  const handleCancelRejection = () => {
    setIsRejectFormOpen(false);
    setFeedback("");
  };

  const handleConfirmRejection = async () => {
    await rejectPR();
    setIsRejectFormOpen(false);
  };

  return (
    <>
      {heroTodo && (
        <div className="bg-gradient-to-br from-slate-500/5 via-slate-500/[0.02] to-slate-500/10 border border-stroke/10 rounded-2xl p-5.5 flex flex-col sm:flex-row sm:items-center sm:justify-between gap-4 shadow-md hover:shadow-lg transition-all duration-200">
          <div className="flex items-center gap-3.5">
            <span className="w-11 h-11 flex items-center justify-center rounded-xl bg-slate-500/10 text-slate-600 dark:text-slate-300 border border-stroke/15 shrink-0 shadow-inner">
              <Clock className="h-5.5 w-5.5" />
            </span>
            <div>
              <div className="text-base font-bold text-foreground">Ready to Start</div>
              <div className="text-xs text-content-muted mt-0.5">Review the task details and start the workflow.</div>
            </div>
          </div>
          <div className="self-end sm:self-center">
            {!isExecutionReady ? (
              <button onClick={analyze} className="px-5 py-2.5 rounded-xl border-none bg-gradient-to-r from-brand-primary/80 to-brand-primary hover:from-brand-primary hover:to-brand-primary text-slate-950 text-xs font-extrabold transition-all duration-150 hover:shadow-md hover:shadow-brand-primary/20 hover:scale-[1.02] cursor-pointer whitespace-nowrap flex items-center gap-2">
                <Sparkles className="h-4 w-4" /> Start Analysis
              </button>
            ) : (
              <button onClick={execute} className="px-5 py-2.5 rounded-xl border-none bg-gradient-to-r from-emerald-500 to-teal-500 hover:from-emerald-400 hover:to-teal-400 text-white text-xs font-extrabold transition-all duration-150 hover:shadow-md hover:shadow-emerald-500/20 hover:scale-[1.02] cursor-pointer whitespace-nowrap flex items-center gap-2">
                <Check className="h-4 w-4" /> Start Execution
              </button>
            )}
          </div>
        </div>
      )}

      {heroLoad && (
        <div className="bg-gradient-to-br from-blue-500/10 via-blue-500/5 to-slate-500/5 border border-blue-500/20 rounded-2xl p-5.5 shadow-sm relative overflow-hidden">
          <div className="absolute -top-10 -right-10 w-24 h-24 bg-blue-500/10 rounded-full blur-2xl pointer-events-none" />
          <div className="flex items-center gap-3 mb-4 z-10">
            <span className="w-5 h-5 rounded-full border-2 border-blue-500/20 border-t-blue-600 dark:border-t-blue-400 animate-spin shrink-0"></span>
            <span className="text-sm font-bold text-blue-700 dark:text-blue-400 tracking-wide capitalize">
              {st === 'context_loading' ? 'Loading Context...' : st === 'planning' ? 'Planning Execution...' : 'Analyzing Requirements...'}
            </span>
          </div>
          <div className="flex flex-col gap-2 pl-1 z-10">
            {workflow?.checkpoints?.map((cp, idx) => (
              <div key={idx} className="flex items-center gap-2.5 py-1 text-xs font-mono text-emerald-800 dark:text-emerald-400/90">
                <span className="w-4 h-4 flex items-center justify-center rounded-full bg-emerald-500/10 border border-emerald-500/20 shrink-0">
                  <Check className="h-3 w-3 text-emerald-600 dark:text-emerald-400" />
                </span>
                <span className="capitalize">{cp.step.replace(/_/g, " ")}</span>
              </div>
            ))}
          </div>
        </div>
      )}

      {heroSpec && (
        <div className="flex flex-col gap-4">
          <div className="bg-gradient-to-br from-amber-500/10 via-amber-500/[0.02] to-orange-500/5 border border-amber-500/25 rounded-2xl p-5 flex flex-col md:flex-row md:items-center md:justify-between gap-4 shadow-md relative overflow-hidden">
            <div className="absolute -top-10 -right-10 w-24 h-24 bg-amber-500/10 rounded-full blur-2xl pointer-events-none" />
            <div className="flex items-center gap-3.5 z-10">
              <span className="w-11 h-11 flex items-center justify-center rounded-xl bg-amber-500/10 text-amber-600 border border-amber-500/20 shrink-0">
                <Pause className="h-5.5 w-5.5 text-amber-600 dark:text-amber-500" />
              </span>
              <div>
                <div className="text-sm font-bold text-foreground">Definition-of-Ready Gate</div>
                <div className="text-xs text-content-muted mt-0.5 leading-normal">Review the specification below and approve it before coding starts.</div>
              </div>
            </div>
            <div className="flex items-center gap-2 z-10 self-end md:self-center">
              <button onClick={requestSpecChanges} className="px-4 py-2 rounded-xl border border-stroke bg-background/50 text-content text-xs font-semibold hover:bg-slate-500/10 transition-all duration-150 cursor-pointer">
                Request Changes
              </button>
              <button onClick={approveSpec} className="px-4.5 py-2 rounded-xl border-none bg-gradient-to-r from-emerald-600 to-teal-600 hover:from-emerald-500 hover:to-teal-500 text-white text-xs font-bold transition-all duration-150 hover:shadow-md hover:shadow-emerald-500/20 active:scale-95 cursor-pointer shadow-sm flex items-center gap-1.5">
                <Check className="h-3.5 w-3.5" /> Approve Spec
              </button>
            </div>
          </div>
          <SpecPanel isExpanded={true} />
        </div>
      )}

      {heroExec && (
        <div className="rounded-2xl border border-stroke/10 bg-slate-950 shadow-lg overflow-hidden transition-all duration-300">
          <div className="flex items-center justify-between gap-3 px-5 py-4 border-b border-stroke/10 bg-slate-900/40">
            <div className="flex items-center gap-2.5">
              <span className="relative flex h-2 w-2">
                <span className={`animate-ping absolute inline-flex h-full w-full rounded-full opacity-75 ${st === 'fixing' ? 'bg-amber-400' : 'bg-blue-400'}`}></span>
                <span className={`relative inline-flex rounded-full h-2 w-2 ${st === 'fixing' ? 'bg-amber-500' : 'bg-blue-500'}`}></span>
              </span>
              <span className="text-xs uppercase font-extrabold tracking-wider text-slate-300 capitalize">{st} in progress</span>
            </div>
          </div>
          <LogConsole logs={logs} isExpanded={true} hideHeader={true} />
        </div>
      )}

      {heroReview && (
        <div className="bg-gradient-to-br from-indigo-500/10 via-indigo-500/5 to-slate-500/5 border border-indigo-500/25 rounded-2xl p-5 flex items-center gap-3.5 shadow-sm relative overflow-hidden">
          <div className="absolute -top-10 -right-10 w-24 h-24 bg-indigo-500/10 rounded-full blur-2xl pointer-events-none" />
          <span className="relative flex h-3.5 w-3.5 shrink-0 z-10">
            <span className="absolute inline-flex h-full w-full animate-ping rounded-full bg-indigo-500 opacity-75" />
            <span className="relative inline-flex h-3.5 w-3.5 rounded-full bg-indigo-500" />
          </span>
          <span className="text-sm font-bold text-indigo-700 dark:text-indigo-400 tracking-wide z-10">AI Review In Progress</span>
        </div>
      )}

      {heroPr && (
        <div className="flex flex-col gap-4">
          <div className="bg-card border border-stroke/10 rounded-2xl shadow-md overflow-hidden" style={{ borderColor: st === 'human_review' ? '#f59e0b' : '#10b981' }}>
            {st === 'human_review' && (
              <div className="flex flex-col md:flex-row md:items-center md:justify-between gap-4 px-5.5 py-4.5 bg-gradient-to-br from-amber-500/10 via-amber-500/[0.02] to-orange-500/5 border-b border-stroke/10">
                <div className="flex items-center gap-3">
                  <span className="w-10 h-10 flex items-center justify-center rounded-xl bg-amber-500/10 border border-amber-500/20 text-amber-600 dark:text-amber-500 shrink-0">
                    <Pause className="h-5 w-5" />
                  </span>
                  <div>
                    <div className="text-sm font-bold text-foreground">Waiting for Human Review</div>
                    <div className="text-xs text-content-muted mt-0.5 leading-normal">Final approval required before merging changes.</div>
                  </div>
                </div>
                <div className="flex items-center gap-2 self-end md:self-center">
                  <button
                    onClick={() => setIsRejectFormOpen(!isRejectFormOpen)}
                    disabled={submittingPR}
                    className="px-4 py-2 rounded-xl border border-rose-500/25 bg-rose-500/5 hover:bg-rose-500/10 text-rose-600 dark:text-rose-400 text-xs font-semibold hover:shadow-sm active:scale-95 transition-all duration-150 cursor-pointer"
                  >
                    <X className="h-3.5 w-3.5 inline mr-1" /> Reject PR
                  </button>
                  <button
                    onClick={approvePR}
                    disabled={submittingPR}
                    className="px-4.5 py-2 rounded-xl border-none bg-gradient-to-r from-emerald-600 to-teal-600 hover:from-emerald-500 hover:to-teal-500 text-white text-xs font-bold hover:shadow-md hover:shadow-emerald-500/20 active:scale-95 transition-all duration-150 cursor-pointer flex items-center gap-1.5"
                  >
                    <Check className="h-3.5 w-3.5" /> Approve Merge
                  </button>
                </div>
              </div>
            )}
            
            {st === 'pr_ready' && (
              <div className="flex flex-col md:flex-row md:items-center md:justify-between gap-4 px-5.5 py-4.5 bg-gradient-to-br from-emerald-500/10 via-emerald-500/[0.02] to-teal-500/5 border-b border-stroke/10">
                <div className="flex items-center gap-3">
                  <span className="w-10 h-10 flex items-center justify-center rounded-xl bg-emerald-500/10 border border-emerald-500/20 text-emerald-600 dark:text-emerald-500 shrink-0">
                    <Sparkles className="h-5 w-5" />
                  </span>
                  <div>
                    <div className="text-sm font-bold text-foreground">Pull Request Ready</div>
                    <div className="text-xs text-content-muted mt-0.5 leading-normal">Review the changes on your Git provider or merge directly from the app.</div>
                  </div>
                </div>
                <div className="flex flex-wrap items-center gap-2 self-end md:self-center">
                  <button
                    onClick={() => startReview()}
                    disabled={submittingPR}
                    className="px-4 py-2 rounded-xl border border-indigo-500/20 bg-indigo-500/5 hover:bg-indigo-500/10 text-indigo-600 dark:text-indigo-400 text-xs font-semibold hover:shadow-sm active:scale-95 transition-all duration-150 flex items-center gap-1.5 cursor-pointer"
                  >
                    <Clock className="h-3.5 w-3.5" /> Start Review
                  </button>
                  <button
                    onClick={() => setIsRejectFormOpen(!isRejectFormOpen)}
                    disabled={submittingPR}
                    className="px-4 py-2 rounded-xl border border-rose-500/25 bg-rose-500/5 hover:bg-rose-500/10 text-rose-600 dark:text-rose-400 text-xs font-semibold hover:shadow-sm active:scale-95 transition-all duration-150 cursor-pointer"
                  >
                    <X className="h-3.5 w-3.5 inline mr-1" /> Reject PR
                  </button>
                  <button
                    onClick={approvePR}
                    disabled={submittingPR}
                    className="px-4.5 py-2 rounded-xl border-none bg-gradient-to-r from-emerald-600 to-teal-600 hover:from-emerald-500 hover:to-teal-500 text-white text-xs font-bold hover:shadow-md hover:shadow-emerald-500/20 active:scale-95 transition-all duration-150 cursor-pointer flex items-center gap-1.5"
                  >
                    <Check className="h-3.5 w-3.5" /> Merge Pull Request
                  </button>
                </div>
              </div>
            )}
 
            {isRejectFormOpen && (
              <div className="p-5.5 border-b border-stroke/10 bg-slate-500/[0.02] flex flex-col gap-3.5 animate-fade-in">
                <div className="text-[10px] font-bold text-rose-600 dark:text-rose-500 uppercase tracking-wider">
                  Provide Rejection Feedback
                </div>
                <p className="text-xs text-content-muted leading-relaxed">
                  Describe what needs to be fixed. The agent will read this feedback and automatically enter the fixing phase.
                </p>
                <textarea
                  value={feedback}
                  onChange={(e) => setFeedback(e.target.value)}
                  placeholder="e.g. The database migration is missing a fallback down function, and the button padding needs adjustment..."
                  className="w-full h-28 rounded-xl border border-stroke/15 bg-background/50 p-3.5 text-xs text-foreground outline-none focus:border-rose-500/40 focus:ring-1 focus:ring-rose-500/20 focus:bg-background/80 transition-all duration-150 resize-none font-sans leading-relaxed shadow-inner"
                  disabled={submittingPR}
                />
                <div className="flex justify-end gap-2.5">
                  <button
                    onClick={handleCancelRejection}
                    disabled={submittingPR}
                    className="px-4 py-2 rounded-xl border border-stroke bg-background/50 hover:bg-slate-500/10 text-xs font-semibold transition-all duration-150 cursor-pointer"
                  >
                    Cancel
                  </button>
                  <button
                    onClick={handleConfirmRejection}
                    disabled={submittingPR || !feedback.trim()}
                    className="px-4.5 py-2 rounded-xl border-none bg-gradient-to-r from-rose-600 to-red-600 hover:from-rose-500 hover:to-red-500 text-white text-xs font-bold transition-all duration-150 hover:shadow-md hover:shadow-rose-500/20 active:scale-95 cursor-pointer shadow-sm flex items-center gap-1.5"
                  >
                    {submittingPR ? (
                      <span className="h-3 w-3 animate-spin rounded-full border border-white border-t-transparent" />
                    ) : (
                      <X className="h-3.5 w-3.5" />
                    )}
                    Confirm Rejection & Retry
                  </button>
                </div>
              </div>
            )}

            <div className="px-5.5 py-5">
              <div className="flex flex-col gap-5">
                {/* PR metadata view */}
                {prSummaries && prSummaries.length > 0 ? (
                  prSummaries.map((prItem, idx: number) => {
                    const pr = prItem as unknown as PRSummary;
                    return (
                    <div key={idx} className="glass-panel p-5 glow-on-hover flex flex-col gap-3.5">
                      <div className="flex justify-between items-start gap-4">
                        <div className="flex items-center gap-2">
                          <GitPullRequest className="h-5 w-5 text-success" />
                          <h3 className="font-semibold text-base text-foreground leading-snug">{pr.title || "Pull Request"}</h3>
                        </div>
                        {pr.pr_url && (
                          <a href={pr.pr_url} target="_blank" rel="noreferrer" className="text-brand-primary hover:underline text-xs flex items-center gap-1 font-medium shrink-0">
                            View on Git Provider <ExternalLink className="h-3.5 w-3.5" />
                          </a>
                        )}
                      </div>
                      
                      {pr.body && (
                        <div className="text-[13px] text-foreground whitespace-pre-wrap font-mono leading-relaxed bg-surface border border-stroke p-3.5 rounded-lg">
                          {pr.body}
                        </div>
                      )}
                      
                      {pr.changed_files && pr.changed_files.length > 0 && (
                        <div className="flex flex-col gap-1.5">
                          <span className="text-[11px] font-bold uppercase tracking-wider text-content-muted flex items-center gap-1">
                            <FileText className="h-3.5 w-3.5" /> Files Changed ({pr.changed_files.length})
                          </span>
                          <div className="flex flex-wrap gap-1.5 mt-1">
                            {pr.changed_files.map((f: string, i: number) => (
                              <span key={i} className="text-[11px] px-2 py-0.5 bg-surface border border-stroke rounded text-foreground font-mono truncate max-w-[280px]" title={f}>
                                {f.split("/").pop()}
                              </span>
                            ))}
                          </div>
                        </div>
                      )}
                      
                      {(pr.risk_level || pr.risk_reason) && (
                        <div className="bg-surface border border-stroke p-3.5 rounded-lg flex flex-col gap-2">
                          <div className="flex items-center gap-2">
                            <span className="text-[11px] font-bold uppercase tracking-wider text-content-muted">Risk Assessment:</span>
                            <span className={`text-[10px] uppercase font-bold tracking-wider px-2 py-0.5 rounded-full flex items-center gap-1 ${
                              pr.risk_level === 'critical' || pr.risk_level === 'high'
                                ? 'bg-danger/10 text-danger border border-danger/25'
                                : pr.risk_level === 'medium'
                                ? 'bg-warning/10 text-warning border border-warning/25'
                                : 'bg-success/10 text-success border border-success/25'
                            }`}>
                              {(pr.risk_level === 'critical' || pr.risk_level === 'high') && <Flame className="h-3 w-3 animate-pulse" />}
                              {pr.risk_level || 'Unknown'}
                            </span>
                          </div>
                          {pr.risk_reason && <p className="text-[12px] text-content-muted leading-relaxed">{pr.risk_reason}</p>}
                          
                          {pr.review_limit_exceeded && (
                            <p className="text-[11px] text-danger font-medium italic flex items-center gap-1.5 mt-1">
                              <AlertTriangle className="h-3.5 w-3.5" /> Auto-review limit exceeded. Human review required.
                            </p>
                          )}
                          {pr.self_review_fallback && (
                            <p className="text-[11px] text-warning font-medium italic flex items-center gap-1.5">
                              <AlertCircle className="h-3.5 w-3.5" /> Self-review fallback was used.
                            </p>
                          )}
                        </div>
                      )}
                    </div>
                    );
                  })
                ) : (
                  <div className="flex flex-col gap-3">
                    <div className="flex items-center gap-2 text-content-muted">
                      <GitPullRequest className="h-4 w-4" />
                      <span className="text-sm font-semibold">Pull Request Status</span>
                    </div>
                    {task?.pr_urls && task.pr_urls.length > 0 ? (
                      <div className="flex flex-col gap-2">
                        {task.pr_urls.map((url, uidx) => {
                          let label = "View Pull Request";
                          const match = url.match(/github\.com\/(.+?)\/(.+?)\/pull\/(\d+)/);
                          if (match) {
                            label = `${match[1]}/${match[2]} #${match[3]}`;
                          }
                          return (
                            <div key={uidx} className="flex items-center justify-between bg-surface border border-stroke rounded-lg p-3 hover:border-brand-primary/50 transition">
                              <div className="flex items-center gap-2.5 min-w-0">
                                <GitPullRequest className="h-4 w-4 text-brand-primary" />
                                <div className="flex flex-col min-w-0">
                                  <span className="text-xs font-semibold text-foreground truncate">
                                    {label}
                                  </span>
                                  <span className="text-[10px] text-content-muted truncate max-w-[250px] sm:max-w-[400px]">
                                    {url}
                                  </span>
                                </div>
                              </div>
                              <a
                                href={url}
                                target="_blank"
                                rel="noreferrer"
                                className="px-3 py-1 bg-brand-primary/10 border border-brand-primary/20 hover:bg-brand-primary/20 text-brand-primary rounded-md text-[11px] font-medium transition flex items-center gap-1 cursor-pointer shrink-0"
                              >
                                View PR <ExternalLink className="h-3 w-3" />
                              </a>
                            </div>
                          );
                        })}
                      </div>
                    ) : (
                      <div className="text-xs text-content-muted bg-surface border border-stroke p-3 rounded-lg">
                        No PR links or metadata registered yet for this task.
                      </div>
                    )}
                  </div>
                )}
              </div>
            </div>
          </div>
          {logs.length > 0 && (
            <LogConsole logs={logs} isExpanded={true} />
          )}
        </div>
      )}

      {heroFailed && (
        <div className="rounded-2xl border border-rose-500/25 bg-slate-950 shadow-lg overflow-hidden flex flex-col transition-all duration-300">
          <div className="flex flex-col md:flex-row md:items-center md:justify-between gap-4 px-5.5 py-5 bg-gradient-to-br from-rose-500/10 via-rose-500/[0.02] to-red-500/5 border-b border-stroke/10">
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
            <button onClick={retry} className="px-4.5 py-2.5 rounded-xl border-none bg-gradient-to-r from-rose-600 to-red-600 hover:from-rose-500 hover:to-red-500 text-white text-xs font-bold transition-all duration-150 hover:shadow-md hover:shadow-rose-500/20 active:scale-95 cursor-pointer whitespace-nowrap flex items-center gap-1.5 self-end md:self-center">
              <RotateCcw className="h-3.5 w-3.5" /> Restart Task
            </button>
          </div>
          {logs.length > 0 && (
            <LogConsole logs={logs} isExpanded={true} hideHeader={true} />
          )}
        </div>
      )}

      {heroMerged && (
        <div className="flex flex-col gap-4">
          <div className="bg-gradient-to-br from-emerald-500/10 via-emerald-500/[0.02] to-teal-500/5 border border-emerald-500/25 rounded-2xl p-5.5 flex gap-3.5 items-start shadow-md relative overflow-hidden">
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
          {logs.length > 0 && (
            <LogConsole logs={logs} isExpanded={true} />
          )}
        </div>
      )}
    </>
  );
}
