import { Database, FileText } from "lucide-react";
import { EmptyState } from "../../web/src/components/ui/empty-state";

export function NoRepository() {
  return (
    <EmptyState
      icon={Database}
      title="No repository connected"
      description="Connect a valid Git repository URL and sync it. The repo root must contain registry.json or registry.min.json."
    />
  );
}

export function NoFileSelected() {
  return (
    <EmptyState
      icon={FileText}
      title="No file selected"
      description="Select a file from the explorer to inspect its content."
    />
  );
}
