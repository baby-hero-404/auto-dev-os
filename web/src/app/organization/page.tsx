"use client";

import { Building, Users, Shield } from "lucide-react";
import { DashboardLayout } from "@/components/dashboard/dashboard-layout";
import { useSession } from "@/lib/session";
import { api } from "@/lib/api";
import { useAuthedSWR } from "@/lib/use-authed-swr";
import { Badge } from "@/components/ui/badge";

export default function OrganizationPage() {
  const session = useSession();
  const orgID = session?.user.org_id ?? "";

  const { data: org } = useAuthedSWR(
    orgID ? ["org", orgID] : null,
    (token) => api.getOrganization(orgID, token),
  );

  const { data: projects = [] } = useAuthedSWR(
    orgID ? ["projects", orgID] : null,
    (token) => api.listProjects(orgID, token),
  );

  return (
    <DashboardLayout>
      <div className="mb-5">
        <h2 className="font-mono text-2xl font-semibold">Organization</h2>
        <p className="mt-1 text-sm text-content-muted">
          Manage your organization, members, and API access.
        </p>
      </div>

      <div className="grid gap-4 md:grid-cols-2">
        <div className="rounded-lg border border-stroke bg-panel p-5">
          <div className="mb-4 flex items-center gap-2">
            <Building size={18} className="text-brand-primary" />
            <h3 className="font-mono font-semibold">Organization Details</h3>
          </div>
          <div className="space-y-3 text-sm">
            <div className="flex justify-between rounded-md border border-stroke bg-slate-950 p-3">
              <span className="text-content-muted">Name</span>
              <span className="font-mono">{org?.name ?? "—"}</span>
            </div>
            <div className="flex justify-between rounded-md border border-stroke bg-slate-950 p-3">
              <span className="text-content-muted">Org ID</span>
              <span className="font-mono text-xs">{orgID || "—"}</span>
            </div>
            <div className="flex justify-between rounded-md border border-stroke bg-slate-950 p-3">
              <span className="text-content-muted">Projects</span>
              <span className="font-mono">{projects.length}</span>
            </div>
            {org?.created_at && (
              <div className="flex justify-between rounded-md border border-stroke bg-slate-950 p-3">
                <span className="text-content-muted">Created</span>
                <span className="text-xs">{new Date(org.created_at).toLocaleDateString()}</span>
              </div>
            )}
          </div>
        </div>

        <div className="space-y-4">
          <div className="rounded-lg border border-stroke bg-panel p-5">
            <div className="mb-3 flex items-center gap-2">
              <Shield size={18} className="text-brand-primary" />
              <h3 className="font-mono font-semibold">Your Account</h3>
            </div>
            <div className="space-y-3 text-sm">
              <div className="flex justify-between rounded-md border border-stroke bg-slate-950 p-3">
                <span className="text-content-muted">Email</span>
                <span>{session?.user.email ?? "—"}</span>
              </div>
              <div className="flex justify-between rounded-md border border-stroke bg-slate-950 p-3">
                <span className="text-content-muted">Role</span>
                <Badge value={session?.user.role ?? "member"} />
              </div>
            </div>
          </div>

          <div className="rounded-lg border border-dashed border-stroke bg-panel/50 p-5">
            <div className="mb-3 flex items-center gap-2">
              <Users size={18} className="text-content-muted" />
              <h3 className="font-mono font-semibold">Members</h3>
            </div>
            <p className="text-sm text-content-muted">
              Member management with role-based permissions will be available in a future release.
              Currently, the first registered user is assigned the admin role.
            </p>
          </div>
        </div>
      </div>
    </DashboardLayout>
  );
}
