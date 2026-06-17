import { useState } from "react";
import { api, ApiError } from "@/lib/api";
import type { LinkRepositoryPayload } from "@/components/projects/repositories-view";

export function useRepoActions(projectID: string, token: string, mutateRepos: () => void) {
  const [repoError, setRepoError] = useState("");
  const [isLinkingRepository, setIsLinkingRepository] = useState(false);

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

  return { repoError, setRepoError, isLinkingRepository, createRepository };
}
