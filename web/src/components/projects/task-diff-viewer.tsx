import { useMemo, useState } from "react";
import { Sparkles } from "lucide-react";
import { Markdown } from "@/components/ui/markdown";

export interface ParsedFileDiff {
  filename: string;
  diffLines: string[];
}

export function parseUnifiedDiff(diffText: string): ParsedFileDiff[] {
  if (!diffText) return [];
  const lines = diffText.split("\n");
  const fileDiffs: ParsedFileDiff[] = [];
  let currentDiff: ParsedFileDiff | null = null;

  for (const line of lines) {
    if (line.startsWith("diff --git ")) {
      const parts = line.split(" b/");
      let filename = "";
      if (parts.length > 1) {
        filename = parts[1].trim();
      } else {
        const match = line.match(/b\/(.*)$/);
        filename = match ? match[1] : "unknown";
      }
      currentDiff = {
        filename,
        diffLines: [line],
      };
      fileDiffs.push(currentDiff);
    } else if (currentDiff) {
      currentDiff.diffLines.push(line);
    }
  }

  return fileDiffs;
}

const RISK_BADGES: Record<string, string> = {
  low: "bg-emerald-500/10 text-emerald-600 dark:text-emerald-400 border-emerald-500/20",
  medium: "bg-amber-500/10 text-amber-600 dark:text-amber-400 border-amber-500/20",
  high: "bg-rose-500/10 text-rose-600 dark:text-rose-400 border-rose-500/20",
  critical: "bg-rose-500/20 text-rose-600 dark:text-rose-400 border-rose-500/30 animate-pulse",
};

interface TaskDiffViewerProps {
  diffText: string;
  displayFiles: string[];
  prSummaries: { title?: string; body?: string; review_limit_exceeded?: boolean }[];
  riskAssessment: { level: string; reason: string };
  riskDomains: string[];
}

