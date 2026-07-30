"use client";

import { FormEvent, useState } from "react";
import { toast } from "sonner";
import { Terminal } from "lucide-react";
import { api, ApiError } from "@/lib/api";
import { useSession } from "@/lib/session";
import { useAuthedSWR } from "@/lib/use-authed-swr";
import type { ExecutionProviderConfig } from "@/lib/types";
import { Card, CardHeader, CardContent } from "@/components/ui/card";
import { Button } from "@/components/ui/button";
import { ExecutionProvidersList } from "@/components/projects/execution-providers-list";

// GlobalRoutingPanel configures Organization.default_execution_providers —
// the org-wide fallback priority list used by any project that hasn't
// configured its own execution_providers (and isn't explicitly on the
// legacy single-engine "cli" path). See
// docs/openspecs/global-execution-providers.
export function GlobalRoutingPanel() {
  const session = useSession();
  const token = session?.token ?? "";
  const orgID = session?.user.org_id ?? "";
  // PATCH /organizations/{orgID} is admin-only server-side (RequireRole) —
  // mirror that client-side (same pattern as knowledge/page.tsx's isAdmin)
  // so a non-admin sees the current global default read-only instead of
  // filling out a form that fails with 403 on save.
  const isAdmin = session?.user.role === "admin";

  // Same cache key as organization/page.tsx's getOrganization call, so the
  // two pages share one fetch/cache entry instead of duplicating it.
  const { data: organization, mutate } = useAuthedSWR(
    orgID ? ["org", orgID] : null,
    (authToken) => api.getOrganization(orgID, authToken),
  );

  const [providers, setProviders] = useState<ExecutionProviderConfig[]>([]);
  const [isSaving, setIsSaving] = useState(false);
  const [saveError, setSaveError] = useState("");

  // Render-phase prop synchronization (matches project-profile.tsx's
  // pattern) to avoid cascading renders from a useEffect setState.
  const [prevOrganization, setPrevOrganization] = useState(organization);
  if (organization !== prevOrganization) {
    setPrevOrganization(organization);
    if (organization) {
      setProviders(organization.default_execution_providers ?? []);
    }
  }

  const isDirty = JSON.stringify(organization?.default_execution_providers ?? []) !== JSON.stringify(providers);

  async function handleSave(e: FormEvent) {
    e.preventDefault();
    if (!token || !orgID) return;
    setIsSaving(true);
    setSaveError("");
    try {
      await api.updateOrganization(orgID, token, { default_execution_providers: providers });
      await mutate();
      toast.success("Global routing defaults updated.");
    } catch (err) {
      const message = err instanceof ApiError ? err.message : "Failed to update global routing defaults.";
      setSaveError(message);
      toast.error(message);
    } finally {
      setIsSaving(false);
    }
  }

  return (
    <div className="max-w-3xl space-y-6">
      <form onSubmit={handleSave}>
        <Card>
          <CardHeader
            title="Global Routing"
            icon={<Terminal size={18} className="text-brand-primary" />}
          />
          <CardContent className="space-y-4">
            <p className="text-xs text-content-muted">
              This priority-ordered fallback applies to every project in your organization that hasn&apos;t
              configured its own Execution Providers. A project with its own explicit list (or already using the
              legacy CLI engine setting) always takes priority over this default.
            </p>
            {!isAdmin && (
              <p className="text-xs text-warning">
                Only organization admins can change this — showing the current configuration read-only.
              </p>
            )}
            <ExecutionProvidersList value={providers} onChange={setProviders} disabled={isSaving || !isAdmin} />

            {saveError && <p className="text-xs text-danger font-medium">{saveError}</p>}

            {isAdmin && (
              <div className="flex items-center justify-end gap-3 pt-2">
                {isDirty && <span className="text-xs text-warning">Unsaved changes</span>}
                <Button type="submit" disabled={isSaving || !isDirty}>
                  {isSaving ? "Saving..." : "Save Global Routing"}
                </Button>
              </div>
            )}
          </CardContent>
        </Card>
      </form>
    </div>
  );
}
