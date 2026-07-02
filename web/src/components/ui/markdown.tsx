"use client";

import ReactMarkdown from "react-markdown";
import remarkGfm from "remark-gfm";

interface MarkdownProps {
  content: string;
}

type ComponentProps<T extends keyof React.JSX.IntrinsicElements> = React.ComponentPropsWithoutRef<T>;

type CodeComponentProps = React.ComponentPropsWithoutRef<"code"> & {
  inline?: boolean;
};

type InputComponentProps = React.ComponentPropsWithoutRef<"input">;

export function Markdown({ content }: MarkdownProps) {
  if (!content) return null;

  return (
    <ReactMarkdown
      remarkPlugins={[remarkGfm]}
      components={{
        h1: ({ ...props }: ComponentProps<"h1">) => (
          <h1 className="text-xl font-bold font-heading mt-6 mb-3 text-foreground border-b border-stroke pb-2 first:mt-0" {...props} />
        ),
        h2: ({ ...props }: ComponentProps<"h2">) => (
          <h2 className="text-lg font-bold font-heading mt-5 mb-2.5 text-foreground border-b border-stroke pb-1" {...props} />
        ),
        h3: ({ ...props }: ComponentProps<"h3">) => (
          <h3 className="text-sm font-bold font-sans mt-4 mb-2 text-foreground" {...props} />
        ),
        h4: ({ ...props }: ComponentProps<"h4">) => (
          <h4 className="text-xs font-bold font-mono tracking-wide uppercase text-content-muted mt-3.5 mb-1.5" {...props} />
        ),
        p: ({ ...props }: ComponentProps<"p">) => (
          <p className="text-sm text-content-muted leading-relaxed mb-3.5" {...props} />
        ),
        ul: ({ ...props }: ComponentProps<"ul">) => (
          <ul className="list-disc pl-5 space-y-1.5 mb-4 text-sm text-content-muted" {...props} />
        ),
        ol: ({ ...props }: ComponentProps<"ol">) => (
          <ol className="list-decimal pl-5 space-y-1.5 mb-4 text-sm text-content-muted" {...props} />
        ),
        li: ({ ...props }: ComponentProps<"li">) => (
          <li className="text-sm text-content-muted" {...props} />
        ),
        code: ({ className, children, ...props }: any) => {
          const match = /language-(\w+)/.exec(className || "");
          // If there's no language match and no newlines, it's inline code
          const isInline = !match && !String(children).includes("\n");
          
          if (isInline) {
            return (
              <code className="bg-surface px-1.5 py-0.5 rounded text-[11px] font-mono text-brand-primary border border-stroke/80 mx-0.5" {...props}>
                {children}
              </code>
            );
          }
          return (
            <code className={`${className || ""} block text-slate-300 font-mono text-[11px]`} {...props}>
              {children}
            </code>
          );
        },
        pre: ({ ...props }: ComponentProps<"pre">) => (
          <pre className="bg-slate-950 p-4 rounded-xl overflow-auto font-mono text-xs text-slate-300 border border-stroke my-4 shadow-inner" {...props} />
        ),
        a: ({ ...props }: ComponentProps<"a">) => (
          <a className="text-brand-primary hover:underline font-semibold" target="_blank" rel="noopener noreferrer" {...props} />
        ),
        blockquote: ({ ...props }: ComponentProps<"blockquote">) => (
          <blockquote className="border-l-4 border-brand-primary/30 pl-4 py-1 my-4 italic text-sm text-content-muted bg-surface/20 rounded-r-md" {...props} />
        ),
        table: ({ ...props }: ComponentProps<"table">) => (
          <div className="overflow-x-auto my-4 rounded-lg border border-stroke">
            <table className="w-full text-left text-sm border-collapse" {...props} />
          </div>
        ),
        thead: ({ ...props }: ComponentProps<"thead">) => (
          <thead className="bg-surface/50 border-b border-stroke text-xs uppercase font-semibold text-content-muted" {...props} />
        ),
        th: ({ ...props }: ComponentProps<"th">) => (
          <th className="px-4 py-2.5 font-semibold" {...props} />
        ),
        td: ({ ...props }: ComponentProps<"td">) => (
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
