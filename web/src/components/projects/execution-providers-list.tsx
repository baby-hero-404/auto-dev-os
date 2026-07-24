import { useState } from "react";
import useSWR from "swr";
import type { ExecutionProviderConfig } from "@/lib/types";
import { CLI_PROFILES, CUSTOM_CLI_REF, cliProfileLabel } from "@/lib/cliProfiles";
import { Field } from "@/components/ui/field";
import { api } from "@/lib/api";
import { useSession } from "@/lib/session";
import {
  CLIEngineConfigForm,
  cliConfigToFormValue,
  formValueToCLIConfig,
  type CLIEngineConfigFormValue,
} from "./cli-engine-config-form";

type RowKey = { type: "api" | "cli"; ref: string };

const ROWS: RowKey[] = [
  { type: "api", ref: "anthropic" },
  { type: "api", ref: "openai" },
  { type: "api", ref: "gemini" },
  { type: "cli", ref: "claude_code" },
  { type: "cli", ref: "openai_codex" },
  { type: "cli", ref: "antigravity" },
  { type: "cli", ref: CUSTOM_CLI_REF },
];

function rowLabel(row: RowKey): string {
  if (row.type === "api") return row.ref.charAt(0).toUpperCase() + row.ref.slice(1) + " API";
  return cliProfileLabel(row.ref);
}

function findEntry(list: ExecutionProviderConfig[], row: RowKey): ExecutionProviderConfig | undefined {
  return list.find((e) => e.type === row.type && e.ref === row.ref);
}

export function defaultExecutionProviders(): ExecutionProviderConfig[] {
  return ROWS.map((row, i) => ({ type: row.type, ref: row.ref, priority: i, enabled: false }));
}

export function ExecutionProvidersList({
  value,
  onChange,
  disabled,
}: {
  value: ExecutionProviderConfig[];
  onChange: (next: ExecutionProviderConfig[]) => void;
  disabled?: boolean;
}) {
  const session = useSession();
  const token = session?.token ?? "";
  const orgID = session?.user.org_id ?? "";

  const { data: credentials = [] } = useSWR(
    orgID && token ? ["provider-credentials", orgID] : null,
    () => api.listProviderCredentials(orgID, token),
  );

  const [customOpen, setCustomOpen] = useState(false);

  const sorted = [...value].sort((a, b) => a.priority - b.priority);

  function update(row: RowKey, patch: Partial<ExecutionProviderConfig>) {
    const existing = findEntry(value, row);
    const next = value.map((e) =>
      e.type === row.type && e.ref === row.ref ? { ...e, ...patch } : e,
    );
    if (!existing) {
      next.push({ type: row.type, ref: row.ref, priority: value.length, enabled: false, ...patch });
    }
    onChange(next);
  }

  function move(row: RowKey, direction: -1 | 1) {
    const list = [...value].sort((a, b) => a.priority - b.priority);
    const idx = list.findIndex((e) => e.type === row.type && e.ref === row.ref);
    const target = idx + direction;
    if (idx < 0 || target < 0 || target >= list.length) return;
    [list[idx], list[target]] = [list[target], list[idx]];
    onChange(list.map((e, i) => ({ ...e, priority: i })));
  }

  function credentialsFor(providerPrefix: string) {
    return credentials.filter((c) => c.provider === providerPrefix || (providerPrefix === "" && c.provider.startsWith("cli:")));
  }

  return (
    <div className="space-y-2">
      {sorted.map((entry, i) => {
        const row: RowKey = { type: entry.type, ref: entry.ref };
        const isCustom = row.type === "cli" && row.ref === CUSTOM_CLI_REF;
        const credentialProvider = row.type === "cli" ? CLI_PROFILES[row.ref]?.credentialProvider ?? "" : row.ref;
        const rowCredentials = credentialsFor(credentialProvider);

        return (
          <div key={`${row.type}:${row.ref}`} className="rounded-md border border-stroke p-3 space-y-2">
            <div className="flex items-center justify-between gap-3">
              <div className="flex items-center gap-3">
                <div className="flex flex-col">
                  <button
                    type="button"
                    onClick={() => move(row, -1)}
                    disabled={disabled || i === 0}
                    className="text-[10px] text-content-muted hover:text-foreground disabled:opacity-30 cursor-pointer"
                  >
                    ▲
                  </button>
                  <button
                    type="button"
                    onClick={() => move(row, 1)}
                    disabled={disabled || i === sorted.length - 1}
                    className="text-[10px] text-content-muted hover:text-foreground disabled:opacity-30 cursor-pointer"
                  >
                    ▼
                  </button>
                </div>
                <span className="text-sm font-medium text-foreground">{rowLabel(row)}</span>
                <span className="rounded-full bg-surface px-2 py-0.5 text-[10px] font-semibold text-content-muted border border-stroke">
                  P{i}
                </span>
              </div>
              <label className="flex items-center gap-2 text-xs text-content-muted cursor-pointer select-none">
                <input
                  type="checkbox"
                  checked={entry.enabled}
                  onChange={(e) => update(row, { enabled: e.target.checked })}
                  disabled={disabled}
                  className="h-4 w-4 rounded border-stroke accent-brand-primary cursor-pointer"
                />
                Enabled
              </label>
            </div>

            {entry.enabled && row.type === "cli" && (
              <Field
                label={isCustom ? "CLI Authentication Profile *" : "CLI Authentication Profile"}
                htmlFor={`cred-${row.ref}`}
              >
                <select
                  id={`cred-${row.ref}`}
                  value={entry.credential_id ?? ""}
                  onChange={(e) => update(row, { credential_id: e.target.value || undefined })}
                  disabled={disabled}
                  required={isCustom}
                  className="w-full appearance-none rounded-md border border-stroke bg-background px-3 py-2 text-sm text-foreground focus:border-brand-primary focus:outline-none focus:ring-2 focus:ring-brand-primary/20"
                >
                  {!isCustom && <option value="">Auto (first available)</option>}
                  {isCustom && <option value="">Select a credential…</option>}
                  {rowCredentials.map((c) => (
                    <option key={c.id} value={c.id}>
                      {c.label} ({c.provider})
                    </option>
                  ))}
                </select>
              </Field>
            )}

            {entry.enabled && isCustom && (
              <div>
                <button
                  type="button"
                  onClick={() => setCustomOpen((o) => !o)}
                  disabled={disabled}
                  className="text-xs text-brand-primary hover:underline cursor-pointer"
                >
                  {customOpen ? "Hide" : "Configure"} custom command/args
                </button>
                {customOpen && (
                  <div className="mt-2">
                    <CLIEngineConfigForm
                      value={cliConfigToFormValue(entry.cli_config) as CLIEngineConfigFormValue}
                      onChange={(v) => update(row, { cli_config: formValueToCLIConfig(v) })}
                      disabled={disabled}
                    />
                  </div>
                )}
              </div>
            )}
          </div>
        );
      })}
    </div>
  );
}
