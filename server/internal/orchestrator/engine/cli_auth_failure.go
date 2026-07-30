package engine

import "regexp"

// CLIAuthFailureRules mirrors CLIQuotaRules (cli_quota.go): per-profile
// patterns that mean the linked credential's session/token is no longer
// valid — not a code bug, not a quota ceiling — so the run failed because
// the CLI genuinely isn't logged in. Keyed by CLIProfile ref, "*" fallback
// for "custom"/unknown refs.
var CLIAuthFailureRules = map[string][]*regexp.Regexp{
	"claude_code": {
		regexp.MustCompile(`(?i)not logged in`),
		regexp.MustCompile(`(?i)please run\s*/login`),
		regexp.MustCompile(`(?i)invalid api key`),
		regexp.MustCompile(`(?i)failed to authenticate`),
		regexp.MustCompile(`(?i)(oauth )?(access )?token (has been |was )?revoked`),
	},
	"openai_codex": {
		regexp.MustCompile(`(?i)not logged in`),
		regexp.MustCompile(`(?i)please run\s*['"]?codex login`),
		regexp.MustCompile(`(?i)authentication required`),
		regexp.MustCompile(`(?i)failed to authenticate`),
		regexp.MustCompile(`(?i)(oauth )?(access )?token (has been |was )?revoked`),
	},
	"antigravity": {
		regexp.MustCompile(`(?i)not logged in`),
		regexp.MustCompile(`(?i)authentication required`),
		regexp.MustCompile(`(?i)please (log|sign) in`),
		regexp.MustCompile(`(?i)failed to authenticate`),
		regexp.MustCompile(`(?i)(oauth )?(access )?token (has been |was )?revoked`),
	},
	"*": {
		regexp.MustCompile(`(?i)not logged in`),
		regexp.MustCompile(`(?i)authentication required`),
		regexp.MustCompile(`(?i)failed to authenticate`),
		regexp.MustCompile(`(?i)unauthorized`),
		regexp.MustCompile(`(?i)(oauth )?(access )?token (has been |was )?revoked`),
		regexp.MustCompile(`\b401\b`),
	},
}

// detectAuthFailure reports whether combined stdout/stderr for the given
// profile ref matches a known "session/token invalid" signature. Checks the
// profile-specific list AND the "*" generic list unconditionally — a known
// ref (e.g. "claude_code") having its own entry must never shadow the
// generic 401/"failed to authenticate"/revoked-token patterns just because
// that specific CLI's wording wasn't anticipated (this exact gap is why a
// real "OAuth access token has been revoked" 401 went undetected in
// production: claude_code's list existed but didn't cover it, and the old
// code only fell back to "*" when the ref was entirely unknown).
func detectAuthFailure(ref string, combined string) bool {
	for _, p := range CLIAuthFailureRules[ref] {
		if p.MatchString(combined) {
			return true
		}
	}
	if ref == "*" {
		return false
	}
	for _, p := range CLIAuthFailureRules["*"] {
		if p.MatchString(combined) {
			return true
		}
	}
	return false
}
