"use client";

import { useEffect } from "react";
import Link from "next/link";
import { AlertTriangle, ArrowLeft, RefreshCw } from "lucide-react";

export default function ProjectDetailError({
  error,
  reset,
}: {
  error: Error & { digest?: string };
  reset: () => void;
}) {
  useEffect(() => {
    // Log the error to an error reporting service if available
    console.error("Project Detail Error:", error);
  }, [error]);

  return (
    <main className="grid min-h-screen place-items-center p-6">
      <div className="w-full max-w-md space-y-6 rounded-lg border border-red-500/20 bg-red-500/5 p-8 text-center shadow-lg">
        <div className="mx-auto flex size-12 items-center justify-center rounded-full bg-red-500/10 text-red-500">
          <AlertTriangle size={24} />
        </div>
        
        <div>
          <h2 className="font-mono text-xl font-bold text-foreground">Something went wrong</h2>
          <p className="mt-2 text-sm text-content-muted leading-relaxed">
            The project workspace encountered an unexpected error. This might be a temporary issue fetching data.
          </p>
        </div>

        {process.env.NODE_ENV === "development" && (
          <div className="rounded border border-red-500/30 bg-red-500/10 p-3 text-left">
            <p className="font-mono text-[10px] font-semibold uppercase tracking-wider text-red-400 mb-1">Developer Error Details</p>
            <p className="font-mono text-xs text-red-300 break-all">{error.message}</p>
          </div>
        )}

        <div className="flex flex-col gap-3 sm:flex-row sm:justify-center">
          <button
            onClick={() => reset()}
            className="inline-flex items-center justify-center gap-2 rounded-md bg-brand-primary px-4 py-2 font-semibold text-slate-950 transition hover:opacity-90 cursor-pointer"
          >
            <RefreshCw size={16} />
            Try again
          </button>
          <Link
            href="/"
            className="inline-flex items-center justify-center gap-2 rounded-md border border-stroke bg-slate-50 dark:bg-slate-900 px-4 py-2 font-semibold text-foreground transition hover:bg-slate-100 dark:hover:bg-slate-800"
          >
            <ArrowLeft size={16} />
            Return to Dashboard
          </Link>
        </div>
      </div>
    </main>
  );
}
