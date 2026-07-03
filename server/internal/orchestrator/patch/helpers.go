package patch

import (
	"path/filepath"
	"regexp"
	"strings"

	"github.com/auto-code-os/auto-code-os/server/internal/orchestrator/workspace"
	"github.com/auto-code-os/auto-code-os/server/pkg/models"
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

func DeriveChangeName(task *models.Task) string {
	slug := strings.ToLower(task.Title)
	reg := regexp.MustCompile(`[^a-z0-9]+`)
	slug = reg.ReplaceAllString(slug, "-")
	slug = strings.Trim(slug, "-")
	if len(slug) > 30 {
		slug = slug[:30]
	}
	slug = strings.Trim(slug, "-")
	if slug == "" {
		slug = "task-" + task.ID
		if len(slug) > 13 {
			slug = slug[:13]
		}
	}
	return slug
}

func repoNameFromURL(repoURL string) string {
	parts := strings.Split(repoURL, "/")
	repoName := parts[len(parts)-1]
	return strings.TrimSuffix(repoName, ".git")
}

func TaskReadyForExecution(task *models.Task) bool {
	switch task.SpecStatus {
	case models.TaskSpecStatusApproved, models.TaskSpecStatusAutoApproved:
		return true
	default:
		return false
	}
}

func MatchAffectedFile(pattern, file string) bool {
	pattern = strings.TrimSpace(pattern)
	file = strings.TrimSpace(file)
	if pattern == "" || file == "" {
		return false
	}

	cleanPattern := filepath.ToSlash(filepath.Clean(pattern))
	cleanFile := filepath.ToSlash(filepath.Clean(file))

	if cleanPattern == cleanFile {
		return true
	}

	if strings.HasPrefix(cleanFile, cleanPattern+"/") {
		return true
	}

	if strings.HasSuffix(cleanFile, "/"+cleanPattern) {
		return true
	}

	if !strings.Contains(cleanPattern, "/") && filepath.Base(cleanFile) == cleanPattern {
		return true
	}

	if strings.ContainsAny(pattern, "*?[]") {
		if matched, err := filepath.Match(cleanPattern, cleanFile); err == nil && matched {
			return true
		}
	}

	return false
}

func IsUnderAffectedDir(file string, affectedFiles []string) bool {
	file = strings.TrimSpace(file)
	if file == "" {
		return false
	}

	fileDir := filepath.ToSlash(filepath.Clean(filepath.Dir(file)))
	for _, pattern := range affectedFiles {
		pattern = strings.TrimSpace(pattern)
		if pattern == "" {
			continue
		}

		candidateDir := affectedPatternDir(pattern)
		if candidateDir == "" {
			continue
		}
		if fileDir == candidateDir || strings.HasPrefix(fileDir, candidateDir+"/") {
			return true
		}
	}

	return false
}

func affectedPatternDir(pattern string) string {
	pattern = strings.TrimSpace(pattern)
	if pattern == "" {
		return ""
	}

	normalized := filepath.ToSlash(pattern)
	if strings.HasSuffix(normalized, "/") {
		dir := strings.TrimSuffix(filepath.ToSlash(filepath.Clean(normalized)), "/")
		if dir == "" {
			return "."
		}
		return dir
	}

	if strings.ContainsAny(normalized, "*?[]") {
		idx := strings.IndexAny(normalized, "*?[")
		if idx == -1 {
			idx = len(normalized)
		}
		base := strings.TrimSuffix(normalized[:idx], "/")
		if base == "" {
			return "."
		}
		return filepath.ToSlash(filepath.Clean(base))
	}

	dir := filepath.ToSlash(filepath.Clean(filepath.Dir(normalized)))
	if dir == "" {
		return "."
	}
	return dir
}

func CleanPatchPaths(patchText string) string {
	re := regexp.MustCompile(`([ab])/` + regexp.QuoteMeta(workspace.ReposDirName) + `/[^/]+/(?:worktrees/[^/]+|[^/]+)/`)
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

			codeReposPrefix := workspace.ReposPrefix()
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
			reposPrefix := workspace.ReposPrefix()
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

	re := regexp.MustCompile(`^a/` + regexp.QuoteMeta(workspace.ReposPrefix()) + `([^/]+)/(?:worktrees/[^/]+|[^/]+)/`)
	repoBlocks := make(map[string][]string)
	for i := 1; i < len(parts); i++ {
		block := parts[i]
		if matches := re.FindStringSubmatch(block); len(matches) > 1 {
			repoName := matches[1]
			cleanBlock := regexp.MustCompile(`([ab])/`+regexp.QuoteMeta(workspace.ReposPrefix())+regexp.QuoteMeta(repoName)+`/(?:worktrees/[^/]+|[^/]+)/`).ReplaceAllString(block, "$1/")
			repoBlocks[repoName] = append(repoBlocks[repoName], "diff --git "+cleanBlock)
		} else if strings.HasPrefix(block, "a/") {
			sub := block[2:]
			sepIdx := strings.Index(sub, " b/")
			if sepIdx != -1 {
				firstPath := sub[:sepIdx]
				idx := strings.Index(firstPath, "/")
				if idx != -1 {
					repoName := firstPath[:idx]
					if repoName != workspace.ReposDirName {
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
					if repoName != workspace.ReposDirName {
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
