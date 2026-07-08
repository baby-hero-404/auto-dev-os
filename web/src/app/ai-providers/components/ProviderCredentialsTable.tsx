import { CheckCircle2, Loader2, Server, Trash2, XCircle } from "lucide-react";
import type { ProviderCredential } from "@/lib/types";

function statusClass(status: ProviderCredential["status"]) {
  switch (status) {
    case "active":
      return "bg-emerald-500/10 text-emerald-700 dark:text-emerald-300 border-emerald-500/20";
    case "rate_limited":
      return "bg-amber-500/10 text-amber-700 dark:text-amber-300 border-amber-500/20";
    case "disabled":
      return "bg-surface text-content-muted border-stroke";
  }
}

function testClass(state: "idle" | "testing" | "success" | "error") {
  switch (state) {
    case "success":
      return "border-emerald-500/30 bg-emerald-500/10 text-emerald-700 dark:text-emerald-300 hover:bg-emerald-500/20";
    case "error":
      return "border-red-500/30 bg-red-500/10 text-red-700 dark:text-red-300 hover:bg-red-500/20";
    default:
      return "border-stroke text-foreground hover:bg-surface";
  }
}

function testStatusClass(state: "success" | "error") {
  switch (state) {
    case "success":
      return "bg-emerald-500/10 text-emerald-700 dark:text-emerald-300 border-emerald-500/20";
    case "error":
      return "bg-red-500/10 text-red-700 dark:text-red-300 border-red-500/20";
  }
}

interface ProviderCredentialsTableProps {
  credentials: ProviderCredential[];
  testingMap: Record<string, "idle" | "testing" | "success" | "error">;
  deleteConfirmID: string | null;
  onTest: (credentialID: string) => void;
  onAskDelete: (credentialID: string) => void;
  onCancelDelete: () => void;
  onDelete: (credentialID: string) => void;
}

export function ProviderCredentialsTable({
  credentials,
  testingMap,
  deleteConfirmID,
  onTest,
  onAskDelete,
  onCancelDelete,
  onDelete,
}: ProviderCredentialsTableProps) {
  return (
    <div className="glass-panel overflow-hidden rounded-lg animate-fade-in">
      <div className="overflow-x-auto">
        <table className="w-full min-w-[980px] text-left text-sm">
          <thead className="border-b border-stroke bg-surface/70 text-[11px] uppercase tracking-wide text-content-muted">
            <tr>
              <th className="w-[18%] px-4 py-3">Provider</th>
              <th className="w-[20%] px-4 py-3">Label</th>
              <th className="w-[16%] px-4 py-3">API Key</th>
              <th className="w-[18%] px-4 py-3">Base URL</th>
              <th className="w-[10%] px-4 py-3">Priority</th>
              <th className="w-[10%] px-4 py-3">Status</th>
              <th className="w-[18%] px-4 py-3 text-right">Actions</th>
            </tr>
          </thead>
          <tbody>
            {credentials.map((credential) => {
              const testingState = testingMap[credential.id] || "idle";
              const isDeleting = deleteConfirmID === credential.id;

              return (
                <tr key={credential.id} className="border-t border-stroke align-middle transition-colors duration-150 hover:bg-surface/35">
                  <td className="px-4 py-4">
                    <div className="flex items-center gap-2.5">
                      <div className="grid size-8 shrink-0 place-items-center rounded-lg bg-brand-primary/10 text-brand-primary">
                        <Server size={15} />
                      </div>
                      <span className="font-semibold capitalize text-foreground">{credential.provider}</span>
                    </div>
                  </td>
                  <td className="px-4 py-4 font-medium text-foreground truncate max-w-[160px]" title={credential.label}>
                    {credential.label}
                  </td>
                  <td className="px-4 py-4 font-mono text-xs text-content-muted">
                    {credential.configured ? `•••• ${credential.key_suffix || "set"}` : "missing"}
                  </td>
                  <td className="px-4 py-4">
                    {credential.base_url ? (
                      <span className="inline-block max-w-[180px] truncate font-mono text-xs text-content-muted bg-surface/50 px-2 py-0.5 rounded border border-stroke" title={credential.base_url}>
                        {credential.base_url}
                      </span>
                    ) : (
                      <span className="text-xs text-content-muted italic">Default</span>
                    )}
                  </td>
                  <td className="px-4 py-4">
                    <span className="inline-flex items-center rounded-md bg-surface px-2 py-0.5 text-xs font-semibold text-content-muted border border-stroke">
                      P{credential.priority}
                    </span>
                  </td>
                  <td className="px-4 py-4">
                    <span className={`inline-flex items-center gap-1.5 rounded-full border px-2.5 py-0.5 text-xs font-semibold transition-all duration-150 ${
                      testingState === "success" || testingState === "error"
                        ? testStatusClass(testingState)
                        : statusClass(credential.status)
                    }`}>
                      {testingState === "testing" && <Loader2 size={11} className="animate-spin" />}
                      {testingState === "success" && <span className="size-1.5 rounded-full bg-emerald-500 dark:bg-emerald-400 animate-pulse-dot" />}
                      {testingState === "error" && <span className="size-1.5 rounded-full bg-red-500 dark:bg-red-400 animate-pulse-dot" />}
                      {testingState === "idle" && credential.status === "active" && (
                        <span className="size-1.5 rounded-full bg-emerald-500 dark:bg-emerald-400 animate-pulse-dot" />
                      )}
                      {testingState === "idle" && credential.status === "rate_limited" && (
                        <span className="size-1.5 rounded-full bg-amber-500 dark:bg-amber-400 animate-pulse-dot" />
                      )}
                      {testingState === "idle" && credential.status === "disabled" && (
                        <span className="size-1.5 rounded-full bg-content-muted" />
                      )}
                      {testingState === "testing" ? "testing" : testingState === "success" ? "success" : testingState === "error" ? "failure" : credential.status}
                    </span>
                  </td>
                  <td className="px-4 py-4">
                    {isDeleting ? (
                      <div className="flex gap-1.5 justify-end">
                        <button className="rounded bg-danger hover:opacity-90 px-2.5 py-1 text-xs font-bold text-white transition-colors cursor-pointer" onClick={() => onDelete(credential.id)} type="button">
                          Confirm
                        </button>
                        <button className="rounded border border-stroke px-2.5 py-1 text-xs text-foreground hover:bg-surface transition-colors cursor-pointer" onClick={onCancelDelete} type="button">
                          Cancel
                        </button>
                      </div>
                    ) : (
                      <div className="flex justify-end gap-2">
                        <button
                          className={`inline-flex items-center gap-1 rounded border px-2.5 py-1 text-xs font-bold transition-all duration-200 cursor-pointer ${testClass(testingState)}`}
                          disabled={testingState === "testing"}
                          onClick={() => onTest(credential.id)}
                          type="button"
                        >
                          {testingState === "testing" && <Loader2 size={12} className="animate-spin" />}
                          {testingState === "success" && <CheckCircle2 size={12} />}
                          {testingState === "error" && <XCircle size={12} />}
                          {testingState === "testing" ? "Testing" : testingState === "success" ? "OK" : testingState === "error" ? "Failed" : "Test"}
                        </button>
                        <button
                          className="rounded p-1.5 text-content-muted transition-colors duration-150 hover:bg-danger/10 hover:text-danger cursor-pointer"
                          onClick={() => onAskDelete(credential.id)}
                          title="Delete credential"
                          type="button"
                        >
                          <Trash2 size={14} />
                        </button>
                      </div>
                    )}
                  </td>
                </tr>
              );
            })}
          </tbody>
        </table>
      </div>
    </div>
  );
}
