"use client";

import { FormEvent, useMemo, useState } from "react";
import useSWR from "swr";
import { CheckCircle2, Cpu, KeyRound, Loader2, Plus, Server, Trash2, XCircle } from "lucide-react";
import { DashboardLayout } from "@/components/dashboard/dashboard-layout";
import { MODEL_OPTIONS_BY_PROVIDER, MODEL_TIER_HINTS, PROVIDERS } from "@/lib/model-options";
import { api, ApiError } from "@/lib/api";
import { useSession } from "@/lib/session";
import type { ProviderCredential } from "@/lib/types";

const PROVIDER_DESCRIPTIONS: Record<string, string> = {
  gateway: "Internal route for virtual keys, budget enforcement, credential pools, and fallback.",
  openai: "GPT-4o and GPT-4o mini via OpenAI API.",
  anthropic: "Claude Sonnet and Opus via Anthropic API.",
  gemini: "Gemini Flash and Pro via Google AI.",
  "9router": "OpenAI-compatible router endpoint with custom base URL support.",
};

type FormState = {
  provider: string;
  label: string;
  apiKey: string;
  baseURL: string;
  priority: string;
};

const initialForm: FormState = {
  provider: "openai",
  label: "",
  apiKey: "",
  baseURL: "",
  priority: "0",
};

