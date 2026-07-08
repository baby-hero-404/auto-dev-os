export function GitAccountsSkeleton() {
  return (
    <div className="grid gap-4 md:grid-cols-2 xl:grid-cols-3">
      {[0, 1, 2].map((item) => (
        <div key={item} className="rounded-lg border border-stroke bg-card p-4 shadow-sm">
          <div className="flex items-start gap-3">
            <div className="skeleton-shimmer size-10 rounded-lg" />
            <div className="flex-1 space-y-2">
              <div className="skeleton-shimmer h-5 w-36 rounded" />
              <div className="skeleton-shimmer h-3 w-48 rounded" />
            </div>
          </div>
          <div className="mt-4 border-t border-stroke pt-4">
            <div className="skeleton-shimmer h-8 w-24 rounded" />
          </div>
        </div>
      ))}
    </div>
  );
}
