"use client";

import { FormEvent, useMemo, useState } from "react";
import useSWR from "swr";
import { CheckCircle2, Eye, EyeOff, GitBranch, KeyRound, Loader2, Plus, Trash2, XCircle } from "lucide-react";
import { api, ApiError } from "@/lib/api";
import { useSession } from "@/lib/session";
import type { GitAccount } from "@/lib/types";

type ProviderOption = "github" | "gitlab";
type ActionState = "idle" | "testing" | "success" | "error";

const initialForm = {
  provider: "github" as ProviderOption,
  displayName: "My GitHub account",
  token: "",
  baseURL: "",
};

export function GitAccountsTab() {
  const session = useSession();
  const token = session?.token ?? "";
  const orgID = session?.user.org_id ?? "";

  const [isFormOpen, setIsFormOpen] = useState(false);
  const [form, setForm] = useState(initialForm);
  const [showToken, setShowToken] = useState(false);
  const [formError, setFormError] = useState("");
  const [isSubmitting, setIsSubmitting] = useState(false);
  const [testingMap, setTestingMap] = useState<Record<string, ActionState>>({});
  const [verifiedMap, setVerifiedMap] = useState<Record<string, boolean>>({});
  const [deleteConfirmID, setDeleteConfirmID] = useState<string | null>(null);

  const { data: accounts = [], error, mutate, isLoading } = useSWR(
    orgID && token ? ["git-accounts", orgID] : null,
    () => api.listGitAccounts(orgID, token),
  );

  const providerCounts = useMemo(() => {
    return accounts.reduce<Record<string, number>>((counts, account) => {
      counts[account.provider] = (counts[account.provider] || 0) + 1;
      return counts;
    }, {});
  }, [accounts]);

  function openForm(provider: ProviderOption = "github") {
    setForm({
      provider,
      displayName: provider === "github" ? "My GitHub account" : "My GitLab account",
      token: "",
      baseURL: "",
    });
    setFormError("");
    setShowToken(false);
    setIsFormOpen(true);
  }

  function closeForm() {
    if (isSubmitting) return;
    setIsFormOpen(false);
    setFormError("");
  }

  async function handleAddAccount(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    if (!token || !orgID) return;

    if (!form.displayName.trim()) {
      setFormError("Display name is required.");
      return;
    }
    if (!form.token.trim()) {
      setFormError("Token is required.");
      return;
    }

    setFormError("");
    setIsSubmitting(true);

    try {
      const created = await api.createGitAccount(orgID, token, {
        provider: form.provider,
        display_name: form.displayName.trim(),
        base_url: form.baseURL.trim() || undefined,
        token: form.token.trim(),
      });

      let verified = false;
      try {
        await api.testGitAccount(created.id, token);
        verified = true;
      } catch (err) {
        const message = err instanceof ApiError ? err.message : "invalid token or unreachable URL";
        setFormError(`Connection failed: ${message}`);
      }

      setVerifiedMap((prev) => ({ ...prev, [created.id]: verified }));
      setTestingMap((prev) => ({ ...prev, [created.id]: verified ? "success" : "error" }));
      await mutate();

      if (verified) {
        setIsFormOpen(false);
        setForm({ ...initialForm });
        setTimeout(() => {
          setTestingMap((prev) => ({ ...prev, [created.id]: "idle" }));
        }, 3000);
      }
    } catch (err) {
      const message = err instanceof ApiError ? err.message : "Failed to connect Git account.";
      setFormError(`Connection failed: ${message}`);
    } finally {
      setIsSubmitting(false);
    }
  }

  async function testConnection(accountID: string) {
    if (!token) return;
    setTestingMap((prev) => ({ ...prev, [accountID]: "testing" }));
    try {
      await api.testGitAccount(accountID, token);
      setVerifiedMap((prev) => ({ ...prev, [accountID]: true }));
      setTestingMap((prev) => ({ ...prev, [accountID]: "success" }));
      setTimeout(() => {
        setTestingMap((prev) => ({ ...prev, [accountID]: "idle" }));
      }, 3000);
    } catch {
      setVerifiedMap((prev) => ({ ...prev, [accountID]: false }));
      setTestingMap((prev) => ({ ...prev, [accountID]: "error" }));
      setTimeout(() => {
        setTestingMap((prev) => ({ ...prev, [accountID]: "idle" }));
      }, 3000);
    }
  }

  async function deleteAccount(accountID: string) {
    if (!token) return;
    try {
      await api.deleteGitAccount(accountID, token);
      setDeleteConfirmID(null);
      setVerifiedMap((prev) => {
        const next = { ...prev };
        delete next[accountID];
        return next;
      });
      await mutate();
    } catch (err) {
      setFormError(err instanceof ApiError ? err.message : "Failed to delete Git account.");
    }
  }

  return (
    <div className="space-y-6">
      <div className="grid gap-3 md:grid-cols-3">
        <MetricCard label="Connected accounts" value={accounts.length.toString()} detail="Organization-level credentials" />
        <MetricCard label="GitHub" value={(providerCounts.github || 0).toString()} detail="Cloud or Enterprise" />
        <MetricCard label="Verification" value={`${Object.values(verifiedMap).filter(Boolean).length}/${accounts.length}`} detail="Passed in this session" />
      </div>

      {error && (
        <div className="rounded-lg border border-red-500/20 bg-red-500/10 p-4 text-sm text-red-500">
          Failed to load Git accounts.
        </div>
      )}

      {isLoading ? (
        <GitAccountsSkeleton />
      ) : accounts.length === 0 && !isFormOpen ? (
        <EmptyGitAccounts onConnectGitHub={() => openForm("github")} />
      ) : (
        <div className="space-y-4">
          <div className="flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between">
            <div>
              <h3 className="font-semibold text-foreground">Connected Git Accounts</h3>
              <p className="text-sm text-content-muted">Agents use these credentials to clone, push, and open pull requests.</p>
            </div>
            {!isFormOpen && (
              <button
                onClick={() => openForm("github")}
                className="inline-flex items-center justify-center gap-2 rounded-md bg-brand-primary px-4 py-2.5 text-sm font-semibold text-white transition hover:opacity-90"
                type="button"
              >
                <Plus size={16} />
                Connect Git Account
              </button>
            )}
          </div>

          {isFormOpen && (
            <ConnectGitAccountForm
              form={form}
              formError={formError}
              isSubmitting={isSubmitting}
              showToken={showToken}
              onSubmit={handleAddAccount}
              onCancel={closeForm}
              onToggleToken={() => setShowToken((value) => !value)}
              onChange={(next) => {
                setForm(next);
                setFormError("");
              }}
            />
          )}

          {accounts.length > 0 && (
            <div className="grid gap-4 md:grid-cols-2 xl:grid-cols-3">
              {accounts.map((account) => (
                <GitAccountCard
                  key={account.id}
                  account={account}
                  isVerified={verifiedMap[account.id]}
                  testingState={testingMap[account.id] || "idle"}
                  isDeleting={deleteConfirmID === account.id}
                  onTest={() => testConnection(account.id)}
                  onAskDelete={() => setDeleteConfirmID(account.id)}
                  onCancelDelete={() => setDeleteConfirmID(null)}
                  onDelete={() => deleteAccount(account.id)}
                />
              ))}
            </div>
          )}
        </div>
      )}
    </div>
  );
}

