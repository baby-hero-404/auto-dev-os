import { Skeleton } from "@/components/ui/skeleton";

export function RulesSkeleton() {
  return (
    <div className="space-y-2">
      {[0, 1, 2].map((i) => (
        <div key={i} className="rounded-lg border border-stroke bg-card p-4">
          <Skeleton className="h-4 w-5/6" />
          <div className="mt-3 flex gap-2">
            <Skeleton className="h-5 w-16" />
            <Skeleton className="h-5 w-14" />
          </div>
        </div>
      ))}
    </div>
  );
}
