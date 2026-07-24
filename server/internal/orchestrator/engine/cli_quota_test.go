package engine

import "testing"

func TestDetectQuotaExceeded(t *testing.T) {
	cases := []struct {
		name     string
		ref      string
		combined string
		exitCode int
		want     bool
	}{
		{"claude_code usage limit", "claude_code", "Error: usage limit reached, please try again later", 1, true},
		{"claude_code unrelated failure", "claude_code", "SyntaxError: unexpected token", 1, false},
		{"openai_codex rate limit", "openai_codex", "OpenAI API error: rate limit exceeded", 1, true},
		{"openai_codex 429", "openai_codex", "request failed with status 429", 1, true},
		{"antigravity quota", "antigravity", "quota exceeded for this account", 1, true},
		{"unknown ref falls back to *", "some_future_tool", "please try again later", 1, true},
		{"unknown ref, no match", "some_future_tool", "panic: nil pointer dereference", 1, false},
		{"empty ref (legacy fallback) matches *", "", "Rate Limit hit", 1, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := detectQuotaExceeded(tc.ref, tc.combined, tc.exitCode)
			if got != tc.want {
				t.Errorf("detectQuotaExceeded(%q, %q, %d) = %v, want %v", tc.ref, tc.combined, tc.exitCode, got, tc.want)
			}
		})
	}
}
