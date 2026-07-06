package parser

import (
	"os"
	"regexp"

	"github.com/auto-code-os/auto-code-os/server/internal/context/source"
)

// wordRegex matches any valid identifier pattern.
var wordRegex = regexp.MustCompile(`[a-zA-Z_][a-zA-Z0-9_]*`)

// ExtractFallbackTags uses simple Regex to blindly extract possible references
// if Tree-sitter fails or does not support the language.
func ExtractFallbackTags(filepath string) ([]source.Tag, error) {
	content, err := os.ReadFile(filepath)
	if err != nil {
		return nil, err
	}

	matches := wordRegex.FindAllString(string(content), -1)
	
	var tags []source.Tag
	// In fallback mode, everything is a "ref" to ensure we don't miss calls.
	for _, match := range matches {
		tags = append(tags, source.Tag{
			Name:     match,
			Kind:     "ref",
			Line:     -1, // Line is unknown or unimportant for fallback blind refs
			EndLine:  -1,
			Filepath: filepath,
		})
	}
	
	return tags, nil
}
