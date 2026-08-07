"use client";

import { useTaskDetail } from "../TaskDetailContext";
import ReactMarkdown from "react-markdown";
import remarkGfm from "remark-gfm";
import { ExternalLink, FileText, AlertTriangle, AlertCircle, GitPullRequest, Flame } from "lucide-react";

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

export function PRMetadataView() {
  const { prSummaries, task } = useTaskDetail();

  return (
    <div className="px-5.5 py-5">
      <div className="flex flex-col gap-5">
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
                  <div className="text-[13px] text-foreground font-sans leading-relaxed bg-surface border border-stroke p-4 rounded-xl overflow-hidden mt-2 relative prose prose-sm dark:prose-invert max-w-none
                  prose-headings:mt-4 prose-headings:mb-2 prose-h3:text-sm prose-h3:font-bold prose-h2:text-base prose-h1:text-lg
                  prose-p:mt-2 prose-p:mb-2
                  prose-a:text-brand-primary prose-a:no-underline hover:prose-a:underline
                  prose-strong:font-bold prose-strong:text-foreground
                  prose-ul:mt-2 prose-ul:mb-2 prose-li:mt-0.5 prose-li:mb-0.5
                  prose-table:mt-4 prose-table:mb-4 prose-table:w-full prose-table:border-collapse prose-table:text-xs
                  prose-th:border prose-th:border-stroke prose-th:bg-surface/50 prose-th:px-3 prose-th:py-2 prose-th:text-left prose-th:font-semibold
                  prose-td:border prose-td:border-stroke prose-td:px-3 prose-td:py-2
                  prose-code:px-1.5 prose-code:py-0.5 prose-code:bg-surface-elevated prose-code:rounded-md prose-code:text-[11px] prose-code:border prose-code:border-stroke/50 prose-code:font-mono prose-code:before:content-none prose-code:after:content-none
                  prose-pre:bg-surface-elevated prose-pre:border prose-pre:border-stroke prose-pre:p-3 prose-pre:rounded-lg prose-pre:text-[11px] prose-pre:font-mono prose-pre:overflow-x-auto
                ">
                    <ReactMarkdown
                      remarkPlugins={[remarkGfm]}
                      components={{
                        table: ({ ...props }) => (
                          <div className="w-full overflow-x-auto my-4 rounded-lg border border-stroke">
                            <table className="w-full text-sm text-left border-collapse" {...props} />
                          </div>
                        ),
                        th: ({ ...props }) => <th className="bg-surface/50 px-4 py-3 font-semibold text-foreground border-b border-stroke" {...props} />,
                        td: ({ ...props }) => <td className="px-4 py-3 text-content-muted border-b border-stroke/50" {...props} />
                      }}
                    >
                      {pr.body}
                    </ReactMarkdown>
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
                      <span className={`text-[10px] uppercase font-bold tracking-wider px-2 py-0.5 rounded-full flex items-center gap-1 ${pr.risk_level === "critical" || pr.risk_level === "high"
                        ? "bg-danger/10 text-danger border border-danger/25"
                        : pr.risk_level === "medium"
                          ? "bg-warning/10 text-warning border border-warning/25"
                          : "bg-success/10 text-success border border-success/25"
                        }`}>
                        {(pr.risk_level === "critical" || pr.risk_level === "high") && <Flame className="h-3 w-3 animate-pulse" />}
                        {pr.risk_level || "Unknown"}
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
  );
}
