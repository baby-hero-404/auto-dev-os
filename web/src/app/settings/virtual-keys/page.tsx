"use client";

import { FormEvent, useMemo, useState } from "react";
import { Check, Copy, KeyRound, Loader2, Pencil, Plus, ShieldCheck, Trash2 } from "lucide-react";
import { DashboardLayout } from "@/components/dashboard/dashboard-layout";
import { api, ApiError } from "@/lib/api";
import { useSession } from "@/lib/session";
import { useAuthedSWR } from "@/lib/use-authed-swr";
import type { VirtualKey } from "@/lib/types";

type FormState = {
  name: string;
  projectID: string;
  agentID: string;
  budgetLimit: string;
  rpmLimit: string;
  tpmLimit: string;
  expiresAt: string;
};

type EditState = {
  name: string;
  budgetLimit: string;
  rpmLimit: string;
  tpmLimit: string;
  expiresAt: string;
  status: VirtualKey["status"];
};

const initialForm: FormState = {
  name: "",
  projectID: "",
  agentID: "",
  budgetLimit: "",
  rpmLimit: "",
  tpmLimit: "",
  expiresAt: "",
};

function numberOrUndefined(value: string) {
  if (value.trim() === "") return undefined;
  const parsed = Number(value);
  return Number.isFinite(parsed) ? parsed : undefined;
}

function dateOrUndefined(value: string) {
  if (!value) return undefined;
  return new Date(value).toISOString();
}

function datetimeLocalValue(value?: string) {
  if (!value) return "";
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) return "";
  return date.toISOString().slice(0, 16);
}

function formatCurrency(value?: number) {
  if (value === undefined || value === null) return "No cap";
  return new Intl.NumberFormat("en-US", { style: "currency", currency: "USD", maximumFractionDigits: 4 }).format(value);
}

function formatDate(value?: string) {
  if (!value) return "No expiry";
  return new Intl.DateTimeFormat("en-US", { dateStyle: "medium", timeStyle: "short" }).format(new Date(value));
}

