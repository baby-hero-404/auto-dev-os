"use client";

import { Building, Users, Shield } from "lucide-react";
import useSWR from "swr";
import { DashboardLayout } from "@/components/dashboard/dashboard-layout";
import { useSession } from "@/lib/session";
import { api } from "@/lib/api";
import { Badge } from "@/components/ui/badge";

export default function OrganizationPage() {
  const session = useSession();
  const token = session?.token ?? "";
  const orgID = session?.user.org_id ?? "";

  const { data: org } = useSWR(
    orgID && token ? ["org", orgID, token] : null,
    ([, id, t]) => api.getOrganization(id, t),
  );

  const { data: projects = [] } = useSWR(
    orgID && token ? ["projects", orgID, token] : null,
    ([, id, t]) => api.listProjects(id, t),
  );

  return (
    <DashboardLayout>
      <div className="mb-5">
        <h2 className="font-mono text-2xl font-semibold">Organization</h2>
        <p className="mt-1 text-sm text-[var(--muted)]">
          Manage your organization, members, and API access.
        </p>
      </div>

      <div className="grid gap-4 md:grid-cols-2">
        <div className="rounded-lg border border-[var(--border)] bg-[var(--primary)] p-5">
          <div className="mb-4 flex items-center gap-2">
            <Building size={18} className="text-[var(--accent)]" />
            <h3 className="font-mono font-semibold">Organization Details</h3>
          </div>
          <div className="space-y-3 text-sm">
            <div className="flex justify-between rounded-md border border-[var(--border)] bg-slate-950 p-3">
              <span className="text-[var(--muted)]">Name</span>
              <span className="font-mono">{org?.name ?? "—"}</span>
            </div>
            <div className="flex justify-between rounded-md border border-[var(--border)] bg-slate-950 p-3">
              <span className="text-[var(--muted)]">Org ID</span>
              <span className="font-mono text-xs">{orgID || "—"}</span>
            </div>
            <div className="flex justify-between rounded-md border border-[var(--border)] bg-slate-950 p-3">
              <span className="text-[var(--muted)]">Projects</span>
              <span className="font-mono">{projects.length}</span>
            </div>
            {org?.created_at && (
              <div className="flex justify-between rounded-md border border-[var(--border)] bg-slate-950 p-3">
                <span className="text-[var(--muted)]">Created</span>
                <span className="text-xs">{new Date(org.created_at).toLocaleDateString()}</span>
              </div>
            )}
          </div>
        </div>

        <div className="space-y-4">
          <div className="rounded-lg border border-[var(--border)] bg-[var(--primary)] p-5">
            <div className="mb-3 flex items-center gap-2">
              <Shield size={18} className="text-[var(--accent)]" />
              <h3 className="font-mono font-semibold">Your Account</h3>
            </div>
            <div className="space-y-3 text-sm">
              <div className="flex justify-between rounded-md border border-[var(--border)] bg-slate-950 p-3">
                <span className="text-[var(--muted)]">Email</span>
                <span>{session?.user.email ?? "—"}</span>
              </div>
              <div className="flex justify-between rounded-md border border-[var(--border)] bg-slate-950 p-3">
                <span className="text-[var(--muted)]">Role</span>
                <Badge value={session?.user.role ?? "member"} />
              </div>
            </div>
          </div>

          <div className="rounded-lg border border-dashed border-[var(--border)] bg-[var(--primary)]/50 p-5">
            <div className="mb-3 flex items-center gap-2">
              <Users size={18} className="text-[var(--muted)]" />
              <h3 className="font-mono font-semibold">Members</h3>
            </div>
            <p className="text-sm text-[var(--muted)]">
              Member management with role-based permissions will be available in a future release.
              Currently, the first registered user is assigned the admin role.
            </p>
          </div>
        </div>
      </div>
    </DashboardLayout>
  );
}
