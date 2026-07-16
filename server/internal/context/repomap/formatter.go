package repomap

import (
	"fmt"
	"sort"
	"strings"

	"github.com/auto-code-os/auto-code-os/server/internal/context/source"
)

// FormatSkeleton groups tags by file and formats them into a lightweight text structure.
// Only "def" tags are rendered — "ref" tags exist purely to drive PageRank edge-weighting
// (graph.BuildGraph) and PruneTags' token-budget selection; rendering them here would leak each
// function's internal call sequence (every stdlib/helper call it makes), violating the "no code
// bodies" acceptance criterion this feature was built against (docs/features/engineering/
// 01-context-management.md AC-4) and burning token budget that should go to showing more files'
// signatures instead of one file's call graph.
func FormatSkeleton(tags []source.Tag) string {
	if len(tags) == 0 {
		return ""
	}

	// Group by filepath, defs only.
	byFile := make(map[string][]source.Tag)
	for _, t := range tags {
		if t.Kind != "def" {
			continue
		}
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

			// Line count gives the model a cheap signal for whether a function is worth a
			// read_file call: a 3-line function likely needs no closer look, a 150-line one
			// probably does. EndLine now spans the real declaration (see symbol.ExtractTags),
			// not just the identifier token, so this is meaningful — before that fix it would
			// always have printed "(1 lines)" regardless of actual size.
			lines := max(t.EndLine-t.Line+1, 1)
			fmt.Fprintf(&sb, "  def %s (%d lines)\n", t.Name, lines)
		}
		sb.WriteString("\n")
	}

	return strings.TrimSpace(sb.String())
}
