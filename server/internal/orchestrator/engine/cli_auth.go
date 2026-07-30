package engine

import "regexp"

// CLIAuthInvalidRules is keyed by CLIProfile ref ("*" fallback for
// "custom"/unknown refs), mirroring CLIQuotaRules (cli_quota.go) but for a
// different failure class: the linked credential is present and marked
// active, yet the CLI itself reports it isn't authenticated. Unlike quota
// (transient, cools down and retries later), this is permanent until a
// human re-runs the CLI auth capture flow — retrying burns attempts for
// nothing since the same credential produces the same failure every time.
var CLIAuthInvalidRules = map[string][]*regexp.Regexp{
	"claude_code": {
		regexp.MustCompile(`(?i)not logged in`),
		regexp.MustCompile(`(?i)please run\s*/login`),
		regexp.MustCompile(`(?i)invalid api key`),
		regexp.MustCompile(`(?i)failed to authenticate`),
		regexp.MustCompile(`(?i)(oauth )?(access )?token (has been |was )?revoked`),
		// "claude auth status" (the auth_check_command, REQ-003) exits 0
		// regardless of login state and reports it via this JSON field
		// instead — verified against the real `claude` binary.
		regexp.MustCompile(`(?i)"loggedIn"\s*:\s*false`),
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
		regexp.MustCompile(`(?i)not authenticated`),
		regexp.MustCompile(`(?i)authentication required`),
		regexp.MustCompile(`(?i)please (log|sign) in`),
		regexp.MustCompile(`(?i)failed to authenticate`),
		regexp.MustCompile(`(?i)(oauth )?(access )?token (has been |was )?revoked`),
	},
	"*": {
		regexp.MustCompile(`(?i)not logged in`),
		regexp.MustCompile(`(?i)not authenticated`),
		regexp.MustCompile(`(?i)authentication required`),
		regexp.MustCompile(`(?i)failed to authenticate`),
		regexp.MustCompile(`(?i)unauthorized`),
		regexp.MustCompile(`(?i)(oauth )?(access )?token (has been |was )?revoked`),
		regexp.MustCompile(`\b401\b`),
	},
}

// detectAuthInvalid reports whether combined stdout/stderr for the given
// profile ref matches a known "not authenticated" signature. Checks the
// profile-specific list AND the "*" generic list unconditionally — a known
// ref (e.g. "claude_code") having its own entry must never shadow the
// generic 401/"failed to authenticate"/revoked-token patterns just because
// that specific CLI's wording wasn't anticipated (this exact gap is why a
// real "OAuth access token has been revoked" 401 went undetected in
// production: claude_code's list existed but didn't cover it, and the old
// code only fell back to "*" when the ref was entirely unknown).
func detectAuthInvalid(ref string, combined string) bool {
	for _, p := range CLIAuthInvalidRules[ref] {
		if p.MatchString(combined) {
			return true
		}
	}
	if ref == "*" {
		return false
	}
	for _, p := range CLIAuthInvalidRules["*"] {
		if p.MatchString(combined) {
			return true
		}
	}
	return false
}
