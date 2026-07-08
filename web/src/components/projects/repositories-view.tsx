"use client";

import { GitBranch, RefreshCw } from "lucide-react";
import type { Repository } from "@/lib/types";
import { RepositoryListItem } from "./RepositoryListItem";
import { AddRepositoryForm } from "./AddRepositoryForm";
import { RepositorySkeleton } from "./utils";

export type LinkRepositoryPayload = {
  url: string;
  provider: string;
  branch: string;
  token: string;
  git_account_id?: string;
};

interface RepositoriesViewProps {
  projectID: string;
  project?: { default_branch?: string } | null;
  token: string;
  orgID: string;
  repositories?: Repository[];
  isLoading: boolean;
  isLinking: boolean;
  error: string;
  repoLoading?: Record<string, boolean>;
  repoOperation?: Record<string, "clone" | "validate" | undefined>;
  repoErrors?: Record<string, string>;
  onLinkRepository: (payload: LinkRepositoryPayload) => Promise<boolean>;
  onCloneRepository: (repoID: string) => void;
  onValidateRepository: (repoID: string) => void;
  onDeleteRepository: (repoID: string) => Promise<boolean>;
  onRefresh?: () => void;
}

export function RepositoriesView({
  project,
  token,
  orgID,
  repositories = [],
  isLoading,
  isLinking,
  error,
  repoLoading = {},
  repoOperation = {},
  repoErrors = {},
  onLinkRepository,
  onCloneRepository,
  onValidateRepository,
  onDeleteRepository,
  onRefresh,
}: RepositoriesViewProps) {
  const safeRepos = Array.isArray(repositories) ? repositories : [];

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
        ) : safeRepos.length === 0 ? (
          <div className="rounded-lg border border-dashed border-stroke bg-card p-6 text-center">
            <p className="font-sans text-sm font-semibold text-foreground">No repository linked.</p>
            <p className="mt-1 text-xs text-content-muted">
              Link a repository so AI agents can analyze files and run test suites.
            </p>
          </div>
        ) : (
          <div className="space-y-3">
            {safeRepos.map((repo) => (
              <RepositoryListItem
                key={repo.id}
                repo={repo}
                token={token}
                repoLoading={repoLoading[repo.id] || false}
                repoOperation={repoOperation[repo.id]}
                repoError={repoErrors[repo.id]}
                onCloneRepository={onCloneRepository}
                onValidateRepository={onValidateRepository}
                onDeleteRepository={onDeleteRepository}
                onRefresh={onRefresh}
              />
            ))}
          </div>
        )}
      </div>

      <AddRepositoryForm
        orgID={orgID}
        token={token}
        projectDefaultBranch={project?.default_branch}
        isLinking={isLinking}
        error={error}
        onLinkRepository={onLinkRepository}
      />
    </div>
  );
}
