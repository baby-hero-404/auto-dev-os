package engine

import "regexp"

var secretRegexes = []*regexp.Regexp{
	regexp.MustCompile(`(?i)ghp_[a-zA-Z0-9]{36}`),
	regexp.MustCompile(`(?i)github_pat_[a-zA-Z0-9_]{82}`),
	regexp.MustCompile(`(?i)sk-[a-zA-Z0-9]{48}`),
	regexp.MustCompile(`(?i)sk-proj-[a-zA-Z0-9-_]{150,}`),
	regexp.MustCompile(`(?i)sk-ant-[a-zA-Z0-9-_]{90,}`),
	regexp.MustCompile(`(?i)AIzaSy[a-zA-Z0-9-_]{33}`),
}

// redactSecrets scrubs known API-key/token shapes from captured CLI output
// before it's persisted to logs or checkpoints.
func redactSecrets(s string) string {
	for _, re := range secretRegexes {
		s = re.ReplaceAllString(s, "[REDACTED]")
	}
	return s
}
