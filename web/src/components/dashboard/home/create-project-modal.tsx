import { useState } from "react";
import { toast } from "sonner";
import { api, ApiError } from "@/lib/api";
import type { GitAccount } from "@/lib/types";
import { useRouter } from "next/navigation";
import { ProjectInfoStep } from "./project-modal/ProjectInfoStep";
import { LinkRepoStep } from "./project-modal/LinkRepoStep";
import { Dialog } from "@/components/ui/dialog";

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
    <Dialog
      open={isOpen}
      onClose={handleClose}
      title={modalStep === 1 ? "Create New Project" : "Link a Repository"}
      description={`Step ${modalStep} of 2`}
      dismissable={!isSubmitting}
      size="md"
    >
      <div className="flex items-center justify-center gap-1.5 mb-4">
        <div className={`h-1.5 rounded-full transition-all duration-300 ${modalStep === 1 ? "bg-brand-primary w-4" : "bg-stroke w-1.5"}`} />
        <div className={`h-1.5 rounded-full transition-all duration-300 ${modalStep === 2 ? "bg-brand-primary w-4" : "bg-stroke w-1.5"}`} />
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
    </Dialog>
  );
}
