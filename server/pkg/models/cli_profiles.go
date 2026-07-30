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

// promptFileInstruction is embedded (not the bare {prompt_file} path alone)
// as the literal prompt-text argument for claude/codex/agy: none of the
// three actually accept a raw file path as their headless prompt argument
// (per docs/guides/*-cli-headless.md, all three take literal instruction
// text — e.g. `claude -p "Analyze this project"`); a bare path there would
// be read as prompt text equal to the path string itself, not the file's
// contents. Wrapping it in a sentence works because all three are agentic
// coding CLIs with their own file-read tool, so they open the referenced
// file themselves. cli.go's {prompt_file} substitution still does a plain
// string replace, so this works for any future profile that instead wires
// a real --file/--prompt-file flag by using a bare {prompt_file} arg.
const promptFileInstruction = "Read and follow the complete task instructions in the file {prompt_file}, then carry them out."

// CLIProfiles is the built-in registry of known CLI coding tools, keyed by
// the same profile ID used in ExecutionProviderConfig.Ref.
var CLIProfiles = map[string]CLIProfile{
	"claude_code": {
		Command:            "claude",
		Args:               []string{"--allowedTools", "Read,Edit,Bash", "-p", promptFileInstruction},
		AuthCheckCommand:   "claude --version",
		TimeoutMinutes:     30,
		CredentialProvider: "cli:claude",
	},
	"openai_codex": {
		Command:            "codex",
		Args:               []string{"exec", "--full-auto", promptFileInstruction},
		AuthCheckCommand:   "codex --version",
		TimeoutMinutes:     30,
		CredentialProvider: "cli:codex",
	},
	"antigravity": {
		Command: "agy",
		// agy's flag parser is Go stdlib `flag` (its --help text is the
		// literal flag.PrintDefaults() format), where -p/--print/--prompt is
		// a value flag: whatever token immediately follows -p becomes its
		// value (the prompt), not the next positional. With
		// {"-p", "--dangerously-skip-permissions", instruction} the CLI
		// literally used "--dangerously-skip-permissions" as the prompt —
		// confirmed from a real run's captured output, which was agy
		// explaining that flag instead of analyzing the repo (no
		// .autocode/analysis.md ever got written). -p's value must be the
		// very next arg, so boolean flags go first.
		Args:               []string{"--dangerously-skip-permissions", "-p", promptFileInstruction},
		AuthCheckCommand:   "agy --version",
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
