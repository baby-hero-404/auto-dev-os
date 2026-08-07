"use client";

/** reviewing */
export function ReviewProgressView() {
  return (
    <div className="bg-linear-to-br from-indigo-500/10 via-indigo-500/5 to-slate-500/5 border border-indigo-500/25 rounded-2xl p-5 flex items-center gap-3.5 shadow-sm relative overflow-hidden animate-fade-in">
      <div className="absolute -top-10 -right-10 w-24 h-24 bg-indigo-500/10 rounded-full blur-2xl pointer-events-none" />
      <span className="relative flex h-3.5 w-3.5 shrink-0 z-10">
        <span className="absolute inline-flex h-full w-full animate-ping rounded-full bg-indigo-500 opacity-75" />
        <span className="relative inline-flex h-3.5 w-3.5 rounded-full bg-indigo-500" />
      </span>
      <span className="text-sm font-bold text-indigo-700 dark:text-indigo-400 tracking-wide z-10">AI Review In Progress</span>
    </div>
  );
}
