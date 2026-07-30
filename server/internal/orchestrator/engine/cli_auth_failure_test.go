package engine

import "testing"

func TestDetectAuthFailure(t *testing.T) {
	cases := []struct {
		name     string
		ref      string
		combined string
		want     bool
	}{
		// Real production message: claude_code's own list didn't cover it
		// (only "not logged in" / "please run /login" / "invalid api key"),
		// so it silently fell through undetected until the "*" list was
		// made an unconditional supplement instead of an unknown-ref fallback.
		{"claude_code revoked oauth token", "claude_code", "Failed to authenticate. API Error: 401 OAuth access token has been revoked.\n", true},
		{"claude_code not logged in", "claude_code", "Not logged in · Please run /login\n", true},
		{"claude_code unrelated failure", "claude_code", "SyntaxError: unexpected token", false},
		{"openai_codex not logged in", "openai_codex", "Error: not logged in, please run codex login", true},
		{"unknown ref matches generic 401", "some_future_tool", "request failed: 401 unauthorized", true},
		{"unknown ref, no match", "some_future_tool", "panic: nil pointer dereference", false},
		{"empty ref matches generic", "", "authentication required", true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := detectAuthFailure(tc.ref, tc.combined)
			if got != tc.want {
				t.Errorf("detectAuthFailure(%q, %q) = %v, want %v", tc.ref, tc.combined, got, tc.want)
			}
		})
	}
}
