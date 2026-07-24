package models

// CLIProfile is a system-level, built-in description of how to invoke a
// specific CLI coding tool (command/args/auth-check/timeout). It is not
// stored in the database — org admins cannot create new profiles in this
// phase; only the "custom" ref (see ExecutionProviderConfig) escapes the
// registry via an inline CLIEngineConfig.
type CLIProfile struct {
	Command            string   `json:"command"`
	Args               []string `json:"args"`
	AuthCheckCommand   string   `json:"auth_check_command,omitempty"`
	TimeoutMinutes     int      `json:"timeout_minutes"`
	CredentialProvider string   `json:"credential_provider"`
}

// CLIProfiles is the built-in registry of known CLI coding tools, keyed by
// the same profile ID used in ExecutionProviderConfig.Ref.
var CLIProfiles = map[string]CLIProfile{
	"claude_code": {
		Command:            "claude",
		Args:               []string{"-p", "--dangerously-skip-permissions", "{prompt_file}"},
		AuthCheckCommand:   "claude --version",
		TimeoutMinutes:     30,
		CredentialProvider: "cli:claude",
	},
	"openai_codex": {
		Command:            "codex",
		Args:               []string{"exec", "--full-auto", "{prompt_file}"},
		AuthCheckCommand:   "codex --version",
		TimeoutMinutes:     30,
		CredentialProvider: "cli:codex",
	},
	"antigravity": {
		Command:            "antigravity",
		Args:               []string{"run", "--yes", "{prompt_file}"},
		AuthCheckCommand:   "antigravity --version",
		TimeoutMinutes:     30,
		CredentialProvider: "cli:antigravity",
	},
}

// ProfileOrEmpty returns the profile registered under key and true, or a
// zero value and false if key is unknown. Callers must not assume a match.
func ProfileOrEmpty(key string) (CLIProfile, bool) {
	p, ok := CLIProfiles[key]
	return p, ok
}
