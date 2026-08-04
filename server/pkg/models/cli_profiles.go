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
		Command: "claude",
		// --output-format json is appended last (confirmed valid ordering
		// against docs/guides/claude-cli-headless.md's own example:
		// `claude -p "..." --output-format json`) so RunCodeStep's telemetry
		// parser (Phase 6) can extract total_cost_usd/duration_ms/usage from
		// the trailing JSON summary object claude prints on exit.
		Args: []string{"--allowedTools", "Read,Edit,Write,Bash", "-p", promptFileInstruction, "--output-format", "json"},
		// "claude auth status" is read-only (no side effects) and always
		// exits 0 regardless of login state, reporting it instead via a
		// `"loggedIn": bool` JSON field (verified against the real binary,
		// see docs/openspecs/cli-execution-reliability/tasks.md REQ-003) —
		// exit-code-only checking would let a logged-out credential pass
		// Preflight, so detectAuthInvalid (cli_auth.go) checks this output
		// for the loggedIn:false signature instead of trusting exit 0.
		AuthCheckCommand:   "claude auth status",
		TimeoutMinutes:     30,
		CredentialProvider: "cli:claude",
	},
	"openai_codex": {
		Command: "codex",
		// NOTE: --output-format json (or equivalent) is intentionally absent here.
		// Unlike claude_code and antigravity (both confirmed against live binaries),
		// codex's structured-output flag name and JSON schema have not been verified
		// against a real `codex exec --help` / live run — guessing the wrong flag
		// would break codex exec's existing text-output parsing (auth/quota/loop
		// detection all regex over raw text). Until confirmed, telemetry will be
		// zero for Codex runs (parseCLITelemetry returns ok=false).
		// TODO: run `codex exec --help` against a live binary, confirm the flag
		// (likely `--output-format json` or `--json`), add it here, and add a
		// matching test case in engine/telemetry_test.go.
		Args: []string{"exec", "--full-auto", promptFileInstruction},
		// "codex login status" is read-only and prints "Logged in using
		// ChatGPT" on success (verified against the real binary, same
		// REQ-003 investigation as claude_code above).
		AuthCheckCommand:   "codex login status",
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
		// very next arg, so boolean flags go first. See also
		// docs/guides/antigravity-cli-headless.md, which independently
		// confirms the real binary is `agy` (not `antigravity`) and that
		// `-p` must sit immediately before the prompt.
		// --output-format json appended last, same rationale/placement as
		// claude_code above (confirmed valid against
		// docs/guides/antigravity-cli-headless.md's own example).
		Args:               []string{"--dangerously-skip-permissions", "-p", promptFileInstruction, "--output-format", "json"},
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
