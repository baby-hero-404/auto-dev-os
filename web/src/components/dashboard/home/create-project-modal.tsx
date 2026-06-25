import { useState } from "react";
import Link from "next/link";
import { ArrowLeft, Loader2, RefreshCw, X } from "lucide-react";
import { toast } from "sonner";
import { api, ApiError } from "@/lib/api";
import type { GitAccount } from "@/lib/types";
import { useRouter } from "next/navigation";

interface CreateProjectModalProps {
  isOpen: boolean;
  onClose: () => void;
  gitAccounts: GitAccount[];
  token: string;
  orgID: string;
  onProjectCreated: () => Promise<void>;
}

export function CreateProjectModal({
  isOpen,
  onClose,
  gitAccounts,
  token,
  orgID,
  onProjectCreated,
}: CreateProjectModalProps) {
  const router = useRouter();
  const [modalStep, setModalStep] = useState<1 | 2>(1);
  const [projectName, setProjectName] = useState("");
  const [projectDescription, setProjectDescription] = useState("");
  const [repoURL, setRepoURL] = useState("");
  const [repoBranch, setRepoBranch] = useState("main");
  const [selectedGitAccountID, setSelectedGitAccountID] = useState("");
  const [creationError, setCreationError] = useState("");
  const [isSubmitting, setIsSubmitting] = useState(false);
  const [fetchedBranches, setFetchedBranches] = useState<string[]>([]);
  const [isFetchingBranches, setIsFetchingBranches] = useState(false);
  const [fetchBranchesError, setFetchBranchesError] = useState("");

  if (!isOpen) return null;

  function resetProjectModal() {
    setModalStep(1);
    setProjectName("");
    setProjectDescription("");
    setRepoURL("");
    setRepoBranch("main");
    setCreationError("");
    setFetchedBranches([]);
    setFetchBranchesError("");
    setSelectedGitAccountID("");
  }

  function handleClose() {
    resetProjectModal();
    onClose();
  }

  function handleProjectInfoNext(e: React.FormEvent) {
    e.preventDefault();
    const trimmed = projectName.trim();
    if (!trimmed) {
      setCreationError("Project name is required.");
      return;
    }
    setCreationError("");
    setModalStep(2);
  }

  async function handleFetchBranches() {
    if (!repoURL.trim() || !selectedGitAccountID) {
      setFetchBranchesError("Repository URL and Git Account required to fetch branches");
      return;
    }
    setIsFetchingBranches(true);
    setFetchBranchesError("");
    try {
      const res = await api.getRemoteBranches(token, {
        url: repoURL.trim(),
        git_account_id: selectedGitAccountID || undefined,
      });
      const branches = res.branches || [];
      setFetchedBranches(branches);
      if (branches.length > 0 && !branches.includes(repoBranch)) {
        setRepoBranch(branches[0]);
      }
    } catch {
      setFetchBranchesError("Failed to fetch branches. Check URL/permissions.");
    } finally {
      setIsFetchingBranches(false);
    }
  }

  async function createProjectWithOptionalRepo(linkRepository: boolean) {
    const trimmedName = projectName.trim();
    if (!trimmedName) {
      setCreationError("Project name is required.");
      setModalStep(1);
      return;
    }
    if (linkRepository && repoURL.trim() && gitAccounts.length === 0) {
      setCreationError("Connect a Git account before linking a repository.");
      return;
    }
    if (linkRepository && repoURL.trim() && !selectedGitAccountID) {
      setCreationError("Select a Git account to link this repository.");
      return;
    }

    if (linkRepository && repoURL.trim()) {
      const urlVal = repoURL.trim();
      const isValid = urlVal.startsWith("/") || urlVal.startsWith("./") || urlVal.startsWith("../") ||
        /^(https?:\/\/|git@|ssh:\/\/|git\+ssh:\/\/)/.test(urlVal) ||
        /^[a-zA-Z0-9.-]+\.[a-zA-Z]{2,}\/[^\s]+$/.test(urlVal);
      if (!isValid) {
        setCreationError("Please enter a valid repository URL (e.g. https://github.com/org/repo.git).");
        return;
      }
    }

    setCreationError("");
    setIsSubmitting(true);
    try {
      const createdProject = await api.createProject(orgID, token, {
        name: trimmedName,
        description: projectDescription.trim(),
      });

      if (linkRepository && repoURL.trim()) {
        try {
          await api.createRepository(createdProject.id, token, {
            url: repoURL.trim(),
            provider: "github",
            branch: repoBranch.trim() || "main",
            git_account_id: selectedGitAccountID || undefined,
          });
        } catch {
          toast.error("Project created, but repo could not be linked. You can add it later from the project page.");
        }
      }

      handleClose();
      await onProjectCreated();
      router.push(`/projects/${createdProject.id}`);
    } catch (err) {
      setCreationError(err instanceof ApiError ? err.message : "Failed to create project");
    } finally {
      setIsSubmitting(false);
    }
  }

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center p-4">
      <div
        className="absolute inset-0 bg-slate-950/80 backdrop-blur-sm transition-opacity duration-300"
        onClick={!isSubmitting ? handleClose : undefined}
      />

      <div className="relative w-full max-w-md transform overflow-hidden rounded-xl border border-stroke bg-card p-6 shadow-2xl transition-all duration-300 animate-modal-in">
        <div className="flex items-center justify-between border-b border-stroke pb-4">
          <div>
            <h3 className="text-lg font-semibold text-foreground">
              {modalStep === 1 ? "Create New Project" : "Link a Repository"}
            </h3>
            <p className="mt-1 text-xs text-content-muted">Step {modalStep} of 2</p>
          </div>
          <button
            onClick={handleClose}
            className="rounded-md p-1 text-content-muted transition hover:bg-surface hover:text-foreground cursor-pointer"
            disabled={isSubmitting}
            type="button"
          >
            <X size={18} />
          </button>
        </div>

        {modalStep === 1 ? (
          <form className="mt-4 flex flex-col gap-4" onSubmit={handleProjectInfoNext}>
            <div className="flex flex-col gap-1.5">
              <label className="text-xs font-semibold uppercase tracking-wider text-content-muted">
                Name <span className="text-brand-primary">*</span>
              </label>
              <input
                value={projectName}
                onChange={(e) => setProjectName(e.target.value)}
                placeholder="e.g. api-backend"
                className="rounded-md border border-stroke bg-background px-3 py-2 text-sm text-foreground focus:outline-none focus:border-brand-primary transition"
                disabled={isSubmitting}
                required
                autoFocus
              />
            </div>

            <div className="flex flex-col gap-1.5">
              <label className="text-xs font-semibold uppercase tracking-wider text-content-muted">
                Description
              </label>
              <textarea
                value={projectDescription}
                onChange={(e) => setProjectDescription(e.target.value)}
                placeholder="Optional goal, scope, or repository context."
                className="min-h-[100px] rounded-md border border-stroke bg-background px-3 py-2 text-sm text-foreground focus:outline-none focus:border-brand-primary transition resize-none"
                disabled={isSubmitting}
              />
            </div>

            {creationError && (
              <p className="rounded-md border border-red-500/20 bg-red-950/40 p-3 text-xs text-red-200">
                {creationError}
              </p>
            )}

            <div className="mt-2 flex items-center justify-end gap-3 border-t border-stroke pt-4">
              <button
                onClick={handleClose}
                className="rounded-md border border-stroke bg-transparent px-4 py-2 text-sm font-semibold text-foreground transition hover:bg-surface cursor-pointer disabled:opacity-50"
                disabled={isSubmitting}
                type="button"
              >
                Cancel
              </button>
              <button
                className="flex items-center gap-2 rounded-md bg-brand-primary px-4 py-2 text-sm font-semibold text-white transition hover:opacity-90 cursor-pointer disabled:opacity-50"
                disabled={isSubmitting}
                type="submit"
              >
                Next
                <ArrowLeft size={16} className="rotate-180" />
              </button>
            </div>
          </form>
        ) : (
          <form
            className="mt-4 flex flex-col gap-4"
            onSubmit={(event) => {
              event.preventDefault();
              createProjectWithOptionalRepo(true);
            }}
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
                onClick={handleFetchBranches}
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
                onClick={() => setModalStep(1)}
                className="inline-flex items-center gap-2 rounded-md border border-stroke bg-transparent px-4 py-2 text-sm font-semibold text-foreground transition hover:bg-surface cursor-pointer disabled:opacity-50"
                disabled={isSubmitting}
                type="button"
              >
                <ArrowLeft size={16} />
                Back
              </button>
              <div className="flex flex-wrap justify-end gap-3">
                <button
                  onClick={() => createProjectWithOptionalRepo(false)}
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
        )}
      </div>
    </div>
  );
}
