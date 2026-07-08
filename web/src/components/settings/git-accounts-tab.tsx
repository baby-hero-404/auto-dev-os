"use client";

import { FormEvent, useMemo, useState } from "react";
import useSWR from "swr";
import { Plus } from "lucide-react";
import { api, ApiError } from "@/lib/api";
import { useSession } from "@/lib/session";

import { initialForm, ProviderOption, ActionState } from "./git-accounts/types";
import { MetricCard } from "./git-accounts/MetricCard";
import { EmptyGitAccounts } from "./git-accounts/EmptyGitAccounts";
import { GitAccountsSkeleton } from "./git-accounts/GitAccountsSkeleton";
import { ConnectGitAccountForm } from "./git-accounts/ConnectGitAccountForm";
import { GitAccountCard } from "./git-accounts/GitAccountCard";

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
