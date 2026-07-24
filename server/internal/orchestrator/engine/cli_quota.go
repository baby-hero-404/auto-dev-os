package engine

import "regexp"

// CLIQuotaRule is one pattern that, if matched against a CLI invocation's
// combined stdout+stderr (or exit code), means the underlying credential hit
// a rate limit / quota ceiling — not a code or environment bug.
type CLIQuotaRule struct {
	ExitCodes []int            // matches if result.ExitCode is one of these (empty = ignored)
	Patterns  []*regexp.Regexp // matches if any pattern is found in the combined output
}

// CLIQuotaRules is keyed by CLIProfile ref (or "*" for a fallback rule set
// applied when the ref has no dedicated entry, e.g. "custom" or the legacy
// fallback path). Config-driven table (mirrors the ERROR_RULES pattern from
// the 9router reference project, docs/references/infrastructure/DISCOVERY-9router.md)
// instead of scattered string checks — adding a 4th CLI tool means adding
// one map entry, not touching RunCodeStep's control flow. Each CLI's
// quota-exceeded message differs, so rules are not shared by default.
var CLIQuotaRules = map[string][]CLIQuotaRule{
	"claude_code": {
		{Patterns: []*regexp.Regexp{
			regexp.MustCompile(`(?i)usage limit reached`),
			regexp.MustCompile(`(?i)rate limit`),
		}},
	},
	"openai_codex": {
		{Patterns: []*regexp.Regexp{
			regexp.MustCompile(`(?i)rate limit`),
			regexp.MustCompile(`(?i)quota exceeded`),
			regexp.MustCompile(`\b429\b`),
		}},
	},
	"antigravity": {
		{Patterns: []*regexp.Regexp{
			regexp.MustCompile(`(?i)quota exceeded`),
			regexp.MustCompile(`(?i)rate limit`),
		}},
	},
	"*": { // fallback for "custom" and any ref without a dedicated rule set
		{Patterns: []*regexp.Regexp{
			regexp.MustCompile(`(?i)rate.?limit`),
			regexp.MustCompile(`(?i)quota`),
			regexp.MustCompile(`(?i)try again later`),
		}},
	},
}

// detectQuotaExceeded reports whether the combined output/exit code for the
// given profile ref matches a known quota-exceeded signature. Never panics
// on an unknown/empty ref — falls back to the "*" rule set.
func detectQuotaExceeded(ref string, combined string, exitCode int) bool {
	rules, ok := CLIQuotaRules[ref]
	if !ok {
		rules = CLIQuotaRules["*"]
	}
	for _, rule := range rules {
		for _, code := range rule.ExitCodes {
			if code == exitCode {
				return true
			}
		}
		for _, p := range rule.Patterns {
			if p.MatchString(combined) {
				return true
			}
		}
	}
	return false
}
