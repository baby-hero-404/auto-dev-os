package patch

import (
	"path/filepath"
	"regexp"
	"strings"

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

	cleanPattern := filepath.Clean(pattern)
	cleanFile := filepath.Clean(file)

	if cleanPattern == cleanFile {
		return true
	}

	if strings.HasPrefix(cleanFile, cleanPattern+string(filepath.Separator)) {
		return true
	}

	if strings.ContainsAny(pattern, "*?[]") {
		if matched, err := filepath.Match(cleanPattern, cleanFile); err == nil && matched {
			return true
		}
	}

	return false
}

func CleanPatchPaths(patchText string) string {
	re := regexp.MustCompile(`([ab])/code/repos/[^/]+/(?:main|worktrees/[^/]+)/`)
	return re.ReplaceAllString(patchText, "$1/")
}

func SplitPatchByRepo(patchText string) map[string]string {
	repos := make(map[string]string)
	parts := strings.Split(patchText, "diff --git ")
	if len(parts) == 0 {
		return repos
	}
	header := parts[0]

	re := regexp.MustCompile(`^a/code/repos/([^/]+)/(?:main|worktrees/[^/]+)/`)
	repoBlocks := make(map[string][]string)
	for i := 1; i < len(parts); i++ {
		block := parts[i]
		if matches := re.FindStringSubmatch(block); len(matches) > 1 {
			repoName := matches[1]
			cleanBlock := regexp.MustCompile(`([ab])/code/repos/`+regexp.QuoteMeta(repoName)+`/(?:main|worktrees/[^/]+)/`).ReplaceAllString(block, "$1/")
			repoBlocks[repoName] = append(repoBlocks[repoName], "diff --git "+cleanBlock)
		} else if strings.HasPrefix(block, "a/") {
			sub := block[2:]
			idx := strings.Index(sub, "/")
			if idx != -1 {
				repoName := sub[:idx]
				if repoName != "code" {
					repoBlocks[repoName] = append(repoBlocks[repoName], "diff --git "+block)
				}
			}
		}
	}

	for repoName, blocks := range repoBlocks {
		repos[repoName] = header + strings.Join(blocks, "")
	}
	return repos
}