export default function AIProvidersPage() {
  const session = useSession();
  const token = session?.token ?? "";
  const orgID = session?.user.org_id ?? "";

  const [form, setForm] = useState<FormState>(initialForm);
  const [formError, setFormError] = useState("");
  const [isSubmitting, setIsSubmitting] = useState(false);
  const [testingMap, setTestingMap] = useState<Record<string, "idle" | "testing" | "success" | "error">>({});
  const [deleteConfirmID, setDeleteConfirmID] = useState<string | null>(null);

  const { data: credentials = [], error, mutate, isLoading } = useSWR(
    orgID && token ? ["provider-credentials", orgID] : null,
    () => api.listProviderCredentials(orgID, token),
  );

  const credentialsByProvider = useMemo(() => {
    return credentials.reduce<Record<string, ProviderCredential[]>>((groups, credential) => {
      groups[credential.provider] = groups[credential.provider] || [];
      groups[credential.provider].push(credential);
      return groups;
    }, {});
  }, [credentials]);

  async function handleAddCredential(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    if (!token || !orgID) return;
    if (!form.label.trim()) {
      setFormError("Label is required.");
      return;
    }
    if (!form.apiKey.trim()) {
      setFormError("API key is required.");
      return;
    }

    setFormError("");
    setIsSubmitting(true);
    try {
      await api.createProviderCredential(orgID, token, {
        provider: form.provider,
        label: form.label.trim(),
        api_key: form.apiKey.trim(),
        base_url: form.baseURL.trim() || undefined,
        priority: Number.parseInt(form.priority, 10) || 0,
      });
      setForm({ ...initialForm, provider: form.provider });
      await mutate();
    } catch (err) {
      setFormError(err instanceof ApiError ? err.message : "Failed to save provider credential.");
    } finally {
      setIsSubmitting(false);
    }
  }

  async function testCredential(credentialID: string) {
    if (!token || !orgID) return;
    setTestingMap((prev) => ({ ...prev, [credentialID]: "testing" }));
    try {
      await api.testProviderCredential(orgID, credentialID, token);
      setTestingMap((prev) => ({ ...prev, [credentialID]: "success" }));
      setTimeout(() => {
        setTestingMap((prev) => ({ ...prev, [credentialID]: "idle" }));
      }, 3000);
    } catch {
      setTestingMap((prev) => ({ ...prev, [credentialID]: "error" }));
    }
  }

  async function deleteCredential(credentialID: string) {
    if (!token || !orgID) return;
    try {
      await api.deleteProviderCredential(orgID, credentialID, token);
      setDeleteConfirmID(null);
      await mutate();
    } catch (err) {
      setFormError(err instanceof ApiError ? err.message : "Failed to delete provider credential.");
    }
  }

  return (
    <DashboardLayout>
      <div className="mb-6 flex flex-col gap-1">
        <h2 className="font-mono text-2xl font-semibold">AI Providers</h2>
        <p className="text-sm text-content-muted">
          Manage encrypted provider credential pools used by the Unified AI Gateway.
        </p>
      </div>

      <div className="grid gap-6 xl:grid-cols-[minmax(0,1fr)_360px]">
        <section className="space-y-4">
          {error && (
            <div className="rounded-lg border border-red-500/20 bg-red-500/10 p-4 text-sm text-red-300">
              Failed to load provider credentials.
            </div>
          )}

          {isLoading ? (
            <div className="glass-panel rounded-lg p-6 text-sm text-content-muted">Loading provider credentials...</div>
          ) : (
            PROVIDERS.map((provider) => {
              const providerCredentials = credentialsByProvider[provider] || [];
              const isGateway = provider === "gateway";

              return (
                <article key={provider} className="glass-panel rounded-lg p-5">
                  <div className="mb-4 flex items-start justify-between gap-4">
                    <div className="flex min-w-0 items-center gap-3">
                      <div className="grid size-10 place-items-center rounded-lg bg-panel text-brand-primary">
                        {isGateway ? <Cpu size={20} /> : <Server size={20} />}
                      </div>
                      <div className="min-w-0">
                        <h3 className="font-mono text-lg font-semibold capitalize text-white">{provider}</h3>
                        <p className="text-xs text-content-muted">{PROVIDER_DESCRIPTIONS[provider]}</p>
                      </div>
                    </div>
                    <StatusBadge count={providerCredentials.length} />
                  </div>

                  <div className="mb-4 flex flex-wrap gap-1.5">
                    {(MODEL_OPTIONS_BY_PROVIDER[provider] || []).map((model) => (
                      <TierBadge key={model} model={model} />
                    ))}
                  </div>

                  {isGateway ? (
                    <div className="rounded-md border border-stroke bg-page px-3 py-2 text-sm text-content-muted">
                      Gateway routes use virtual keys and resolve provider credentials from the pools below.
                    </div>
                  ) : providerCredentials.length === 0 ? (
                    <div className="rounded-md border border-dashed border-stroke px-3 py-4 text-sm text-content-muted">
                      No credentials configured for this provider.
                    </div>
                  ) : (
                    <div className="overflow-hidden rounded-md border border-stroke">
                      <table className="w-full text-left text-sm">
                        <thead className="bg-page text-xs uppercase text-content-muted">
                          <tr>
                            <th className="px-3 py-2">Label</th>
                            <th className="px-3 py-2">Status</th>
                            <th className="px-3 py-2">Priority</th>
                            <th className="px-3 py-2">Key</th>
                            <th className="px-3 py-2 text-right">Actions</th>
                          </tr>
                        </thead>
                        <tbody>
                          {providerCredentials.map((credential) => (
                            <CredentialRow
                              key={credential.id}
                              credential={credential}
                              testingState={testingMap[credential.id] || "idle"}
                              isDeleting={deleteConfirmID === credential.id}
                              onTest={() => testCredential(credential.id)}
                              onAskDelete={() => setDeleteConfirmID(credential.id)}
                              onCancelDelete={() => setDeleteConfirmID(null)}
                              onDelete={() => deleteCredential(credential.id)}
                            />
                          ))}
                        </tbody>
                      </table>
                    </div>
                  )}
                </article>
              );
            })
          )}
        </section>

        <aside className="glass-panel h-fit rounded-lg p-5">
          <div className="mb-4 flex items-center gap-2">
            <Plus size={18} className="text-brand-primary" />
            <h3 className="font-mono font-semibold text-white">Add Credential</h3>
          </div>

          {formError && (
            <div className="mb-4 rounded border border-red-500/20 bg-red-500/10 p-3 text-xs text-red-300">
              {formError}
            </div>
          )}

          <form onSubmit={handleAddCredential} className="space-y-4">
            <label className="flex flex-col gap-1.5">
              <span className="font-mono text-xs font-bold uppercase tracking-wider text-content-muted">Provider</span>
              <select
                value={form.provider}
                onChange={(event) => setForm((prev) => ({ ...prev, provider: event.target.value }))}
                className="rounded-md border border-stroke bg-page px-3 py-2 text-sm text-white focus:border-brand-primary focus:outline-none"
              >
                {PROVIDERS.filter((provider) => provider !== "gateway").map((provider) => (
                  <option key={provider} value={provider}>
                    {provider}
                  </option>
                ))}
              </select>
            </label>

            <label className="flex flex-col gap-1.5">
              <span className="font-mono text-xs font-bold uppercase tracking-wider text-content-muted">Label</span>
              <input
                value={form.label}
                onChange={(event) => setForm((prev) => ({ ...prev, label: event.target.value }))}
                placeholder="primary, backup, team-api"
                className="rounded-md border border-stroke bg-page px-3 py-2 text-sm text-white focus:border-brand-primary focus:outline-none"
              />
            </label>

            <label className="flex flex-col gap-1.5">
              <span className="font-mono text-xs font-bold uppercase tracking-wider text-content-muted">API Key</span>
              <div className="relative">
                <KeyRound className="absolute left-3 top-2.5 text-slate-500" size={14} />
                <input
                  type="password"
                  value={form.apiKey}
                  onChange={(event) => setForm((prev) => ({ ...prev, apiKey: event.target.value }))}
                  placeholder="sk-..."
                  className="w-full rounded-md border border-stroke bg-page py-2 pl-9 pr-3 text-sm text-white focus:border-brand-primary focus:outline-none"
                />
              </div>
            </label>

            <label className="flex flex-col gap-1.5">
              <span className="font-mono text-xs font-bold uppercase tracking-wider text-content-muted">Base URL</span>
              <input
                value={form.baseURL}
                onChange={(event) => setForm((prev) => ({ ...prev, baseURL: event.target.value }))}
                placeholder="https://api.openai.com/v1"
                className="rounded-md border border-stroke bg-page px-3 py-2 text-sm text-white focus:border-brand-primary focus:outline-none"
              />
            </label>

            <label className="flex flex-col gap-1.5">
              <span className="font-mono text-xs font-bold uppercase tracking-wider text-content-muted">Priority</span>
              <input
                type="number"
                value={form.priority}
                onChange={(event) => setForm((prev) => ({ ...prev, priority: event.target.value }))}
                className="rounded-md border border-stroke bg-page px-3 py-2 text-sm text-white focus:border-brand-primary focus:outline-none"
              />
            </label>

            <button
              disabled={isSubmitting || !token || !orgID}
              className="inline-flex w-full items-center justify-center gap-2 rounded-md bg-brand-primary px-4 py-2 font-mono text-xs font-bold uppercase tracking-wider text-black transition hover:bg-brand-primary/90 disabled:cursor-not-allowed disabled:opacity-50"
              type="submit"
            >
              {isSubmitting && <Loader2 size={14} className="animate-spin" />}
              Save Credential
            </button>
          </form>
        </aside>
      </div>
    </DashboardLayout>
  );
}

