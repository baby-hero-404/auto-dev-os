import {
  BarChart,
  Bar,
  CartesianGrid,
  ResponsiveContainer,
  Tooltip,
  XAxis,
  YAxis,
} from "recharts";
import { compactNumber, formatDuration } from "../utils";

export function WorkflowAnalytics({ workflowAnalytics }: { workflowAnalytics: any }) {
  return (
    <section className="mb-6 rounded-lg border border-stroke bg-panel p-5">
      <div className="mb-4 flex items-center justify-between">
        <div>
          <h3 className="font-mono font-semibold">Workflow Performance</h3>
          <p className="text-sm text-content-muted">Completion rates and average step durations.</p>
        </div>
        <div className="flex gap-4 text-center text-sm">
          <div>
            <div className="font-mono text-xl font-semibold text-brand-primary">
              {Math.round(workflowAnalytics?.completion_rate ?? 0)}%
            </div>
            <div className="text-xs text-content-muted">Completion</div>
          </div>
          <div>
            <div className="font-mono text-xl font-semibold">
              {formatDuration(workflowAnalytics?.avg_duration_ms ?? 0)}
            </div>
            <div className="text-xs text-content-muted">Avg Duration</div>
          </div>
          <div>
            <div className="font-mono text-xl font-semibold">{compactNumber(workflowAnalytics?.total_workflows ?? 0)}</div>
            <div className="text-xs text-content-muted">Total Runs</div>
          </div>
        </div>
      </div>
      <div className="h-56">
        <ResponsiveContainer width="100%" height="100%">
          <BarChart data={workflowAnalytics?.step_stats ?? []} layout="vertical">
            <CartesianGrid stroke="rgba(148, 163, 184, 0.1)" horizontal={false} />
            <XAxis type="number" stroke="#94a3b8" fontSize={11} tickFormatter={(v) => `${Math.round(v / 1000)}s`} />
            <YAxis type="category" dataKey="step" stroke="#94a3b8" fontSize={11} width={100} />
            <Tooltip
              contentStyle={{ background: "#0f172a", border: "1px solid rgba(148,163,184,0.22)", borderRadius: 8, fontSize: 12 }}
              formatter={(value: unknown) => [`${Math.round(Number(value) / 1000)}s`, "Avg Duration"]}
            />
            <Bar dataKey="avg_ms" fill="var(--color-brand-primary)" radius={[0, 6, 6, 0]} />
          </BarChart>
        </ResponsiveContainer>
      </div>
    </section>
  );
}
