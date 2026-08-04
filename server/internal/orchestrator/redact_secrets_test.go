package orchestrator

import "testing"

// TestRedactSecrets_CoversProviderCredentialFormats guards Phase 7's "Secure
// Auth Injection" requirement that provider credentials never leak into
// persisted logs/checkpoints: redactSecrets is the single choke point
// tracker.go's log() and engine/cli.go's CodeStepResult construction both
// run captured CLI output through before it's ever written to
// TaskLog/WorkflowCheckpoint/artifact storage. This asserts every credential
// shape the orchestrator actually injects (ANTHROPIC_API_KEY, OPENAI_API_KEY,
// GEMINI_API_KEY, a GitHub PAT) is matched, not just that the function runs.
func TestRedactSecrets_CoversProviderCredentialFormats(t *testing.T) {
	cases := []struct {
		name   string
		secret string
	}{
		{"anthropic", "sk-ant-" + repeatChar("a", 95)},
		{"openai_legacy", "sk-" + repeatChar("b", 48)},
		{"openai_project", "sk-proj-" + repeatChar("c", 160)},
		{"gemini", "AIzaSy" + repeatChar("D", 33)},
		{"github_pat_classic", "ghp_" + repeatChar("e", 36)},
		{"github_pat_fine_grained", "github_pat_" + repeatChar("f", 82)},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			input := "some CLI output leaked env var ANTHROPIC_API_KEY=" + tc.secret + " during a debug print"
			got := redactSecrets(input)
			if got == input {
				t.Fatalf("redactSecrets did not redact %s credential: %q", tc.name, tc.secret)
			}
			if containsSubstring(got, tc.secret) {
				t.Fatalf("redacted output still contains raw %s secret: %q", tc.name, got)
			}
		})
	}
}

func repeatChar(c string, n int) string {
	out := make([]byte, 0, n*len(c))
	for i := 0; i < n; i++ {
		out = append(out, c...)
	}
	return string(out)
}

func containsSubstring(haystack, needle string) bool {
	if needle == "" {
		return false
	}
	for i := 0; i+len(needle) <= len(haystack); i++ {
		if haystack[i:i+len(needle)] == needle {
			return true
		}
	}
	return false
}
