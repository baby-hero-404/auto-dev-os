package patch

import (
	"regexp"
	"strings"

	"github.com/auto-code-os/auto-code-os/server/pkg/paths"
)

func ExtractPatch(parsed map[string]any) string {
	if parsed == nil {
		return ""
	}
	var p string
	if v, ok := parsed["patch"].(string); ok && v != "" {
		p = v
	} else if v, ok := parsed["patch_text"].(string); ok && v != "" {
		p = v
	} else if v, ok := parsed["diff"].(string); ok && v != "" {
		p = v
	}
	if p == "" {
		return ""
	}
	p = strings.TrimSpace(p)
	if strings.HasPrefix(p, "```") {
		lines := strings.Split(p, "\n")
		if len(lines) >= 2 {
			endIdx := len(lines) - 1
			for i := len(lines) - 1; i > 0; i-- {
				if strings.HasPrefix(strings.TrimSpace(lines[i]), "```") {
					endIdx = i
					break
				}
			}
			p = strings.Join(lines[1:endIdx], "\n")
		}
	}
	return strings.TrimSpace(p) + "\n"
}

func CleanPatchPaths(patchText string) string {
	re := regexp.MustCompile(`([ab])/` + regexp.QuoteMeta(paths.ReposDirName) + `/[^/]+/(?:worktrees/[^/]+|[^/]+)/`)
	return re.ReplaceAllString(patchText, "$1/")
}

func CleanJunkLines(patchText string) string {
	lines := strings.Split(patchText, "\n")
	var cleaned []string

	validDiffHeaders := []string{
		"diff --git ",
		"index ",
		"--- ",
		"+++ ",
		"@@ ",
		"new file mode ",
		"deleted file mode ",
		"similarity index ",
		"rename from ",
		"rename to ",
		"copy from ",
		"copy to ",
		"old mode ",
		"new mode ",
		"\\ No newline at end of file",
	}

	inHunk := false
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)

		// Check if it's a valid diff header line
		isHeader := false
		for _, prefix := range validDiffHeaders {
			if strings.HasPrefix(line, prefix) {
				isHeader = true
				break
			}
		}

		if isHeader {
			inHunk = false
			if strings.HasPrefix(line, "@@ ") {
				inHunk = true
			}
			cleaned = append(cleaned, line)
			continue
		}

		if inHunk {
			// In unified diff hunks, lines must start with '+', '-', ' ' (space), or be empty (sometimes LLMs omit the space)
			if strings.HasPrefix(line, "+") || strings.HasPrefix(line, "-") || strings.HasPrefix(line, " ") || line == "" {
				cleaned = append(cleaned, line)
			} else {
				inHunk = false
			}
		} else {
			// Outside of a hunk, we only preserve empty lines (or header lines which were already handled)
			if trimmed == "" {
				cleaned = append(cleaned, line)
			}
		}
	}
	return strings.Join(cleaned, "\n") + "\n"
}

func CleanRepoPrefix(block string, repoName string) string {
	escapedRepo := regexp.QuoteMeta(repoName)
	block = regexp.MustCompile(`^(a/)`+escapedRepo+`/`).ReplaceAllString(block, "${1}")
	block = regexp.MustCompile(`( b/)`+escapedRepo+`/`).ReplaceAllString(block, "${1}")
	block = regexp.MustCompile(`(?m)^(--- a/)`+escapedRepo+`/`).ReplaceAllString(block, "${1}")
	block = regexp.MustCompile(`(?m)^(\+\+\+ b/)`+escapedRepo+`/`).ReplaceAllString(block, "${1}")
	block = regexp.MustCompile(`(?m)^(rename from )`+escapedRepo+`/`).ReplaceAllString(block, "${1}")
	block = regexp.MustCompile(`(?m)^(rename to )`+escapedRepo+`/`).ReplaceAllString(block, "${1}")
	block = regexp.MustCompile(`(?m)^(copy from )`+escapedRepo+`/`).ReplaceAllString(block, "${1}")
	block = regexp.MustCompile(`(?m)^(copy to )`+escapedRepo+`/`).ReplaceAllString(block, "${1}")
	return block
}

// SplitPatchByRepo splits a unified diff into per-repository patches.
// It delegates to a zero-workspace SplitPatchByRepoWithWorkspace for consistency.
func SplitPatchByRepo(patchText string) map[string]string {
	r := &Runner{}
	return r.SplitPatchByRepoWithWorkspace(patchText, nil, "")
}
