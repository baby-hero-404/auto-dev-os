import { type LucideIcon } from "lucide-react";

export function StatCard({ icon: Icon, label, value, accent }: { icon: LucideIcon; label: string; value: string; accent?: boolean }) {
  return (
    <article className="group rounded-lg border border-stroke bg-panel p-4 transition hover:border-brand-primary/40">
      <div className="mb-2 grid size-8 place-items-center rounded-md bg-brand-primary/10 text-brand-primary">
        <Icon size={16} />
      </div>
      <div className={`font-mono text-xl font-semibold transition ${accent ? "text-brand-primary" : "group-hover:text-brand-primary"}`}>
        {value}
      </div>
      <div className="text-xs text-content-muted">{label}</div>
    </article>
  );
}
