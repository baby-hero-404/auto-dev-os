"use client";

import { FormEvent, useEffect, useMemo, useState } from "react";
import useSWR from "swr";
import { CheckCircle2, ChevronDown, Cpu, Eye, EyeOff, KeyRound, Loader2, Plus, Server, Trash2, X, XCircle } from "lucide-react";
import { Toaster, toast } from "sonner";
import { DashboardLayout } from "@/components/dashboard/dashboard-layout";
import { PROVIDERS } from "@/lib/model-options";
import { api, ApiError } from "@/lib/api";
import { useSession } from "@/lib/session";
import type { ProviderCredential } from "@/lib/types";

const BASE_URL_PLACEHOLDERS: Record<string, string> = {
  openai: "https://api.openai.com/v1 (optional)",
  anthropic: "https://api.anthropic.com (optional)",
  gemini: "https://generativelanguage.googleapis.com (optional)",
  "9router": "https://api.9router.com/v1 (optional)",
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
  label: generatedCredentialLabel("openai"),
  apiKey: "",
  baseURL: "",
  priority: "0",
};

function generatedCredentialLabel(provider: string, apiKey = "") {
  const cleanKey = apiKey.trim();
  if (cleanKey.length > 4) {
    const suffix = cleanKey.slice(-4);
    return `${provider} key ${suffix}`;
  }
  return `${provider} key`;
}

