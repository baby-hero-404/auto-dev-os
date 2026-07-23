package repomap

import "testing"

func TestExtractMentionedIdents(t *testing.T) {
	t.Run("extracts CalculatePageRank, excludes short/lowercase words", func(t *testing.T) {
		// Boundary case: "CalculatePageRank" (17 runes, has uppercase) must be
		// extracted; "ranking" (7 runes, below the 8-rune threshold) and
		// "module" (6 runes, lowercase-only) must both be excluded, along with
		// short common words like "fix"/"the"/"in".
		got := ExtractMentionedIdents("fix the CalculatePageRank function in ranking module")

		if !got["CalculatePageRank"] {
			t.Errorf("expected CalculatePageRank to be extracted, got %v", got)
		}
		if got["ranking"] {
			t.Errorf("expected 'ranking' (7 runes, below threshold) to be excluded, got %v", got)
		}
		if got["module"] {
			t.Errorf("expected 'module' (lowercase, no identifier signal) to be excluded, got %v", got)
		}
		if got["function"] {
			t.Errorf("expected 'function' (lowercase, no identifier signal) to be excluded, got %v", got)
		}
		if len(got) != 1 {
			t.Errorf("expected exactly 1 extracted ident, got %d: %v", len(got), got)
		}
	})

	t.Run("empty text returns empty map", func(t *testing.T) {
		got := ExtractMentionedIdents("")
		if len(got) != 0 {
			t.Errorf("expected empty map, got %v", got)
		}
	})

	t.Run("snake_case and kebab-case tokens length >= 8 are extracted regardless of case", func(t *testing.T) {
		got := ExtractMentionedIdents("update user_service and repo-mapper now")
		if !got["user_service"] {
			t.Errorf("expected user_service to be extracted, got %v", got)
		}
		if !got["repo-mapper"] {
			t.Errorf("expected repo-mapper to be extracted, got %v", got)
		}
	})

	t.Run("8-rune all-lowercase token with no separators is excluded", func(t *testing.T) {
		// "algorithm" is 9 runes, all lowercase, no '_'/'-' — should not look
		// like a code identifier and must be excluded.
		got := ExtractMentionedIdents("the algorithm needs work")
		if got["algorithm"] {
			t.Errorf("expected 'algorithm' to be excluded (no identifier signal), got %v", got)
		}
	})
}

func TestExtractMentionedPaths(t *testing.T) {
	t.Run("extracts bare filename and slash path", func(t *testing.T) {
		got := ExtractMentionedPaths("please fix `policy_engine.go` and server/internal/policy/loader.go")
		if !got["policy_engine.go"] {
			t.Errorf("expected policy_engine.go to be extracted, got %v", got)
		}
		if !got["server/internal/policy/loader.go"] {
			t.Errorf("expected server/internal/policy/loader.go to be extracted, got %v", got)
		}
	})

	t.Run("empty text returns empty map", func(t *testing.T) {
		got := ExtractMentionedPaths("")
		if len(got) != 0 {
			t.Errorf("expected empty map, got %v", got)
		}
	})

	t.Run("bilingual (Vietnamese) task text still extracts path", func(t *testing.T) {
		got := ExtractMentionedPaths("sửa lỗi trong ranking.go giúp mình với")
		if !got["ranking.go"] {
			t.Errorf("expected ranking.go to be extracted from Vietnamese text, got %v", got)
		}
	})

	t.Run("plain prose without path-shaped tokens returns empty map", func(t *testing.T) {
		got := ExtractMentionedPaths("please fix the login flow soon")
		if len(got) != 0 {
			t.Errorf("expected empty map, got %v", got)
		}
	})
}
