export function RulesSkeleton() {
  return (
    <div className="space-y-2">
      {[0, 1, 2].map((i) => (
        <div key={i} className="rounded-lg border border-stroke bg-card p-4">
          <div className="skeleton-shimmer h-4 w-5/6 rounded" />
          <div className="mt-3 flex gap-2">
            <div className="skeleton-shimmer h-5 w-16 rounded" />
            <div className="skeleton-shimmer h-5 w-14 rounded" />
          </div>
        </div>
      ))}
    </div>
  );
}
