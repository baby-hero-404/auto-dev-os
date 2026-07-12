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
	
	// Check hunk count mismatches
	errors = append(errors, ValidateHunkCounts(patch)...)

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

func ValidateHunkCounts(patch string) []ValidationError {
	var errors []ValidationError
	lines := strings.Split(patch, "\n")

	var currentFile string
	var inHunk bool
	var currentHunkHeader string
	var expectedOldLen, expectedNewLen int
	var actualOldLen, actualNewLen int

	flushHunk := func() {
		if !inHunk {
			return
		}
		if actualOldLen != expectedOldLen || actualNewLen != expectedNewLen {
			errors = append(errors, ValidationError{
				Filepath: currentFile,
				Reason:   fmt.Sprintf("Hunk line count mismatch in header %q: expected old=%d/new=%d, got old=%d/new=%d", currentHunkHeader, expectedOldLen, expectedNewLen, actualOldLen, actualNewLen),
				IsFatal:  true,
			})
		}
		inHunk = false
	}

	for _, line := range lines {
		if strings.HasPrefix(line, "+++ ") {
			flushHunk()
			matches := diffFileHeaderRegex.FindStringSubmatch(line)
			if len(matches) > 1 {
				currentFile = strings.TrimSpace(matches[1])
				currentFile = strings.TrimPrefix(currentFile, "b/")
				if currentFile == "/dev/null" {
					currentFile = ""
				}
			}
			continue
		}
		if strings.HasPrefix(line, "--- ") {
			flushHunk()
			continue
		}
		if strings.HasPrefix(line, "diff ") {
			flushHunk()
			continue
		}
		if strings.HasPrefix(line, "@@ ") {
			flushHunk()
			matches := hunkHeaderRegex.FindStringSubmatch(line)
			if len(matches) > 1 {
				inHunk = true
				currentHunkHeader = strings.TrimSpace(line)
				// old length
				if matches[2] != "" {
					fmt.Sscanf(matches[2], "%d", &expectedOldLen)
				} else {
					expectedOldLen = 1
				}
				// new length
				if matches[4] != "" {
					fmt.Sscanf(matches[4], "%d", &expectedNewLen)
				} else {
					expectedNewLen = 1
				}
				actualOldLen = 0
				actualNewLen = 0
			}
			continue
		}

		if inHunk {
			if strings.HasPrefix(line, "-") {
				actualOldLen++
			} else if strings.HasPrefix(line, "+") {
				actualNewLen++
			} else if strings.HasPrefix(line, " ") {
				actualOldLen++
				actualNewLen++
			} else if strings.HasPrefix(line, "\\") {
				// ignore metadata line e.g. \ No newline at end of file
			} else if line == "" {
				if actualOldLen < expectedOldLen || actualNewLen < expectedNewLen {
					actualOldLen++
					actualNewLen++
				}
			}
		}
	}
	flushHunk()
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

		// If search block is empty, it means create or overwrite entire file.
		// In Aider style, creating/overwriting a file has empty search.
		if block.Search == "" {
			continue
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
