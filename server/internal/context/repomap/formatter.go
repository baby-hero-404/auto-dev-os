package repomap

import (
	"fmt"
	"sort"
	"strings"
	"github.com/auto-code-os/auto-code-os/server/internal/context/source"
)

// FormatSkeleton groups tags by file and formats them into a lightweight text structure
func FormatSkeleton(tags []source.Tag) string {
	if len(tags) == 0 {
		return ""
	}
	
	// Group by filepath
	byFile := make(map[string][]source.Tag)
	for _, t := range tags {
		byFile[t.Filepath] = append(byFile[t.Filepath], t)
	}
	
	// Sort files alphabetically to ensure deterministic output for LLM
	var files []string
	for f := range byFile {
		files = append(files, f)
	}
	sort.Strings(files)
	
	var sb strings.Builder
	
	for _, f := range files {
		fileTags := byFile[f]
		
		// Sort tags by line number within the file to maintain top-to-bottom layout
		sort.Slice(fileTags, func(i, j int) bool {
			return fileTags[i].Line < fileTags[j].Line
		})
		
		sb.WriteString(f)
		sb.WriteString(":\n")
		
		// De-duplicate lines to avoid printing the same line twice
		lastLine := -1
		
		for _, t := range fileTags {
			if t.Line == lastLine {
				continue
			}
			lastLine = t.Line
			
			// Indentation formatting based on tag kind.
			if t.Kind == "def" {
				sb.WriteString(fmt.Sprintf("  def %s\n", t.Name))
			} else {
				sb.WriteString(fmt.Sprintf("    ref %s\n", t.Name))
			}
		}
		sb.WriteString("\n")
	}
	
	return strings.TrimSpace(sb.String())
}
