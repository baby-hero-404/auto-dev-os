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
	return strings.Join(cleaned, "\n")
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

func SplitPatchByRepo(patchText string) map[string]string {
	repos := make(map[string]string)
	parts := strings.Split(patchText, "diff --git ")
	if len(parts) <= 1 || (len(parts) == 2 && parts[0] == "" && !strings.Contains(patchText, "diff --git ")) {
		trimmed := strings.TrimSpace(patchText)
		if trimmed == "" {
			return repos
		}
		lines := strings.Split(trimmed, "\n")
		repoName := ""
		for _, line := range lines {
			var path string
			if strings.HasPrefix(line, "--- a/") {
				path = line[len("--- a/"):]
			} else if strings.HasPrefix(line, "+++ b/") {
				path = line[len("+++ b/"):]
			} else {
				continue
			}
			path = strings.ReplaceAll(path, "\\", "/")
			path = strings.TrimSpace(path)

			codeReposPrefix := paths.ReposPrefix()
			if strings.HasPrefix(path, codeReposPrefix) {
				after := path[len(codeReposPrefix):]
				idx := strings.Index(after, "/")
				if idx != -1 {
					repoName = after[:idx]
					break
				}
			} else {
				idx := strings.Index(path, "/")
				if idx != -1 {
					repoName = path[:idx]
					break
				}
			}
		}
		if repoName != "" {
			cleanPatch := trimmed
			reposPrefix := paths.ReposPrefix()
			if strings.Contains(cleanPatch, reposPrefix) {
				cleanPatch = regexp.MustCompile(`([ab])/`+regexp.QuoteMeta(reposPrefix)+regexp.QuoteMeta(repoName)+`/(?:worktrees/[^/]+|[^/]+)/`).ReplaceAllString(cleanPatch, "$1/")
			} else {
				cleanPatch = CleanRepoPrefix(cleanPatch, repoName)
			}
			repos[repoName] = cleanPatch
		} else {
			hasDiffHeader := false
			for _, line := range lines {
				if strings.HasPrefix(line, "--- a/") || strings.HasPrefix(line, "+++ b/") {
					hasDiffHeader = true
					break
				}
			}
			if hasDiffHeader {
				repos[""] = trimmed
			}
		}
		return repos
	}
	header := parts[0]

	re := regexp.MustCompile(`^a/` + regexp.QuoteMeta(paths.ReposPrefix()) + `([^/]+)/(?:worktrees/[^/]+|[^/]+)/`)
	repoBlocks := make(map[string][]string)
	for i := 1; i < len(parts); i++ {
		block := parts[i]
		if matches := re.FindStringSubmatch(block); len(matches) > 1 {
			repoName := matches[1]
			cleanBlock := regexp.MustCompile(`([ab])/`+regexp.QuoteMeta(paths.ReposPrefix())+regexp.QuoteMeta(repoName)+`/(?:worktrees/[^/]+|[^/]+)/`).ReplaceAllString(block, "$1/")
			repoBlocks[repoName] = append(repoBlocks[repoName], "diff --git "+cleanBlock)
		} else if strings.HasPrefix(block, "a/") {
			sub := block[2:]
			sepIdx := strings.Index(sub, " b/")
			if sepIdx != -1 {
				firstPath := sub[:sepIdx]
				idx := strings.Index(firstPath, "/")
				if idx != -1 {
					repoName := firstPath[:idx]
					if repoName != paths.ReposDirName {
						cleanBlock := CleanRepoPrefix(block, repoName)
						repoBlocks[repoName] = append(repoBlocks[repoName], "diff --git "+cleanBlock)
					}
				} else {
					repoBlocks[""] = append(repoBlocks[""], "diff --git "+block)
				}
			} else {
				idx := strings.Index(sub, "/")
				if idx != -1 {
					repoName := sub[:idx]
					if repoName != paths.ReposDirName {
						cleanBlock := CleanRepoPrefix(block, repoName)
						repoBlocks[repoName] = append(repoBlocks[repoName], "diff --git "+cleanBlock)
					}
				}
			}
		}
	}

	for repoName, blocks := range repoBlocks {
		repos[repoName] = header + strings.Join(blocks, "")
	}
	return repos
}
