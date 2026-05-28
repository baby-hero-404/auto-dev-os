"use client";

export function InfoBlock({ title, items }: { title: string; items: string[] }) {
  return (
    <div className="rounded-md border border-[var(--border)] bg-[var(--background)] p-3">
      <div className="mb-2 text-xs font-semibold uppercase tracking-wider text-[var(--muted)]">{title}</div>
      {items.length ? (
        <ul className="space-y-1 text-sm text-slate-200">
          {items.map((item, index) => (
            <li key={`${item}-${index}`} className="flex items-start gap-2">
              <span className="mt-1.5 block size-1.5 shrink-0 rounded-full bg-[var(--accent)]" />
              {item}
            </li>
          ))}
        </ul>
      ) : (
        <p className="text-sm text-[var(--muted)]">None</p>
      )}
    </div>
  );
}
