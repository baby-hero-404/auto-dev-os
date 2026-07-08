export function AgentCardsSkeleton() {
  return (
    <div className="grid gap-4 md:grid-cols-2 xl:grid-cols-3">
      {[0, 1, 2].map((item) => (
        <div key={item} className="glass-panel rounded-lg p-5">
          <div className="mb-3 flex items-start gap-3">
            <div className="skeleton-shimmer size-10 rounded-lg" />
            <div className="flex-1 space-y-2">
              <div className="skeleton-shimmer h-5 w-40 rounded" />
              <div className="skeleton-shimmer h-3 w-28 rounded" />
            </div>
          </div>
          <div className="skeleton-shimmer h-12 rounded" />
          <div className="mt-4 flex gap-2">
            <div className="skeleton-shimmer h-5 w-16 rounded" />
            <div className="skeleton-shimmer h-5 w-20 rounded" />
            <div className="skeleton-shimmer h-5 w-14 rounded" />
          </div>
          <div className="mt-4 border-t border-stroke pt-3">
            <div className="skeleton-shimmer h-8 rounded" />
          </div>
        </div>
      ))}
    </div>
  );
}