export default function AIProvidersPage() {
  const session = useSession();
  const token = session?.token ?? "";
  const orgID = session?.user.org_id ?? "";

  const [form, setForm] = useState<FormState>(initialForm);
  const [formError, setFormError] = useState("");
  const [isSubmitting, setIsSubmitting] = useState(false);
  const [saveState, setSaveState] = useState<"idle" | "saved">("idle");
  const [draftTestState, setDraftTestState] = useState<"idle" | "testing" | "success" | "error">("idle");
  const [testingMap, setTestingMap] = useState<Record<string, "idle" | "testing" | "success" | "error">>({});
  const [deleteConfirmID, setDeleteConfirmID] = useState<string | null>(null);
  const [showApiKey, setShowApiKey] = useState(false);
  const [isAddOpen, setIsAddOpen] = useState(false);
  const [selectedProvider, setSelectedProvider] = useState<string>("openai");
  const [isAddModelOpen, setIsAddModelOpen] = useState(false);
  const [addModelLevel, setAddModelLevel] = useState<"fast" | "balanced" | "powerful">("fast");
  const [newModelName, setNewModelName] = useState("");
  const [newModelPriority, setNewModelPriority] = useState("0");
  const [isAddingModel, setIsAddingModel] = useState(false);

  const { data: credentials = [], error, mutate, isLoading } = useSWR(
    orgID && token ? ["provider-credentials", orgID] : null,
    () => api.listProviderCredentials(orgID, token),
  );

  const { data: providerModels = [], mutate: mutateModels } = useSWR(
    orgID && token ? ["provider-models", orgID] : null,
    () => api.listProviderModels(orgID, token),
  );

  const credentialsByProvider = useMemo(() => {
    return credentials.reduce<Record<string, ProviderCredential[]>>((groups, credential) => {
      groups[credential.provider] = groups[credential.provider] || [];
      groups[credential.provider].push(credential);
      return groups;
    }, {});
  }, [credentials]);

  const configuredProviderCount = useMemo(() => {
    return PROVIDERS.filter((provider) => provider !== "gateway" && (credentialsByProvider[provider]?.length || 0) > 0).length;
  }, [credentialsByProvider]);

  const activeCredentialCount = useMemo(() => {
    return credentials.filter((credential) => credential.status === "active").length;
  }, [credentials]);

  async function handleAddCredential(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    if (!token || !orgID) return;
    if (!form.apiKey.trim()) {
      setFormError("API key is required.");
      return;
    }
    if (draftTestState !== "success") {
      setFormError("Test the connection successfully before saving.");
      return;
    }

    setFormError("");
    setIsSubmitting(true);
    const label = form.label.trim() || generatedCredentialLabel(form.provider);
    try {
      await api.createProviderCredential(orgID, token, {
        provider: form.provider,
        label,
        api_key: form.apiKey.trim(),
        base_url: form.baseURL.trim() || undefined,
        priority: Number.parseInt(form.priority, 10) || 0,
      });
      setForm({ ...initialForm, provider: form.provider, label: generatedCredentialLabel(form.provider) });
      await mutate();
      await mutateModels();
      setSaveState("saved");
      setDraftTestState("idle");
      setTimeout(() => {
        setSaveState("idle");
        setIsAddOpen(false);
      }, 600);
    } catch (err) {
      const message = err instanceof ApiError ? err.message : "Failed to save provider credential.";
      setFormError(message);
    } finally {
      setIsSubmitting(false);
    }
  }

  async function testDraftCredential() {
    if (!token || !orgID) return;
    if (!form.apiKey.trim()) {
      setFormError("API key is required before testing.");
      return;
    }
    setFormError("");
    setDraftTestState("testing");
    try {
      await api.testProviderCredentialInput(orgID, token, {
        provider: form.provider,
        api_key: form.apiKey.trim(),
        base_url: form.baseURL.trim() || undefined,
      });
      setDraftTestState("success");
      toast.success("API key test passed.");
    } catch (err) {
      const message = err instanceof ApiError ? err.message : "API key test failed.";
      setDraftTestState("error");
      setFormError(message);
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
    } catch (err) {
      setTestingMap((prev) => ({ ...prev, [credentialID]: "error" }));
      const message = err instanceof ApiError ? err.message : "Credential test failed.";
      toast.error(message);
      setTimeout(() => {
        setTestingMap((prev) => ({ ...prev, [credentialID]: "idle" }));
      }, 3000);
    }
  }

  async function deleteCredential(credentialID: string) {
    if (!token || !orgID) return;
    try {
      await api.deleteProviderCredential(orgID, credentialID, token);
      setDeleteConfirmID(null);
      await mutate();
    } catch (err) {
      const message = err instanceof ApiError ? err.message : "Failed to delete provider credential.";
      toast.error(message);
    }
  }

  async function handleAddModel(event: FormEvent) {
    event.preventDefault();
    if (!token || !orgID || !newModelName.trim()) return;
    setIsAddingModel(true);
    try {
      await api.createProviderModel(orgID, token, {
        provider: selectedProvider,
        level_group: addModelLevel,
        model_name: newModelName.trim(),
        priority: parseInt(newModelPriority, 10) || 0,
      });
      setNewModelName("");
      setNewModelPriority("0");
      setIsAddModelOpen(false);
      await mutateModels();
      toast.success("Provider model added successfully.");
    } catch (err) {
      const message = err instanceof ApiError ? err.message : "Failed to add model.";
      toast.error(message);
    } finally {
      setIsAddingModel(false);
    }
  }

  async function toggleModelActive(modelID: string, currentActive: boolean) {
    if (!token || !orgID) return;
    try {
      const updatedModels = providerModels.map((m) =>
        m.id === modelID ? { ...m, is_active: !currentActive } : m
      );
      mutateModels(updatedModels, false);

      await api.updateProviderModel(orgID, modelID, token, {
        is_active: !currentActive,
      });
      await mutateModels();
      toast.success(`Model ${currentActive ? "disabled" : "enabled"}.`);
    } catch (err) {
      const message = err instanceof ApiError ? err.message : "Failed to toggle model status.";
      toast.error(message);
      await mutateModels();
    }
  }

  async function adjustModelPriority(modelID: string, currentPriority: number, amount: number) {
    if (!token || !orgID) return;
    const nextPriority = Math.max(0, currentPriority + amount);
    if (nextPriority === currentPriority) return;

    try {
      const updatedModels = providerModels.map((m) =>
        m.id === modelID ? { ...m, priority: nextPriority } : m
      );
      mutateModels(updatedModels, false);

      await api.updateProviderModel(orgID, modelID, token, {
        priority: nextPriority,
      });
      await mutateModels();
    } catch (err) {
      const message = err instanceof ApiError ? err.message : "Failed to update priority.";
      toast.error(message);
      await mutateModels();
    }
  }

  async function deleteModel(modelID: string) {
    if (!token || !orgID) return;
    try {
      const updatedModels = providerModels.filter((m) => m.id !== modelID);
      mutateModels(updatedModels, false);

      await api.deleteProviderModel(orgID, modelID, token);
      await mutateModels();
      toast.success("Model deleted successfully.");
    } catch (err) {
      const message = err instanceof ApiError ? err.message : "Failed to delete model.";
      toast.error(message);
      await mutateModels();
    }
  }

  return (
    <DashboardLayout>
      <Toaster richColors position="top-right" />
      <div className="mb-6 flex flex-col gap-4 sm:flex-row sm:items-start sm:justify-between">
        <div className="flex flex-col gap-1">
          <h2 className="text-2xl font-semibold text-foreground">AI Providers</h2>
          <p className="text-sm text-content-muted">
            Manage encrypted provider credential pools used by the Unified AI Gateway.
          </p>
        </div>
        <button
          type="button"
          disabled={!token || !orgID}
          onClick={() => {
            setFormError("");
            setSaveState("idle");
            setDraftTestState("idle");
            setForm((prev) => ({
              ...prev,
              label: prev.label.trim() || generatedCredentialLabel(prev.provider),
            }));
            setIsAddOpen(true);
          }}
          className="inline-flex items-center justify-center gap-2 rounded-md bg-brand-primary px-4 py-2.5 text-sm font-semibold text-white transition hover:opacity-90 disabled:cursor-not-allowed disabled:opacity-50"
        >
          <Plus size={16} />
          Add Credential
        </button>
      </div>

      <div className="grid gap-6">
        <section className="space-y-4">
          {error && (
            <div className="rounded-lg border border-red-500/20 bg-red-500/10 p-4 text-sm text-red-300">
              Failed to load provider credentials.
            </div>
          )}

          {isLoading ? (
            <>
              <ProviderSummarySkeleton />
              <ProviderTableSkeleton />
            </>
          ) : credentials.length === 0 ? (
            <div className="flex flex-col items-center justify-center rounded-lg border border-dashed border-stroke bg-card p-12 text-center animate-fade-in">
              <div className="grid size-12 place-items-center rounded-xl bg-surface text-brand-primary">
                <Cpu size={24} />
              </div>
              <h3 className="mt-4 font-semibold text-foreground">No credentials configured</h3>
              <p className="mt-2 max-w-sm text-sm text-content-muted font-normal leading-relaxed">
                Add an API key for OpenAI, Anthropic, Gemini, or a custom router to enable LLM access for your agents.
              </p>
              <button
                onClick={() => {
                  setFormError("");
                  setSaveState("idle");
                  setDraftTestState("idle");
                  setForm((prev) => ({
                    ...prev,
                    label: prev.label.trim() || generatedCredentialLabel(prev.provider),
                  }));
                  setIsAddOpen(true);
                }}
                className="mt-5 flex items-center justify-center gap-2 rounded-md bg-brand-primary px-4 py-2 text-sm font-semibold text-white transition hover:opacity-90 cursor-pointer shadow-[0_0_15px_rgba(34,197,94,0.15)]"
                type="button"
              >
                <Plus size={16} />
                Add Credential
              </button>
            </div>
          ) : (
            <>
              <ProviderSummary
                configuredProviderCount={configuredProviderCount}
                totalCredentialCount={credentials.length}
                activeCredentialCount={activeCredentialCount}
              />
              <ProviderCredentialsTable
                credentials={credentials}
                testingMap={testingMap}
                deleteConfirmID={deleteConfirmID}
                onTest={testCredential}
                onAskDelete={setDeleteConfirmID}
                onCancelDelete={() => setDeleteConfirmID(null)}
                onDelete={deleteCredential}
              />
            </>
          )}
        </section>

        {/* Model level group config */}
        {credentials.length > 0 && (
          <section className="space-y-4">
            <div className="flex flex-col gap-1">
              <h3 className="text-lg font-semibold text-foreground">Model Routing Rules</h3>
              <p className="text-sm text-content-muted">
                Configure specific routing groups (Fast, Balanced, Powerful) for each provider.
              </p>
            </div>

            {/* Provider Tabs */}
            <div className="flex gap-2 border-b border-stroke pb-px">
              {PROVIDERS.filter((provider) => provider !== "gateway").map((provider) => {
                const count = credentialsByProvider[provider]?.length || 0;
                const active = selectedProvider === provider;
                return (
                  <button
                    key={provider}
                    type="button"
                    onClick={() => setSelectedProvider(provider)}
                    className={`flex items-center gap-2 border-b-2 px-4 py-2.5 text-sm font-semibold capitalize transition-all cursor-pointer ${
                      active
                        ? "border-brand-primary text-brand-primary"
                        : "border-transparent text-content-muted hover:text-foreground"
                    }`}
                  >
                    {provider}
                    {count > 0 && (
                      <span className={`rounded-full px-1.5 py-0.5 text-[10px] font-bold ${
                        active ? "bg-brand-primary-muted text-brand-primary" : "bg-surface text-content-muted"
                      }`}>
                        {count} key{count === 1 ? "" : "s"}
                      </span>
                    )}
                  </button>
                );
              })}
            </div>

            {/* Stacked Level Sections */}
            <div className="grid gap-6">
              {(["fast", "balanced", "powerful"] as const).map((level) => {
                const levelModels = providerModels.filter(
                  (m) => m.provider === selectedProvider && m.level_group === level
                );
                
                // Sort by priority ascending
                levelModels.sort((a, b) => a.priority - b.priority);

                return (
                  <div key={level} className="glass-panel overflow-hidden rounded-lg p-5">
                    <div className="mb-4 flex items-center justify-between">
                      <div className="flex items-center gap-2">
                        <span className="text-base font-semibold capitalize text-foreground flex items-center gap-1.5">
                          {level === "fast" && "⚡ Fast Models"}
                          {level === "balanced" && "⚖️ Balanced Models"}
                          {level === "powerful" && "🚀 Powerful Models"}
                        </span>
                        <span className="rounded-full bg-surface px-2 py-0.5 text-xs font-semibold text-content-muted border border-stroke">
                          {levelModels.length} model{levelModels.length === 1 ? "" : "s"}
                        </span>
                      </div>
                      <button
                        type="button"
                        onClick={() => {
                          setAddModelLevel(level);
                          setNewModelName("");
                          setNewModelPriority("0");
                          setIsAddModelOpen(true);
                        }}
                        className="inline-flex items-center gap-1 rounded bg-brand-primary px-3 py-1.5 text-xs font-semibold text-white transition hover:opacity-90 cursor-pointer"
                      >
                        <Plus size={12} />
                        Add Model
                      </button>
                    </div>

                    {levelModels.length === 0 ? (
                      <div className="flex flex-col items-center justify-center py-6 text-center border border-dashed border-stroke rounded-lg bg-surface/25">
                        <p className="text-sm text-content-muted">
                          No {level} models configured yet. Add one to enable high-speed tasks.
                        </p>
                      </div>
                    ) : (
                      <div className="overflow-x-auto">
                        <table className="w-full text-left text-sm">
                          <thead className="border-b border-stroke text-[10px] uppercase tracking-wide text-content-muted">
                            <tr>
                              <th className="px-4 py-2 w-[40%]">Model Name</th>
                              <th className="px-4 py-2 w-[20%]">Priority</th>
                              <th className="px-4 py-2 w-[20%]">Status</th>
                              <th className="px-4 py-2 w-[20%] text-right">Actions</th>
                            </tr>
                          </thead>
                          <tbody>
                            {levelModels.map((model) => (
                              <tr key={model.id} className="border-b border-stroke/50 last:border-b-0 align-middle hover:bg-surface/10">
                                <td className="px-4 py-3 font-medium text-foreground font-mono text-xs">
                                  {model.model_name}
                                </td>
                                <td className="px-4 py-3">
                                  <div className="flex items-center gap-2">
                                    <span className="inline-flex items-center rounded bg-surface px-2 py-0.5 text-xs font-semibold text-content-muted border border-stroke">
                                      P{model.priority}
                                    </span>
                                    <div className="flex flex-col">
                                      <button
                                        type="button"
                                        onClick={() => adjustModelPriority(model.id, model.priority, -1)}
                                        disabled={model.priority === 0}
                                        className="text-[10px] text-content-muted hover:text-foreground cursor-pointer disabled:opacity-30 disabled:cursor-not-allowed"
                                        title="Increase priority"
                                      >
                                        ▲
                                      </button>
                                      <button
                                        type="button"
                                        onClick={() => adjustModelPriority(model.id, model.priority, 1)}
                                        className="text-[10px] text-content-muted hover:text-foreground cursor-pointer"
                                        title="Decrease priority"
                                      >
                                        ▼
                                      </button>
                                    </div>
                                  </div>
                                </td>
                                <td className="px-4 py-3">
                                  <button
                                    type="button"
                                    onClick={() => toggleModelActive(model.id, model.is_active)}
                                    className={`inline-flex items-center gap-1.5 rounded-full border px-2 py-0.5 text-xs font-semibold transition-all cursor-pointer ${
                                      model.is_active
                                        ? "bg-emerald-500/10 text-emerald-700 dark:text-emerald-300 border-emerald-500/20"
                                        : "bg-surface text-content-muted border-stroke"
                                    }`}
                                  >
                                    <span className={`size-1.5 rounded-full ${model.is_active ? "bg-emerald-500" : "bg-content-muted"}`} />
                                    {model.is_active ? "Active" : "Inactive"}
                                  </button>
                                </td>
                                <td className="px-4 py-3 text-right">
                                  <button
                                    type="button"
                                    onClick={() => deleteModel(model.id)}
                                    className="rounded p-1 text-content-muted transition-colors hover:bg-danger/10 hover:text-danger cursor-pointer"
                                    title="Delete model"
                                  >
                                    <Trash2 size={13} />
                                  </button>
                                </td>
                              </tr>
                            ))}
                          </tbody>
                        </table>
                      </div>
                    )}
                  </div>
                );
              })}
            </div>
          </section>
        )}
      </div>

      {isAddOpen && (
        <AddCredentialModal
          form={form}
          formError={formError}
          isSubmitting={isSubmitting}
          saveState={saveState}
          draftTestState={draftTestState}
          showApiKey={showApiKey}
          token={token}
          orgID={orgID}
          onClose={() => {
            if (!isSubmitting) {
              setIsAddOpen(false);
            }
          }}
          onSubmit={handleAddCredential}
          onSetForm={(next) => {
            setDraftTestState("idle");
            setForm(next);
          }}
          onTestKey={testDraftCredential}
          onToggleApiKey={() => setShowApiKey((value) => !value)}
        />
      )}

      {isAddModelOpen && (
        <div
          className="fixed inset-0 z-modal grid place-items-center bg-black/45 px-4 py-6 backdrop-blur-sm"
          role="dialog"
          aria-modal="true"
          onMouseDown={() => setIsAddModelOpen(false)}
        >
          <div
            className="glass-panel animate-modal-in w-full max-w-sm rounded-lg p-5 shadow-2xl"
            onMouseDown={(event) => event.stopPropagation()}
          >
            <div className="mb-4 flex items-center justify-between gap-4">
              <div className="flex items-center gap-2">
                <Plus size={18} className="text-brand-primary" />
                <h3 className="font-semibold text-foreground">
                  Add Model ({addModelLevel})
                </h3>
              </div>
              <button
                type="button"
                onClick={() => setIsAddModelOpen(false)}
                className="rounded p-1.5 text-content-muted transition-colors hover:bg-surface hover:text-foreground"
                title="Close"
              >
                <X size={16} />
              </button>
            </div>

            <form onSubmit={handleAddModel} className="space-y-4">
              <label className="flex flex-col gap-1.5">
                <span className="text-xs font-semibold uppercase tracking-wider text-content-muted">Model Name</span>
                <input
                  required
                  value={newModelName}
                  onChange={(event) => setNewModelName(event.target.value)}
                  placeholder="e.g. gpt-4o-mini"
                  className="rounded-md border border-stroke bg-background px-3 py-2 text-sm text-foreground transition-all focus:border-brand-primary focus:outline-none focus:ring-2 focus:ring-brand-primary/20"
                />
              </label>

              <label className="flex flex-col gap-1.5">
                <div className="flex items-center justify-between gap-3">
                  <span className="text-xs font-semibold uppercase tracking-wider text-content-muted">Priority</span>
                  <span className="text-right text-[10px] font-medium text-content-muted">Lower = runs first (0 = highest)</span>
                </div>
                <input
                  type="number"
                  min="0"
                  value={newModelPriority}
                  onChange={(event) => setNewModelPriority(event.target.value)}
                  className="rounded-md border border-stroke bg-background px-3 py-2 text-sm text-foreground transition-all focus:border-brand-primary focus:outline-none focus:ring-2 focus:ring-brand-primary/20"
                />
              </label>

              <div className="flex justify-end gap-2 pt-1">
                <button
                  type="button"
                  onClick={() => setIsAddModelOpen(false)}
                  className="rounded-md border border-stroke px-4 py-2 text-sm font-semibold text-foreground transition hover:bg-surface"
                >
                  Cancel
                </button>
                <button
                  type="submit"
                  disabled={isAddingModel || !newModelName.trim()}
                  className="inline-flex min-w-24 items-center justify-center gap-2 rounded-md bg-brand-primary px-4 py-2 text-sm font-semibold text-white transition hover:opacity-90 disabled:opacity-50 disabled:cursor-not-allowed"
                >
                  {isAddingModel && <Loader2 size={14} className="animate-spin" />}
                  Add Model
                </button>
              </div>
            </form>
          </div>
        </div>
      )}
    </DashboardLayout>
  );
}

