import { TerminalSquare } from "lucide-react";
import type { RealtimeLog } from "@/lib/store/use-realtime-log-store";

interface LogConsoleProps {
  logs: RealtimeLog[];
}

export function LogConsole({ logs }: LogConsoleProps) {
  return (
    <div className="rounded-lg border border-stroke bg-panel p-5">
      <div className="mb-4 flex items-center gap-2">
        <TerminalSquare size={18} className="text-brand-primary" />
        <h2 className="font-mono text-lg font-semibold text-foreground dark:text-white">Execution Logs</h2>
      </div>
      <div className="max-h-[520px] overflow-auto rounded-md bg-slate-50 dark:bg-slate-950 p-4 font-mono text-xs border border-stroke">
        {logs.map((log) => (
          <div key={log.id} className="mb-2 grid gap-2 border-b border-stroke/50 pb-2 md:grid-cols-[150px_70px_1fr]">
            <span className="text-content-muted">{new Date(log.createdAtEpoch).toLocaleTimeString()}</span>
            <span className={log.level === "error" ? "text-red-600 dark:text-red-300" : log.level === "warn" ? "text-amber-600 dark:text-amber-300" : "text-emerald-600 dark:text-emerald-300"}>{log.level}</span>
            <span className="whitespace-pre-wrap text-slate-800 dark:text-slate-100">{log.message}</span>
          </div>
        ))}
        {logs.length === 0 && <p className="text-content-muted">No logs yet. Execute the workflow to start.</p>}
      </div>
    </div>
  );
}
