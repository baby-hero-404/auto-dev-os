package codecontext

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

func extractAffectedFiles(raw []byte) []string {
	// Simple approach: scan for "affected_files" key in the JSON.
	// Avoids importing encoding/json to keep this lightweight.
	var files []string
	s := string(raw)
	idx := strings.Index(s, `"affected_files"`)
	if idx < 0 {
		return nil
	}
	// Find the array start.
	arrStart := strings.Index(s[idx:], "[")
	if arrStart < 0 {
		return nil
	}
	arrEnd := strings.Index(s[idx+arrStart:], "]")
	if arrEnd < 0 {
		return nil
	}
	arr := s[idx+arrStart : idx+arrStart+arrEnd+1]
	// Extract quoted strings from the array.
	re := regexp.MustCompile(`"([^"]+)"`)
	matches := re.FindAllStringSubmatch(arr, -1)
	for _, m := range matches {
		if len(m) > 1 {
			files = append(files, m[1])
		}
	}
	return files
}

func extractKeywords(text string) []string {
	// Simple tokenization: lowercase, split on whitespace/punctuation.
	re := regexp.MustCompile(`[a-zA-Z_]{3,}`)
	tokens := re.FindAllString(strings.ToLower(text), -1)
	// Filter out very common stop words.
	stop := map[string]bool{
		"the": true, "and": true, "for": true, "with": true, "from": true,
		"this": true, "that": true, "should": true, "will": true, "have": true,
		"not": true, "are": true, "was": true, "been": true, "also": true,
	}
	var keywords []string
	for _, t := range tokens {
		if !stop[t] {
			keywords = append(keywords, t)
		}
	}
	return keywords
}

func (r *FileContextRetriever) searchByKeywords(repoPath string, keywords []string) []string {
	var found []string
	_ = filepath.WalkDir(repoPath, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return filepath.SkipDir
		}
		if d.IsDir() && (d.Name() == ".git" || d.Name() == "node_modules" || d.Name() == "vendor" || d.Name() == ".next") {
			return filepath.SkipDir
		}
		if d.IsDir() {
			return nil
		}
		ext := filepath.Ext(d.Name())
		if ext != ".go" && ext != ".ts" && ext != ".tsx" && ext != ".js" && ext != ".jsx" {
			return nil
		}

		rel, _ := filepath.Rel(repoPath, path)
		pathLower := strings.ToLower(rel)
		for _, kw := range keywords {
			if strings.Contains(pathLower, kw) {
				found = append(found, rel)
				break
			}
		}
		return nil
	})
	return found
}

// Utility helpers.

func appendUnique(base []string, items ...string) []string {
	seen := make(map[string]bool, len(base))
	for _, b := range base {
		seen[b] = true
	}
	for _, item := range items {
		if !seen[item] {
			base = append(base, item)
			seen[item] = true
		}
	}
	return base
}

func dedup(items []string) []string {
	seen := make(map[string]bool, len(items))
	var result []string
	for _, item := range items {
		if !seen[item] {
			result = append(result, item)
			seen[item] = true
		}
	}
	return result
}