function AddCredentialModal({
  form,
  formError,
  isSubmitting,
  saveState,
  draftTestState,
  showApiKey,
  token,
  orgID,
  onClose,
  onSubmit,
  onSetForm,
  onTestKey,
  onToggleApiKey,
}: {
  form: FormState;
  formError: string;
  isSubmitting: boolean;
  saveState: "idle" | "saved";
  draftTestState: "idle" | "testing" | "success" | "error";
  showApiKey: boolean;
  token: string;
  orgID: string;
  onClose: () => void;
  onSubmit: (event: FormEvent<HTMLFormElement>) => void;
  onSetForm: React.Dispatch<React.SetStateAction<FormState>>;
  onTestKey: () => void;
  onToggleApiKey: () => void;
}) {
  const [showBaseUrl, setShowBaseUrl] = useState(false);

  useEffect(() => {
    function handleKeyDown(event: KeyboardEvent) {
      if (event.key === "Escape" && !isSubmitting) {
        onClose();
      }
    }
    window.addEventListener("keydown", handleKeyDown);
    return () => window.removeEventListener("keydown", handleKeyDown);
  }, [onClose, isSubmitting]);

  return (
    <div
      className="fixed inset-0 z-modal grid place-items-center bg-black/45 px-4 py-6 backdrop-blur-sm"
      role="dialog"
      aria-modal="true"
      aria-labelledby="add-credential-title"
      onMouseDown={onClose}
    >
      <div
        className="glass-panel animate-modal-in w-full max-w-md rounded-lg p-5 shadow-2xl"
        onMouseDown={(event) => event.stopPropagation()}
      >
        <div className="mb-4 flex items-center justify-between gap-4">
          <div className="flex items-center gap-2">
            <Plus size={18} className="text-brand-primary" />
            <h3 id="add-credential-title" className="font-semibold text-foreground">
              Add Credential
            </h3>
          </div>
          <button
            type="button"
            onClick={onClose}
            disabled={isSubmitting}
            className="rounded p-1.5 text-content-muted transition-colors hover:bg-surface hover:text-foreground disabled:cursor-not-allowed disabled:opacity-50"
            title="Close"
          >
            <X size={16} />
          </button>
        </div>

        {formError && (
          <div className="mb-4 rounded-md border border-red-500/25 bg-red-500/10 p-3 text-xs font-medium text-red-600 dark:text-red-300">
            {formError}
          </div>
        )}

        <div className="mb-4 grid grid-cols-3 overflow-hidden rounded-md border border-stroke bg-surface/40 text-xs">
          <FlowStep index="1" label="Enter key" active={form.apiKey.trim().length > 0} />
          <FlowStep index="2" label="Test" active={draftTestState === "success"} pending={draftTestState === "testing"} />
          <FlowStep index="3" label="Save" active={saveState === "saved"} />
        </div>

        <form onSubmit={onSubmit} className="space-y-4">
          <label className="flex flex-col gap-1.5">
            <span className="text-xs font-semibold uppercase tracking-wider text-content-muted">Provider</span>
            <div className="relative">
              <select
                value={form.provider}
                onChange={(event) => {
                  const provider = event.target.value;
                  onSetForm((prev) => {
                    const currentGen = generatedCredentialLabel(prev.provider, prev.apiKey);
                    const isAutogenerated = !prev.label.trim() || prev.label === currentGen || prev.label === `${prev.provider} key`;
                    return {
                      ...prev,
                      provider,
                      label: isAutogenerated ? generatedCredentialLabel(provider, prev.apiKey) : prev.label,
                    };
                  });
                }}
                className="w-full appearance-none rounded-md border border-stroke bg-background pl-3 pr-10 py-2 text-sm text-foreground transition-all duration-150 focus:border-brand-primary focus:outline-none focus:ring-2 focus:ring-brand-primary/20"
              >
                {PROVIDERS.filter((provider) => provider !== "gateway").map((provider) => (
                  <option key={provider} value={provider}>
                    {provider}
                  </option>
                ))}
              </select>
              <ChevronDown className="absolute right-3 top-3 text-content-muted pointer-events-none" size={14} />
            </div>
          </label>

          <label className="flex flex-col gap-1.5">
            <span className="text-xs font-semibold uppercase tracking-wider text-content-muted">Label</span>
            <input
              value={form.label}
              onChange={(event) => onSetForm((prev) => ({ ...prev, label: event.target.value }))}
              placeholder="primary, backup, team-api"
              className="rounded-md border border-stroke bg-background px-3 py-2 text-sm text-foreground transition-all duration-150 focus:border-brand-primary focus:outline-none focus:ring-2 focus:ring-brand-primary/20"
            />
          </label>

          <label className="flex flex-col gap-1.5">
            <span className="text-xs font-semibold uppercase tracking-wider text-content-muted">API Key</span>
            <div className="relative">
              <KeyRound className="absolute left-3 top-2.5 text-content-muted" size={14} />
              <input
                type={showApiKey ? "text" : "password"}
                value={form.apiKey}
                onChange={(event) => {
                  const key = event.target.value;
                  onSetForm((prev) => {
                    const currentGen = generatedCredentialLabel(prev.provider, prev.apiKey);
                    const isAutogenerated = !prev.label.trim() || prev.label === currentGen || prev.label === `${prev.provider} key`;
                    return {
                      ...prev,
                      apiKey: key,
                      label: isAutogenerated ? generatedCredentialLabel(prev.provider, key) : prev.label,
                    };
                  });
                }}
                placeholder="sk-..."
                className="w-full rounded-md border border-stroke bg-background py-2 pl-9 pr-10 text-sm text-foreground transition-all duration-150 focus:border-brand-primary focus:outline-none focus:ring-2 focus:ring-brand-primary/20"
              />
              <button
                type="button"
                onClick={onToggleApiKey}
                className="absolute right-3 top-2.5 text-content-muted transition-colors duration-150 hover:text-foreground"
                title={showApiKey ? "Hide API Key" : "Show API Key"}
              >
                {showApiKey ? <EyeOff size={14} /> : <Eye size={14} />}
              </button>
            </div>
          </label>

          <div className="flex items-center gap-2 py-0.5">
            <input
              type="checkbox"
              id="toggle-base-url"
              checked={showBaseUrl}
              onChange={(e) => {
                setShowBaseUrl(e.target.checked);
                if (!e.target.checked) {
                  onSetForm((prev) => ({ ...prev, baseURL: "" }));
                }
              }}
              className="size-3.5 cursor-pointer rounded border-stroke bg-background text-brand-primary focus:ring-0 focus:ring-offset-0"
            />
            <label htmlFor="toggle-base-url" className="text-xs font-medium text-content-muted cursor-pointer select-none hover:text-foreground transition-colors">
              Use custom Base URL
            </label>
          </div>

          {showBaseUrl && (
            <label className="flex flex-col gap-1.5 animate-fade-in">
              <span className="text-xs font-semibold uppercase tracking-wider text-content-muted">Base URL</span>
              <input
                value={form.baseURL}
                onChange={(event) => onSetForm((prev) => ({ ...prev, baseURL: event.target.value }))}
                placeholder={BASE_URL_PLACEHOLDERS[form.provider] || "https://api.openai.com/v1 (optional)"}
                className="rounded-md border border-stroke bg-background px-3 py-2 text-sm text-foreground transition-all duration-150 focus:border-brand-primary focus:outline-none focus:ring-2 focus:ring-brand-primary/20"
              />
            </label>
          )}

          <label className="flex flex-col gap-1.5">
            <div className="flex items-center justify-between gap-3">
              <span className="text-xs font-semibold uppercase tracking-wider text-content-muted">Priority</span>
              <span className="text-right text-[10px] font-medium text-content-muted">Lower = runs first (0 = highest)</span>
            </div>
            <input
              type="number"
              value={form.priority}
              onChange={(event) => onSetForm((prev) => ({ ...prev, priority: event.target.value }))}
              className="rounded-md border border-stroke bg-background px-3 py-2 text-sm text-foreground transition-all duration-150 focus:border-brand-primary focus:outline-none focus:ring-2 focus:ring-brand-primary/20"
            />
          </label>

          <div className="flex justify-end gap-2 pt-1">
            <button
              type="button"
              onClick={onClose}
              disabled={isSubmitting}
              className="rounded-md border border-stroke px-4 py-2.5 text-sm font-semibold text-foreground transition hover:bg-surface disabled:cursor-not-allowed disabled:opacity-50"
            >
              Cancel
            </button>
            <button
              type="button"
              onClick={onTestKey}
              disabled={draftTestState === "testing" || isSubmitting || !form.apiKey.trim() || !token || !orgID}
              className={`inline-flex min-w-36 items-center justify-center gap-2 rounded-md border px-4 py-2.5 text-sm font-semibold transition disabled:cursor-not-allowed disabled:opacity-50 ${testKeyButtonClass(draftTestState)}`}
            >
              {draftTestState === "testing" && <Loader2 size={14} className="animate-spin" />}
              {draftTestState === "success" && <CheckCircle2 size={14} />}
              {draftTestState === "error" && <XCircle size={14} />}
              {draftTestState === "testing" ? "Testing" : draftTestState === "success" ? "Connected" : draftTestState === "error" ? "Failed" : "Test connection"}
            </button>
            <button
              disabled={isSubmitting || draftTestState !== "success" || !token || !orgID}
              className="inline-flex min-w-32 items-center justify-center gap-2 rounded-md bg-brand-primary px-4 py-2.5 text-sm font-semibold text-white transition hover:opacity-90 disabled:cursor-not-allowed disabled:opacity-50"
              type="submit"
            >
              {isSubmitting && <Loader2 size={14} className="animate-spin" />}
              {saveState === "saved" && !isSubmitting ? (
                <>
                  <CheckCircle2 size={14} />
                  Saved
                </>
              ) : (
                "Save"
              )}
            </button>
          </div>
          {draftTestState !== "success" && (
            <p className="text-right text-xs text-content-muted">Save unlocks after a successful connection test.</p>
          )}
        </form>
      </div>
    </div>
  );
}

