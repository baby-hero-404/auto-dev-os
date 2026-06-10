"use client";

import { FormEvent, useState } from "react";
import useSWR from "swr";
import { GitBranch, KeyRound, Loader2, Plus, Trash2, X } from "lucide-react";
import { api, ApiError } from "@/lib/api";
import { useSession } from "@/lib/session";
import { EmptyState } from "@/components/ui/empty-state";

export function GitAccountsPanel() {
  const session = useSession();
  const token = session?.token ?? "";
  const orgID = session?.user.org_id ?? "";

  const [isAddModalOpen, setIsAddModalOpen] = useState(false);
  const [displayName, setDisplayName] = useState("");
  const [provider, setProvider] = useState("github");
  const [baseURL, setBaseURL] = useState("");
  const [accToken, setAccToken] = useState("");
  const [formError, setFormError] = useState("");
  const [isSubmitting, setIsSubmitting] = useState(false);
  const [testingMap, setTestingMap] = useState<Record<string, "idle" | "testing" | "success" | "error">>({});
  const [deleteConfirmID, setDeleteConfirmID] = useState<string | null>(null);

  const { data: accounts = [], error, mutate } = useSWR(
    orgID && token ? ["git-accounts", orgID] : null,
    () => api.listGitAccounts(orgID, token)
  );

  async function handleAddAccount(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    if (!token || !orgID) return;

    if (!displayName.trim()) {
      setFormError("Display name is required.");
      return;
    }
    if (!accToken.trim()) {
      setFormError("Token is required.");
      return;
    }

    setFormError("");
    setIsSubmitting(true);

    try {
      await api.createGitAccount(orgID, token, {
        provider,
        display_name: displayName.trim(),
        base_url: baseURL.trim() || undefined,
        token: accToken.trim(),
      });
      setIsAddModalOpen(false);
      setDisplayName("");
      setBaseURL("");
      setAccToken("");
      mutate();
    } catch (err) {
      setFormError(err instanceof ApiError ? err.message : "Failed to add git account");
    } finally {
      setIsSubmitting(false);
    }
  }

  async function testConnection(accID: string) {
    if (!token) return;
    setTestingMap((prev) => ({ ...prev, [accID]: "testing" }));
    try {
      await api.testGitAccount(accID, token);
      setTestingMap((prev) => ({ ...prev, [accID]: "success" }));
      setTimeout(() => {
        setTestingMap((prev) => ({ ...prev, [accID]: "idle" }));
      }, 3000);
    } catch {
      setTestingMap((prev) => ({ ...prev, [accID]: "error" }));
    }
  }

  async function deleteAccount(accID: string) {
    if (!token) return;
    try {
      await api.deleteGitAccount(accID, token);
      setDeleteConfirmID(null);
      mutate();
    } catch (err) {
      console.error(err);
    }
  }

  return (
    <div className="space-y-6">
      <div className="flex items-center justify-between">
        <div>
          <h3 className="font-mono text-lg font-semibold text-white">Linked Git Accounts</h3>
          <p className="text-sm text-content-muted">
            Manage organization-level GitHub credentials for repository automation.
          </p>
        </div>
        <button
          onClick={() => setIsAddModalOpen(true)}
          className="inline-flex items-center gap-1.5 rounded-md bg-brand-primary px-3.5 py-2 font-mono text-xs font-bold uppercase tracking-wider text-black transition hover:bg-brand-primary/90"
          type="button"
        >
          <Plus size={14} />
          Add Account
        </button>
      </div>

      {error ? (
        <div className="rounded-lg border border-red-500/20 bg-red-500/10 p-4 text-sm text-red-400">
          Failed to load Git accounts.
        </div>
      ) : accounts.length === 0 ? (
        <div className="space-y-4">
          <EmptyState
            title="No Git accounts linked"
            description="Link a GitHub or GitHub Enterprise account to clone repositories, push commits, and open pull requests."
            icon={GitBranch}
          />
          <div className="flex justify-center">
            <button
              onClick={() => setIsAddModalOpen(true)}
              className="inline-flex items-center gap-1.5 rounded-md border border-stroke px-4 py-2 text-sm font-semibold text-white hover:bg-slate-900"
              type="button"
            >
              Add first account
            </button>
          </div>
        </div>
      ) : (
        <div className="grid gap-4 md:grid-cols-2 lg:grid-cols-3">
          {accounts.map((acc) => {
            const testingState = testingMap[acc.id] || "idle";
            const isDeleting = deleteConfirmID === acc.id;

            return (
              <article
                key={acc.id}
                className="glass-panel glow-on-hover relative flex flex-col justify-between rounded-lg p-5"
              >
                <div>
                  <div className="mb-4 flex items-center justify-between gap-3">
                    <div className="flex items-center gap-2.5">
                      <div className="grid size-9 place-items-center rounded-lg bg-brand-primary/10 text-brand-primary">
                        <GitBranch size={18} />
                      </div>
                      <div>
                        <h4 className="font-mono font-semibold text-white">{acc.display_name}</h4>
                        <p className="text-xs text-content-muted capitalize">{acc.provider}</p>
                      </div>
                    </div>
                  </div>
                  {acc.base_url && (
                    <div className="mb-4 rounded bg-page px-2 py-1 font-mono text-xs text-content-muted truncate">
                      {acc.base_url}
                    </div>
                  )}
                </div>

                <div className="mt-4 flex items-center justify-between border-t border-stroke pt-4">
                  {isDeleting ? (
                    <div className="flex w-full flex-col gap-2">
                      <span className="text-xs font-semibold text-rose-400">Confirm deletion?</span>
                      <div className="flex gap-2">
                        <button
                          onClick={() => deleteAccount(acc.id)}
                          className="flex-1 rounded bg-rose-500 py-1 text-xs font-bold text-white hover:bg-rose-600"
                          type="button"
                        >
                          Yes, Delete
                        </button>
                        <button
                          onClick={() => setDeleteConfirmID(null)}
                          className="flex-1 rounded border border-stroke py-1 text-xs font-bold text-white hover:bg-slate-800"
                          type="button"
                        >
                          Cancel
                        </button>
                      </div>
                    </div>
                  ) : (
                    <>
                      <div className="flex gap-2">
                        <button
                          onClick={() => testConnection(acc.id)}
                          disabled={testingState === "testing"}
                          className={`rounded px-3 py-1.5 font-mono text-xs font-bold uppercase tracking-wider border transition ${
                            testingState === "success"
                              ? "border-emerald-500/20 bg-emerald-500/10 text-emerald-400"
                              : testingState === "error"
                              ? "border-red-500/20 bg-red-500/10 text-red-400"
                              : "border-stroke text-white hover:bg-slate-900"
                          }`}
                          type="button"
                        >
                          {testingState === "testing" ? (
                            <span className="flex items-center gap-1">
                              <Loader2 className="animate-spin" size={12} />
                              Testing
                            </span>
                          ) : testingState === "success" ? (
                            "Success"
                          ) : testingState === "error" ? (
                            "Failed"
                          ) : (
                            "Test"
                          )}
                        </button>
                      </div>
                      <button
                        onClick={() => setDeleteConfirmID(acc.id)}
                        className="rounded p-1.5 text-slate-500 hover:bg-rose-500/10 hover:text-rose-400 transition"
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
          })}
        </div>
      )}

      {isAddModalOpen && (
        <div className="fixed inset-0 z-50 flex items-center justify-center p-4 bg-black/60 backdrop-blur-sm">
          <section className="glass-panel w-full max-w-md rounded-lg p-6">
            <div className="mb-4 flex items-center justify-between">
              <h3 className="font-mono text-lg font-semibold text-white">Add Git Account</h3>
              <button
                onClick={() => setIsAddModalOpen(false)}
                className="text-content-muted hover:text-white"
                type="button"
              >
                <X size={18} />
              </button>
            </div>

            <form onSubmit={handleAddAccount} className="space-y-4">
              {formError && (
                <div className="rounded border border-red-500/20 bg-red-500/10 p-3 text-xs text-red-400">
                  {formError}
                </div>
              )}

              <label className="flex flex-col gap-1.5">
                <span className="font-mono text-xs font-bold uppercase tracking-wider text-content-muted">Display Name</span>
                <input
                  type="text"
                  value={displayName}
                  onChange={(e) => setDisplayName(e.target.value)}
                  placeholder="e.g. Personal GitHub, Company GitHub Enterprise"
                  className="rounded-md border border-stroke bg-page px-3 py-2 text-sm text-white focus:border-brand-primary focus:outline-none"
                  required
                />
              </label>

              <label className="flex flex-col gap-1.5">
                <span className="font-mono text-xs font-bold uppercase tracking-wider text-content-muted">Provider</span>
                <select
                  value={provider}
                  onChange={(e) => setProvider(e.target.value)}
                  className="rounded-md border border-stroke bg-page px-3 py-2 text-sm text-white focus:border-brand-primary focus:outline-none"
                >
                  <option value="github">GitHub</option>
                </select>
              </label>

              <label className="flex flex-col gap-1.5">
                <span className="font-mono text-xs font-bold uppercase tracking-wider text-content-muted">GitHub API Base URL (Optional)</span>
                <input
                  type="url"
                  value={baseURL}
                  onChange={(e) => setBaseURL(e.target.value)}
                  placeholder="https://github.example.com/api/v3"
                  className="rounded-md border border-stroke bg-page px-3 py-2 text-sm text-white focus:border-brand-primary focus:outline-none"
                />
              </label>

              <label className="flex flex-col gap-1.5">
                <span className="font-mono text-xs font-bold uppercase tracking-wider text-content-muted">Personal Access Token</span>
                <div className="relative">
                  <KeyRound className="absolute left-3 top-2.5 text-content-muted" size={15} />
                  <input
                    type="password"
                    value={accToken}
                    onChange={(e) => setAccToken(e.target.value)}
                    placeholder="ghp_..."
                    className="w-full rounded-md border border-stroke bg-page py-2 pl-9 pr-3 text-sm text-white focus:border-brand-primary focus:outline-none"
                    required
                  />
                </div>
              </label>

              <div className="flex gap-3 border-t border-stroke pt-4">
                <button
                  type="button"
                  onClick={() => setIsAddModalOpen(false)}
                  className="flex-1 rounded-md border border-stroke py-2 text-sm font-semibold text-white hover:bg-slate-900"
                >
                  Cancel
                </button>
                <button
                  type="submit"
                  disabled={isSubmitting}
                  className="flex-1 inline-flex items-center justify-center gap-1.5 rounded-md bg-brand-primary py-2 text-sm font-bold text-black hover:bg-brand-primary/90 disabled:opacity-50"
                >
                  {isSubmitting ? <Loader2 className="animate-spin" size={15} /> : "Save"}
                </button>
              </div>
            </form>
          </section>
        </div>
      )}
    </div>
  );
}
