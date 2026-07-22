package patch

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
)

// ParseSearchReplace parses the LLM output into EditBlocks.
// It looks for Aider-style blocks:
// <<<<<<< SEARCH
// old code
// =======
// new code
// >>>>>>> REPLACE
func ParseSearchReplace(patchData string) []EditBlock {
	var blocks []EditBlock
	var currentBlock EditBlock

	lines := strings.Split(patchData, "\n")

	const (
		StateNormal = iota
		StateSearch
		StateReplace
	)

	state := StateNormal
	var searchLines []string
	var replaceLines []string
	var lastNonEmptyLine string

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)

		switch state {
		case StateNormal:
			if strings.HasPrefix(trimmed, "<<<<<<< SEARCH") {
				state = StateSearch
				searchLines = nil
				replaceLines = nil

				// Try to extract filepath from the last non-empty line
				// which often looks like `path/to/file.go` or `File: path/to/file.go`
				currentBlock = EditBlock{}

				// Basic heuristic to get filename
				if lastNonEmptyLine != "" {
					cleanPath := strings.TrimPrefix(lastNonEmptyLine, "File: ")
					cleanPath = strings.TrimPrefix(cleanPath, "File:")
					cleanPath = strings.TrimPrefix(cleanPath, "file: ")
					cleanPath = strings.TrimSpace(cleanPath)
					cleanPath = strings.TrimPrefix(cleanPath, "`")
					cleanPath = strings.TrimSuffix(cleanPath, "`")
					currentBlock.Filepath = cleanPath
				}
			} else if trimmed != "" {
				// Don't save backticks as the filename
				if trimmed != "```" && trimmed != "```diff" {
					lastNonEmptyLine = trimmed
				}
			}
		case StateSearch:
			if strings.HasPrefix(trimmed, "=======") {
				state = StateReplace
			} else {
				searchLines = append(searchLines, line)
			}
		case StateReplace:
			if strings.HasPrefix(trimmed, ">>>>>>> REPLACE") {
				state = StateNormal
				currentBlock.Search = strings.Join(searchLines, "\n")
				// If there's search content, append a trailing newline if it originally had one.
				// Since strings.Split removed the newline, Join adds them between lines.
				// Let's add a trailing newline if there were lines.
				if len(searchLines) > 0 {
					currentBlock.Search += "\n"
				}

				currentBlock.Replace = strings.Join(replaceLines, "\n")
				if len(replaceLines) > 0 {
					currentBlock.Replace += "\n"
				}

				blocks = append(blocks, currentBlock)
			} else {
				replaceLines = append(replaceLines, line)
			}
		}
	}

	return blocks
}

// ApplySearchReplace applies the edit blocks to the files on disk.
func ApplySearchReplace(blocks []EditBlock, basePath string) error {
	// Group blocks by file to apply multiple edits to the same file
	blocksByFile := make(map[string][]EditBlock)
	for _, b := range blocks {
		if b.Filepath == "" {
			return fmt.Errorf("missing filepath in edit block")
		}
		blocksByFile[b.Filepath] = append(blocksByFile[b.Filepath], b)
	}

	for relPath, fileBlocks := range blocksByFile {
		var fullPath string
		if basePath == "" {
			fullPath = relPath
		} else {
			fullPath = filepath.Join(basePath, relPath)
		}
		contentBytes, err := os.ReadFile(fullPath)
		var content string
		if err != nil {
			if os.IsNotExist(err) {
				content = "" // New file
			} else {
				return fmt.Errorf("cannot read file %s: %w", relPath, err)
			}
		} else {
			content = string(contentBytes)
		}

		// Normalize newlines for safer replacement
		content = strings.ReplaceAll(content, "\r\n", "\n")

		for _, block := range fileBlocks {
			search := strings.ReplaceAll(block.Search, "\r\n", "\n")
			replace := strings.ReplaceAll(block.Replace, "\r\n", "\n")

			if search == "" {
				// Overwrite entire file (create or replace)
				content = replace
			} else {
				count := strings.Count(content, search)
				if count == 1 {
					content = strings.Replace(content, search, replace, 1)
					continue
				}
				if count > 1 {
					return fmt.Errorf("ambiguous match in %s (found %d times)", relPath, count)
				}

				// Exact match failed (count == 0) — try the tiered fuzzy
				// fallback pipeline before giving up. Tiers are tried in
				// increasing order of permissiveness, stopping at the first
				// tier that produces a unique match; any tier that matches
				// 2+ locations fails fast rather than falling through to a
				// fuzzier tier (a fuzzier tier is even more likely to also
				// be ambiguous, and multi-match at any tier is itself a
				// signal the patch is unsafe to guess at).
				matchers := []struct {
					tier int
					fn   func(string, string) (int, int, []int, byte, bool, int)
				}{
					{1, trailingWSMatch},
					{2, relativeIndentMatch},
					{3, trimmedLineMatch},
				}

				matched := false
				for _, m := range matchers {
					start, end, deltas, indentChar, ok, matchCount := m.fn(content, search)
					if matchCount > 1 {
						return fmt.Errorf("ambiguous match in %s (%s fallback found %d candidates)", relPath, tierNames[m.tier], matchCount)
					}
					if ok {
						content = content[:start] + reindentReplace(replace, deltas, indentChar) + content[end:]
						log.Printf("search_replace tier=%d (%s) file=%s", m.tier, tierNames[m.tier], relPath)
						matched = true
						break
					}
				}
				if matched {
					continue
				}

				startLine, endLine, snippet := nearestSimilarRange(content, search)
				if snippet == "" {
					return fmt.Errorf("search block not found in %s", relPath)
				}
				return fmt.Errorf("search block not found in %s; closest match is lines %d-%d:\n%s", relPath, startLine, endLine, snippet)
			}
		}

		if err := os.MkdirAll(filepath.Dir(fullPath), 0755); err != nil {
			return fmt.Errorf("cannot create directories for %s: %w", relPath, err)
		}

		if err := os.WriteFile(fullPath, []byte(content), 0644); err != nil {
			return fmt.Errorf("cannot write file %s: %w", relPath, err)
		}
	}

	return nil
}
