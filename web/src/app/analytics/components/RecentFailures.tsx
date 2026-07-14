import Link from "next/link";
import { AlertTriangle } from "lucide-react";
import type { RecentFailure } from "@/lib/types";

export function RecentFailures({ recentFailures }: { recentFailures: RecentFailure[] }) {
  return (
    <section className="mb-6 overflow-hidden rounded-lg border border-stroke bg-panel">
      <div className="border-b border-stroke p-5">
        <div className="flex items-center gap-2">
          <AlertTriangle size={17} className="text-red-400" />
          <h3 className="font-mono font-semibold">Recent Failures</h3>
        </div>
        <p className="mt-1 text-sm text-content-muted">Latest failed tasks and the last workflow error recorded for each one.</p>
      </div>
      <div className="overflow-x-auto">
        <table className="w-full text-left text-sm">
          <thead className="border-b border-stroke text-xs uppercase tracking-wide text-content-muted">
            <tr>
              <th className="px-4 py-3">Task</th>
              <th className="px-4 py-3">Project</th>
              <th className="px-4 py-3">Step</th>
              <th className="px-4 py-3">Reason</th>
              <th className="px-4 py-3">Failed</th>
            </tr>
          </thead>
          <tbody>
            {(recentFailures || []).map((failure) => (
              <tr key={failure.task_id} className="border-b border-stroke/60 transition hover:bg-slate-900/50">
                <td className="px-4 py-3">
                  <Link href={`/projects/${failure.project_id}/tasks/${failure.task_id}`} className="font-medium text-white hover:text-brand-primary">
                    {failure.title}
                  </Link>
                </td>
                <td className="px-4 py-3 text-content-muted">{failure.project_name}</td>
                <td className="px-4 py-3 font-mono text-xs text-content-muted">{failure.workflow_step || "unknown"}</td>
                <td className="max-w-xl px-4 py-3 text-content-muted">
                  <span className="line-clamp-2">{failure.failure_reason || "No workflow error recorded."}</span>
                </td>
                <td className="px-4 py-3 font-mono text-xs text-content-muted">
                  {new Date(failure.failed_at).toLocaleString()}
                </td>
              </tr>
            ))}
            {(!recentFailures || recentFailures.length === 0) && (
              <tr>
                <td className="px-4 py-8 text-center text-content-muted" colSpan={5}>
                  No failed tasks recorded.
                </td>
              </tr>
            )}
          </tbody>
        </table>
      </div>
    </section>
  );
}
