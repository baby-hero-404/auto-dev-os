package engine

import (
	"regexp"
	"time"
)

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
			regexp.MustCompile(`(?i)session limit`),
			regexp.MustCompile(`(?i)rate limit`),
		}},
	},
	"openai_codex": {
		{Patterns: []*regexp.Regexp{
			regexp.MustCompile(`(?i)rate limit`),
			regexp.MustCompile(`(?i)quota exceeded`),
			regexp.MustCompile(`(?i)too many requests`),
			regexp.MustCompile(`\b429\b`),
		}},
	},
	"antigravity": {
		{Patterns: []*regexp.Regexp{
			regexp.MustCompile(`(?i)quota (?:exceeded|reached)`),
			regexp.MustCompile(`(?i)rate limit`),
			regexp.MustCompile(`(?i)resource_exhausted`),
			regexp.MustCompile(`(?i)too many requests`),
			regexp.MustCompile(`\b429\b`),
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
// It also attempts to dynamically parse the cooldown duration from the error message.
func detectQuotaExceeded(ref string, combined string, exitCode int) (bool, time.Duration) {
	rules, ok := CLIQuotaRules[ref]
	if !ok {
		rules = CLIQuotaRules["*"]
	}

	matchFound := false
	for _, rule := range rules {
		for _, code := range rule.ExitCodes {
			if code == exitCode {
				matchFound = true
				break
			}
		}
		if matchFound {
			break
		}
		for _, p := range rule.Patterns {
			if p.MatchString(combined) {
				matchFound = true
				break
			}
		}
		if matchFound {
			break
		}
	}

	if !matchFound {
		return false, 0
	}

	var cooldown time.Duration

	// Attempt to parse dynamic cooldown duration based on provider
	if ref == "antigravity" {
		// Example: "Resets in 17h40m33s"
		re := regexp.MustCompile(`(?i)resets\s+in\s+(?:(\d+)h)?(?:(\d+)m)?(?:(\d+)s)?`)
		if m := re.FindStringSubmatch(combined); len(m) > 0 {
			var d time.Duration
			if len(m) > 1 && m[1] != "" {
				h, _ := time.ParseDuration(m[1] + "h")
				d += h
			}
			if len(m) > 2 && m[2] != "" {
				m_, _ := time.ParseDuration(m[2] + "m")
				d += m_
			}
			if len(m) > 3 && m[3] != "" {
				s, _ := time.ParseDuration(m[3] + "s")
				d += s
			}
			cooldown = d
		}
	} else if ref == "claude_code" {
		// Example: "Resets 12:10pm (UTC)"
		re := regexp.MustCompile(`(?i)resets\s+(\d{1,2}:\d{2}(?:am|pm)?\s*(?:\([A-Z]+\))?)`)
		if m := re.FindStringSubmatch(combined); len(m) > 1 {
			timeStr := m[1]
			// Very basic parsing attempt. Time formats vary, so we do best effort.
			// Try "3:04pm (MST)"
			t, err := time.Parse("3:04pm (MST)", timeStr)
			if err == nil {
				now := time.Now().UTC()
				target := time.Date(now.Year(), now.Month(), now.Day(), t.Hour(), t.Minute(), 0, 0, time.UTC)
				if target.Before(now) {
					target = target.Add(24 * time.Hour)
				}
				cooldown = target.Sub(now)
			}
		}
	}

	return true, cooldown
}
