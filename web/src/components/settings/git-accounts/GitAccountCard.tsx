import { CheckCircle2, GitBranch, Loader2, Trash2, XCircle } from "lucide-react";
import type { GitAccount } from "@/lib/types";
import { relativeTime, testClass } from "./utils";
import type { ActionState } from "./types";

export function GitAccountCard({
  account,
  isVerified,
  testingState,
  isDeleting,
  onTest,
  onAskDelete,
  onCancelDelete,
  onDelete,
}: {
  account: GitAccount;
  isVerified?: boolean;
  testingState: ActionState;
  isDeleting: boolean;
  onTest: () => void;
  onAskDelete: () => void;
  onCancelDelete: () => void;
  onDelete: () => void;
}) {
  const host = account.base_url || (account.provider === "gitlab" ? "gitlab.com" : "github.com");

  return (
    <article className="rounded-lg border border-stroke bg-card p-4 shadow-sm">
      <div className="flex items-start justify-between gap-3">
        <div className="flex min-w-0 items-start gap-3">
          <div className="grid size-10 shrink-0 place-items-center rounded-lg bg-surface text-brand-primary border border-stroke">
            <GitBranch size={19} />
          </div>
          <div className="min-w-0">
            <h4 className="truncate font-semibold text-foreground">{account.display_name}</h4>
            <p className="mt-1 truncate text-xs text-content-muted">
              {host} · connected {relativeTime(account.created_at)}
            </p>
          </div>
        </div>
        {isVerified === false || testingState === "error" ? (
          <span className="rounded-full border border-amber-500/20 bg-amber-500/10 px-2.5 py-1 text-xs font-semibold text-amber-600 dark:text-amber-300">
            Not verified
          </span>
        ) : (
          <span className="inline-flex items-center gap-1 rounded-full border border-emerald-500/20 bg-emerald-500/10 px-2.5 py-1 text-xs font-semibold text-emerald-600 dark:text-emerald-300">
            <span className="size-1.5 rounded-full bg-emerald-500 animate-pulse-dot" />
            Connected
          </span>
        )}
      </div>

      <div className="mt-4 flex items-center justify-between border-t border-stroke pt-4">
        {isDeleting ? (
          <div className="flex w-full items-center justify-between gap-3">
            <span className="text-xs font-semibold text-danger">Delete this account?</span>
            <div className="flex gap-2">
              <button onClick={onDelete} className="rounded bg-danger px-2.5 py-1 text-xs font-bold text-white hover:opacity-90" type="button">
                Delete
              </button>
              <button onClick={onCancelDelete} className="rounded border border-stroke px-2.5 py-1 text-xs font-bold text-foreground hover:bg-surface" type="button">
                Cancel
              </button>
            </div>
          </div>
        ) : (
          <>
            <button
              onClick={onTest}
              disabled={testingState === "testing"}
              className={`inline-flex items-center gap-1.5 rounded border px-3 py-1.5 text-xs font-bold transition disabled:cursor-not-allowed disabled:opacity-50 ${testClass(testingState)}`}
              type="button"
            >
              {testingState === "testing" && <Loader2 size={12} className="animate-spin" />}
              {testingState === "success" && <CheckCircle2 size={12} />}
              {testingState === "error" && <XCircle size={12} />}
              {testingState === "testing" ? "Testing" : testingState === "success" ? "OK" : testingState === "error" ? "Failed" : "Test"}
            </button>
            <button
              onClick={onAskDelete}
              className="rounded p-1.5 text-content-muted transition hover:bg-danger/10 hover:text-danger"
              type="button"
              title="Delete account"
            >
              <Trash2 size={15} />
            </button>
          </>
        )}
      </div>
    </article>
  );
}
