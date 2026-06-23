"use client";

import Link from "next/link";
import { FormEvent, useMemo, useState } from "react";
import { GitBranch, Loader2, Plus, RefreshCw, ShieldCheck, Edit3, Check, X } from "lucide-react";
import type { Repository } from "@/lib/types";
import { api } from "@/lib/api";
import { useAuthedSWR } from "@/lib/use-authed-swr";

export type LinkRepositoryPayload = {
  url: string;
  provider: string;
  branch: string;
  token: string;
  git_account_id?: string;
};

interface RepositoriesViewProps {
  projectID: string;
  project: { default_branch?: string };
  token: string;
  orgID: string;
  repositories: Repository[];
  isLoading: boolean;
  isLinking: boolean;
  error: string;
  onLinkRepository: (payload: LinkRepositoryPayload) => Promise<boolean>;
  onCloneRepository: (repoID: string) => void;
  onValidateRepository: (repoID: string) => void;
  onRefresh?: () => void;
}

export function RepositoriesView({
  project,
  token,
  orgID,
  repositories,
  isLoading,
  isLinking,
  error,
  onLinkRepository,
  onCloneRepository,
  onValidateRepository,
  onRefresh,
}: RepositoriesViewProps) {
  const [url, setURL] = useState("");
  const [branch, setBranch] = useState(project.default_branch || "main");
  const [tokenOverride, setTokenOverride] = useState("");
  const [gitAccountID, setGitAccountID] = useState("");

  // Branch fetching state
  const [fetchedBranches, setFetchedBranches] = useState<string[]>([]);
  const [isFetchingBranches, setIsFetchingBranches] = useState(false);
  const [fetchBranchesError, setFetchBranchesError] = useState("");

  // Branch editing state for existing repos
  const [editingRepoId, setEditingRepoId] = useState<string | null>(null);
  const [editingBranch, setEditingBranch] = useState("");
  const [editingBranches, setEditingBranches] = useState<string[]>([]);
  const [isFetchingEditingBranches, setIsFetchingEditingBranches] = useState(false);
  const [editingError, setEditingError] = useState("");

  const provider = useMemo(() => detectProvider(url), [url]);

  // Fetch Git Accounts
  const gitAccountsSWR = useAuthedSWR(
    orgID ? ["git-accounts", orgID] : null,
    (t) => api.listGitAccounts(orgID, t)
  );
  const gitAccounts = gitAccountsSWR.data || [];

  async function handleFetchBranches() {
    const trimmedURL = url.trim();
    if (!trimmedURL) return;

    setIsFetchingBranches(true);
    setFetchBranchesError("");
    setFetchedBranches([]);

    try {
      const res = await api.projects.repositories.getBranches(token, {
        url: trimmedURL,
        token: tokenOverride.trim() || undefined,
        git_account_id: gitAccountID || undefined,
      });
      setFetchedBranches(res.branches || []);
      if (res.branches && res.branches.length > 0) {
        const hasMain = res.branches.includes("main");
        const hasMaster = res.branches.includes("master");
        setBranch(hasMain ? "main" : (hasMaster ? "master" : res.branches[0]));
      }
    } catch (err) {
      setFetchBranchesError(err instanceof Error ? err.message : "Failed to fetch branches. Check URL/token.");
    } finally {
      setIsFetchingBranches(false);
    }
  }

  async function startEditing(repo: Repository) {
    setEditingRepoId(repo.id);
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

  async function handleSaveEditedBranch(repoId: string) {
    if (!editingBranch) return;
    try {
      await api.projects.repositories.update(repoId, token, { branch: editingBranch });
      setEditingRepoId(null);
      if (onRefresh) onRefresh();
      onValidateRepository(repoId);
    } catch {
      setEditingError("Failed to save branch change.");
    }
  }

  async function handleSubmit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    const trimmedURL = url.trim();
    if (!trimmedURL) return;

    const linked = await onLinkRepository({
      url: trimmedURL,
      provider,
      branch: branch.trim() || project?.default_branch || "main",
      token: tokenOverride.trim(),
      git_account_id: gitAccountID || undefined,
    });
    if (!linked) return;

    setURL("");
    setBranch("main");
    setTokenOverride("");
    setGitAccountID("");
    setFetchedBranches([]);
  }

  return (
    <div className="grid gap-6 lg:grid-cols-[1fr_380px]">
      {/* Repositories List */}
      <div className="space-y-4">
        <div className="flex items-center justify-between">
          <div className="flex items-center gap-2">
            <GitBranch size={18} className="text-brand-primary" />
            <h2 className="font-sans text-lg font-semibold text-foreground">Git Repositories</h2>
          </div>
          {onRefresh && (
            <button
              onClick={onRefresh}
              className="text-xs text-content-muted hover:text-foreground flex items-center gap-1 transition"
              type="button"
            >
              <RefreshCw size={12} />
              Sync status
            </button>
          )}
        </div>

        {isLoading ? (
          <RepositorySkeleton />
        ) : repositories.length === 0 ? (
          <div className="rounded-lg border border-dashed border-stroke bg-card p-6 text-center">
            <p className="font-sans text-sm font-semibold text-foreground">No repository linked.</p>
            <p className="mt-1 text-xs text-content-muted">
              Link a repository so AI agents can analyze files and run test suits.
            </p>
          </div>
        ) : (
          <div className="space-y-3">
            {repositories.map((repo) => {
              const isEditing = editingRepoId === repo.id;
              return (
                <div key={repo.id} className="rounded-lg border border-stroke bg-card p-4">
                  <div className="break-all text-sm font-semibold text-foreground">
                    {repo.url.replace(/^https?:\/\//, "")}
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
                          onClick={() => handleSaveEditedBranch(repo.id)}
                          className="rounded bg-brand-primary p-1.5 text-slate-950 hover:opacity-90 transition cursor-pointer"
                          title="Save branch"
                          disabled={isFetchingEditingBranches}
                        >
                          <Check size={14} />
                        </button>
                        <button
                          type="button"
                          onClick={() => setEditingRepoId(null)}
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
                      <span>
                        Branch: <strong className="text-foreground">{repo.branch || "main"}</strong>
                        <span className="px-1.5">•</span>
                        Status: <strong className="text-foreground">{repo.clone_status || "not_cloned"}</strong>
                      </span>
                      <button
                        type="button"
                        onClick={() => startEditing(repo)}
                        className="text-brand-primary hover:underline flex items-center gap-1 transition"
                      >
                        <Edit3 size={11} /> Edit Branch
                      </button>
                    </div>
                  )}

                  <div className="mt-4 flex flex-wrap gap-2">
                    <button
                      className="inline-flex items-center gap-1.5 rounded border border-stroke bg-surface px-3 py-1.5 text-xs text-foreground transition hover:bg-surface/80 cursor-pointer"
                      onClick={() => onCloneRepository(repo.id)}
                      type="button"
                    >
                      <RefreshCw size={13} />
                      Clone Repository
                    </button>
                    <button
                      className="inline-flex items-center gap-1.5 rounded border border-stroke bg-surface px-3 py-1.5 text-xs text-foreground transition hover:bg-surface/80 cursor-pointer"
                      onClick={() => onValidateRepository(repo.id)}
                      type="button"
                    >
                      <ShieldCheck size={13} />
                      Validate
                    </button>
                  </div>
                </div>
              );
            })}
          </div>
        )}
      </div>

      {/* Link Repository Form Card */}
      <div className="rounded-lg border border-stroke bg-card p-5 h-fit">
        <h3 className="font-sans font-semibold text-foreground mb-4">Link New Repository</h3>
        <form className="space-y-4" onSubmit={handleSubmit}>
          <div className="flex flex-col gap-1.5">
            <label className="font-mono text-[10px] font-bold uppercase tracking-wider text-content-muted">
              Repository URL
            </label>
            <input
              value={url}
              onChange={(event) => setURL(event.target.value)}
              placeholder="https://github.com/org/repo.git"
              className="w-full rounded border border-stroke bg-surface px-3 py-2 text-sm text-foreground focus:border-brand-primary focus:outline-none transition"
              disabled={isLinking}
              required
            />
          </div>

          {gitAccounts.length === 0 ? (
            <div className="text-xs text-content-muted py-1">
              No Git accounts connected.{" "}
              <Link className="font-semibold text-brand-primary hover:underline" href="/git-accounts">
                Connect account
              </Link>
            </div>
          ) : (
            <div className="flex flex-col gap-1.5">
              <label className="flex flex-col gap-1.5">
                <span className="font-mono text-[10px] font-bold uppercase tracking-wider text-content-muted">
                  Git Account
                </span>
                <select
                  value={gitAccountID}
                  onChange={(event) => setGitAccountID(event.target.value)}
                  className="w-full rounded border border-stroke bg-surface px-3 py-2 text-sm text-foreground focus:border-brand-primary focus:outline-none cursor-pointer"
                  disabled={isLinking}
                >
                  <option value="">Manual token / no account</option>
                  {gitAccounts.map((account) => (
                    <option key={account.id} value={account.id}>
                      {account.display_name}
                    </option>
                  ))}
                </select>
              </label>
            </div>
          )}

          <div className="flex flex-col gap-1.5">
            <label className="font-mono text-[10px] font-bold uppercase tracking-wider text-content-muted">
              Token Override (optional)
            </label>
            <input
              value={tokenOverride}
              onChange={(event) => setTokenOverride(event.target.value)}
              placeholder="Manual token override"
              type="password"
              className="w-full rounded border border-stroke bg-surface px-3 py-2 text-sm text-foreground focus:border-brand-primary focus:outline-none transition"
              disabled={isLinking}
            />
          </div>

          <div className="flex gap-2">
            <button
              type="button"
              onClick={handleFetchBranches}
              disabled={isFetchingBranches || !url.trim()}
              className="rounded border border-stroke bg-surface px-3 py-2 text-xs font-semibold hover:bg-surface/80 disabled:opacity-50 flex items-center gap-1.5 transition cursor-pointer"
            >
              {isFetchingBranches ? <Loader2 size={13} className="animate-spin" /> : <RefreshCw size={13} />}
              Fetch Branches
            </button>
            <div className="flex-1 rounded border border-stroke bg-surface/50 px-3 py-2 text-xs text-content-muted flex items-center justify-center">
              Provider: {provider}
            </div>
          </div>

          {fetchBranchesError && <p className="text-xs text-red-400">{fetchBranchesError}</p>}

          <label className="flex flex-col gap-1.5">
            <span className="font-mono text-[10px] font-bold uppercase tracking-wider text-content-muted">
              Target Branch
            </span>
            {fetchedBranches.length > 0 ? (
              <select
                value={branch}
                onChange={(e) => setBranch(e.target.value)}
                className="w-full rounded border border-stroke bg-surface px-3 py-2 text-sm text-foreground focus:border-brand-primary focus:outline-none cursor-pointer"
                disabled={isLinking}
              >
                {fetchedBranches.map((b) => (
                  <option key={b} value={b}>
                    {b}
                  </option>
                ))}
              </select>
            ) : (
              <input
                value={branch}
                onChange={(event) => setBranch(event.target.value)}
                placeholder="main"
                className="w-full rounded border border-stroke bg-surface px-3 py-2 text-sm text-foreground focus:border-brand-primary focus:outline-none"
                disabled={isLinking}
              />
            )}
          </label>

          {error && (
            <p className="rounded border border-red-400/30 bg-red-950/40 p-2 text-xs text-red-200">{error}</p>
          )}

          <button
            className="flex w-full items-center justify-center gap-2 rounded bg-brand-primary px-3 py-2.5 text-sm font-semibold text-slate-950 transition hover:opacity-90 disabled:opacity-50 cursor-pointer"
            type="submit"
            disabled={isLinking || !url.trim()}
          >
            {isLinking ? <Loader2 size={16} className="animate-spin" /> : <Plus size={16} />}
            Link Repository
          </button>
        </form>
      </div>
    </div>
  );
}

function RepositorySkeleton() {
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

function detectProvider(url: string) {
  const lower = url.toLowerCase();
  if (lower.includes("gitlab")) return "gitlab";
  if (lower.includes("bitbucket")) return "bitbucket";
  return "github";
}