function FlowStep({
  index,
  label,
  active,
  pending = false,
}: {
  index: string;
  label: string;
  active: boolean;
  pending?: boolean;
}) {
  return (
    <div className="flex items-center justify-center gap-2 border-r border-stroke px-2 py-2 last:border-r-0">
      <span
        className={`grid size-5 place-items-center rounded-full text-[10px] font-bold ${
          active
            ? "bg-emerald-500 text-white"
            : pending
              ? "bg-brand-primary-muted text-brand-primary"
              : "bg-background text-content-muted"
        }`}
      >
        {pending ? <Loader2 size={11} className="animate-spin" /> : active ? <CheckCircle2 size={11} /> : index}
      </span>
      <span className={active ? "font-semibold text-foreground" : "font-medium text-content-muted"}>{label}</span>
    </div>
  );
}

function ProviderSummary({
  configuredProviderCount,
  totalCredentialCount,
  activeCredentialCount,
}: {
  configuredProviderCount: number;
  totalCredentialCount: number;
  activeCredentialCount: number;
}) {
  return (
    <div className="grid gap-3 md:grid-cols-3">
      <SummaryCard
        icon={<Server size={18} />}
        label="Configured providers"
        value={`${configuredProviderCount}/4`}
        detail="OpenAI, Anthropic, Gemini, 9router"
      />
      <SummaryCard
        icon={<KeyRound size={18} />}
        label="Credential pool"
        value={`${totalCredentialCount}`}
        detail={`${activeCredentialCount} active key${activeCredentialCount === 1 ? "" : "s"}`}
      />
      <SummaryCard
        icon={<Cpu size={18} />}
        label="Gateway readiness"
        value={activeCredentialCount > 0 ? "Ready" : "Waiting"}
        detail="Agents route through the gateway pool"
        tone={activeCredentialCount > 0 ? "success" : "muted"}
      />
    </div>
  );
}

