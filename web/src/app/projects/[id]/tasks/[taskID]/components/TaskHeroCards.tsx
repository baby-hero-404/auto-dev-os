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
        <div className="bg-card border border-stroke rounded-xl px-5.5 py-5 flex items-center justify-between shadow-sm">
          <div className="flex items-center gap-3.5">
            <span className="w-10 h-10 flex items-center justify-center rounded-full bg-surface text-content-muted border border-stroke shrink-0">
              <Clock className="h-5 w-5 text-content-muted" />
            </span>
            <div>
              <div className="text-[15px] font-bold text-foreground">Ready to Start</div>
              <div className="text-[13px] text-content-muted">Review the task details and start the workflow.</div>
            </div>
          </div>
          <div>
            {!isExecutionReady ? (
              <button onClick={analyze} className="px-4 py-2 rounded-lg border-none bg-brand-primary text-slate-950 text-[13px] font-semibold hover:opacity-90 cursor-pointer shadow-sm whitespace-nowrap flex items-center gap-1.5">
                <Sparkles className="h-4 w-4" /> Start Analysis
              </button>
            ) : (
              <button onClick={execute} className="px-4 py-2 rounded-lg border-none bg-brand-primary text-slate-950 text-[13px] font-semibold hover:opacity-90 cursor-pointer shadow-sm whitespace-nowrap flex items-center gap-1.5">
                <Check className="h-4 w-4" /> Start Execution
              </button>
            )}
          </div>
        </div>
      )}

      {heroLoad && (
        <div className="bg-card border border-[#90c5ff] rounded-xl p-5.5">
          <div className="flex items-center gap-2.5 mb-3.5">
            <span className="w-4 h-4 rounded-full border-2 border-[#90c5ff] border-t-[#005bb8] animate-spin"></span>
            <span className="text-sm font-semibold text-[#005bb8]">
              {st === 'context_loading' ? 'Loading context...' : st === 'planning' ? 'Planning execution steps...' : 'Analyzing requirements...'}
            </span>
          </div>
          {workflow?.checkpoints?.map((cp, idx) => (
            <div key={idx} className="flex items-center gap-2.5 py-1 text-[13px]">
              <span className="w-4 text-center text-[#00590e]">
                <Check className="h-3.5 w-3.5 inline text-success" />
              </span>
              <span className="text-[#00590e] capitalize">{cp.step.replace(/_/g, " ")}</span>
            </div>
          ))}
        </div>
      )}

      {heroSpec && (
        <div className="flex flex-col gap-4">
          <div className="bg-warning/10 border border-warning/30 rounded-xl px-5.5 py-4 flex items-center justify-between">
            <div className="flex items-center gap-3">
              <span className="w-10 h-10 flex items-center justify-center rounded-full bg-warning/10 text-warning border border-warning/20 shrink-0">
                <Pause className="h-5 w-5" />
              </span>
              <div>
                <div className="text-[14px] font-bold text-foreground">Definition-of-Ready Gate</div>
                <div className="text-[13px] text-content-muted">Review the specification below and approve it before coding starts.</div>
              </div>
            </div>
            <div className="flex gap-2">
              <button onClick={requestSpecChanges} className="px-3.5 py-1.5 rounded-lg border border-stroke bg-surface text-content text-[13px] font-medium hover:bg-muted/10 transition cursor-pointer">
                Request Changes
              </button>
              <button onClick={approveSpec} className="px-4 py-1.5 rounded-lg border-none bg-success text-white text-[13px] font-semibold hover:opacity-90 transition cursor-pointer shadow-sm flex items-center gap-1">
                <Check className="h-3.5 w-3.5" /> Approve Spec
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
          <LogConsole logs={logs} isExpanded={true} hideHeader={true} />
        </div>
      )}

      {heroReview && (
        <div className="bg-card border border-brand-primary/30 rounded-xl px-5.5 py-5 flex items-center gap-3">
          <span className="relative flex h-3 w-3 shrink-0">
            <span className="absolute inline-flex h-full w-full animate-ping rounded-full bg-brand-primary opacity-75" />
            <span className="relative inline-flex h-3 w-3 rounded-full bg-brand-primary" />
          </span>
          <span className="text-sm font-semibold text-brand-primary">AI review in progress</span>
        </div>
      )}

      {heroPr && (
        <div className="flex flex-col gap-4">
          <div className="bg-card border rounded-xl overflow-hidden" style={{ borderColor: st === 'human_review' ? 'var(--warning)' : 'var(--success)' }}>
            {st === 'human_review' && (
              <div className="flex flex-col sm:flex-row sm:items-center sm:justify-between gap-3 px-5.5 py-4 bg-warning/10 border-b border-warning/20">
                <div className="flex items-center gap-3">
                  <span className="w-9 h-9 flex items-center justify-center rounded-full bg-warning/20 text-warning shrink-0">
                    <Pause className="h-4.5 w-4.5" />
                  </span>
                  <div>
                    <div className="text-[14px] font-bold text-foreground">Waiting for human review</div>
                    <div className="text-[13px] text-content-muted">Final approval required before merging changes.</div>
                  </div>
                </div>
                <div className="flex items-center gap-2">
                  <button
                    onClick={() => setIsRejectFormOpen(!isRejectFormOpen)}
                    disabled={submittingPR}
                    className="px-3.5 py-1.5 rounded-lg border border-danger/30 bg-card text-danger text-[13px] font-medium hover:bg-danger/10 transition cursor-pointer"
                  >
                    <X className="h-3.5 w-3.5 inline mr-1" /> Reject PR
                  </button>
                  <button
                    onClick={approvePR}
                    disabled={submittingPR}
                    className="px-4 py-1.5 rounded-lg border-none bg-success text-white text-[13px] font-semibold hover:opacity-90 transition cursor-pointer shadow-sm flex items-center gap-1"
                  >
                    <Check className="h-3.5 w-3.5" /> Approve Merge
                  </button>
                </div>
              </div>
            )}
            
            {st === 'pr_ready' && (
              <div className="flex flex-col sm:flex-row sm:items-center sm:justify-between gap-3 px-5.5 py-4 bg-success/10 border-b border-success/20">
                <div className="flex items-center gap-3">
                  <span className="w-9 h-9 flex items-center justify-center rounded-full bg-success/20 text-success shrink-0">
                    <Sparkles className="h-4.5 w-4.5" />
                  </span>
                  <div>
                    <div className="text-[14px] font-bold text-foreground">Pull Request Ready</div>
                    <div className="text-[13px] text-content-muted">Review the changes on your Git provider or merge directly from the app.</div>
                  </div>
                </div>
                <div className="flex items-center gap-2">
                  <button
                    onClick={() => startReview()}
                    disabled={submittingPR}
                    className="px-3.5 py-1.5 rounded-lg border border-brand-primary/30 bg-card text-brand-primary text-[13px] font-medium hover:bg-brand-primary/10 transition cursor-pointer flex items-center gap-1"
                  >
                    <Clock className="h-3.5 w-3.5" /> Start Review
                  </button>
                  <button
                    onClick={() => setIsRejectFormOpen(!isRejectFormOpen)}
                    disabled={submittingPR}
                    className="px-3.5 py-1.5 rounded-lg border border-danger/30 bg-card text-danger text-[13px] font-medium hover:bg-danger/10 transition cursor-pointer"
                  >
                    <X className="h-3.5 w-3.5 inline mr-1" /> Reject PR
                  </button>
                  <button
                    onClick={approvePR}
                    disabled={submittingPR}
                    className="px-4 py-1.5 rounded-lg border-none bg-success text-white text-[13px] font-semibold hover:opacity-90 transition cursor-pointer shadow-sm flex items-center gap-1"
                  >
                    <Check className="h-3.5 w-3.5" /> Merge Pull Request
                  </button>
                </div>
              </div>
            )}

            {isRejectFormOpen && (
              <div className="px-5.5 py-4 border-b border-stroke bg-surface/50 flex flex-col gap-3 animate-fade-in">
                <div className="text-xs font-semibold text-danger uppercase tracking-wider">
                  Provide Rejection Feedback
                </div>
                <p className="text-xs text-content-muted">
                  Describe what needs to be fixed. The agent will read this feedback and automatically enter the fixing phase.
                </p>
                <textarea
                  value={feedback}
                  onChange={(e) => setFeedback(e.target.value)}
                  placeholder="e.g. The database migration is missing a fallback down function, and the button padding needs adjustment..."
                  className="w-full h-24 rounded-lg border border-stroke bg-card p-3 text-sm text-foreground outline-none focus:border-danger transition-all resize-none font-mono"
                  disabled={submittingPR}
                />
                <div className="flex justify-end gap-2">
                  <button
                    onClick={handleCancelRejection}
                    disabled={submittingPR}
                    className="px-3.5 py-1.5 rounded-lg border border-stroke bg-card text-[13px] font-medium hover:bg-surface transition cursor-pointer"
                  >
                    Cancel
                  </button>
                  <button
                    onClick={handleConfirmRejection}
                    disabled={submittingPR || !feedback.trim()}
                    className="px-4 py-1.5 rounded-lg border-none bg-danger text-white text-[13px] font-semibold hover:opacity-90 transition cursor-pointer shadow-sm flex items-center gap-1.5"
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
        <div className="bg-card border border-danger/30 rounded-xl overflow-hidden flex flex-col">
          <div className="flex items-start justify-between px-5.5 py-5 bg-danger/10 border-b border-danger/20">
            <div className="flex gap-3.5 items-start">
              <span className="inline-flex items-center justify-center w-8 h-8 rounded-full bg-danger text-white text-base font-bold shrink-0">
                <X className="h-4.5 w-4.5" />
              </span>
              <div>
                <div className="text-[15px] font-bold text-danger mb-1">Task failed</div>
                <div className="text-[13px] text-content-muted leading-relaxed max-w-2xl break-words font-mono">
                  {workflow?.job?.last_error || "Unrecoverable error. Restart the task."}
                </div>
              </div>
            </div>
            <button onClick={retry} className="px-4 py-1.5 rounded-lg border-none bg-danger text-white text-[13px] font-semibold hover:opacity-90 cursor-pointer shadow-sm whitespace-nowrap flex items-center gap-1">
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
          <div className="bg-success/10 border border-success/30 rounded-xl p-5.5 flex gap-3.5 items-start">
            <span className="inline-flex items-center justify-center w-8 h-8 rounded-full bg-success text-white text-base font-bold shrink-0">
              <Check className="h-5 w-5" />
            </span>
            <div>
              <div className="text-[15px] font-bold text-success mb-1">Merged into main</div>
              <div className="text-[13px] text-content-muted leading-relaxed">
                Task completed successfully and code is now integrated.
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