export default function VirtualKeysPage() {
  const session = useSession();
  const token = session?.token ?? "";
  const orgID = session?.user.org_id ?? "";
  const [form, setForm] = useState<FormState>(initialForm);
  const [formError, setFormError] = useState("");
  const [isSubmitting, setIsSubmitting] = useState(false);
  const [createdKey, setCreatedKey] = useState("");
  const [copied, setCopied] = useState(false);
  const [editingID, setEditingID] = useState("");
  const [editForm, setEditForm] = useState<EditState | null>(null);
  const [savingID, setSavingID] = useState("");
  const [revokeID, setRevokeID] = useState("");

  const { data: keys = [], mutate: mutateKeys, isLoading, error } = useAuthedSWR(
    orgID ? ["virtual-keys", orgID] : null,
    (t) => api.listVirtualKeys(orgID, t),
  );
  const { data: projects = [] } = useAuthedSWR(
    orgID ? ["projects", orgID] : null,
    (t) => api.listProjects(orgID, t),
  );
  const { data: agents = [] } = useAuthedSWR(
    orgID ? ["org-agents", orgID] : null,
    (t) => api.listOrgAgents(orgID, t),
  );

  const totals = useMemo(() => {
    return keys.reduce(
      (acc, key) => ({
        active: acc.active + (key.status === "active" ? 1 : 0),
        budget: acc.budget + (key.budget_limit_usd || 0),
        used: acc.used + key.budget_used_usd,
      }),
      { active: 0, budget: 0, used: 0 },
    );
  }, [keys]);

  async function createKey(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    if (!token || !orgID) return;
    if (!form.name.trim()) {
      setFormError("Name is required.");
      return;
    }
    setFormError("");
    setCreatedKey("");
    setIsSubmitting(true);
    try {
      const created = await api.createVirtualKey(orgID, token, {
        name: form.name.trim(),
        project_id: form.projectID || undefined,
        agent_id: form.agentID || undefined,
        budget_limit_usd: numberOrUndefined(form.budgetLimit),
        rpm_limit: numberOrUndefined(form.rpmLimit),
        tpm_limit: numberOrUndefined(form.tpmLimit),
        expires_at: dateOrUndefined(form.expiresAt),
      });
      setCreatedKey(created.key);
      setCopied(false);
      setForm(initialForm);
      await mutateKeys();
    } catch (err) {
      setFormError(err instanceof ApiError ? err.message : "Failed to create virtual key.");
    } finally {
      setIsSubmitting(false);
    }
  }

  async function copyCreatedKey() {
    if (!createdKey) return;
    await navigator.clipboard.writeText(createdKey);
    setCopied(true);
  }

  function startEdit(key: VirtualKey) {
    setEditingID(key.id);
    setEditForm({
      name: key.name,
      budgetLimit: key.budget_limit_usd?.toString() || "",
      rpmLimit: key.rpm_limit?.toString() || "",
      tpmLimit: key.tpm_limit?.toString() || "",
      expiresAt: datetimeLocalValue(key.expires_at),
      status: key.status,
    });
  }

  async function saveKey(keyID: string) {
    if (!token || !orgID || !editForm) return;
    setSavingID(keyID);
    setFormError("");
    try {
      await api.updateVirtualKey(orgID, keyID, token, {
        name: editForm.name.trim() || undefined,
        budget_limit_usd: numberOrUndefined(editForm.budgetLimit),
        rpm_limit: numberOrUndefined(editForm.rpmLimit),
        tpm_limit: numberOrUndefined(editForm.tpmLimit),
        status: editForm.status,
        expires_at: dateOrUndefined(editForm.expiresAt),
      });
      setEditingID("");
      setEditForm(null);
      await mutateKeys();
    } catch (err) {
      setFormError(err instanceof ApiError ? err.message : "Failed to update virtual key.");
    } finally {
      setSavingID("");
    }
  }

  async function revokeKey(keyID: string) {
    if (!token || !orgID) return;
    setSavingID(keyID);
    setFormError("");
    try {
      await api.revokeVirtualKey(orgID, keyID, token);
      setRevokeID("");
      await mutateKeys();
    } catch (err) {
      setFormError(err instanceof ApiError ? err.message : "Failed to revoke virtual key.");
    } finally {
      setSavingID("");
    }
  }

  return (
    <DashboardLayout>
      <div className="mb-6 flex flex-col gap-2 md:flex-row md:items-end md:justify-between">
        <div>
          <h2 className="font-mono text-2xl font-semibold">Virtual Keys</h2>
          <p className="mt-1 text-sm text-content-muted">Create sk-aco keys with budget, rate limits, and optional project or agent scope.</p>
        </div>
        <div className="rounded-full border border-stroke bg-panel px-3 py-1 text-xs text-content-muted">Gateway budget controls</div>
      </div>

      <div className="mb-5 grid gap-4 md:grid-cols-3">
        <Metric label="Active keys" value={totals.active.toString()} />
        <Metric label="Budget cap" value={totals.budget > 0 ? formatCurrency(totals.budget) : "Uncapped"} />
        <Metric label="Spend used" value={formatCurrency(totals.used)} />
      </div>

      {formError && <div className="mb-5 rounded-lg border border-red-500/20 bg-red-500/10 p-4 text-sm text-red-300">{formError}</div>}

      {createdKey && (
        <section className="mb-5 rounded-lg border border-emerald-500/30 bg-emerald-500/10 p-4">
          <div className="mb-2 flex items-center gap-2 text-sm font-semibold text-emerald-200">
            <ShieldCheck size={16} />
            Copy this key now. It will not be shown again.
          </div>
          <div className="flex flex-col gap-2 sm:flex-row">
            <code className="min-w-0 flex-1 rounded border border-emerald-500/20 bg-page px-3 py-2 font-mono text-xs text-emerald-100">{createdKey}</code>
            <button onClick={copyCreatedKey} className="inline-flex items-center justify-center gap-2 rounded-md bg-emerald-400 px-3 py-2 text-sm font-semibold text-slate-950" type="button">
              {copied ? <Check size={15} /> : <Copy size={15} />}
              {copied ? "Copied" : "Copy"}
            </button>
          </div>
        </section>
      )}

      <div className="grid gap-6 xl:grid-cols-[minmax(0,1fr)_380px]">
        <section className="space-y-4">
          {error && <div className="rounded-lg border border-red-500/20 bg-red-500/10 p-4 text-sm text-red-300">Failed to load virtual keys.</div>}
          {isLoading ? (
            <div className="glass-panel rounded-lg p-6 text-sm text-content-muted">Loading virtual keys...</div>
          ) : keys.length === 0 ? (
            <div className="glass-panel rounded-lg p-6 text-sm text-content-muted">No virtual keys created yet.</div>
          ) : (
            keys.map((key) => {
              const isEditing = editingID === key.id && editForm;
              const budgetPct = key.budget_limit_usd && key.budget_limit_usd > 0 ? Math.min(100, (key.budget_used_usd / key.budget_limit_usd) * 100) : 0;
              return (
                <article key={key.id} className="glass-panel rounded-lg p-5">
                  <div className="mb-4 flex flex-col justify-between gap-3 sm:flex-row sm:items-start">
                    <div className="min-w-0">
                      <div className="flex items-center gap-2">
                        <KeyRound size={18} className="text-brand-primary" />
                        {isEditing ? (
                          <input
                            value={editForm.name}
                            onChange={(event) => setEditForm((prev) => prev && { ...prev, name: event.target.value })}
                            className="rounded border border-stroke bg-page px-2 py-1 font-mono text-sm text-white focus:border-brand-primary focus:outline-none"
                          />
                        ) : (
                          <h3 className="font-mono font-semibold text-white">{key.name}</h3>
                        )}
                      </div>
                      <p className="mt-1 font-mono text-xs text-content-muted">{key.key_prefix}</p>
                    </div>
                    <div className="flex items-center gap-2">
                      <span className={`rounded-full px-2.5 py-1 text-xs ${statusClass(key.status)}`}>{key.status}</span>
                      {isEditing ? (
                        <>
                          <button disabled={savingID === key.id} onClick={() => saveKey(key.id)} className="rounded bg-brand-primary px-2 py-1 text-xs font-semibold text-slate-950" type="button">
                            {savingID === key.id ? "Saving" : "Save"}
                          </button>
                          <button onClick={() => { setEditingID(""); setEditForm(null); }} className="rounded border border-stroke px-2 py-1 text-xs text-white" type="button">
                            Cancel
                          </button>
                        </>
                      ) : (
                        <>
                          <button onClick={() => startEdit(key)} className="rounded p-1.5 text-content-muted transition hover:bg-panel hover:text-white" title="Edit key" type="button">
                            <Pencil size={15} />
                          </button>
                          <button onClick={() => setRevokeID(key.id)} className="rounded p-1.5 text-content-muted transition hover:bg-red-950/40 hover:text-red-300" title="Revoke key" type="button">
                            <Trash2 size={15} />
                          </button>
                        </>
                      )}
                    </div>
                  </div>

                  {isEditing ? (
                    <div className="grid gap-3 md:grid-cols-6">
                      <EditField label="Budget" value={editForm.budgetLimit} onChange={(value) => setEditForm((prev) => prev && { ...prev, budgetLimit: value })} />
                      <EditField label="RPM" value={editForm.rpmLimit} onChange={(value) => setEditForm((prev) => prev && { ...prev, rpmLimit: value })} />
                      <EditField label="TPM" value={editForm.tpmLimit} onChange={(value) => setEditForm((prev) => prev && { ...prev, tpmLimit: value })} />
                      <label className="flex min-w-0 flex-col gap-1.5">
                        <span className="font-mono text-[10px] font-bold uppercase tracking-wider text-content-muted">Status</span>
                        <select value={editForm.status} onChange={(event) => setEditForm((prev) => prev && { ...prev, status: event.target.value as VirtualKey["status"] })} className="rounded border border-stroke bg-page px-2 py-1.5 text-xs text-white focus:border-brand-primary focus:outline-none">
                          <option value="active">active</option>
                          <option value="exhausted">exhausted</option>
                          <option value="revoked">revoked</option>
                        </select>
                      </label>
                      <label className="flex min-w-0 flex-col gap-1.5 md:col-span-2">
                        <span className="font-mono text-[10px] font-bold uppercase tracking-wider text-content-muted">Expires</span>
                        <input type="datetime-local" value={editForm.expiresAt} onChange={(event) => setEditForm((prev) => prev && { ...prev, expiresAt: event.target.value })} className="rounded border border-stroke bg-page px-2 py-1.5 text-xs text-white focus:border-brand-primary focus:outline-none" />
                      </label>
                    </div>
                  ) : (
                    <>
                      <div className="mb-3 grid gap-3 md:grid-cols-4">
                        <Info label="Budget" value={`${formatCurrency(key.budget_used_usd)} / ${formatCurrency(key.budget_limit_usd)}`} />
                        <Info label="RPM" value={key.rpm_limit?.toString() || "No cap"} />
                        <Info label="TPM" value={key.tpm_limit?.toString() || "No cap"} />
                        <Info label="Expires" value={formatDate(key.expires_at)} />
                      </div>
                      {key.budget_limit_usd && (
                        <div className="h-2 overflow-hidden rounded-full bg-slate-800">
                          <div className="h-full rounded-full bg-brand-primary" style={{ width: `${budgetPct}%` }} />
                        </div>
                      )}
                    </>
                  )}

                  {revokeID === key.id && (
                    <div className="mt-4 flex flex-col justify-between gap-3 rounded-md border border-red-500/20 bg-red-950/30 p-3 sm:flex-row sm:items-center">
                      <p className="text-xs text-red-100">Revoke this virtual key? Existing clients using it will stop working.</p>
                      <div className="flex gap-2">
                        <button disabled={savingID === key.id} onClick={() => revokeKey(key.id)} className="rounded bg-red-500 px-3 py-1 text-xs font-semibold text-white" type="button">
                          {savingID === key.id ? "Revoking" : "Revoke"}
                        </button>
                        <button onClick={() => setRevokeID("")} className="rounded border border-stroke px-3 py-1 text-xs text-white" type="button">Cancel</button>
                      </div>
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
            <h3 className="font-mono font-semibold text-white">Create Virtual Key</h3>
          </div>
          <form onSubmit={createKey} className="space-y-4">
            <FormField label="Name">
              <input value={form.name} onChange={(event) => setForm((prev) => ({ ...prev, name: event.target.value }))} placeholder="agent-budget-prod" className="rounded-md border border-stroke bg-page px-3 py-2 text-sm text-white focus:border-brand-primary focus:outline-none" />
            </FormField>
            <FormField label="Project Scope">
              <select value={form.projectID} onChange={(event) => setForm((prev) => ({ ...prev, projectID: event.target.value }))} className="rounded-md border border-stroke bg-page px-3 py-2 text-sm text-white focus:border-brand-primary focus:outline-none">
                <option value="">All projects</option>
                {projects.map((project) => <option key={project.id} value={project.id}>{project.name}</option>)}
              </select>
            </FormField>
            <FormField label="Agent Scope">
              <select value={form.agentID} onChange={(event) => setForm((prev) => ({ ...prev, agentID: event.target.value }))} className="rounded-md border border-stroke bg-page px-3 py-2 text-sm text-white focus:border-brand-primary focus:outline-none">
                <option value="">Any agent</option>
                {agents.map((agent) => <option key={agent.id} value={agent.id}>{agent.name} ({agent.role})</option>)}
              </select>
            </FormField>
            <div className="grid gap-3 sm:grid-cols-3">
              <FormField label="Budget USD">
                <input type="number" min="0" step="0.0001" value={form.budgetLimit} onChange={(event) => setForm((prev) => ({ ...prev, budgetLimit: event.target.value }))} className="rounded-md border border-stroke bg-page px-3 py-2 text-sm text-white focus:border-brand-primary focus:outline-none" />
              </FormField>
              <FormField label="RPM">
                <input type="number" min="0" value={form.rpmLimit} onChange={(event) => setForm((prev) => ({ ...prev, rpmLimit: event.target.value }))} className="rounded-md border border-stroke bg-page px-3 py-2 text-sm text-white focus:border-brand-primary focus:outline-none" />
              </FormField>
              <FormField label="TPM">
                <input type="number" min="0" value={form.tpmLimit} onChange={(event) => setForm((prev) => ({ ...prev, tpmLimit: event.target.value }))} className="rounded-md border border-stroke bg-page px-3 py-2 text-sm text-white focus:border-brand-primary focus:outline-none" />
              </FormField>
            </div>
            <FormField label="Expires At">
              <input type="datetime-local" value={form.expiresAt} onChange={(event) => setForm((prev) => ({ ...prev, expiresAt: event.target.value }))} className="rounded-md border border-stroke bg-page px-3 py-2 text-sm text-white focus:border-brand-primary focus:outline-none" />
            </FormField>
            <button disabled={isSubmitting || !token || !orgID} className="inline-flex w-full items-center justify-center gap-2 rounded-md bg-brand-primary px-4 py-2 font-mono text-xs font-bold uppercase tracking-wider text-black transition hover:bg-brand-primary/90 disabled:cursor-not-allowed disabled:opacity-50" type="submit">
              {isSubmitting && <Loader2 size={14} className="animate-spin" />}
              Create Virtual Key
            </button>
          </form>
        </aside>
      </div>
    </DashboardLayout>
  );
}

function Metric({ label, value }: { label: string; value: string }) {
  return (
    <article className="glass-panel rounded-lg p-4">
      <div className="font-mono text-lg font-semibold text-white">{value}</div>
      <div className="text-xs text-content-muted">{label}</div>
    </article>
  );
}

function Info({ label, value }: { label: string; value: string }) {
  return (
    <div className="rounded-md border border-stroke bg-page px-3 py-2">
      <div className="font-mono text-[10px] font-bold uppercase tracking-wider text-content-muted">{label}</div>
      <div className="mt-1 truncate text-sm text-white">{value}</div>
    </div>
  );
}

function FormField({ label, children }: { label: string; children: React.ReactNode }) {
  return (
    <label className="flex min-w-0 flex-col gap-1.5">
      <span className="font-mono text-xs font-bold uppercase tracking-wider text-content-muted">{label}</span>
      {children}
    </label>
  );
}

function EditField({ label, value, onChange }: { label: string; value: string; onChange: (value: string) => void }) {
  return (
    <label className="flex min-w-0 flex-col gap-1.5">
      <span className="font-mono text-[10px] font-bold uppercase tracking-wider text-content-muted">{label}</span>
      <input type="number" min="0" step="0.0001" value={value} onChange={(event) => onChange(event.target.value)} className="rounded border border-stroke bg-page px-2 py-1.5 text-xs text-white focus:border-brand-primary focus:outline-none" />
    </label>
  );
}

function statusClass(status: VirtualKey["status"]) {
  switch (status) {
    case "active":
      return "bg-emerald-500/10 text-emerald-300";
    case "exhausted":
      return "bg-amber-500/10 text-amber-300";
    case "revoked":
      return "bg-slate-700 text-slate-300";
  }
}
