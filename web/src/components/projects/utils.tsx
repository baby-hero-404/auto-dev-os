export function detectProvider(url: string) {
  const lower = url.toLowerCase();
  if (lower.includes("gitlab")) return "gitlab";
  if (lower.includes("bitbucket")) return "bitbucket";
  return "github";
}

export function RepositorySkeleton() {
  return (
    <div className="rounded-lg border border-stroke bg-card p-4">
      <div className="skeleton-shimmer h-4 w-4/5 rounded" />
      <div className="mt-3 skeleton-shimmer h-3 w-2/3 rounded" />
      <div className="mt-4 flex gap-2">
        <div className="skeleton-shimmer h-7 w-24 rounded" />
        <div className="skeleton-shimmer h-7 w-20 rounded" />
      </div>
    </div>
  );
}
