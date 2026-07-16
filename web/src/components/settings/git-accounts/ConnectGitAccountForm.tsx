import { FormEvent } from "react";
import { Eye, EyeOff, KeyRound, Loader2 } from "lucide-react";
import type { initialForm, ProviderOption } from "./types";

export function ConnectGitAccountForm({
  form,
  formError,
  isSubmitting,
  showToken,
  onSubmit,
  onCancel,
  onToggleToken,
  onChange,
}: {
  form: typeof initialForm;
  formError: string;
  isSubmitting: boolean;
  showToken: boolean;
  onSubmit: (event: FormEvent<HTMLFormElement>) => void;
  onCancel: () => void;
  onToggleToken: () => void;
  onChange: (form: typeof initialForm) => void;
}) {
  return (
    <form onSubmit={onSubmit} className="rounded-lg border border-stroke bg-card p-4 shadow-sm animate-fade-in">
      <div className="grid gap-4 lg:grid-cols-2">
        <label className="flex flex-col gap-1.5">
          <span className="text-xs font-semibold uppercase tracking-wide text-content-muted">Provider</span>
          <select
            value={form.provider}
            onChange={(event) => {
              const provider = event.target.value as ProviderOption;
              onChange({
                ...form,
                provider,
                displayName: provider === "github" ? "My GitHub account" : "My GitLab account",
              });
            }}
            className="rounded-md border border-stroke bg-background px-3 py-2 text-sm text-foreground transition focus:border-brand-primary focus:outline-none focus:ring-2 focus:ring-brand-primary/20"
          >
            <option value="github">GitHub</option>
            <option value="gitlab" disabled>
              GitLab (soon)
            </option>
          </select>
        </label>

        <label className="flex flex-col gap-1.5">
          <span className="text-xs font-semibold uppercase tracking-wide text-content-muted">Display Name</span>
          <input
            value={form.displayName}
            onChange={(event) => onChange({ ...form, displayName: event.target.value })}
            placeholder="My GitHub account"
            className="rounded-md border border-stroke bg-background px-3 py-2 text-sm text-foreground transition focus:border-brand-primary focus:outline-none focus:ring-2 focus:ring-brand-primary/20"
          />
        </label>

        <label className="flex flex-col gap-1.5">
          <span className="text-xs font-semibold uppercase tracking-wide text-content-muted">Token</span>
          <div className="relative">
            <KeyRound className="absolute left-3 top-2.5 text-content-muted" size={14} />
            <input
              type={showToken ? "text" : "password"}
              value={form.token}
              onChange={(event) => onChange({ ...form, token: event.target.value })}
              placeholder="ghp_xxxx"
              className="w-full rounded-md border border-stroke bg-background py-2 pl-9 pr-10 text-sm text-foreground transition focus:border-brand-primary focus:outline-none focus:ring-2 focus:ring-brand-primary/20"
            />
            <button
              type="button"
              onClick={onToggleToken}
              className="absolute right-3 top-2.5 text-content-muted transition hover:text-foreground"
              title={showToken ? "Hide token" : "Show token"}
            >
              {showToken ? <EyeOff size={14} /> : <Eye size={14} />}
            </button>
          </div>
        </label>

        <label className="flex flex-col gap-1.5">
          <span className="text-xs font-semibold uppercase tracking-wide text-content-muted">Base URL</span>
          <input
            value={form.baseURL}
            onChange={(event) => onChange({ ...form, baseURL: event.target.value })}
            placeholder="Optional, for GitHub Enterprise"
            className="rounded-md border border-stroke bg-background px-3 py-2 text-sm text-foreground transition focus:border-brand-primary focus:outline-none focus:ring-2 focus:ring-brand-primary/20"
          />
        </label>
      </div>

      <div className="mt-4 rounded-md border border-stroke/50 bg-surface/20 p-3 text-xs text-content-muted leading-relaxed">
        <p className="font-semibold text-foreground mb-1">💡 Quick Guide: Generating a GitHub Personal Access Token (classic)</p>
        <ol className="list-decimal pl-4 space-y-1">
          <li>
            Visit the{" "}
            <a
              href="https://github.com/settings/tokens"
              target="_blank"
              rel="noopener noreferrer"
              className="text-brand-primary font-medium hover:underline"
            >
              GitHub Token Settings
            </a>.
          </li>
          <li>Click <strong>Generate new token</strong> &gt; <strong>Generate new token (classic)</strong>.</li>
          <li>
            Select the <strong>repo</strong> scope (full control of private and public repositories).
          </li>
          <li>Click <strong>Generate token</strong> at the bottom and copy the generated key (starts with <code>ghp_</code>).</li>
        </ol>
      </div>

      {formError && (
        <div className="mt-4 rounded-md border border-red-500/20 bg-red-500/10 p-3 text-sm text-red-500">
          {formError}
        </div>
      )}

      <div className="mt-4 flex justify-end gap-2">
        <button
          type="button"
          onClick={onCancel}
          disabled={isSubmitting}
          className="rounded-md border border-stroke px-4 py-2 text-sm font-semibold text-foreground transition hover:bg-surface disabled:cursor-not-allowed disabled:opacity-50"
        >
          Cancel
        </button>
        <button
          type="submit"
          disabled={isSubmitting}
          className="inline-flex min-w-36 items-center justify-center gap-2 rounded-md bg-brand-primary px-4 py-2 text-sm font-semibold text-white transition hover:opacity-90 disabled:cursor-not-allowed disabled:opacity-50"
        >
          {isSubmitting && <Loader2 size={14} className="animate-spin" />}
          Connect & Test
        </button>
      </div>
    </form>
  );
}
