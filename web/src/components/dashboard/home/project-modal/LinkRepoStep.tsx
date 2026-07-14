"use client";

import { FormEvent } from "react";
import Link from "next/link";
import { ArrowLeft, RefreshCw } from "lucide-react";
import type { GitAccount } from "@/lib/types";
import { Input } from "@/components/ui/input";
import { Select } from "@/components/ui/select";
import { Field } from "@/components/ui/field";
import { Button } from "@/components/ui/button";

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
      className="mt-2 flex flex-col gap-4"
      onSubmit={onSubmit}
    >
      <Field label="Repository URL" htmlFor="repo-url" error={creationError}>
        <Input
          id="repo-url"
          value={repoURL}
          onChange={(e) => setRepoURL(e.target.value)}
          placeholder="https://github.com/org/repo.git"
          disabled={isSubmitting}
        />
      </Field>

      {gitAccounts.length === 0 ? (
        <div className="rounded-md border border-amber-500/20 bg-amber-500/10 p-3 text-xs text-amber-700 dark:text-amber-200">
          Connect a Git account before linking a repository.{" "}
          <Link className="font-semibold underline underline-offset-2 hover:text-amber-800 dark:hover:text-amber-100" href="/git-accounts">
            Connect a Git account first
          </Link>
        </div>
      ) : (
        <Field label="Git Account" htmlFor="git-account">
          <Select
            id="git-account"
            value={selectedGitAccountID}
            onChange={(e) => setSelectedGitAccountID(e.target.value)}
            disabled={isSubmitting}
          >
            <option value="">Select an account</option>
            {gitAccounts.map((account) => (
              <option key={account.id} value={account.id}>
                {account.display_name} ({account.base_url ? "GitHub Enterprise" : "GitHub"})
              </option>
            ))}
          </Select>
        </Field>
      )}

      <div className="flex gap-2">
        <Button
          type="button"
          onClick={onFetchBranches}
          disabled={isFetchingBranches || !repoURL.trim() || !selectedGitAccountID}
          variant="secondary"
          size="sm"
          isLoading={isFetchingBranches}
        >
          {!isFetchingBranches && <RefreshCw size={13} />}
          Fetch Branches
        </Button>
      </div>

      {fetchBranchesError && (
        <span className="text-xs text-danger font-medium leading-normal">{fetchBranchesError}</span>
      )}

      <Field label="Branch" htmlFor="repo-branch">
        {fetchedBranches.length > 0 ? (
          <Select
            id="repo-branch"
            value={repoBranch}
            onChange={(e) => setRepoBranch(e.target.value)}
            disabled={isSubmitting}
          >
            {fetchedBranches.map((b) => (
              <option key={b} value={b}>{b}</option>
            ))}
          </Select>
        ) : (
          <Input
            id="repo-branch"
            value={repoBranch}
            onChange={(e) => setRepoBranch(e.target.value)}
            placeholder="main"
            disabled={isSubmitting}
          />
        )}
      </Field>

      <div className="mt-2 flex flex-wrap items-center justify-between gap-3 border-t border-stroke pt-4">
        <Button
          variant="ghost"
          onClick={onBack}
          disabled={isSubmitting}
          type="button"
          size="sm"
        >
          <ArrowLeft size={16} />
          Back
        </Button>
        <div className="flex flex-wrap justify-end gap-3">
          <Button
            variant="secondary"
            onClick={onSkip}
            disabled={isSubmitting}
            type="button"
            size="sm"
          >
            Skip for now
          </Button>
          <Button
            variant="primary"
            disabled={isSubmitting || !repoURL.trim() || gitAccounts.length === 0 || !selectedGitAccountID}
            type="submit"
            size="sm"
            isLoading={isSubmitting}
          >
            Create
          </Button>
        </div>
      </div>
    </form>
  );
}
