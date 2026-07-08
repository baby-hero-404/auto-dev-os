export function ChecklistSkeleton() {
  return (
    <div className="grid gap-1">
      {[0, 1, 2, 3, 4, 5, 6, 7].map((i) => (
        <div key={i} className="flex items-center gap-3 rounded-lg px-3 py-2.5">
          <div className="skeleton-shimmer size-[18px] rounded-full" />
          <div className="skeleton-shimmer size-7 rounded-md" />
          <div className="skeleton-shimmer h-4 flex-1 rounded" />
        </div>
      ))}
    </div>
  );
}
