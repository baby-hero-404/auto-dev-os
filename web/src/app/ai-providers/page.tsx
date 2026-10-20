"use client";

import { useMemo, useState } from "react";
import useSWR from "swr";
import { Cpu } from "lucide-react";
import { Toaster, toast } from "sonner";
import { DashboardLayout } from "@/components/dashboard/dashboard-layout";
import { PROVIDERS } from "@/lib/model-options";
import { api, ApiError } from "@/lib/api";
import { useSession } from "@/lib/session";
import type { ProviderCredential } from "@/lib/types";

import { ProviderSummarySkeleton, ProviderTableSkeleton } from "./components/ProviderSkeletons";
import { ProviderSummary } from "./components/ProviderSummary";
import { ProviderCredentialsTable } from "./components/ProviderCredentialsTable";
import { CliAuthModal } from "./components/CliAuthModal";
import { TestCliModal } from "./components/TestCliModal";
import { GlobalRoutingPanel } from "./components/GlobalRoutingPanel";

export default function AIProvidersPage() {
  const session = useSession();
  const token = session?.token ?? "";
  const orgID = session?.user.org_id ?? "";

  const [activeTab, setActiveTab] = useState<"cli" | "global">("cli");
  const [testingMap, setTestingMap] = useState<Record<string, "idle" | "testing" | "success" | "error">>({});
  const [deleteConfirmID, setDeleteConfirmID] = useState<string | null>(null);

  const [selectedCliProvider, setSelectedCliProvider] = useState<string | null>(null);
  const [reauthCliCredential, setReauthCliCredential] = useState<ProviderCredential | null>(null);

  const [testingCliCredentialID, setTestingCliCredentialID] = useState<string | null>(null);
  const [testingCliProvider, setTestingCliProvider] = useState<string | null>(null);

  const { data: credentials = [], error, mutate, isLoading } = useSWR(
    orgID && token ? ["provider-credentials", orgID] : null,
    () => api.listProviderCredentials(orgID, token),
  );

  const activeTabCredentials = useMemo(() => {
    return credentials.filter((credential) => credential.provider.startsWith("cli:"));
  }, [credentials]);

  const credentialsByProvider = useMemo(() => {
    return activeTabCredentials.reduce<Record<string, ProviderCredential[]>>((groups, credential) => {
      groups[credential.provider] = groups[credential.provider] || [];
      groups[credential.provider].push(credential);
      return groups;
    }, {});
  }, [activeTabCredentials]);

  const configuredProviderCount = useMemo(() => {
    const validProviders = PROVIDERS.filter((p) => p.startsWith("cli:"));
    return validProviders.filter((provider) => (credentialsByProvider[provider]?.length || 0) > 0).length;
  }, [credentialsByProvider]);

  const activeCredentialCount = useMemo(() => {
    return activeTabCredentials.filter((credential) => credential.status === "active").length;
  }, [activeTabCredentials]);

  async function handleSaveCliCredential(provider: string, label: string, authPayload: string): Promise<string | null> {
    if (!token || !orgID) return "Session expired. Please log in again.";
    try {
      await api.createProviderCredential(orgID, token, {
        provider,
        label,
        api_key: authPayload.trim(),
        priority: 0,
      });
      await mutate();
      setSelectedCliProvider(null);
      toast.success(`Successfully connected ${provider.replace("cli:", "")} CLI profile.`);
      return null;
    } catch (err) {
      const message = err instanceof ApiError ? err.message : "Failed to save CLI credential.";
      return message;
    }
  }

  async function handleReauthCliCredential(provider: string, label: string, authPayload: string): Promise<string | null> {
    if (!token || !orgID || !reauthCliCredential) return "Session expired.";
    try {
      await api.updateProviderCredential(orgID, reauthCliCredential.id, token, {
        label,
        api_key: authPayload.trim(),
        status: "active",
      });
      await mutate();
      toast.success("CLI profile re-authenticated successfully.");
      setReauthCliCredential(null);
      return null;
    } catch (err) {
      const message = err instanceof ApiError ? err.message : "Failed to update CLI credential.";
      toast.error(message);
      setReauthCliCredential(null);
      return null;
    }
  }

  async function testCredential(credentialID: string) {
    if (!token || !orgID) return;

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
      toast.success("Profile deleted successfully.");
    } catch (err) {
      const message = err instanceof ApiError ? err.message : "Failed to delete provider credential.";
      toast.error(message);
    }
  }

  return (
    <DashboardLayout>
      <Toaster richColors position="top-right" />
      <div className="mb-4 flex flex-col gap-4 sm:flex-row sm:items-start sm:justify-between">
        <div className="flex flex-col gap-1">
          <h2 className="text-2xl font-semibold text-foreground">AI Providers</h2>
          <p className="text-sm text-content-muted">
            Manage AI CLI execution profiles (Claude Code, Codex, Antigravity) and global routing.
          </p>
        </div>
        <div className="flex gap-2">
          {activeTab !== "global" && (
            <div className="flex flex-wrap gap-2">
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

      <div className="mb-6 border-b border-stroke flex items-center gap-6">
        <button
          onClick={() => setActiveTab("cli")}
          className={`pb-3 text-sm font-medium transition-colors border-b-2 cursor-pointer ${
            activeTab === "cli"
              ? "border-brand-primary text-foreground"
              : "border-transparent text-content-muted hover:text-foreground"
          }`}
        >
          CLI Profiles
        </button>
        <button
          onClick={() => setActiveTab("global")}
          className={`pb-3 text-sm font-medium transition-colors border-b-2 cursor-pointer ${
            activeTab === "global"
              ? "border-brand-primary text-foreground"
              : "border-transparent text-content-muted hover:text-foreground"
          }`}
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
                  No CLI profiles configured
                </h3>
                <p className="mt-2 max-w-sm text-sm text-content-muted font-normal leading-relaxed">
                  Connect a CLI profile (Claude Code, Codex, or Antigravity) to enable AI coding agents for your tasks.
                </p>
                <div className="mt-6 flex flex-wrap justify-center gap-3">
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
              </div>
            ) : (
              <>
                <ProviderSummary
                  configuredProviderCount={configuredProviderCount}
                  totalCredentialCount={activeTabCredentials.length}
                  activeCredentialCount={activeCredentialCount}
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
                />
              </>
            )}
          </section>
        </div>
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