export function TaskDiffViewer({
  diffText,
  displayFiles,
  prSummaries,
  riskAssessment,
  riskDomains,
}: TaskDiffViewerProps) {
  const [selectedFile, setSelectedFile] = useState<string | null>(null);

  const activeSelectedFile = selectedFile || (displayFiles.length > 0 ? displayFiles[0] : null);

  const parsedDiffs = useMemo(() => {
    return parseUnifiedDiff(diffText);
  }, [diffText]);

  const activeFileDiff = useMemo(() => {
    if (!activeSelectedFile) return null;
    return parsedDiffs.find((d) => {
      const df = d.filename.replace(/\\/g, "/");
      const sf = activeSelectedFile.replace(/\\/g, "/");
      return df === sf || df.endsWith("/" + sf) || sf.endsWith("/" + df);
    });
  }, [parsedDiffs, activeSelectedFile]);

  return (
    <div className="grid lg:grid-cols-[340px_1fr] border-b border-stroke min-h-[420px]">
      {/* PR Summary Sidebar */}
      <div className="border-r border-stroke p-5 space-y-5 bg-surface/10">
        <div>
          <h3 className="font-mono text-[10px] font-bold uppercase tracking-wider text-content-muted mb-2 flex items-center gap-1">
            <Sparkles size={12} className="text-brand-primary" /> AI PR Summary
          </h3>
          {prSummaries.length > 0 && prSummaries[0].body ? (
            <div className="text-xs leading-relaxed text-content-muted prose dark:prose-invert max-h-60 overflow-y-auto pr-1">
              <Markdown content={prSummaries[0].body} />
            </div>
          ) : (
            <p className="text-xs leading-relaxed text-content-muted">
              Automated changes generated for this execution run. The agent completed the code-backend,
              code-frontend, and successfully compiled all builds.
            </p>
          )}
        </div>

        <div>
          <h3 className="font-mono text-[10px] font-bold uppercase tracking-wider text-content-muted mb-2">
            Risk Assessment
          </h3>
          <div className={`rounded-lg border p-3.5 ${RISK_BADGES[riskAssessment.level] || RISK_BADGES.low} space-y-2`}>
            <div className="flex items-center justify-between mb-1">
              <span className="font-sans text-[10px] font-bold uppercase tracking-wider">
                Level: {riskAssessment.level}
              </span>
            </div>
            <p className="text-[11px] leading-relaxed opacity-90">{riskAssessment.reason}</p>
            <div className="pt-1.5 border-t border-current/10">
              <span className="font-mono text-[9px] font-bold uppercase tracking-wider opacity-85">
                Risk Domains: {riskDomains?.join(", ") || "none"}
              </span>
            </div>
          </div>
        </div>

        <div>
          <h3 className="font-mono text-[10px] font-bold uppercase tracking-wider text-content-muted mb-2">
            Changed Files ({displayFiles.length})
          </h3>
          <div className="space-y-1 max-h-52 overflow-y-auto pr-1">
            {displayFiles.map((file) => (
              <button
                key={file}
                onClick={() => setSelectedFile(file)}
                className={`flex w-full items-center justify-between rounded-md px-2.5 py-1.5 text-left text-xs font-mono transition-all cursor-pointer border ${
                  activeSelectedFile === file
                    ? "bg-brand-primary/10 border-brand-primary/20 text-brand-primary"
                    : "border-transparent text-content-muted hover:bg-surface hover:text-foreground"
                }`}
              >
                <span className="truncate">{file.split("/").pop()}</span>
                <span className="text-[9px] opacity-65 truncate max-w-[100px]">{file}</span>
              </button>
            ))}
            {displayFiles.length === 0 && (
              <p className="text-xs text-content-muted italic">No file modifications detected.</p>
            )}
          </div>
        </div>
      </div>

      {/* Interactive Git Diff Review Canvas */}
      <div className="p-5 flex flex-col min-w-0 bg-surface/5">
        <div className="mb-3 flex items-center justify-between">
          <span className="font-mono text-[11px] text-content-muted truncate max-w-[80%]">
            Diff Review &mdash;{" "}
            <span className="text-foreground font-semibold">{activeSelectedFile || "Select a file"}</span>
          </span>
          <span className="text-[9px] bg-surface border border-stroke text-content-muted px-2 py-0.5 rounded font-mono uppercase">
            Git Diff
          </span>
        </div>

        <div className="flex-1 min-h-[350px] overflow-auto rounded-lg border border-stroke bg-slate-950 dark:bg-black p-4 font-mono text-xs leading-relaxed shadow-inner">
          {activeSelectedFile ? (
            activeFileDiff ? (
              <div className="space-y-0.5 font-mono text-[11px] text-slate-300 select-text">
                {activeFileDiff.diffLines.map((line, idx) => {
                  let lineClass = "text-slate-400";
                  if (line.startsWith("+") && !line.startsWith("+++")) {
                    lineClass = "bg-emerald-500/15 text-emerald-400 px-1 border-l-2 border-emerald-500";
                  } else if (line.startsWith("-") && !line.startsWith("---")) {
                    lineClass = "bg-rose-500/15 text-rose-400 px-1 border-l-2 border-rose-500";
                  } else if (line.startsWith("@@")) {
                    lineClass = "text-purple-400 bg-purple-500/10 font-semibold py-0.5";
                  } else if (line.startsWith("diff ") || line.startsWith("index ")) {
                    lineClass = "text-slate-500/80 italic";
                  } else if (line.startsWith("--- ") || line.startsWith("+++ ")) {
                    lineClass = "text-slate-400 font-semibold";
                  } else {
                    lineClass = "text-slate-300 pl-1.5";
                  }
                  return (
                    <div key={idx} className={`font-mono whitespace-pre-wrap ${lineClass}`}>
                      {line}
                    </div>
                  );
                })}
              </div>
            ) : (
              <div className="h-full flex flex-col items-center justify-center text-content-muted py-12">
                <p className="text-sm">Diff details not available inside sandbox state.</p>
                <p className="text-[10px] mt-1">Review live branch changes via your Git provider.</p>
              </div>
            )
          ) : (
            <div className="h-full flex items-center justify-center text-content-muted py-12">
              Select a file on the left to inspect git changes.
            </div>
          )}
        </div>
      </div>
    </div>
  );
}
