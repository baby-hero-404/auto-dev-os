package orchestrator

import "github.com/baby-hero-404/aicli/run"

// redactSecrets scrubs credential shapes from text before it reaches a log
// file, SSE event, or persistent checkpoint. Delegated to aicli/run.RedactSecrets.
func redactSecrets(s string) string { return run.RedactSecrets(s) }
