package patch

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

var hunkHeaderRegex = regexp.MustCompile(`^@@ -(\d+)(?:,(\d+))? \+(\d+)(?:,(\d+))? @@`)
var diffFileHeaderRegex = regexp.MustCompile(`^\+\+\+ (?:b/)?(.*)$`)

func ValidateUnifiedDiff(patch string, basePath string) []ValidationError {
	var errors []ValidationError
	lines := strings.Split(patch, "\n")

	var currentFile string
	for _, line := range lines {
		if strings.HasPrefix(line, "+++ ") {
			matches := diffFileHeaderRegex.FindStringSubmatch(line)
			if len(matches) > 1 {
				currentFile = strings.TrimSpace(matches[1])
				currentFile = strings.TrimPrefix(currentFile, "b/")
				if currentFile == "/dev/null" {
					currentFile = ""
				}
			}
		} else if strings.HasPrefix(line, "@@ ") {
			matches := hunkHeaderRegex.FindStringSubmatch(line)
			if len(matches) > 1 && currentFile != "" {
				var oldStart int
				_, err := fmt.Sscanf(matches[1], "%d", &oldStart)
				if err == nil {
					fullPath := filepath.Join(basePath, currentFile)
					contentBytes, err := os.ReadFile(fullPath)
					if err == nil {
						content := string(contentBytes)
						fileLines := strings.Split(content, "\n")
						if oldStart > len(fileLines) && len(fileLines) > 0 {
							errors = append(errors, ValidationError{
								Filepath: currentFile,
								Reason:   fmt.Sprintf("Hunk start line %d exceeds file line count %d", oldStart, len(fileLines)),
								IsFatal:  true,
							})
						}
					}
				}
			}
		}
	}
	return errors
}

// ValidateSearchReplace verifies that each search block matches exactly once in the target file.
func ValidateSearchReplace(blocks []EditBlock, basePath string) []ValidationError {
	var errors []ValidationError

	for _, block := range blocks {
		if block.Filepath == "" {
			errors = append(errors, ValidationError{
				Reason:  "Missing file path in edit block",
				IsFatal: true,
			})
			continue
		}

		fullPath := filepath.Join(basePath, block.Filepath)
		contentBytes, err := os.ReadFile(fullPath)
		if err != nil {
			if os.IsNotExist(err) {
				// If file doesn't exist, it might be a create operation (search is empty)
				if strings.TrimSpace(block.Search) != "" {
					errors = append(errors, ValidationError{
						Filepath: block.Filepath,
						Reason:   "File does not exist, but search block is not empty",
						IsFatal:  true,
					})
				}
			} else {
				errors = append(errors, ValidationError{
					Filepath: block.Filepath,
					Reason:   fmt.Sprintf("Cannot read file: %v", err),
					IsFatal:  true,
				})
			}
			continue
		}

		content := string(contentBytes)

		// If search block is empty, it means create or append. In Aider style, creating a new file has empty search.
		// For now we enforce search must exist.
		if block.Search == "" {
			continue // Or validate it's an empty file.
		}

		// Normalize newlines for counting
		normalizedContent := strings.ReplaceAll(content, "\r\n", "\n")
		normalizedSearch := strings.ReplaceAll(block.Search, "\r\n", "\n")

		count := strings.Count(normalizedContent, normalizedSearch)
		if count == 0 {
			errors = append(errors, ValidationError{
				Filepath: block.Filepath,
				Reason:   "Search block not found in file",
				IsFatal:  true,
			})
		} else if count > 1 {
			errors = append(errors, ValidationError{
				Filepath: block.Filepath,
				Reason:   fmt.Sprintf("Ambiguous match: Search block matches %d times in file", count),
				IsFatal:  true,
			})
		}
	}

	return errors
}
