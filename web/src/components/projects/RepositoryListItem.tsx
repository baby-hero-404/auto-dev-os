import { useState } from "react";
import { Check, Edit3, Loader2, RefreshCw, ShieldCheck, Trash2, X, AlertTriangle } from "lucide-react";
import type { Repository } from "@/lib/types";
import { api } from "@/lib/api";

interface RepositoryListItemProps {
  repo: Repository;
  token: string;
  repoLoading: boolean;
  repoOperation: "clone" | "validate" | undefined;
  repoError: string | undefined;
  onCloneRepository: (repoID: string) => void;
  onValidateRepository: (repoID: string) => void;
  onDeleteRepository: (repoID: string) => Promise<boolean>;
  onRefresh?: () => void;
}

export function RepositoryListItem({
  repo,
  token,
  repoLoading,
  repoOperation,
  repoError,
  onCloneRepository,
  onValidateRepository,
  onDeleteRepository,
  onRefresh,
}: RepositoryListItemProps) {
  const [isEditing, setIsEditing] = useState(false);
  const [isEditingName, setIsEditingName] = useState(false);
  const [confirmingDelete, setConfirmingDelete] = useState(false);
  const [editingName, setEditingName] = useState("");
  const [editingBranch, setEditingBranch] = useState("");
  const [editingBranches, setEditingBranches] = useState<string[]>([]);
  const [isFetchingEditingBranches, setIsFetchingEditingBranches] = useState(false);
  const [editingError, setEditingError] = useState("");

  const repoName = repo.display_name || repo.url.replace(/^https?:\/\//, "").replace(/\.git$/, "").split("/").pop() || "repository";

  async function startEditing() {
    setIsEditing(true);
    setEditingBranch(repo.branch || "main");
    setEditingBranches([]);
    setIsFetchingEditingBranches(true);
    setEditingError("");

    try {
      const res = await api.projects.repositories.getBranches(token, {
        url: repo.url,
        token: repo.token || undefined,
        git_account_id: repo.git_account_id || undefined,
      });
      setEditingBranches(res.branches || []);
    } catch {
      setEditingError("Failed to fetch branches for editing.");
    } finally {
      setIsFetchingEditingBranches(false);
    }
  }

  function startEditingName() {
    setIsEditingName(true);
    setEditingName(repoName);
    setEditingError("");
  }

  async function handleSaveEditedBranch() {
    if (!editingBranch) return;
    try {
      await api.projects.repositories.update(repo.id, token, { branch: editingBranch });
      setIsEditing(false);
      if (onRefresh) onRefresh();
      onValidateRepository(repo.id);
    } catch {
      setEditingError("Failed to save branch change.");
    }
  }

  async function handleSaveEditedName() {
    const trimmedName = editingName.trim();
    if (!trimmedName) {
      setEditingError("Repository name is required.");
      return;
    }
    try {
      await api.projects.repositories.update(repo.id, token, { display_name: trimmedName });
      setIsEditingName(false);
      if (onRefresh) onRefresh();
      onValidateRepository(repo.id);
    } catch {
      setEditingError("Failed to save repository name.");
    }
  }

  return (
    <div className="rounded-lg border border-stroke bg-card p-4">
      <div className="flex items-start justify-between gap-3">
        <div className="min-w-0">
          {isEditingName ? (
            <div className="flex items-center gap-2">
              <input
                value={editingName}
                onChange={(event) => setEditingName(event.target.value)}
                className="min-w-0 flex-1 rounded border border-stroke bg-surface px-3 py-1.5 text-sm font-semibold text-foreground outline-none transition focus:border-brand-primary focus:ring-2 focus:ring-brand-primary/20"
                autoFocus
              />
              <button
                type="button"
                onClick={handleSaveEditedName}
                className="rounded bg-brand-primary p-1.5 text-slate-950 hover:opacity-90 transition cursor-pointer"
                title="Save repository name"
              >
                <Check size={14} />
              </button>
              <button
                type="button"
                onClick={() => setIsEditingName(false)}
                className="rounded border border-stroke p-1.5 text-foreground hover:bg-surface transition cursor-pointer"
                title="Cancel"
              >
                <X size={14} />
              </button>
            </div>
          ) : (
            <div className="flex items-center gap-2">
              <div className="break-all text-sm font-semibold text-foreground">{repoName}</div>
              <button
                type="button"
                onClick={startEditingName}
                className="rounded p-1 text-content-muted hover:bg-surface hover:text-foreground transition cursor-pointer"
                title="Edit repository name"
              >
                <Edit3 size={11} />
              </button>
            </div>
          )}
          <div className="mt-0.5 break-all text-[11px] text-content-muted">
            {repo.url.replace(/^https?:\/\//, "")}
          </div>
        </div>
      </div>

      {isEditing ? (
        <div className="mt-3 space-y-2 border border-brand-primary/20 rounded-md p-3 bg-surface">
          <label className="block text-[10px] font-mono font-bold uppercase tracking-wider text-content-muted">
            Change Branch
          </label>
          <div className="flex gap-2">
            {isFetchingEditingBranches ? (
              <div className="flex items-center gap-2 text-xs text-content-muted py-2">
                <Loader2 size={12} className="animate-spin" /> Fetching remote branches...
              </div>
            ) : (
              <select
                value={editingBranch}
                onChange={(e) => setEditingBranch(e.target.value)}
                className="min-w-0 flex-1 rounded border border-stroke bg-card px-2 py-1 text-xs text-foreground focus:border-brand-primary focus:outline-none cursor-pointer"
              >
                {editingBranches.length === 0 ? (
                  <option value={repo.branch}>{repo.branch || "main"}</option>
                ) : (
                  editingBranches.map((b) => (
                    <option key={b} value={b}>
                      {b}
                    </option>
                  ))
                )}
              </select>
            )}
            <button
              type="button"
              onClick={handleSaveEditedBranch}
              className="rounded bg-brand-primary p-1.5 text-slate-950 hover:opacity-90 transition cursor-pointer"
              title="Save branch"
              disabled={isFetchingEditingBranches}
            >
              <Check size={14} />
            </button>
            <button
              type="button"
              onClick={() => setIsEditing(false)}
              className="rounded border border-stroke p-1.5 text-foreground hover:bg-surface transition cursor-pointer"
              title="Cancel"
            >
              <X size={14} />
            </button>
          </div>
          {editingError && <p className="text-[10px] text-red-400">{editingError}</p>}
        </div>
      ) : (
        <div className="mt-2.5 flex items-center justify-between text-xs text-content-muted">
          <span className="flex items-center gap-1.5 flex-wrap">
            <span>
              Branch: <strong className="text-foreground">{repo.branch || "main"}</strong>
            </span>
            <span className="px-1.5 opacity-50">•</span>
            <span>
              Status: <strong className="text-foreground">{repo.clone_status || "not_cloned"}</strong>
            </span>
            {!repo.last_validated_at && (
              <>
                <span className="px-1.5 opacity-50">•</span>
                <span className="inline-flex items-center gap-1 rounded bg-amber-500/10 px-1.5 py-0.5 text-[10px] font-semibold text-amber-600 dark:text-amber-400 border border-amber-500/20">
                  <AlertTriangle size={10} /> Unverified
                </span>
              </>
            )}
          </span>
          <button
            type="button"
            onClick={startEditing}
            className="text-brand-primary hover:underline flex items-center gap-1 transition"
          >
            <Edit3 size={11} /> Edit Branch
          </button>
        </div>
      )}

      <div className="mt-4 flex flex-wrap justify-between items-center gap-2">
        <div className="flex flex-wrap gap-2">
          <button
            className="inline-flex items-center gap-1.5 rounded border border-stroke bg-surface px-3 py-1.5 text-xs text-foreground transition hover:bg-surface/80 cursor-pointer disabled:opacity-50"
            onClick={() => onCloneRepository(repo.id)}
            disabled={repoLoading}
            type="button"
          >
            {repoOperation === "clone" ? <Loader2 size={13} className="animate-spin" /> : <RefreshCw size={13} />}
            {repoOperation === "clone" ? "Cloning..." : "Clone Repository"}
          </button>
          <button
            className="inline-flex items-center gap-1.5 rounded border border-stroke bg-surface px-3 py-1.5 text-xs text-foreground transition hover:bg-surface/80 cursor-pointer disabled:opacity-50"
            onClick={() => onValidateRepository(repo.id)}
            disabled={repoLoading}
            type="button"
          >
            {repoOperation === "validate" ? <Loader2 size={13} className="animate-spin" /> : <ShieldCheck size={13} />}
            {repoOperation === "validate" ? "Validating..." : "Validate"}
          </button>
        </div>

        <button
          className="inline-flex items-center gap-1.5 rounded border border-red-500/25 bg-red-500/10 px-3 py-1.5 text-xs text-red-600 hover:bg-red-500/20 transition cursor-pointer disabled:opacity-50"
          onClick={() => setConfirmingDelete(true)}
          disabled={repoLoading}
          type="button"
        >
          <Trash2 size={13} />
          Unlink
        </button>
      </div>

      {repoError && (
        <p className="mt-2 text-xs text-red-500 dark:text-red-400 font-medium">
          {repoError}
        </p>
      )}

      {confirmingDelete && (
        <div className="mt-3 rounded border border-red-500/20 bg-red-500/10 p-3">
          <p className="text-xs text-red-600 dark:text-red-400 font-medium">
            Are you sure you want to unlink this repository from the project?
          </p>
          <div className="mt-2 flex gap-2">
            <button
              onClick={async () => {
                const success = await onDeleteRepository(repo.id);
                if (success) {
                  setConfirmingDelete(false);
                }
              }}
              className="rounded bg-red-600 px-3 py-1 text-xs font-semibold text-white hover:bg-red-700 transition cursor-pointer"
              type="button"
            >
              Confirm
            </button>
            <button
              onClick={() => setConfirmingDelete(false)}
              className="rounded border border-stroke bg-surface px-3 py-1 text-xs font-semibold text-foreground hover:bg-surface/80 transition cursor-pointer"
              type="button"
            >
              Cancel
            </button>
          </div>
        </div>
      )}
    </div>
  );
}
