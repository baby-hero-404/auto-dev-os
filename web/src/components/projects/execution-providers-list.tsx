import { useState, useMemo } from "react";
import useSWR from "swr";
import type { ExecutionProviderConfig } from "@/lib/types";
import { CLI_PROFILES, CUSTOM_CLI_REF, cliProfileLabel } from "@/lib/cliProfiles";
import { Field } from "@/components/ui/field";
import { api } from "@/lib/api";
import { useSession } from "@/lib/session";
import { Plus, Trash2 } from "lucide-react";
import { Input } from "@/components/ui/input";
import { Textarea } from "@/components/ui/textarea";
import { Button } from "@/components/ui/button";
import type { CLIEngineConfig } from "@/lib/types";

export type CLIEngineConfigFormValue = {
  command: string;
  argsText: string;
  env: { key: string; value: string }[];
  timeoutMinutes: number;
  authCheckCommand: string;
  allowNoop: boolean;
  credentialID: string;
  underlyingProvider: string;
};

export function cliConfigToFormValue(cfg: CLIEngineConfig | undefined): CLIEngineConfigFormValue {
  return {
    command: cfg?.command ?? "",
    argsText: (cfg?.args ?? []).join("\n"),
    env: Object.entries(cfg?.env ?? {}).map(([key, value]) => ({ key, value })),
    timeoutMinutes: cfg?.timeout_minutes ?? 30,
    authCheckCommand: cfg?.auth_check_command ?? "",
    allowNoop: cfg?.allow_noop ?? false,
    credentialID: cfg?.credential_id ?? "",
    underlyingProvider: cfg?.underlying_provider ?? "",
  };
}

export function formValueToCLIConfig(v: CLIEngineConfigFormValue): CLIEngineConfig {
  const env: Record<string, string> = {};
  for (const row of v.env) {
    if (row.key.trim()) env[row.key.trim()] = row.value;
  }
  return {
    command: v.command.trim(),
    args: v.argsText.split("\n").map((a) => a.trim()).filter(Boolean),
    env,
    timeout_minutes: v.timeoutMinutes,
    auth_check_command: v.authCheckCommand.trim() || undefined,
    allow_noop: v.allowNoop,
    credential_id: v.credentialID || undefined,
    underlying_provider: v.underlyingProvider.trim() || undefined,
  };
}

export function formValuesEqual(a: CLIEngineConfigFormValue, b: CLIEngineConfigFormValue): boolean {
  return JSON.stringify(formValueToCLIConfig(a)) === JSON.stringify(formValueToCLIConfig(b));
}

