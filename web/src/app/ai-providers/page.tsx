"use client";

import { FormEvent, useMemo, useState } from "react";
import useSWR from "swr";
import { Cpu, Plus } from "lucide-react";
import { Toaster, toast } from "sonner";
import { DashboardLayout } from "@/components/dashboard/dashboard-layout";
import { PROVIDERS } from "@/lib/model-options";
import { api, ApiError } from "@/lib/api";
import { useSession } from "@/lib/session";
import type { ProviderCredential } from "@/lib/types";

import { ProviderSummarySkeleton, ProviderTableSkeleton } from "./components/ProviderSkeletons";
import { ProviderSummary } from "./components/ProviderSummary";
import { ProviderCredentialsTable } from "./components/ProviderCredentialsTable";
import { AddApiCredentialModal } from "./components/AddApiCredentialModal";
import { CliAuthModal } from "./components/CliAuthModal";
import { generatedCredentialLabel } from "./components/AddApiCredentialModal";
import { AddModelModal } from "./components/AddModelModal";
import { ModelRoutingRules } from "./components/ModelRoutingRules";
import { TestCliModal } from "./components/TestCliModal";
import { GlobalRoutingPanel } from "./components/GlobalRoutingPanel";

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

export default function AIProvidersPage() {
  const session = useSession();
  const token = session?.token ?? "";
  const orgID = session?.user.org_id ?? "";

  const [activeTab, setActiveTab] = useState<"api" | "cli" | "global">("api");
  
  const [form, setForm] = useState<FormState>(initialForm);
  const [formError, setFormError] = useState("");
  const [isSubmitting, setIsSubmitting] = useState(false);
  const [saveState, setSaveState] = useState<"idle" | "saved">("idle");
  const [draftTestState, setDraftTestState] = useState<"idle" | "testing" | "success" | "error">("idle");
  const [testingMap, setTestingMap] = useState<Record<string, "idle" | "testing" | "success" | "error">>({});
  const [deleteConfirmID, setDeleteConfirmID] = useState<string | null>(null);
  const [showApiKey, setShowApiKey] = useState(false);
  
  const [isAddApiOpen, setIsAddApiOpen] = useState(false);
  const [selectedCliProvider, setSelectedCliProvider] = useState<string | null>(null);
  const [reauthCliCredential, setReauthCliCredential] = useState<ProviderCredential | null>(null);
  const [selectedProvider, setSelectedProvider] = useState<string>("openai");
  
  const [isAddModelOpen, setIsAddModelOpen] = useState(false);
  const [addModelLevel, setAddModelLevel] = useState<"fast" | "balanced" | "powerful">("fast");

  const [testingCliCredentialID, setTestingCliCredentialID] = useState<string | null>(null);
  const [testingCliProvider, setTestingCliProvider] = useState<string | null>(null);

  const handleSetForm: React.Dispatch<React.SetStateAction<FormState>> = (update) => {
    setForm(update);
    setDraftTestState("idle");
    setFormError("");
  };

  const { data: credentials = [], error, mutate, isLoading } = useSWR(
    orgID && token ? ["provider-credentials", orgID] : null,
    () => api.listProviderCredentials(orgID, token),
  );

  const { data: providerModels = [], mutate: mutateModels } = useSWR(
    orgID && token ? ["provider-models", orgID] : null,
    () => api.listProviderModels(orgID, token),
  );

  const activeTabCredentials = useMemo(() => {
    return credentials.filter((credential) => 
      activeTab === "api" ? !credential.provider.startsWith("cli:") : credential.provider.startsWith("cli:")
    );
  }, [credentials, activeTab]);

  const credentialsByProvider = useMemo(() => {
    return activeTabCredentials.reduce<Record<string, ProviderCredential[]>>((groups, credential) => {
      groups[credential.provider] = groups[credential.provider] || [];
      groups[credential.provider].push(credential);
      return groups;
    }, {});
  }, [activeTabCredentials]);

  const configuredProviderCount = useMemo(() => {
    const validProviders = activeTab === "api" ? PROVIDERS.filter(p => p !== "gateway" && !p.startsWith("cli:")) : PROVIDERS.filter(p => p.startsWith("cli:"));
    return validProviders.filter((provider) => (credentialsByProvider[provider]?.length || 0) > 0).length;
  }, [credentialsByProvider, activeTab]);

  const activeCredentialCount = useMemo(() => {
    return activeTabCredentials.filter((credential) => credential.status === "active").length;
  }, [activeTabCredentials]);

  async function handleAddCredential(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    if (!token || !orgID) return;
    if (!form.apiKey.trim()) {
      setFormError(activeTab === "api" ? "API key is required." : "Session data is required.");
      return;
    }
    if (activeTab === "api" && draftTestState !== "success") {
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
        setIsAddApiOpen(false);
      }, 600);
    } catch (err) {
      const message = err instanceof ApiError ? err.message : "Failed to save provider credential.";
      setFormError(message);
    } finally {
      setIsSubmitting(false);
    }
  }

  async function handleSaveCliCredential(provider: string, label: string, apiKey: string): Promise<string | null> {
    if (!token || !orgID) return null;
    try {
      await api.createProviderCredential(orgID, token, {
        provider,
        label,
        api_key: apiKey.trim(),
        priority: 0,
      });
      setSelectedCliProvider(null);
      toast.success("CLI Profile added successfully");
      Promise.all([mutate(), mutateModels()]).catch(err => {
        console.error("Failed to mutate after saving CLI profile:", err);
      });
      return null;
    } catch (err) {
      const message = err instanceof ApiError ? err.message : "Failed to save CLI profile.";
      // If it's a conflict (duplicate label), return the error so the modal
      // can show it inline and let the user rename — do NOT close the modal.
      if (err instanceof ApiError && err.status === 409) {
        return message;
      }
      toast.error(message);
      setSelectedCliProvider(null);
      return null;
    }
  }

  async function handleReauthCliCredential(provider: string, label: string, apiKey: string) {
    if (!token || !orgID || !reauthCliCredential) return "Missing required data.";
    try {
      await api.updateProviderCredential(orgID, reauthCliCredential.id, token, {
        label,
        api_key: apiKey.trim(),
        priority: reauthCliCredential.priority,
        status: "active",
      });
      setReauthCliCredential(null);
      toast.success("CLI Profile updated successfully");
      Promise.all([mutate(), mutateModels()]).catch(err => {
        console.error("Failed to mutate after updating CLI profile:", err);
      });
      return null;
    } catch (err) {
      const message = err instanceof ApiError ? err.message : "Failed to update CLI profile.";
      if (err instanceof ApiError && err.status === 409) {
        return message;
      }
      toast.error(message);
      setReauthCliCredential(null);
      return null;
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
    
    // Find the credential
    const cred = credentials.find((c: ProviderCredential) => c.id === credentialID);
    if (cred && cred.provider.startsWith("cli:")) {
      setTestingCliCredentialID(credentialID);
      setTestingCliProvider(cred.provider);
      return;
    }

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

  async function handleAddModelSubmit(name: string, priority: number) {
    if (!token || !orgID || !name) return;
    try {
      await api.createProviderModel(orgID, token, {
        provider: selectedProvider,
        level_group: addModelLevel,
        model_name: name,
        priority,
      });
      await mutateModels();
      toast.success("Provider model added successfully.");
    } catch (err) {
      const message = err instanceof ApiError ? err.message : "Failed to add model.";
      toast.error(message);
      throw err;
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
      <div className="mb-4 flex flex-col gap-4 sm:flex-row sm:items-start sm:justify-between">
        <div className="flex flex-col gap-1">
          <h2 className="text-2xl font-semibold text-foreground">AI Providers</h2>
          <p className="text-sm text-content-muted">
            Manage encrypted provider credential pools used by the Unified AI Gateway.
          </p>
        </div>
        <div className="flex gap-2">
          {activeTab === "global" ? null : activeTab === "api" ? (
            <button
              type="button"
              disabled={!token || !orgID}
              onClick={() => {
                setFormError("");
                setSaveState("idle");
                setDraftTestState("idle");
                setForm((prev) => ({
                  ...prev,
                  provider: "openai",
                  label: generatedCredentialLabel("openai"),
                  apiKey: "",
                  baseURL: "",
                }));
                setIsAddApiOpen(true);
              }}
              className="inline-flex items-center justify-center gap-2 rounded-md bg-brand-primary px-4 py-2.5 text-sm font-semibold text-white transition hover:opacity-90 disabled:cursor-not-allowed disabled:opacity-50"
            >
              <Plus size={16} />
              Add API Key
            </button>
          ) : (
            <div className="flex gap-2">
              <button
                type="button"
                disabled={!token || !orgID}
                onClick={() => setSelectedCliProvider("cli:claude")}
                className="inline-flex items-center justify-center gap-2 rounded-md border border-brand-primary/30 bg-brand-primary/10 px-4 py-2 text-sm font-semibold text-brand-primary transition hover:bg-brand-primary/20 disabled:cursor-not-allowed disabled:opacity-50"
              >
                + Connect Claude CLI
              </button>
              <button
                type="button"
                disabled={!token || !orgID}
                onClick={() => setSelectedCliProvider("cli:codex")}
                className="inline-flex items-center justify-center gap-2 rounded-md border border-brand-primary/30 bg-brand-primary/10 px-4 py-2 text-sm font-semibold text-brand-primary transition hover:bg-brand-primary/20 disabled:cursor-not-allowed disabled:opacity-50"
              >
                + Connect Codex CLI
              </button>
              <button
                type="button"
                disabled={!token || !orgID}
                onClick={() => setSelectedCliProvider("cli:antigravity")}
                className="inline-flex items-center justify-center gap-2 rounded-md border border-brand-primary/30 bg-brand-primary/10 px-4 py-2 text-sm font-semibold text-brand-primary transition hover:bg-brand-primary/20 disabled:cursor-not-allowed disabled:opacity-50"
              >
                + Connect Antigravity CLI
              </button>
            </div>
          )}
        </div>
      </div>

      <div className="mb-6 border-b border-stroke flex items-center gap-6">
        <button
          onClick={() => setActiveTab("api")}
          className={`pb-3 text-sm font-medium transition-colors border-b-2 ${activeTab === "api" ? "border-brand-primary text-foreground" : "border-transparent text-content-muted hover:text-foreground"}`}
        >
          API Connections
        </button>
        <button
          onClick={() => setActiveTab("cli")}
          className={`pb-3 text-sm font-medium transition-colors border-b-2 ${activeTab === "cli" ? "border-brand-primary text-foreground" : "border-transparent text-content-muted hover:text-foreground"}`}
        >
          CLI Profiles
        </button>
        <button
          onClick={() => setActiveTab("global")}
          className={`pb-3 text-sm font-medium transition-colors border-b-2 ${activeTab === "global" ? "border-brand-primary text-foreground" : "border-transparent text-content-muted hover:text-foreground"}`}
        >
          Global Routing
        </button>
      </div>

      {activeTab === "global" ? (
        <GlobalRoutingPanel />
      ) : (
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
          ) : activeTabCredentials.length === 0 ? (
            <div className="flex flex-col items-center justify-center rounded-lg border border-dashed border-stroke bg-card p-12 text-center animate-fade-in">
              <div className="grid size-12 place-items-center rounded-xl bg-surface text-brand-primary">
                <Cpu size={24} />
              </div>
              <h3 className="mt-4 font-semibold text-foreground">
                {activeTab === "api" ? "No API keys configured" : "No CLI profiles configured"}
              </h3>
              <p className="mt-2 max-w-sm text-sm text-content-muted font-normal leading-relaxed">
                {activeTab === "api" 
                  ? "Add an API key for OpenAI, Anthropic, Gemini, or a custom router to enable LLM access for your agents."
                  : "Add CLI authentication profiles for Claude CLI or Cursor to enable sandbox execution."}
              </p>
              <div className="mt-5 flex gap-3 justify-center">
                {activeTab === "api" ? (
                  <button
                    onClick={() => {
                      setFormError("");
                      setSaveState("idle");
                      setDraftTestState("idle");
                      setForm((prev) => ({
                        ...prev,
                        provider: "openai",
                        label: generatedCredentialLabel("openai"),
                        apiKey: "",
                      }));
                      setIsAddApiOpen(true);
                    }}
                    className="flex items-center justify-center gap-2 rounded-md bg-brand-primary px-4 py-2 text-sm font-semibold text-white transition hover:opacity-90 cursor-pointer shadow-[0_0_15px_rgba(34,197,94,0.15)]"
                    type="button"
                  >
                    <Plus size={16} />
                    Add API Key
                  </button>
                ) : (
                  <div className="flex flex-col gap-2 sm:flex-row">
                    <button
                      type="button"
                      disabled={!token || !orgID}
                      onClick={() => setSelectedCliProvider("cli:claude")}
                      className="inline-flex items-center justify-center gap-2 rounded-md border border-brand-primary/30 bg-brand-primary/10 px-4 py-2 text-sm font-semibold text-brand-primary transition hover:bg-brand-primary/20 disabled:cursor-not-allowed disabled:opacity-50 cursor-pointer"
                    >
                      + Connect Claude CLI
                    </button>
                    <button
                      type="button"
                      disabled={!token || !orgID}
                      onClick={() => setSelectedCliProvider("cli:codex")}
                      className="inline-flex items-center justify-center gap-2 rounded-md border border-brand-primary/30 bg-brand-primary/10 px-4 py-2 text-sm font-semibold text-brand-primary transition hover:bg-brand-primary/20 disabled:cursor-not-allowed disabled:opacity-50 cursor-pointer"
                    >
                      + Connect Codex CLI
                    </button>
                    <button
                      type="button"
                      disabled={!token || !orgID}
                      onClick={() => setSelectedCliProvider("cli:antigravity")}
                      className="inline-flex items-center justify-center gap-2 rounded-md border border-brand-primary/30 bg-brand-primary/10 px-4 py-2 text-sm font-semibold text-brand-primary transition hover:bg-brand-primary/20 disabled:cursor-not-allowed disabled:opacity-50 cursor-pointer"
                    >
                      + Connect Antigravity CLI
                    </button>
                  </div>
                )}
              </div>
            </div>
          ) : (
            <>
              <ProviderSummary
                configuredProviderCount={configuredProviderCount}
                totalCredentialCount={activeTabCredentials.length}
                activeCredentialCount={activeCredentialCount}
                activeTab={activeTab}
              />
              <ProviderCredentialsTable
                credentials={activeTabCredentials}
                testingMap={testingMap}
                deleteConfirmID={deleteConfirmID}
                onTest={testCredential}
                onReauth={(cred) => setReauthCliCredential(cred)}
                onAskDelete={setDeleteConfirmID}
                onCancelDelete={() => setDeleteConfirmID(null)}
                onDelete={deleteCredential}
                activeTab={activeTab}
              />
            </>
          )}
        </section>

        {/* Model level group config - Only show for API */}
        {activeTab === "api" && activeTabCredentials.length > 0 && (
          <ModelRoutingRules
            selectedProvider={selectedProvider}
            setSelectedProvider={setSelectedProvider}
            providerModels={providerModels}
            credentialsByProvider={credentialsByProvider}
            onAdjustPriority={adjustModelPriority}
            onToggleActive={toggleModelActive}
            onDeleteModel={deleteModel}
            onOpenAddModel={(level) => {
              setAddModelLevel(level);
              setIsAddModelOpen(true);
            }}
          />
        )}
      </div>
      )}

      {isAddApiOpen && (
        <AddApiCredentialModal
          form={form}
          formError={formError}
          isSubmitting={isSubmitting}
          saveState={saveState}
          draftTestState={draftTestState}
          showApiKey={showApiKey}
          token={token}
          orgID={orgID}
          onClose={() => setIsAddApiOpen(false)}
          onSubmit={handleAddCredential}
          onSetForm={handleSetForm}
          onTestKey={testDraftCredential}
          onToggleApiKey={() => setShowApiKey(!showApiKey)}
        />
      )}

      {selectedCliProvider && (
        <CliAuthModal
          provider={selectedCliProvider}
          token={token}
          orgID={orgID}
          existingLabels={credentials
            .filter((c: ProviderCredential) => c.provider === selectedCliProvider)
            .map((c: ProviderCredential) => c.label as string)}
          onClose={() => setSelectedCliProvider(null)}
          onSave={handleSaveCliCredential}
        />
      )}

      {reauthCliCredential && (
        <CliAuthModal
          provider={reauthCliCredential.provider}
          token={token}
          orgID={orgID}
          existingLabels={credentials
            .filter((c: ProviderCredential) => c.provider === reauthCliCredential.provider && c.id !== reauthCliCredential.id)
            .map((c: ProviderCredential) => c.label as string)}
          defaultLabelOverride={reauthCliCredential.label}
          isReauth={true}
          onClose={() => setReauthCliCredential(null)}
          onSave={handleReauthCliCredential}
        />
      )}

      {isAddModelOpen && (
        <AddModelModal
          level={addModelLevel}
          onClose={() => setIsAddModelOpen(false)}
          onSubmit={handleAddModelSubmit}
        />
      )}

      {testingCliCredentialID && testingCliProvider && (
        <TestCliModal
          isOpen={true}
          onClose={() => {
            setTestingCliCredentialID(null);
            setTestingCliProvider(null);
          }}
          orgID={orgID}
          token={token}
          credentialID={testingCliCredentialID}
          provider={testingCliProvider}
        />
      )}
    </DashboardLayout>
  );
}
