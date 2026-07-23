import { Plus, Trash2 } from "lucide-react";
import type { CLIEngineConfig } from "@/lib/types";
import { Input } from "@/components/ui/input";
import { Textarea } from "@/components/ui/textarea";
import { Field } from "@/components/ui/field";
import { Button } from "@/components/ui/button";
import useSWR from "swr";
import { api } from "@/lib/api";
import { useSession } from "@/lib/session";

export type CLIEngineConfigFormValue = {
  command: string;
  argsText: string;
  env: { key: string; value: string }[];
  timeoutMinutes: number;
  authCheckCommand: string;
  allowNoop: boolean;
  credentialID: string;
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
    onChange({
      ...value,
      command: "claude",
      argsText: "-p\n{prompt_file}",
      authCheckCommand: "claude --version",
      env: newEnv,
    });
  }

  return (
    <div className="space-y-4">
      <div className="flex items-center justify-end mb-2">
        <Button type="button" variant="secondary" size="sm" onClick={applyClaudePreset} disabled={disabled} className="text-xs h-7">
          Fill Claude Code Preset
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
                className="font-mono text-xs"
                disabled={disabled}
              />
              <Input
                value={row.value}
                onChange={(e) => updateEnvRow(i, { value: e.target.value })}
                placeholder="value"
                type={row.value === "***" ? "password" : "text"}
                className="font-mono text-xs"
                disabled={disabled}
              />
              <Button
                type="button"
                variant="secondary"
                onClick={() => removeEnvRow(i)}
                disabled={disabled}
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