export function CLIEngineConfigForm({
  value,
  onChange,
  disabled,
}: {
  value: CLIEngineConfigFormValue;
  onChange: (next: CLIEngineConfigFormValue) => void;
  disabled?: boolean;
}) {
  const session = useSession();
  const token = session?.token ?? "";
  const orgID = session?.user.org_id ?? "";

  const { data: credentials = [] } = useSWR(
    orgID && token ? ["provider-credentials", orgID] : null,
    () => api.listProviderCredentials(orgID, token),
  );

  const cliCredentials = credentials.filter((c) => c.provider.startsWith("cli:"));

  function updateEnvRow(index: number, patch: Partial<{ key: string; value: string }>) {
    const env = value.env.map((row, i) => (i === index ? { ...row, ...patch } : row));
    onChange({ ...value, env });
  }

  function removeEnvRow(index: number) {
    onChange({ ...value, env: value.env.filter((_, i) => i !== index) });
  }

  function addEnvRow() {
    onChange({ ...value, env: [...value.env, { key: "", value: "" }] });
  }

  function applyClaudePreset() {
    const newEnv = value.env.filter((e) => e.key && e.key !== "ANTHROPIC_AUTH_TOKEN");
    newEnv.push({ key: "ANTHROPIC_AUTH_TOKEN", value: "" });
    
    // Auto-select a Claude credential if the user has one configured
    const claudeCred = cliCredentials.find(c => c.provider === "cli:claude");

    onChange({
      ...value,
      command: "claude",
      argsText: "-p\n{prompt_file}",
      authCheckCommand: "claude --version",
      env: newEnv,
      credentialID: claudeCred ? claudeCred.id : value.credentialID,
    });
  }

  return (
    <div className="space-y-4">
      <div className="flex items-center justify-end mb-2">
        <Button type="button" variant="secondary" size="sm" onClick={applyClaudePreset} disabled={disabled} className="text-xs h-7">
          Fill Claude CLI Preset
        </Button>
      </div>

      <Field label="CLI Authentication Profile" htmlFor="cli-credential">
        <select
          id="cli-credential"
          value={value.credentialID}
          onChange={(e) => onChange({ ...value, credentialID: e.target.value })}
          disabled={disabled}
          className="w-full appearance-none rounded-md border border-stroke bg-background px-3 py-2 text-sm text-foreground transition-all duration-150 focus:border-brand-primary focus:outline-none focus:ring-2 focus:ring-brand-primary/20"
        >
          <option value="">No centralized credential (use env vars)</option>
          {cliCredentials.map((c) => (
            <option key={c.id} value={c.id}>
              {c.label} ({c.provider})
            </option>
          ))}
        </select>
        <p className="text-[10px] text-content-muted mt-1">Select a CLI config saved in AI Providers to sync its OAuth session across runs.</p>
      </Field>

      <Field label="Command *" htmlFor="cli-command" hint='e.g. "claude"'>
        <Input
          id="cli-command"
          value={value.command}
          onChange={(e) => onChange({ ...value, command: e.target.value })}
          placeholder="claude"
          required
          disabled={disabled}
        />
      </Field>

      <Field label="Args" htmlFor="cli-args" hint='One per line. Use {prompt_file} and {workdir} as placeholders.'>
        <Textarea
          id="cli-args"
          value={value.argsText}
          onChange={(e) => onChange({ ...value, argsText: e.target.value })}
          placeholder={"-p\n--dangerously-skip-permissions\n{prompt_file}"}
          className="font-mono text-xs resize-none"
          rows={4}
          disabled={disabled}
        />
      </Field>

      <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
        <Field label="Timeout (minutes)" htmlFor="cli-timeout">
          <Input
            id="cli-timeout"
            type="number"
            min={1}
            max={120}
            value={value.timeoutMinutes}
            onChange={(e) => onChange({ ...value, timeoutMinutes: Number(e.target.value) })}
            disabled={disabled}
          />
        </Field>
        <Field label="Auth Check Command" htmlFor="cli-auth-check" hint='e.g. "claude auth status"'>
          <Input
            id="cli-auth-check"
            value={value.authCheckCommand}
            onChange={(e) => onChange({ ...value, authCheckCommand: e.target.value })}
            placeholder="claude auth status"
            disabled={disabled}
          />
        </Field>
      </div>

      <Field
        label="Underlying Provider"
        htmlFor="cli-underlying-provider"
        hint='Optional. The API provider this CLI runs on top of (e.g. "anthropic"), used so cross-harness review can pick a genuinely different provider instead of assuming the CLI is automatically different.'
      >
        <Input
          id="cli-underlying-provider"
          value={value.underlyingProvider}
          onChange={(e) => onChange({ ...value, underlyingProvider: e.target.value })}
          placeholder="anthropic"
          disabled={disabled}
        />
      </Field>

      <label className="flex items-center gap-2 text-sm text-foreground cursor-pointer select-none">
        <input
          type="checkbox"
          checked={value.allowNoop}
          onChange={(e) => onChange({ ...value, allowNoop: e.target.checked })}
          disabled={disabled}
          className="h-4 w-4 rounded border-stroke accent-brand-primary cursor-pointer"
        />
        Allow no-op runs (don&apos;t fail the step when the CLI makes zero file changes)
      </label>

      <Field label="Environment Variables" hint='Stored values are masked as "***" once saved; leave a masked value untouched to keep it.'>
        <div className="space-y-2">
          {value.env.map((row, i) => (
            <div key={i} className="flex items-center gap-2">
              <Input
                value={row.key}
                onChange={(e) => updateEnvRow(i, { key: e.target.value })}
                placeholder="KEY"
                className="font-mono text-xs focus:ring-brand-primary/20"
                disabled={disabled}
              />
              <Input
                value={row.value}
                onChange={(e) => updateEnvRow(i, { value: e.target.value })}
                placeholder="value"
                type={row.value === "***" ? "password" : "text"}
                className="font-mono text-xs focus:ring-brand-primary/20"
                disabled={disabled}
              />
              <Button
                type="button"
                variant="secondary"
                onClick={() => removeEnvRow(i)}
                disabled={disabled}
                className="hover:bg-rose-500/10 hover:text-rose-500 transition-colors"
                aria-label={`Remove ${row.key || "env var"}`}
              >
                <Trash2 size={14} />
              </Button>
            </div>
          ))}
          <Button type="button" variant="secondary" onClick={addEnvRow} disabled={disabled}>
            <Plus size={14} />
            Add Variable
          </Button>
        </div>
      </Field>
    </div>
  );
}

