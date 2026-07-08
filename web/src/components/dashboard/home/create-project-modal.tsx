import { useState } from "react";
import { X } from "lucide-react";
import { toast } from "sonner";
import { api, ApiError } from "@/lib/api";
import type { GitAccount } from "@/lib/types";
import { useRouter } from "next/navigation";
import { ProjectInfoStep } from "./project-modal/ProjectInfoStep";
import { LinkRepoStep } from "./project-modal/LinkRepoStep";

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
          <ProjectInfoStep
            projectName={projectName}
            setProjectName={setProjectName}
            projectDescription={projectDescription}
            setProjectDescription={setProjectDescription}
            isSubmitting={isSubmitting}
            creationError={creationError}
            onNext={handleProjectInfoNext}
            onCancel={handleClose}
          />
        ) : (
          <LinkRepoStep
            repoURL={repoURL}
            setRepoURL={setRepoURL}
            gitAccounts={gitAccounts}
            selectedGitAccountID={selectedGitAccountID}
            setSelectedGitAccountID={setSelectedGitAccountID}
            isSubmitting={isSubmitting}
            isFetchingBranches={isFetchingBranches}
            onFetchBranches={handleFetchBranches}
            fetchBranchesError={fetchBranchesError}
            fetchedBranches={fetchedBranches}
            repoBranch={repoBranch}
            setRepoBranch={setRepoBranch}
            creationError={creationError}
            onBack={() => setModalStep(1)}
            onSkip={() => createProjectWithOptionalRepo(false)}
            onSubmit={(event) => {
              event.preventDefault();
              createProjectWithOptionalRepo(true);
            }}
          />
        )}
      </div>
    </div>
  );
}
