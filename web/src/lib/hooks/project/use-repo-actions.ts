import { useState } from "react";
import { api, ApiError } from "@/lib/api";
import type { LinkRepositoryPayload } from "@/components/projects/repositories-view";

export function useRepoActions(projectID: string, token: string, mutateRepos: () => void) {
  const [repoError, setRepoError] = useState("");
  const [isLinkingRepository, setIsLinkingRepository] = useState(false);

  const [repoLoading, setRepoLoading] = useState<Record<string, boolean>>({});
  const [repoOperation, setRepoOperation] = useState<Record<string, "clone" | "validate" | undefined>>({});
  const [repoErrors, setRepoErrors] = useState<Record<string, string>>({});

  async function createRepository(payload: LinkRepositoryPayload) {
    if (!projectID || !token) return false;
    if (!payload.url) { setRepoError("Repository URL is required."); return false; }
    setRepoError("");
    setIsLinkingRepository(true);
    try {
      await api.createRepository(projectID, token, {
        url: payload.url, provider: payload.provider, branch: payload.branch,
        token: payload.token, git_account_id: payload.git_account_id,
      });
      mutateRepos();
      return true;
    } catch (err) {
      setRepoError(err instanceof ApiError ? err.message : "Failed to link repository");
      return false;
    } finally { setIsLinkingRepository(false); }
  }

  async function deleteRepository(repoID: string) {
    if (!token) return false;
    setRepoError("");
    try {
      await api.deleteRepository(repoID, token);
      mutateRepos();
      return true;
    } catch (err) {
      setRepoError(err instanceof ApiError ? err.message : "Failed to delete repository");
      return false;
    }
  }

  async function cloneRepository(repoID: string) {
    if (!token) return;
    setRepoLoading((prev) => ({ ...prev, [repoID]: true }));
    setRepoOperation((prev) => ({ ...prev, [repoID]: "clone" }));
    setRepoErrors((prev) => ({ ...prev, [repoID]: "" }));
    try {
      await api.cloneRepository(repoID, token);
      mutateRepos();
    } catch (err) {
      setRepoErrors((prev) => ({ ...prev, [repoID]: err instanceof ApiError ? err.message : "Failed to clone repository" }));
    } finally {
      setRepoLoading((prev) => ({ ...prev, [repoID]: false }));
      setRepoOperation((prev) => ({ ...prev, [repoID]: undefined }));
    }
  }

  async function validateRepository(repoID: string) {
    if (!token) return;
    setRepoLoading((prev) => ({ ...prev, [repoID]: true }));
    setRepoOperation((prev) => ({ ...prev, [repoID]: "validate" }));
    setRepoErrors((prev) => ({ ...prev, [repoID]: "" }));
    try {
      await api.validateRepository(repoID, token);
      mutateRepos();
    } catch (err) {
      setRepoErrors((prev) => ({ ...prev, [repoID]: err instanceof ApiError ? err.message : "Failed to validate repository" }));
    } finally {
      setRepoLoading((prev) => ({ ...prev, [repoID]: false }));
      setRepoOperation((prev) => ({ ...prev, [repoID]: undefined }));
    }
  }

  return {
    repoError,
    setRepoError,
    isLinkingRepository,
    repoLoading,
    repoOperation,
    repoErrors,
    createRepository,
    deleteRepository,
    cloneRepository,
    validateRepository,
  };
}
