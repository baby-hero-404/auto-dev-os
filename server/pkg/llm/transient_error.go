package llm

import (
	"errors"
	"strings"
)

// IsTransientError is the single canonical classifier for whether an LLM provider/gateway
// call error is worth retrying (and, at the gateway layer, worth cooling the credential down
// for). Combines structured HTTP status codes and explicit "status NNN" substrings with
// generic network-level failure keywords (timeout, connection refused, EOF, dial errors) —
// the latter were previously only recognized by a second, separate classifier one layer up
// (llmrunner.Runner), which meant network errors skipped credential cooldown entirely and
// were retried by the wrong, coarser layer instead of the credential-aware one (REQ-M05).
//
// Call sites must not maintain their own copy of this logic — see docs/openspecs/
// llm-prompt-tool-flow-hardening/specs.md REQ-M05.
func IsTransientError(err error) bool {
	if err == nil {
		return false
	}

	var statusErr interface{ HTTPStatusCode() int }
	if errors.As(err, &statusErr) {
		switch statusErr.HTTPStatusCode() {
		case 429, 402, 500, 502, 503, 504:
			return true
		}
	}

	msg := strings.ToLower(err.Error())

	statusSubstrings := []string{
		"status 429", "status 402", "status 500", "status 502", "status 503", "status 504",
	}
	for _, s := range statusSubstrings {
		if strings.Contains(msg, s) {
			return true
		}
	}

	keywords := []string{
		"rate limit", "quota", "temporarily unavailable", "high demand",
		"limit exceeded", "resource exhausted",
		"timeout", "deadline exceeded", "connection refused", "eof",
	}
	for _, k := range keywords {
		if strings.Contains(msg, k) {
			return true
		}
	}

	return false
}
