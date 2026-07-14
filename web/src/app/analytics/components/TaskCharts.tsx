import {
  AreaChart,
  Area,
  CartesianGrid,
  Cell,
  PieChart,
  Pie,
  ResponsiveContainer,
  Tooltip,
  XAxis,
  YAxis,
} from "recharts";
import type { TaskAnalytics, TaskStatusDistribution } from "@/lib/types";
import { STATUS_COLORS } from "../utils";

export function TaskCharts({ taskAnalytics }: { taskAnalytics: TaskAnalytics | undefined }) {
  return (
    <div className="mb-6 grid gap-5 lg:grid-cols-2">
      {/* Task Throughput Chart */}
      <section className="rounded-lg border border-stroke bg-panel p-5">
        <h3 className="mb-1 font-mono font-semibold">Task Throughput</h3>
        <p className="mb-4 text-sm text-content-muted">Tasks created, completed, and failed over time.</p>
        <div className="h-64">
          <ResponsiveContainer width="100%" height="100%">
            <AreaChart data={taskAnalytics?.time_series ?? []}>
              <defs>
                <linearGradient id="gradCreated" x1="0" y1="0" x2="0" y2="1">
                  <stop offset="5%" stopColor="#60a5fa" stopOpacity={0.3} />
                  <stop offset="95%" stopColor="#60a5fa" stopOpacity={0} />
                </linearGradient>
                <linearGradient id="gradCompleted" x1="0" y1="0" x2="0" y2="1">
                  <stop offset="5%" stopColor="#22c55e" stopOpacity={0.3} />
                  <stop offset="95%" stopColor="#22c55e" stopOpacity={0} />
                </linearGradient>
              </defs>
              <CartesianGrid stroke="rgba(148, 163, 184, 0.1)" vertical={false} />
              <XAxis
                dataKey="bucket"
                stroke="#94a3b8"
                fontSize={11}
                tickFormatter={(v) => new Date(v).toLocaleDateString("en", { month: "short", day: "numeric" })}
              />
              <YAxis stroke="#94a3b8" fontSize={11} allowDecimals={false} />
              <Tooltip
                contentStyle={{ background: "#0f172a", border: "1px solid rgba(148,163,184,0.22)", borderRadius: 8, fontSize: 12 }}
                labelFormatter={(v) => new Date(v).toLocaleDateString("en", { month: "long", day: "numeric" })}
              />
              <Area type="monotone" dataKey="created" stroke="#60a5fa" fill="url(#gradCreated)" />
              <Area type="monotone" dataKey="completed" stroke="#22c55e" fill="url(#gradCompleted)" />
              <Area type="monotone" dataKey="failed" stroke="#ef4444" fillOpacity={0} />
            </AreaChart>
          </ResponsiveContainer>
        </div>
      </section>

      {/* Status Distribution Chart */}
      <section className="rounded-lg border border-stroke bg-panel p-5">
        <h3 className="mb-1 font-mono font-semibold">Task Status Distribution</h3>
        <p className="mb-4 text-sm text-content-muted">Current task breakdown by lifecycle state.</p>
        <div className="flex items-center gap-6">
          <div className="h-52 w-52 shrink-0">
            <ResponsiveContainer width="100%" height="100%">
              <PieChart>
                <Pie
                  data={taskAnalytics?.distribution ?? []}
                  dataKey="count"
                  nameKey="status"
                  cx="50%"
                  cy="50%"
                  innerRadius={45}
                  outerRadius={80}
                  paddingAngle={2}
                  strokeWidth={0}
                >
                  {(taskAnalytics?.distribution ?? []).map((entry: TaskStatusDistribution) => (
                    <Cell key={entry.status} fill={STATUS_COLORS[entry.status] ?? "#475569"} />
                  ))}
                </Pie>
                <Tooltip
                  contentStyle={{ background: "#0f172a", border: "1px solid rgba(148,163,184,0.22)", borderRadius: 8, fontSize: 12 }}
                />
              </PieChart>
            </ResponsiveContainer>
          </div>
          <div className="flex flex-wrap gap-x-4 gap-y-2 text-xs">
            {(taskAnalytics?.distribution ?? []).map((d: TaskStatusDistribution) => (
              <div key={d.status} className="flex items-center gap-2">
                <span className="block size-2.5 rounded-full" style={{ background: STATUS_COLORS[d.status] ?? "#475569" }} />
                <span className="text-content-muted">{d.status}</span>
                <span className="font-mono font-semibold">{d.count}</span>
              </div>
            ))}
          </div>
        </div>
      </section>
    </div>
  );
}