function SummaryCard({
  icon,
  label,
  value,
  detail,
  tone = "default",
}: {
  icon: React.ReactNode;
  label: string;
  value: string;
  detail: string;
  tone?: "default" | "success" | "muted";
}) {
  return (
    <div className="rounded-lg border border-stroke bg-card p-4 shadow-sm">
      <div className="flex items-start justify-between gap-3">
        <div>
          <p className="text-xs font-semibold uppercase tracking-wide text-content-muted">{label}</p>
          <p className={`mt-2 text-2xl font-semibold ${tone === "success" ? "text-emerald-600 dark:text-emerald-300" : "text-foreground"}`}>
            {value}
          </p>
          <p className="mt-1 text-xs text-content-muted">{detail}</p>
        </div>
        <div className={`grid size-9 place-items-center rounded-lg ${tone === "success" ? "bg-emerald-500/10 text-emerald-600 dark:text-emerald-300" : "bg-brand-primary-muted text-brand-primary"}`}>
          {icon}
        </div>
      </div>
    </div>
  );
}

function ProviderCredentialsTable({
  credentials,
  testingMap,
  deleteConfirmID,
  onTest,
  onAskDelete,
  onCancelDelete,
  onDelete,
}: {
  credentials: ProviderCredential[];
  testingMap: Record<string, "idle" | "testing" | "success" | "error">;
  deleteConfirmID: string | null;
  onTest: (credentialID: string) => void;
  onAskDelete: (credentialID: string) => void;
  onCancelDelete: () => void;
  onDelete: (credentialID: string) => void;
}) {
  return (
    <div className="glass-panel overflow-hidden rounded-lg animate-fade-in">
      <div className="overflow-x-auto">
        <table className="w-full min-w-[980px] text-left text-sm">
          <thead className="border-b border-stroke bg-surface/70 text-[11px] uppercase tracking-wide text-content-muted">
            <tr>
              <th className="w-[18%] px-4 py-3">Provider</th>
              <th className="w-[20%] px-4 py-3">Label</th>
              <th className="w-[16%] px-4 py-3">API Key</th>
              <th className="w-[18%] px-4 py-3">Base URL</th>
              <th className="w-[10%] px-4 py-3">Priority</th>
              <th className="w-[10%] px-4 py-3">Status</th>
              <th className="w-[18%] px-4 py-3 text-right">Actions</th>
            </tr>
          </thead>
          <tbody>
            {credentials.map((credential) => {
              const testingState = testingMap[credential.id] || "idle";
              const isDeleting = deleteConfirmID === credential.id;

              return (
                <tr key={credential.id} className="border-t border-stroke align-middle transition-colors duration-150 hover:bg-surface/35">
                  <td className="px-4 py-4">
                    <div className="flex items-center gap-2.5">
                      <div className="grid size-8 shrink-0 place-items-center rounded-lg bg-brand-primary/10 text-brand-primary">
                        <Server size={15} />
                      </div>
                      <span className="font-semibold capitalize text-foreground">{credential.provider}</span>
                    </div>
                  </td>
                  <td className="px-4 py-4 font-medium text-foreground truncate max-w-[160px]" title={credential.label}>
                    {credential.label}
                  </td>
                  <td className="px-4 py-4 font-mono text-xs text-content-muted">
                    {credential.configured ? `•••• ${credential.key_suffix || "set"}` : "missing"}
                  </td>
                  <td className="px-4 py-4">
                    {credential.base_url ? (
                      <span className="inline-block max-w-[180px] truncate font-mono text-xs text-content-muted bg-surface/50 px-2 py-0.5 rounded border border-stroke" title={credential.base_url}>
                        {credential.base_url}
                      </span>
                    ) : (
                      <span className="text-xs text-content-muted italic">Default</span>
                    )}
                  </td>
                  <td className="px-4 py-4">
                    <span className="inline-flex items-center rounded-md bg-surface px-2 py-0.5 text-xs font-semibold text-content-muted border border-stroke">
                      P{credential.priority}
                    </span>
                  </td>
                  <td className="px-4 py-4">
                    <span className={`inline-flex items-center gap-1.5 rounded-full border px-2.5 py-0.5 text-xs font-semibold transition-all duration-150 ${
                      testingState === "success" || testingState === "error"
                        ? testStatusClass(testingState)
                        : statusClass(credential.status)
                    }`}>
                      {testingState === "testing" && <Loader2 size={11} className="animate-spin" />}
                      {testingState === "success" && <span className="size-1.5 rounded-full bg-emerald-500 dark:bg-emerald-400 animate-pulse-dot" />}
                      {testingState === "error" && <span className="size-1.5 rounded-full bg-red-500 dark:bg-red-400 animate-pulse-dot" />}
                      {testingState === "idle" && credential.status === "active" && (
                        <span className="size-1.5 rounded-full bg-emerald-500 dark:bg-emerald-400 animate-pulse-dot" />
                      )}
                      {testingState === "idle" && credential.status === "rate_limited" && (
                        <span className="size-1.5 rounded-full bg-amber-500 dark:bg-amber-400 animate-pulse-dot" />
                      )}
                      {testingState === "idle" && credential.status === "disabled" && (
                        <span className="size-1.5 rounded-full bg-content-muted" />
                      )}
                      {testingState === "testing" ? "testing" : testingState === "success" ? "success" : testingState === "error" ? "failure" : credential.status}
                    </span>
                  </td>
                  <td className="px-4 py-4">
                    {isDeleting ? (
                      <div className="flex gap-1.5 justify-end">
                        <button className="rounded bg-danger hover:opacity-90 px-2.5 py-1 text-xs font-bold text-white transition-colors cursor-pointer" onClick={() => onDelete(credential.id)} type="button">
                          Confirm
                        </button>
                        <button className="rounded border border-stroke px-2.5 py-1 text-xs text-foreground hover:bg-surface transition-colors cursor-pointer" onClick={onCancelDelete} type="button">
                          Cancel
                        </button>
                      </div>
                    ) : (
                      <div className="flex justify-end gap-2">
                        <button
                          className={`inline-flex items-center gap-1 rounded border px-2.5 py-1 text-xs font-bold transition-all duration-200 cursor-pointer ${testClass(testingState)}`}
                          disabled={testingState === "testing"}
                          onClick={() => onTest(credential.id)}
                          type="button"
                        >
                          {testingState === "testing" && <Loader2 size={12} className="animate-spin" />}
                          {testingState === "success" && <CheckCircle2 size={12} />}
                          {testingState === "error" && <XCircle size={12} />}
                          {testingState === "testing" ? "Testing" : testingState === "success" ? "OK" : testingState === "error" ? "Failed" : "Test"}
                        </button>
                        <button
                          className="rounded p-1.5 text-content-muted transition-colors duration-150 hover:bg-danger/10 hover:text-danger cursor-pointer"
                          onClick={() => onAskDelete(credential.id)}
                          title="Delete credential"
                          type="button"
                        >
                          <Trash2 size={14} />
                        </button>
                      </div>
                    )}
                  </td>
                </tr>
              );
            })}
          </tbody>
        </table>
      </div>
    </div>
  );
}

