import { AlertTriangle, GitBranch, Loader2, RefreshCw, Trash2 } from "lucide-react";
import type { SkillSource } from "@/lib/types";
import { formatDateTime, repoNameFromURL } from "../utils";
import { StatusPill } from "./CommonUI";

interface RepositoryConnectionBarProps {
  sources: SkillSource[];
  newSourceURL: string;
  setNewSourceURL: (url: string) => void;
  isAddingSource: boolean;
  syncingSourceID: string;
  deletingSourceID: string;
  sourceUrlError: string | null;
  onAddSource: (event: React.FormEvent<HTMLFormElement>) => void;
  onSyncSource: (sourceID: string) => void;
  onDeleteSource: (sourceID: string) => void;
  onUseDefault: () => void;
}

export function RepositoryConnectionBar({
  sources,
  newSourceURL,
  setNewSourceURL,
  isAddingSource,
  syncingSourceID,
  deletingSourceID,
  sourceUrlError,
  onAddSource,
  onSyncSource,
  onDeleteSource,
  onUseDefault,
}: RepositoryConnectionBarProps) {
  if (sources.length === 0) {
    return (
      <div className="rounded-lg border border-stroke bg-card/60 backdrop-blur-sm p-4 animate-fade-in">
        <div className="flex flex-col gap-4 md:flex-row md:items-center md:justify-between">
          <div className="flex items-center gap-3">
            <div className="flex h-9 w-9 shrink-0 items-center justify-center rounded-lg bg-brand-primary/10 text-brand-primary">
              <GitBranch size={18} />
            </div>
            <div>
              <h3 className="text-sm font-semibold text-foreground">Connect Skills Repository</h3>
              <p className="text-xs text-content-muted">Connect a Git repo that contains registry.json or registry.min.json at the root.</p>
            </div>
          </div>
          <form onSubmit={onAddSource} className="flex min-w-0 max-w-xl flex-1 flex-row items-center gap-2">
            <input
              type="url"
              value={newSourceURL}
              onChange={(event) => setNewSourceURL(event.target.value)}
              placeholder="https://github.com/baby-hero-404/prompt_base.git"
              aria-invalid={Boolean(sourceUrlError)}
              className="min-w-0 flex-1 rounded-md border border-stroke bg-background px-3 py-1.5 text-xs text-foreground outline-none transition focus:border-brand-primary focus:ring-2 focus:ring-brand-primary/20"
              required
            />
            <button
              type="submit"
              disabled={isAddingSource || !newSourceURL.trim() || Boolean(sourceUrlError)}
              className="inline-flex shrink-0 items-center gap-1.5 rounded-md bg-brand-primary px-3.5 py-1.5 text-xs font-semibold text-white transition hover:opacity-90 disabled:cursor-not-allowed disabled:opacity-50 cursor-pointer"
            >
              {isAddingSource ? <Loader2 size={13} className="animate-spin" /> : <GitBranch size={13} />}
              Connect
            </button>
            <button
              type="button"
              onClick={onUseDefault}
              className="inline-flex shrink-0 items-center gap-1 rounded-md border border-stroke bg-surface/50 px-2.5 py-1.5 text-xs font-medium text-foreground transition hover:bg-surface cursor-pointer"
            >
              Use Default
            </button>
          </form>
        </div>
        <p className={`mt-2 text-[11px] ${sourceUrlError ? "text-danger" : "text-content-muted"}`}>
          {sourceUrlError || "Supported: HTTPS, SSH, or local file Git URLs. Toggle setup guide above for details."}
        </p>
      </div>
    );
  }

  return (
    <div className="rounded-lg border border-stroke bg-card/60 backdrop-blur-sm p-4">
      {sources.map((source) => {
        const isSyncing = syncingSourceID === source.id;
        const isDeleting = deletingSourceID === source.id;
        return (
          <div key={source.id}>
            <div className="flex flex-col gap-4 sm:flex-row sm:items-center sm:justify-between">
              <div className="flex items-center gap-3 min-w-0">
                <div className="flex h-9 w-9 shrink-0 items-center justify-center rounded-lg bg-success/10 text-success">
                  <GitBranch size={18} />
                </div>
                <div className="min-w-0">
                  <div className="flex items-center gap-2">
                    <span className="truncate text-sm font-semibold text-foreground">{repoNameFromURL(source.url)}</span>
                    <StatusPill status={source.status} />
                  </div>
                  <div className="mt-0.5 truncate font-mono text-[11px] text-content-muted">{source.url}</div>
                </div>
              </div>

              <div className="flex flex-wrap items-center gap-3 text-xs">
                <div className="flex items-center gap-1.5 rounded-md border border-stroke bg-surface/30 px-2.5 py-1">
                  <span className="text-[10px] uppercase tracking-wider text-content-muted">Last Synced:</span>
                  <span className="font-mono font-medium text-foreground">{formatDateTime(source.last_synced_at)}</span>
                </div>
                <div className="flex items-center gap-2">
                  <button
                    type="button"
                    onClick={() => onSyncSource(source.id)}
                    disabled={isSyncing || isDeleting}
                    className="inline-flex items-center gap-1.5 rounded-md bg-brand-primary px-3.5 py-1.5 font-semibold text-white transition hover:opacity-95 disabled:cursor-not-allowed disabled:opacity-50 cursor-pointer"
                  >
                    {isSyncing ? <Loader2 size={13} className="animate-spin" /> : <RefreshCw size={13} />}
                    Sync
                  </button>
                  <button
                    type="button"
                    onClick={() => onDeleteSource(source.id)}
                    disabled={isDeleting || isSyncing}
                    className="inline-flex items-center gap-1.5 rounded-md border border-danger/30 bg-background px-3 py-1.5 font-semibold text-danger transition hover:bg-danger/5 disabled:cursor-not-allowed disabled:opacity-50 cursor-pointer"
                  >
                    {isDeleting ? <Loader2 size={13} className="animate-spin" /> : <Trash2 size={13} />}
                    Disconnect
                  </button>
                </div>
              </div>
            </div>
            {source.error && (
              <div className="mt-3 flex flex-col gap-2 rounded-md border border-danger/20 bg-danger/5 p-3 text-xs text-danger">
                <div className="flex gap-2">
                  <AlertTriangle size={14} className="mt-0.5 shrink-0" />
                  <span className="break-words flex-1">{source.error}</span>
                </div>
                <div className="flex justify-end">
                  <button
                    type="button"
                    onClick={() => onSyncSource(source.id)}
                    disabled={isSyncing}
                    className="inline-flex items-center gap-1 rounded bg-danger/20 px-2.5 py-1 font-semibold text-danger border border-danger/30 hover:bg-danger/30 transition disabled:opacity-50 cursor-pointer"
                  >
                    {isSyncing ? <Loader2 size={11} className="animate-spin" /> : <RefreshCw size={11} />}
                    Retry Sync
                  </button>
                </div>
              </div>
            )}
          </div>
        );
      })}
    </div>
  );
}
