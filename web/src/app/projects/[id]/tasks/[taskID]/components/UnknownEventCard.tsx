import type { TaskEvent } from "@/lib/types/task-event";
import { EntryTimestamp } from "./TimelineEntry";

export function UnknownEventCard({
  event,
  previousTimestamp,
}: {
  event: TaskEvent;
  previousTimestamp?: string;
}) {
  return (
    <details className="rounded-md border border-stroke bg-card/50 p-2 text-xs">
      <summary className="flex cursor-pointer items-start justify-between gap-2 text-content-secondary">
        <span>
          {event.type} (schema v{event.schema_version})
        </span>
        <EntryTimestamp timestamp={event.created_at} previousTimestamp={previousTimestamp} />
      </summary>
      <pre className="mt-2 overflow-x-auto whitespace-pre-wrap text-[11px] text-content-secondary">
        {JSON.stringify(event.payload, null, 2)}
      </pre>
    </details>
  );
}
