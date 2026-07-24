package engine

import "regexp"

var secretRegexes = []*regexp.Regexp{
	regexp.MustCompile(`(?i)ghp_[a-zA-Z0-9]{36}`),
	regexp.MustCompile(`(?i)github_pat_[a-zA-Z0-9_]{82}`),
	regexp.MustCompile(`(?i)sk-[a-zA-Z0-9]{48}`),
	regexp.MustCompile(`(?i)sk-proj-[a-zA-Z0-9-_]{150,}`),
	regexp.MustCompile(`(?i)sk-ant-[a-zA-Z0-9-_]{90,}`),
	regexp.MustCompile(`(?i)AIzaSy[a-zA-Z0-9-_]{33}`),
	// Generic JWT shape (header.payload.signature, base64url segments). Covers
	// OAuth access/refresh/id tokens from any provider, not just a specific
	// vendor's key format — relevant now that materialized CLI credentials
	// (see cliEngine.resolveCredentialFiles) put real OAuth session data
	// inside the sandbox, and cli_output is surfaced directly in the task UI.
	regexp.MustCompile(`\beyJ[a-zA-Z0-9_-]{10,}\.[a-zA-Z0-9_-]{10,}\.[a-zA-Z0-9_-]{10,}\b`),
	// JSON/env-style "key": "value" or key=value pairs where the key name
	// itself signals a credential, regardless of the value's shape (catches
	// a stray `cat ~/.claude.json`, `env`, or debug dump of the materialized
	// credential files).
	regexp.MustCompile(`(?i)("(?:access_token|refresh_token|id_token|api_key|apikey|client_secret|password|secret)"\s*:\s*)"[^"]*"`),
	regexp.MustCompile(`(?i)\b((?:access_token|refresh_token|id_token|api_key|apikey|client_secret|password|secret)\s*=\s*)\S+`),
}

// redactSecrets scrubs known API-key/token shapes from captured CLI output
// before it's persisted to logs or checkpoints.
func redactSecrets(s string) string {
	for _, re := range secretRegexes {
		s = re.ReplaceAllString(s, "[REDACTED]")
	}
	return s
}
