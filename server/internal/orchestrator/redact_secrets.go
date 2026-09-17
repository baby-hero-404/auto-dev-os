package orchestrator

import "github.com/auto-code-os/auto-code-os/server/internal/orchestrator/engine"

// redactSecrets scrubs credential shapes from text before it reaches a log
// line or a task event. It delegates to engine.RedactSecrets so the pattern
// set has exactly one definition — this package used to carry a second,
// near-identical copy that could silently drift from the one actually applied
// to captured CLI output.
func redactSecrets(s string) string { return engine.RedactSecrets(s) }
