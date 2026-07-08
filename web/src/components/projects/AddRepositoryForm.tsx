import { FormEvent, useMemo, useState, useEffect } from "react";
import Link from "next/link";
import { Loader2, Plus, RefreshCw } from "lucide-react";
import { api } from "@/lib/api";
import { useAuthedSWR } from "@/lib/use-authed-swr";
import { detectProvider } from "./utils";
import type { LinkRepositoryPayload } from "./repositories-view";

export function AddRepositoryForm({
  orgID,
  token,
  projectDefaultBranch,
  isLinking,
  error,
  onLinkRepository,
}: {
  orgID: string;
  token: string;
  projectDefaultBranch?: string | null;
  isLinking: boolean;
  error: string;
  onLinkRepository: (payload: LinkRepositoryPayload) => Promise<boolean>;
}) {
  const [url, setURL] = useState("");
  const [branch, setBranch] = useState(projectDefaultBranch || "main");
  const [tokenOverride, setTokenOverride] = useState("");
  const [gitAccountID, setGitAccountID] = useState("");
  const [localError, setLocalError] = useState("");

  const [fetchedBranches, setFetchedBranches] = useState<string[]>([]);
  const [isFetchingBranches, setIsFetchingBranches] = useState(false);
  const [fetchBranchesError, setFetchBranchesError] = useState("");

  useEffect(() => {
    if (projectDefaultBranch) {
      setBranch(projectDefaultBranch);
    }
  }, [projectDefaultBranch]);

  const isInputUrlValid = useMemo(() => {
    const trimmed = url.trim();
    if (!trimmed) return true;
    return trimmed.startsWith("/") || trimmed.startsWith("./") || trimmed.startsWith("../") ||
      /^(https?:\/\/|git@|ssh:\/\/|git\+ssh:\/\/)/.test(trimmed) ||
      /^[a-zA-Z0-9.-]+\.[a-zA-Z]{2,}\/[^\s]+$/.test(trimmed);
  }, [url]);

  const provider = useMemo(() => detectProvider(url), [url]);

  const { data: gitAccounts = [] } = useAuthedSWR(
    orgID ? ["git-accounts", orgID] : null,
    (t) => api.listGitAccounts(orgID, t)
  );

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

  async function handleSubmit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    setLocalError("");
    const trimmedURL = url.trim();
    if (!trimmedURL) return;

    if (!isInputUrlValid) {
      setLocalError("Please enter a valid repository URL (e.g. https://github.com/org/repo.git).");
      return;
    }

    const linked = await onLinkRepository({
      url: trimmedURL,
      provider,
      branch: branch.trim() || projectDefaultBranch || "main",
      token: tokenOverride.trim(),
      git_account_id: gitAccountID || undefined,
    });
    
    if (linked) {
      setURL("");
      setBranch("main");
      setTokenOverride("");
      setGitAccountID("");
      setFetchedBranches([]);
    }
  }

  return (
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
            className={`w-full rounded border bg-surface px-3 py-2 text-sm text-foreground focus:outline-none transition ${
              !isInputUrlValid && url.trim()
                ? "border-red-500/50 focus:border-red-500 focus:ring-1 focus:ring-red-500/20"
                : "border-stroke focus:border-brand-primary"
            }`}
            disabled={isLinking}
            required
          />
          {!isInputUrlValid && url.trim() && (
            <p className="text-[10px] text-red-500 font-medium">
              Invalid repository URL format. Use https://, git@, or local path.
            </p>
          )}
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
                {gitAccounts.map((account: any) => (
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

        {(error || localError) && (
          <p className="rounded border border-red-400/30 bg-red-950/40 p-2 text-xs text-red-200">{error || localError}</p>
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
  );
}
