package engine

import "regexp"

// awaitingInputPatterns match a CLI's last output line when it looks like
// the process stopped to ask a yes/no or open-ended confirmation question.
// Checked only against the last non-empty line (see detectAwaitingInput) —
// these tools legitimately print a lot of "?"-ending progress text, so
// matching mid-transcript would false-positive constantly.
var awaitingInputPatterns = []*regexp.Regexp{
	regexp.MustCompile(`(?i)\(y/n\)\s*$`),
	regexp.MustCompile(`(?i)do you want to\b.*\?\s*$`),
	regexp.MustCompile(`(?i)please confirm\b`),
	regexp.MustCompile(`(?i)waiting for (user )?input`),
	regexp.MustCompile(`(?i)\bwhich (option|approach|one)\b.*\?\s*$`),
}

// detectAwaitingInput reports whether lastLine (the last non-empty line of
// a finished/killed CLI run's combined output) looks like the process was
// blocked waiting for an answer that never came (no stdin is ever attached
// to sandboxed CLI runs — see RunCodeStep).
func detectAwaitingInput(lastLine string) bool {
	for _, p := range awaitingInputPatterns {
		if p.MatchString(lastLine) {
			return true
		}
	}
	return false
}