function ConnectGitAccountForm({
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

function EmptyGitAccounts({ onConnectGitHub }: { onConnectGitHub: () => void }) {
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

function GitAccountCard({
  account,
  isVerified,
  testingState,
  isDeleting,
  onTest,
  onAskDelete,
  onCancelDelete,
  onDelete,
}: {
  account: GitAccount;
  isVerified?: boolean;
  testingState: ActionState;
  isDeleting: boolean;
  onTest: () => void;
  onAskDelete: () => void;
  onCancelDelete: () => void;
  onDelete: () => void;
}) {
  const host = account.base_url || (account.provider === "gitlab" ? "gitlab.com" : "github.com");

  return (
    <article className="glass-panel glow-on-hover rounded-lg p-4">
      <div className="flex items-start justify-between gap-3">
        <div className="flex min-w-0 items-start gap-3">
          <div className="grid size-10 shrink-0 place-items-center rounded-lg bg-brand-primary-muted text-brand-primary">
            <GitBranch size={19} />
          </div>
          <div className="min-w-0">
            <h4 className="truncate font-semibold text-foreground">{account.display_name}</h4>
            <p className="mt-1 truncate text-xs text-content-muted">
              {host} · connected {relativeTime(account.created_at)}
            </p>
          </div>
        </div>
        {isVerified === false || testingState === "error" ? (
          <span className="rounded-full border border-amber-500/20 bg-amber-500/10 px-2.5 py-1 text-xs font-semibold text-amber-600 dark:text-amber-300">
            Not verified
          </span>
        ) : (
          <span className="inline-flex items-center gap-1 rounded-full border border-emerald-500/20 bg-emerald-500/10 px-2.5 py-1 text-xs font-semibold text-emerald-600 dark:text-emerald-300">
            <span className="size-1.5 rounded-full bg-emerald-500 animate-pulse-dot" />
            Connected
          </span>
        )}
      </div>

      <div className="mt-4 flex items-center justify-between border-t border-stroke pt-4">
        {isDeleting ? (
          <div className="flex w-full items-center justify-between gap-3">
            <span className="text-xs font-semibold text-danger">Delete this account?</span>
            <div className="flex gap-2">
              <button onClick={onDelete} className="rounded bg-danger px-2.5 py-1 text-xs font-bold text-white hover:opacity-90" type="button">
                Delete
              </button>
              <button onClick={onCancelDelete} className="rounded border border-stroke px-2.5 py-1 text-xs font-bold text-foreground hover:bg-surface" type="button">
                Cancel
              </button>
            </div>
          </div>
        ) : (
          <>
            <button
              onClick={onTest}
              disabled={testingState === "testing"}
              className={`inline-flex items-center gap-1.5 rounded border px-3 py-1.5 text-xs font-bold transition disabled:cursor-not-allowed disabled:opacity-50 ${testClass(testingState)}`}
              type="button"
            >
              {testingState === "testing" && <Loader2 size={12} className="animate-spin" />}
              {testingState === "success" && <CheckCircle2 size={12} />}
              {testingState === "error" && <XCircle size={12} />}
              {testingState === "testing" ? "Testing" : testingState === "success" ? "OK" : testingState === "error" ? "Failed" : "Test"}
            </button>
            <button
              onClick={onAskDelete}
              className="rounded p-1.5 text-content-muted transition hover:bg-danger/10 hover:text-danger"
              type="button"
              title="Delete account"
            >
              <Trash2 size={15} />
            </button>
          </>
        )}
      </div>
    </article>
  );
}

function MetricCard({ label, value, detail }: { label: string; value: string; detail: string }) {
  return (
    <div className="rounded-lg border border-stroke bg-card p-4 shadow-sm">
      <p className="text-xs font-semibold uppercase tracking-wide text-content-muted">{label}</p>
      <p className="mt-2 text-2xl font-semibold text-foreground">{value}</p>
      <p className="mt-1 text-xs text-content-muted">{detail}</p>
    </div>
  );
}

function GitAccountsSkeleton() {
  return (
    <div className="grid gap-4 md:grid-cols-2 xl:grid-cols-3">
      {[0, 1, 2].map((item) => (
        <div key={item} className="rounded-lg border border-stroke bg-card p-4 shadow-sm">
          <div className="flex items-start gap-3">
            <div className="skeleton-shimmer size-10 rounded-lg" />
            <div className="flex-1 space-y-2">
              <div className="skeleton-shimmer h-5 w-36 rounded" />
              <div className="skeleton-shimmer h-3 w-48 rounded" />
            </div>
          </div>
          <div className="mt-4 border-t border-stroke pt-4">
            <div className="skeleton-shimmer h-8 w-24 rounded" />
          </div>
        </div>
      ))}
    </div>
  );
}

function testClass(state: ActionState) {
  switch (state) {
    case "success":
      return "border-emerald-500/30 bg-emerald-500/10 text-emerald-600 dark:text-emerald-300";
    case "error":
      return "border-red-500/30 bg-red-500/10 text-red-600 dark:text-red-300";
    default:
      return "border-stroke text-foreground hover:bg-surface";
  }
}

function relativeTime(value: string) {
  const timestamp = new Date(value).getTime();
  if (Number.isNaN(timestamp)) return "recently";

  const diffSeconds = Math.max(0, Math.floor((Date.now() - timestamp) / 1000));
  if (diffSeconds < 60) return "just now";

  const diffMinutes = Math.floor(diffSeconds / 60);
  if (diffMinutes < 60) return `${diffMinutes}m ago`;

  const diffHours = Math.floor(diffMinutes / 60);
  if (diffHours < 24) return `${diffHours}h ago`;

  const diffDays = Math.floor(diffHours / 24);
  return `${diffDays}d ago`;
}
