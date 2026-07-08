import { Loader2, type LucideIcon } from "lucide-react";

export function PanelHeader({
  icon: Icon,
  title,
  detail,
  compact = false,
}: {
  icon: LucideIcon;
  title: string;
  detail?: string;
  compact?: boolean;
}) {
  return (
    <div className={compact ? "min-w-0" : "mb-4"}>
      <div className="flex min-w-0 items-center gap-2">
        <Icon size={16} className="shrink-0 text-brand-primary" />
        <span className="truncate text-sm font-semibold text-foreground">{title}</span>
      </div>
      {detail && <div className="mt-0.5 truncate text-xs text-content-muted">{detail}</div>}
    </div>
  );
}

export function StatusPill({ status }: { status: string }) {
  const normalized = status.toLowerCase();
  const color =
    normalized === "synced"
      ? "border-success/30 bg-success/10 text-success"
      : normalized === "syncing"
        ? "border-warning/30 bg-warning/10 text-warning"
        : "border-danger/30 bg-danger/10 text-danger";

  return (
    <span className={`rounded-full border px-2 py-0.5 text-[10px] font-semibold uppercase tracking-wide ${color}`}>
      {status || "unknown"}
    </span>
  );
}

export function Detail({ label, value }: { label: string; value: string }) {
  return (
    <div className="min-w-0 rounded-md border border-stroke bg-surface/35 px-2.5 py-2">
      <div className="text-[10px] uppercase tracking-wide text-content-muted">{label}</div>
      <div className="mt-0.5 truncate font-mono text-xs font-semibold text-foreground" title={value}>
        {value}
      </div>
    </div>
  );
}

export function Tag({ children }: { children: string }) {
  return (
    <span className="rounded border border-stroke bg-background px-1.5 py-0.5 font-mono text-[10px] font-semibold uppercase tracking-wide text-content-muted">
      {children}
    </span>
  );
}

export function CategoryChip({
  active,
  children,
  onClick,
}: {
  active: boolean;
  children: string;
  onClick: () => void;
}) {
  return (
    <button
      type="button"
      onClick={onClick}
      className={`whitespace-nowrap rounded-md border px-2.5 py-1 text-xs font-semibold transition cursor-pointer ${
        active
          ? "border-brand-primary bg-brand-primary/10 text-brand-primary"
          : "border-stroke bg-background text-content-muted hover:bg-surface hover:text-foreground"
      }`}
    >
      {children}
    </button>
  );
}

export function LoadingState({ label }: { label: string }) {
  return (
    <div className="flex min-h-28 flex-col items-center justify-center gap-2 text-content-muted">
      <Loader2 size={20} className="animate-spin text-brand-primary" />
      <span className="text-xs">{label}</span>
    </div>
  );
}

export function EmptyState({
  icon: Icon,
  title,
  description,
}: {
  icon: LucideIcon;
  title: string;
  description: string;
}) {
  return (
    <div className="flex min-h-56 flex-col items-center justify-center rounded-md border border-dashed border-stroke p-6 text-center">
      <Icon size={28} className="text-content-muted/60" />
      <div className="mt-3 text-sm font-semibold text-foreground">{title}</div>
      <p className="mt-1 max-w-sm text-xs leading-relaxed text-content-muted">{description}</p>
    </div>
  );
}
