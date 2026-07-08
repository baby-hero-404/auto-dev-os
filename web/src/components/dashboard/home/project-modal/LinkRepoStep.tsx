"use client";

import { FormEvent } from "react";
import Link from "next/link";
import { ArrowLeft, Loader2, RefreshCw } from "lucide-react";
import type { GitAccount } from "@/lib/types";

interface LinkRepoStepProps {
  repoURL: string;
  setRepoURL: (val: string) => void;
  gitAccounts: GitAccount[];
  selectedGitAccountID: string;
  setSelectedGitAccountID: (val: string) => void;
  isSubmitting: boolean;
  isFetchingBranches: boolean;
  onFetchBranches: () => void;
  fetchBranchesError: string;
  fetchedBranches: string[];
  repoBranch: string;
  setRepoBranch: (val: string) => void;
  creationError: string;
  onBack: () => void;
  onSkip: () => void;
  onSubmit: (e: FormEvent) => void;
}

export function LinkRepoStep({
  repoURL,
  setRepoURL,
  gitAccounts,
  selectedGitAccountID,
  setSelectedGitAccountID,
  isSubmitting,
  isFetchingBranches,
  onFetchBranches,
  fetchBranchesError,
  fetchedBranches,
  repoBranch,
  setRepoBranch,
  creationError,
  onBack,
  onSkip,
  onSubmit,
}: LinkRepoStepProps) {
  return (
    <form
      className="mt-4 flex flex-col gap-4"
      onSubmit={onSubmit}
    >
      <div className="flex flex-col gap-1.5">
        <label className="text-xs font-semibold uppercase tracking-wider text-content-muted">Repository URL</label>
        <input
          value={repoURL}
          onChange={(e) => setRepoURL(e.target.value)}
          placeholder="https://github.com/org/repo.git"
          className="rounded-md border border-stroke bg-background px-3 py-2 text-sm text-foreground focus:outline-none focus:border-brand-primary transition"
          disabled={isSubmitting}
        />
      </div>

      {gitAccounts.length === 0 ? (
        <div className="rounded-md border border-amber-500/20 bg-amber-500/10 p-3 text-sm text-amber-700 dark:text-amber-200">
          Connect a Git account before linking a repository.{" "}
          <Link className="font-semibold underline underline-offset-2" href="/git-accounts">
            Connect a Git account first
          </Link>
        </div>
      ) : (
        <div className="flex flex-col gap-1.5">
          <label className="text-xs font-semibold uppercase tracking-wider text-content-muted">Git Account</label>
          <select
            value={selectedGitAccountID}
            onChange={(e) => setSelectedGitAccountID(e.target.value)}
            className="rounded-md border border-stroke bg-background px-3 py-2 text-sm text-foreground focus:outline-none focus:border-brand-primary transition"
            disabled={isSubmitting}
          >
            <option value="">Select an account</option>
            {gitAccounts.map((account) => (
              <option key={account.id} value={account.id}>
                {account.display_name} ({account.base_url ? "GitHub Enterprise" : "GitHub"})
              </option>
            ))}
          </select>
        </div>
      )}

      <div className="flex gap-2">
        <button
          type="button"
          onClick={onFetchBranches}
          disabled={isFetchingBranches || !repoURL.trim() || !selectedGitAccountID}
          className="rounded border border-stroke bg-slate-100 dark:bg-slate-900 px-3 py-2 text-xs font-semibold hover:bg-slate-200 dark:hover:bg-slate-800 disabled:opacity-50 flex items-center gap-1.5 transition cursor-pointer"
        >
          {isFetchingBranches ? <Loader2 size={13} className="animate-spin" /> : <RefreshCw size={13} />}
          Fetch Branches
        </button>
      </div>

      {fetchBranchesError && (
        <p className="text-xs text-red-400">{fetchBranchesError}</p>
      )}

      <div className="flex flex-col gap-1.5">
        <label className="text-xs font-semibold uppercase tracking-wider text-content-muted">Branch</label>
        {fetchedBranches.length > 0 ? (
          <select
            value={repoBranch}
            onChange={(e) => setRepoBranch(e.target.value)}
            className="rounded-md border border-stroke bg-background px-3 py-2 text-sm text-foreground focus:outline-none focus:border-brand-primary transition cursor-pointer"
            disabled={isSubmitting}
          >
            {fetchedBranches.map((b) => (
              <option key={b} value={b}>{b}</option>
            ))}
          </select>
        ) : (
          <input
            value={repoBranch}
            onChange={(e) => setRepoBranch(e.target.value)}
            placeholder="main"
            className="rounded-md border border-stroke bg-background px-3 py-2 text-sm text-foreground focus:outline-none focus:border-brand-primary transition"
            disabled={isSubmitting}
          />
        )}
      </div>

      {creationError && (
        <p className="rounded-md border border-red-500/20 bg-red-950/40 p-3 text-xs text-red-200">
          {creationError}
        </p>
      )}

      <div className="mt-2 flex flex-wrap items-center justify-between gap-3 border-t border-stroke pt-4">
        <button
          onClick={onBack}
          className="inline-flex items-center gap-2 rounded-md border border-stroke bg-transparent px-4 py-2 text-sm font-semibold text-foreground transition hover:bg-surface cursor-pointer disabled:opacity-50"
          disabled={isSubmitting}
          type="button"
        >
          <ArrowLeft size={16} />
          Back
        </button>
        <div className="flex flex-wrap justify-end gap-3">
          <button
            onClick={onSkip}
            className="rounded-md border border-stroke bg-transparent px-4 py-2 text-sm font-semibold text-foreground transition hover:bg-surface cursor-pointer disabled:opacity-50"
            disabled={isSubmitting}
            type="button"
          >
            Skip for now
          </button>
          <button
            className="flex items-center gap-2 rounded-md bg-brand-primary px-4 py-2 text-sm font-semibold text-white transition hover:opacity-90 cursor-pointer disabled:opacity-50"
            disabled={isSubmitting || !repoURL.trim() || gitAccounts.length === 0 || !selectedGitAccountID}
            type="submit"
          >
            {isSubmitting ? (
              <>
                <Loader2 size={16} className="animate-spin" />
                Creating...
              </>
            ) : (
              "Create"
            )}
          </button>
        </div>
      </div>
    </form>
  );
}