type RowKey = { type: "api" | "cli"; ref: string };

const ROWS: RowKey[] = [
  { type: "cli", ref: "claude_code" },
  { type: "cli", ref: "antigravity" },
  { type: "cli", ref: "openai_codex" },
  { type: "cli", ref: CUSTOM_CLI_REF },
  { type: "api", ref: "anthropic" },
  { type: "api", ref: "openai" },
  { type: "api", ref: "gemini" },
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

  const merged = useMemo(() => {
    const list = [...(value || [])];
    ROWS.forEach((r) => {
      if (!list.find((e) => e.type === r.type && e.ref === r.ref)) {
        list.push({ type: r.type, ref: r.ref, priority: list.length, enabled: false });
      }
    });
    return list.sort((a, b) => a.priority - b.priority);
  }, [value]);

  function update(row: RowKey, patch: Partial<ExecutionProviderConfig>) {
    const existing = findEntry(merged, row);
    const next = merged.map((e) =>
      e.type === row.type && e.ref === row.ref ? { ...e, ...patch } : e,
    );
    if (!existing) {
      next.push({ type: row.type, ref: row.ref, priority: merged.length, enabled: false, ...patch });
    }
    onChange(next);
  }

  function move(row: RowKey, direction: -1 | 1) {
    const list = [...merged].sort((a, b) => a.priority - b.priority);
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
      {merged.map((entry, i) => {
        const row: RowKey = { type: entry.type, ref: entry.ref };
        const isCustom = row.type === "cli" && row.ref === CUSTOM_CLI_REF;
        const credentialProvider = row.type === "cli" ? CLI_PROFILES[row.ref]?.credentialProvider ?? "" : row.ref;
        const rowCredentials = credentialsFor(credentialProvider);
        const selectedCredential = credentials.find((c) => c.id === entry.credential_id);
        const selectedOnCooldown =
          !!selectedCredential?.cooldown_until && new Date(selectedCredential.cooldown_until) > new Date();

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
                    disabled={disabled || i === merged.length - 1}
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
                  {rowCredentials.map((c) => {
                    const onCooldown = !!c.cooldown_until && new Date(c.cooldown_until) > new Date();
                    const needsReauth = c.status === "needs_reauth";
                    return (
                      <option key={c.id} value={c.id}>
                        {c.label} ({c.provider})
                        {needsReauth ? " — needs login" : onCooldown ? " — on cooldown" : ""}
                      </option>
                    );
                  })}
                </select>
                {rowCredentials.length === 0 && (
                  <p className="mt-1 text-xs text-content-muted">
                    No CLI credentials found.{" "}
                    <a href="/ai-providers" className="text-brand-primary underline hover:no-underline">
                      Authenticate a CLI provider
                    </a>{" "}
                    on the AI Providers page first.
                  </p>
                )}
                {selectedOnCooldown && selectedCredential?.cooldown_until && (
                  <p className="mt-1 text-xs text-amber-600 dark:text-amber-400">
                    This credential is rate-limited and on cooldown until{" "}
                    {new Date(selectedCredential.cooldown_until).toLocaleString()}. Tasks will fail or fall back to
                    another provider until then.
                  </p>
                )}
                {selectedCredential?.status === "needs_reauth" && (
                  <p className="mt-1 text-xs text-red-600 dark:text-red-400">
                    This credential&apos;s CLI session has expired.{" "}
                    <a href="/ai-providers" className="underline hover:no-underline">
                      Log in again
                    </a>{" "}
                    on the AI Providers page — tasks will fail until then.
                  </p>
                )}
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
