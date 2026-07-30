// Mirrors server/pkg/models/cli_profiles.go — labels/icons only, no command/args.
// Command/args stay server-side; the frontend only needs to render the picker.
export interface CLIProfileMeta {
  label: string;
  credentialProvider: string;
}

export const CLI_PROFILES: Record<string, CLIProfileMeta> = {
  claude_code: { label: "Claude CLI", credentialProvider: "cli:claude" },
  openai_codex: { label: "Codex CLI", credentialProvider: "cli:codex" },
  antigravity: { label: "Antigravity CLI", credentialProvider: "cli:antigravity" },
};

export const CUSTOM_CLI_REF = "custom";

export function cliProfileLabel(ref: string): string {
  if (ref === CUSTOM_CLI_REF) return "Custom CLI";
  return CLI_PROFILES[ref]?.label ?? ref;
}
