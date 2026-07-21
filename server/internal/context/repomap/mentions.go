package repomap

import "unicode"

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
