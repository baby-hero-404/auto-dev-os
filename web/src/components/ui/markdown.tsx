"use client";

import ReactMarkdown from "react-markdown";
import remarkGfm from "remark-gfm";

interface MarkdownProps {
  content: string;
}

interface ComponentProps {
  children?: React.ReactNode;
  [key: string]: unknown;
}

interface CodeComponentProps {
  inline?: boolean;
  className?: string;
  children?: React.ReactNode;
  [key: string]: unknown;
}

interface InputComponentProps {
  type?: string;
  checked?: boolean;
  [key: string]: unknown;
}

export function Markdown({ content }: MarkdownProps) {
  if (!content) return null;

  return (
    <ReactMarkdown
      remarkPlugins={[remarkGfm]}
      components={{
        h1: ({ ...props }: ComponentProps) => (
          <h1 className="text-xl font-bold font-heading mt-6 mb-3 text-foreground border-b border-stroke pb-2 first:mt-0" {...props} />
        ),
        h2: ({ ...props }: ComponentProps) => (
          <h2 className="text-lg font-bold font-heading mt-5 mb-2.5 text-foreground border-b border-stroke pb-1" {...props} />
        ),
        h3: ({ ...props }: ComponentProps) => (
          <h3 className="text-sm font-bold font-sans mt-4 mb-2 text-foreground" {...props} />
        ),
        h4: ({ ...props }: ComponentProps) => (
          <h4 className="text-xs font-bold font-mono tracking-wide uppercase text-content-muted mt-3.5 mb-1.5" {...props} />
        ),
        p: ({ ...props }: ComponentProps) => (
          <p className="text-sm text-content-muted leading-relaxed mb-3.5" {...props} />
        ),
        ul: ({ ...props }: ComponentProps) => (
          <ul className="list-disc pl-5 space-y-1.5 mb-4 text-sm text-content-muted" {...props} />
        ),
        ol: ({ ...props }: ComponentProps) => (
          <ol className="list-decimal pl-5 space-y-1.5 mb-4 text-sm text-content-muted" {...props} />
        ),
        li: ({ ...props }: ComponentProps) => (
          <li className="text-sm text-content-muted" {...props} />
        ),
        code: ({ inline, children, ...props }: CodeComponentProps) => {
          if (inline) {
            return (
              <code className="bg-surface px-1.5 py-0.5 rounded text-xs font-mono text-brand-primary border border-stroke" {...props}>
                {children}
              </code>
            );
          }
          return (
            <code className="block text-slate-300 font-mono text-[11px]" {...props}>
              {children}
            </code>
          );
        },
        pre: ({ ...props }: ComponentProps) => (
          <pre className="bg-slate-950 p-4 rounded-xl overflow-auto font-mono text-xs text-slate-300 border border-stroke my-4 shadow-inner" {...props} />
        ),
        a: ({ ...props }: ComponentProps) => (
          <a className="text-brand-primary hover:underline font-semibold" target="_blank" rel="noopener noreferrer" {...props} />
        ),
        blockquote: ({ ...props }: ComponentProps) => (
          <blockquote className="border-l-4 border-brand-primary/30 pl-4 py-1 my-4 italic text-sm text-content-muted bg-surface/20 rounded-r-md" {...props} />
        ),
        table: ({ ...props }: ComponentProps) => (
          <div className="overflow-x-auto my-4 rounded-lg border border-stroke">
            <table className="w-full text-left text-sm border-collapse" {...props} />
          </div>
        ),
        thead: ({ ...props }: ComponentProps) => (
          <thead className="bg-surface/50 border-b border-stroke text-xs uppercase font-semibold text-content-muted" {...props} />
        ),
        th: ({ ...props }: ComponentProps) => (
          <th className="px-4 py-2.5 font-semibold" {...props} />
        ),
        td: ({ ...props }: ComponentProps) => (
          <td className="px-4 py-2 border-b border-stroke/50 text-content-muted" {...props} />
        ),
        input: ({ type, checked, ...props }: InputComponentProps) => {
          if (type === "checkbox") {
            return (
              <input
                type="checkbox"
                checked={checked}
                readOnly
                className="rounded border-stroke text-brand-primary focus:ring-brand-primary mr-2 size-3.5 align-middle cursor-default"
                {...props}
              />
            );
          }
          return <input type={type} checked={checked} {...props} />;
        },
      }}
    >
      {content}
    </ReactMarkdown>
  );
}