function StatusBadge({ count }: { count: number }) {
  if (count === 0) {
    return <span className="rounded-full border border-stroke px-2.5 py-1 text-xs text-content-muted">No keys</span>;
  }
  return (
    <span className="inline-flex items-center gap-1 rounded-full border border-emerald-500/30 bg-emerald-500/10 px-2.5 py-1 text-xs text-emerald-300">
      <CheckCircle2 size={12} />
      {count} key{count === 1 ? "" : "s"}
    </span>
  );
}

function TierBadge({ model }: { model: string }) {
  const hint = MODEL_TIER_HINTS[model];
  const tone =
    hint === "fast"
      ? "border-emerald-500/30 bg-emerald-500/10 text-emerald-300"
      : hint === "powerful" || hint === "premium"
        ? "border-fuchsia-500/30 bg-fuchsia-500/10 text-fuchsia-300"
        : "border-blue-500/30 bg-blue-500/10 text-blue-300";
  return <span className={`rounded-full border px-2.5 py-1 font-mono text-[10px] ${tone}`}>{model}</span>;
}

function CredentialRow({
  credential,
  testingState,
  isDeleting,
  onTest,
  onAskDelete,
  onCancelDelete,
  onDelete,
}: {
  credential: ProviderCredential;
  testingState: "idle" | "testing" | "success" | "error";
  isDeleting: boolean;
  onTest: () => void;
  onAskDelete: () => void;
  onCancelDelete: () => void;
  onDelete: () => void;
}) {
  return (
    <tr className="border-t border-stroke">
      <td className="px-3 py-3">
        <div className="font-medium text-white">{credential.label}</div>
        {credential.base_url && <div className="max-w-[220px] truncate text-xs text-content-muted">{credential.base_url}</div>}
      </td>
      <td className="px-3 py-3">
        <span className={`rounded-full px-2 py-1 text-xs ${statusClass(credential.status)}`}>{credential.status}</span>
      </td>
      <td className="px-3 py-3 font-mono text-content-muted">{credential.priority}</td>
      <td className="px-3 py-3 font-mono text-content-muted">
        {credential.configured ? `**** ${credential.key_suffix || "set"}` : "missing"}
      </td>
      <td className="px-3 py-3">
        {isDeleting ? (
          <div className="flex justify-end gap-2">
            <button className="rounded bg-rose-500 px-2 py-1 text-xs font-semibold text-white" onClick={onDelete} type="button">
              Delete
            </button>
            <button className="rounded border border-stroke px-2 py-1 text-xs text-white" onClick={onCancelDelete} type="button">
              Cancel
            </button>
          </div>
        ) : (
          <div className="flex justify-end gap-2">
            <button
              className={`inline-flex items-center gap-1 rounded border px-2 py-1 text-xs font-semibold ${testClass(testingState)}`}
              disabled={testingState === "testing"}
              onClick={onTest}
              type="button"
            >
              {testingState === "testing" && <Loader2 size={12} className="animate-spin" />}
              {testingState === "success" && <CheckCircle2 size={12} />}
              {testingState === "error" && <XCircle size={12} />}
              {testingState === "testing" ? "Testing" : testingState === "success" ? "OK" : testingState === "error" ? "Failed" : "Test"}
            </button>
            <button
              className="rounded p-1.5 text-slate-500 transition hover:bg-rose-500/10 hover:text-rose-300"
              onClick={onAskDelete}
              title="Delete credential"
              type="button"
            >
              <Trash2 size={15} />
            </button>
          </div>
        )}
      </td>
    </tr>
  );
}

function statusClass(status: ProviderCredential["status"]) {
  switch (status) {
    case "active":
      return "bg-emerald-500/10 text-emerald-300";
    case "rate_limited":
      return "bg-amber-500/10 text-amber-300";
    case "disabled":
      return "bg-slate-700 text-slate-300";
  }
}

function testClass(state: "idle" | "testing" | "success" | "error") {
  switch (state) {
    case "success":
      return "border-emerald-500/30 bg-emerald-500/10 text-emerald-300";
    case "error":
      return "border-red-500/30 bg-red-500/10 text-red-300";
    default:
      return "border-stroke text-white hover:bg-slate-900";
  }
}
