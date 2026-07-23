package repomap

import (
	"regexp"
	"unicode"
)

// pathLikePattern matches file-path-shaped tokens in free text: bare
// filenames with an extension ("policy_engine.go") or slash-separated
// paths ("server/internal/policy_engine.go").
var pathLikePattern = regexp.MustCompile(`[A-Za-z0-9_\-./]*[A-Za-z0-9_\-]+\.[A-Za-z0-9]+`)

// ExtractMentionedPaths pulls path-like tokens out of free text (e.g. a task
// title/description), per Aider's rule that a directly-mentioned file path
// carries the same intent signal as an actively-open file (REQ-003).
// Filtering against files that actually exist in the repo happens at the
// ranking layer (mentionedFileNodeIDs), mirroring how ExtractMentionedIdents
// is filtered against known definitions there.
func ExtractMentionedPaths(text string) map[string]bool {
	paths := make(map[string]bool)
	if text == "" {
		return paths
	}
	for _, m := range pathLikePattern.FindAllString(text, -1) {
		paths[m] = true
	}
	return paths
}

// ExtractMentionedIdents tokenizes free text (e.g. a task title/description)
// into identifier candidates using the same heuristic Aider applies to
// mentioned_idents: snake_case, camelCase, or kebab-case tokens of length
// >= 8. This is distinct from symbol.ExtractTags, which parses source files
// via tree-sitter rather than prose.
func ExtractMentionedIdents(text string) map[string]bool {
	idents := make(map[string]bool)
	if text == "" {
		return idents
	}

	isIdentRune := func(r rune) bool {
		return unicode.IsLetter(r) || unicode.IsDigit(r) || r == '_' || r == '-'
	}

	start := -1
	for i, r := range text {
		if isIdentRune(r) {
			if start == -1 {
				start = i
			}
			continue
		}
		if start != -1 {
			addIfCandidate(idents, text[start:i])
			start = -1
		}
	}
	if start != -1 {
		addIfCandidate(idents, text[start:])
	}

	return idents
}

func addIfCandidate(idents map[string]bool, token string) bool {
	if len([]rune(token)) < 8 {
		return false
	}
	looksLikeIdent := false
	for _, r := range token {
		if r == '_' || r == '-' || unicode.IsUpper(r) {
			looksLikeIdent = true
			break
		}
	}
	if !looksLikeIdent {
		return false
	}
	idents[token] = true
	return true
}
