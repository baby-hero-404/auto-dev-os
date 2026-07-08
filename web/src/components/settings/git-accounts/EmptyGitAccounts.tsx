import { GitBranch } from "lucide-react";

export function EmptyGitAccounts({ onConnectGitHub }: { onConnectGitHub: () => void }) {
  return (
    <div className="rounded-lg border border-dashed border-stroke bg-card p-8 text-center">
      <div className="mx-auto grid size-12 place-items-center rounded-xl bg-brand-primary-muted text-brand-primary">
        <GitBranch size={24} />
      </div>
      <h3 className="mt-4 font-semibold text-foreground">No Git accounts connected</h3>
      <p className="mx-auto mt-2 max-w-md text-sm leading-6 text-content-muted">
        Connect GitHub to let agents clone repositories, push branches, and open pull requests.
      </p>
      <div className="mt-5 flex flex-wrap justify-center gap-2">
        <button
          type="button"
          onClick={onConnectGitHub}
          className="inline-flex items-center gap-2 rounded-md bg-brand-primary px-4 py-2 text-sm font-semibold text-white transition hover:opacity-90"
        >
          <GitBranch size={16} />
          Connect GitHub
        </button>
        <button
          type="button"
          disabled
          className="inline-flex items-center gap-2 rounded-md border border-stroke px-4 py-2 text-sm font-semibold text-content-muted opacity-60"
        >
          <GitBranch size={16} />
          Connect GitLab
        </button>
      </div>
    </div>
  );
}
