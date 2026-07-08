export function Metric({ label, value }: { label: string; value: string }) {
  return (
    <article className="glass-panel glow-on-hover rounded-lg p-4">
      <div className="font-mono text-lg font-semibold text-foreground">{value}</div>
      <div className="text-xs text-content-muted">{label}</div>
    </article>
  );
}
