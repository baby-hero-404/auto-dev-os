export function MetricCard({ label, value, detail }: { label: string; value: string; detail: string }) {
  return (
    <div className="rounded-lg border border-stroke bg-card p-4 shadow-sm">
      <p className="text-xs font-semibold uppercase tracking-wide text-content-muted">{label}</p>
      <p className="mt-2 text-2xl font-semibold text-foreground">{value}</p>
      <p className="mt-1 text-xs text-content-muted">{detail}</p>
    </div>
  );
}