function ProviderSummarySkeleton() {
  return (
    <div className="grid gap-3 md:grid-cols-3">
      {[0, 1, 2].map((item) => (
        <div key={item} className="rounded-lg border border-stroke bg-card p-4 shadow-sm">
          <div className="flex items-start justify-between gap-3">
            <div className="w-full space-y-2">
              <div className="skeleton-shimmer h-3 w-32 rounded" />
              <div className="skeleton-shimmer h-7 w-20 rounded" />
              <div className="skeleton-shimmer h-3 w-44 rounded" />
            </div>
            <div className="skeleton-shimmer size-9 shrink-0 rounded-lg" />
          </div>
        </div>
      ))}
    </div>
  );
}

function ProviderTableSkeleton() {
  return (
    <div className="glass-panel overflow-hidden rounded-lg animate-fade-in">
      <div className="overflow-x-auto">
        <table className="w-full min-w-[980px] text-left text-sm">
          <thead className="border-b border-stroke bg-surface/70 text-[11px] uppercase tracking-wide text-content-muted">
            <tr>
              <th className="w-[30%] px-4 py-3">Provider</th>
              <th className="w-[12%] px-4 py-3">Status</th>
              <th className="w-[24%] px-4 py-3">Models</th>
              <th className="w-[34%] px-4 py-3">Credentials</th>
            </tr>
          </thead>
          <tbody>
            {PROVIDERS.map((provider) => (
              <tr key={provider} className="border-t border-stroke">
                <td className="px-4 py-4">
                  <div className="flex items-start gap-3">
                    <div className="skeleton-shimmer size-10 shrink-0 rounded-lg" />
                    <div className="min-w-0 flex-1 space-y-2">
                      <div className="skeleton-shimmer h-5 w-28 rounded" />
                      <div className="skeleton-shimmer h-3 w-full max-w-[300px] rounded" />
                    </div>
                  </div>
                </td>
                <td className="px-4 py-4">
                  <div className="skeleton-shimmer h-6 w-16 rounded-full" />
                </td>
                <td className="px-4 py-4">
                  <div className="flex flex-wrap gap-1.5">
                    <div className="skeleton-shimmer h-5 w-24 rounded-full" />
                    <div className="skeleton-shimmer h-5 w-28 rounded-full" />
                    <div className="skeleton-shimmer h-5 w-20 rounded-full" />
                  </div>
                </td>
                <td className="px-4 py-4">
                  <div className="skeleton-shimmer h-16 rounded-md" />
                </td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>
    </div>
  );
}

function statusClass(status: ProviderCredential["status"]) {
  switch (status) {
    case "active":
      return "bg-emerald-500/10 text-emerald-700 dark:text-emerald-300 border-emerald-500/20";
    case "rate_limited":
      return "bg-amber-500/10 text-amber-700 dark:text-amber-300 border-amber-500/20";
    case "disabled":
      return "bg-surface text-content-muted border-stroke";
  }
}

function testClass(state: "idle" | "testing" | "success" | "error") {
  switch (state) {
    case "success":
      return "border-emerald-500/30 bg-emerald-500/10 text-emerald-700 dark:text-emerald-300 hover:bg-emerald-500/20";
    case "error":
      return "border-red-500/30 bg-red-500/10 text-red-700 dark:text-red-300 hover:bg-red-500/20";
    default:
      return "border-stroke text-foreground hover:bg-surface";
  }
}

function testKeyButtonClass(state: "idle" | "testing" | "success" | "error") {
  switch (state) {
    case "success":
      return "border-emerald-500/30 bg-emerald-500/10 text-emerald-700 hover:bg-emerald-500/20 dark:text-emerald-300";
    case "error":
      return "border-red-500/30 bg-red-500/10 text-red-700 hover:bg-red-500/20 dark:text-red-300";
    case "testing":
      return "border-brand-primary/30 bg-brand-primary-muted text-brand-primary";
    default:
      return "border-stroke text-foreground hover:bg-surface";
  }
}

function testStatusClass(state: "success" | "error") {
  switch (state) {
    case "success":
      return "bg-emerald-500/10 text-emerald-700 dark:text-emerald-300 border-emerald-500/20";
    case "error":
      return "bg-red-500/10 text-red-700 dark:text-red-300 border-red-500/20";
  }
}
