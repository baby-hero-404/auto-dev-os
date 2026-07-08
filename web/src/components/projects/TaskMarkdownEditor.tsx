import ReactMarkdown from "react-markdown";
import remarkGfm from "remark-gfm";
import { FileText, Minimize2 } from "lucide-react";

interface TaskMarkdownEditorProps {
  description: string;
  setDescription: (desc: string) => void;
  onClose: () => void;
}

export function TaskMarkdownEditor({ description, setDescription, onClose }: TaskMarkdownEditorProps) {
  return (
    <div className="absolute inset-4 z-20 flex flex-col rounded-xl border border-stroke bg-card shadow-2xl overflow-hidden animate-modal-in">
      {/* Header */}
      <div className="flex items-center justify-between border-b border-stroke p-4 shrink-0 bg-card/95 backdrop-blur-sm">
        <div className="flex items-center gap-3">
          <div className="flex h-8 w-8 items-center justify-center rounded-lg bg-brand-primary-muted border border-brand-primary/20 text-brand-primary">
            <FileText size={16} />
          </div>
          <div>
            <h3 className="font-sans text-sm font-bold text-foreground">Rich Description Editor</h3>
            <p className="text-[10px] text-content-muted font-medium mt-0.5">Compose markdown description with live preview</p>
          </div>
        </div>

        <div className="flex items-center gap-3">
          {/* Template selector */}
          <div className="relative">
            <select
              onChange={(e) => {
                const templateVal = e.target.value;
                if (templateVal === "feature") {
                  setDescription(
                    "## Objective\nBrief summary of the feature.\n\n## Acceptance Criteria\n- [ ] Item 1\n- [ ] Item 2\n\n## Affected Components\n- List files/modules to modify."
                  );
                } else if (templateVal === "bug") {
                  setDescription(
                    "## Problem\nDetail what is wrong and what errors are displayed.\n\n## Expected Behavior\nWhat is the correct flow?\n\n## Steps to Reproduce\n1. Go to...\n2. Run..."
                  );
                } else if (templateVal === "refactor") {
                  setDescription(
                    "## Goals\n- [ ] Clean up redundant code\n- [ ] Enhance performance/readability\n\n## Plan\nDetail the changes planned."
                  );
                }
                e.target.value = ""; // Reset
              }}
              className="rounded-lg border border-stroke bg-surface px-2.5 py-1.5 text-xs text-foreground cursor-pointer focus:outline-none focus:ring-1 focus:ring-stroke-focus font-medium"
            >
              <option value="">Insert Template...</option>
              <option value="feature">Feature Spec Template</option>
              <option value="bug">Bug Fix Template</option>
              <option value="refactor">Refactoring Template</option>
            </select>
          </div>

          <button
            type="button"
            onClick={onClose}
            className="inline-flex items-center gap-1.5 rounded-lg border border-stroke bg-surface px-3 py-1.5 text-xs font-semibold text-foreground hover:bg-surface-code hover:border-stroke-focus transition-all duration-150 cursor-pointer focus:outline-none focus:ring-1 focus:ring-stroke-focus"
          >
            <Minimize2 size={12} />
            Done
          </button>
        </div>
      </div>

      {/* Editor Grid */}
      <div className="flex-1 grid grid-cols-2 overflow-hidden bg-surface/5">
        {/* Left: Input Textarea */}
        <div className="flex flex-col border-r border-stroke overflow-hidden h-full">
          <div className="flex items-center gap-2 border-b border-stroke px-4 py-2 bg-surface/30 shrink-0">
            <span className="font-mono text-[9px] font-bold uppercase tracking-wider text-content-muted/80">Markdown Source</span>
          </div>
          <textarea
            value={description}
            onChange={(e) => setDescription(e.target.value)}
            className="flex-1 w-full p-4 text-xs font-mono bg-transparent text-foreground placeholder-content-muted/40 outline-none resize-none overflow-y-auto scrollbar-thin leading-relaxed"
            placeholder="Write your markdown description here..."
          />
          <div className="border-t border-stroke px-4 py-2 bg-surface/30 shrink-0 flex items-center justify-between text-[10px] text-content-muted/80">
            <span>{description.length} characters</span>
            <span>{description.split(/\s+/).filter(Boolean).length} words</span>
          </div>
        </div>

        {/* Right: Markdown Live Preview */}
        <div className="flex flex-col overflow-hidden h-full">
          <div className="flex items-center gap-2 border-b border-stroke px-4 py-2 bg-surface/30 shrink-0">
            <span className="font-mono text-[9px] font-bold uppercase tracking-wider text-content-muted/80">Live Preview</span>
          </div>
          <div className="flex-1 p-4 overflow-y-auto scrollbar-thin text-xs text-foreground leading-relaxed bg-card/40">
            {description.trim() ? (
              <div className="space-y-3 font-sans break-words">
                <ReactMarkdown
                  remarkPlugins={[remarkGfm]}
                  components={{
                    h1: ({ node, ...props }) => <h1 className="text-base font-bold text-foreground border-b border-stroke pb-1 pt-2 first:mt-0" {...props} />,
                    h2: ({ node, ...props }) => <h2 className="text-sm font-bold text-foreground border-b border-stroke/50 pb-0.5 pt-2 first:mt-0" {...props} />,
                    h3: ({ node, ...props }) => <h3 className="text-xs font-bold text-foreground pt-1.5" {...props} />,
                    ul: ({ node, ...props }) => <ul className="list-disc pl-4 space-y-1 my-1" {...props} />,
                    ol: ({ node, ...props }) => <ol className="list-decimal pl-4 space-y-1 my-1" {...props} />,
                    li: ({ node, ...props }) => <li className="text-content font-medium" {...props} />,
                    p: ({ node, ...props }) => <p className="text-content leading-relaxed my-1.5 font-medium" {...props} />,
                    code: ({ node, ...props }) => <code className="bg-surface border border-stroke rounded px-1 py-0.5 font-mono text-[11px]" {...props} />,
                    pre: ({ node, ...props }) => <pre className="bg-surface border border-stroke rounded-lg p-2.5 font-mono text-[11px] overflow-x-auto my-2" {...props} />,
                    blockquote: ({ node, ...props }) => <blockquote className="border-l-2 border-brand-primary/40 pl-3 text-content-muted italic my-1.5" {...props} />,
                    table: ({ node, ...props }) => <table className="w-full border-collapse border border-stroke text-[11px] my-2" {...props} />,
                    th: ({ node, ...props }) => <th className="border border-stroke bg-surface p-1.5 text-left font-bold" {...props} />,
                    td: ({ node, ...props }) => <td className="border border-stroke p-1.5" {...props} />,
                    input: ({ node, ...props }) => <input type="checkbox" className="mr-1.5 cursor-pointer accent-brand-primary" disabled checked={props.checked} />,
                  }}
                >
                  {description}
                </ReactMarkdown>
              </div>
            ) : (
              <div className="flex h-full items-center justify-center text-content-muted/50 italic text-[11px]">
                Preview will render here as you type...
              </div>
            )}
          </div>
        </div>
      </div>
    </div>
  );
}
