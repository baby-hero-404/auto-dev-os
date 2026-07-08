package patch

import (
	"context"
	"encoding/json"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/auto-code-os/auto-code-os/server/pkg/models"
	"github.com/auto-code-os/auto-code-os/server/pkg/paths"
)

func (r *Runner) appendNewAffectedFiles(ctx context.Context, task *models.Task, files map[string]bool) error {
	if len(files) == 0 {
		return nil
	}

	var analysis models.TaskAnalysis
	if len(task.Analysis) > 0 {
		if err := json.Unmarshal(task.Analysis, &analysis); err != nil {
			return err
		}
	}

	changed := false
	existing := make(map[string]bool, len(analysis.AffectedFiles))
	for _, file := range analysis.AffectedFiles {
		existing[file.File] = true
	}
	for file := range files {
		if !existing[file] {
			analysis.AffectedFiles = append(analysis.AffectedFiles, models.AffectedFile{File: file})
			existing[file] = true
			changed = true
		}
	}
	if !changed {
		return nil
	}

	raw, err := json.Marshal(analysis)
	if err != nil {
		return err
	}
	task.Analysis = raw
	if r.UpdateTaskAnalysis != nil {
		if err := r.UpdateTaskAnalysis(ctx, task.ID, raw); err != nil {
			return err
		}
	}
	return nil
}

func IsSafeNewFilePath(path string) bool {
	if filepath.IsAbs(path) || strings.HasPrefix(path, "/") {
		return false
	}
	cleaned := filepath.Clean(path)
	if strings.HasPrefix(cleaned, "..") || strings.Contains(cleaned, "../") || strings.Contains(cleaned, "..\\") {
		return false
	}
	parts := strings.Split(cleaned, string(filepath.Separator))
	for _, p := range parts {
		if p == ".git" {
			return false
		}
	}
	return true
}

func (r *Runner) NormalizePatchPath(firstPath string, ws *models.TaskWorkspace, role string) (repoName string, repoRelPath string) {
	if firstPath == "" {
		return "", ""
	}
	// Clean and normalize separators
	firstPath = strings.ReplaceAll(firstPath, "\\", "/")
	firstPath = filepath.ToSlash(filepath.Clean(firstPath))
	firstPath = strings.TrimPrefix(firstPath, "/")

	// Strip git diff a/ or b/ prefixes if present
	if strings.HasPrefix(firstPath, "a/") || strings.HasPrefix(firstPath, "b/") {
		firstPath = firstPath[2:]
	}

	// Helper to check and strip prefix
	stripPrefix := func(p string, prefix string) (string, bool) {
		if p == prefix {
			return "", true
		}
		if strings.HasPrefix(p, prefix+"/") {
			return p[len(prefix)+1:], true
		}
		return "", false
	}

	// 1. Check if it starts with code/repos/ prefix
	if rem, ok := stripPrefix(firstPath, paths.ReposDirName); ok {
		firstPath = rem
	}

	// 2. Try to match against workspace repository names
	if ws != nil {
		for _, repo := range ws.Repos {
			if rem, ok := stripPrefix(firstPath, repo.Name); ok {
				// Strip worktrees/<role> or worktrees/<any_role> or main branch dir if present
				if rem2, ok2 := stripPrefix(rem, "worktrees/"+role); ok2 {
					return repo.Name, rem2
				}
				if strings.HasPrefix(rem, "worktrees/") {
					parts := strings.Split(rem, "/")
					if len(parts) >= 3 {
						return repo.Name, strings.Join(parts[2:], "/")
					}
				}
				
				mainBranchDir := "main"
				if repo.Paths.Main != "" {
					mainBranchDir = filepath.Base(repo.Paths.Main)
				} else if repo.DefaultBranch != "" {
					mainBranchDir = repo.DefaultBranch
				}

				if rem2, ok2 := stripPrefix(rem, mainBranchDir); ok2 {
					return repo.Name, rem2
				}
				if mainBranchDir != "main" {
					if rem2, ok2 := stripPrefix(rem, "main"); ok2 {
						return repo.Name, rem2
					}
				}
				return repo.Name, rem
			}
		}
	}

	// 3. Fallback: if we only have 1 repository in the workspace, it must belong to it!
	if ws != nil && len(ws.Repos) == 1 {
		repo := ws.Repos[0]
		// Still check if the path starts with worktrees/role or role to clean it
		rem := firstPath
		if rem2, ok2 := stripPrefix(rem, "worktrees/"+role); ok2 {
			return repo.Name, rem2
		}
		if strings.HasPrefix(rem, "worktrees/") {
			parts := strings.Split(rem, "/")
			if len(parts) >= 3 {
				return repo.Name, strings.Join(parts[2:], "/")
			}
		}
		if rem2, ok2 := stripPrefix(rem, role); ok2 {
			return repo.Name, rem2
		}

		mainBranchDir := "main"
		if repo.Paths.Main != "" {
			mainBranchDir = filepath.Base(repo.Paths.Main)
		} else if repo.DefaultBranch != "" {
			mainBranchDir = repo.DefaultBranch
		}
		if rem2, ok2 := stripPrefix(rem, mainBranchDir); ok2 {
			return repo.Name, rem2
		}
		if mainBranchDir != "main" {
			if rem2, ok2 := stripPrefix(rem, "main"); ok2 {
				return repo.Name, rem2
			}
		}

		return repo.Name, rem
	}

	// 4. Fallback: parse first component as repoName
	idx := strings.Index(firstPath, "/")
	if idx != -1 {
		return firstPath[:idx], firstPath[idx+1:]
	}
	return "", firstPath
}

func (r *Runner) CleanPatchBlock(block string, repoName string, repoRelPath string, rawFirstPath string) string {
	if !strings.HasSuffix(rawFirstPath, repoRelPath) {
		return CleanRepoPrefix(block, repoName)
	}
	prefixToStrip := rawFirstPath[:len(rawFirstPath)-len(repoRelPath)]
	if prefixToStrip == "" {
		return block
	}

	escapedPrefix := regexp.QuoteMeta(prefixToStrip)
	block = regexp.MustCompile(`^(a/)`+escapedPrefix).ReplaceAllString(block, "${1}")
	block = regexp.MustCompile(`( b/)`+escapedPrefix).ReplaceAllString(block, "${1}")
	block = regexp.MustCompile(`(?m)^(--- a/)`+escapedPrefix).ReplaceAllString(block, "${1}")
	block = regexp.MustCompile(`(?m)^(\+\+\+ b/)`+escapedPrefix).ReplaceAllString(block, "${1}")
	block = regexp.MustCompile(`(?m)^(rename from )`+escapedPrefix).ReplaceAllString(block, "${1}")
	block = regexp.MustCompile(`(?m)^(rename to )`+escapedPrefix).ReplaceAllString(block, "${1}")
	block = regexp.MustCompile(`(?m)^(copy from )`+escapedPrefix).ReplaceAllString(block, "${1}")
	block = regexp.MustCompile(`(?m)^(copy to )`+escapedPrefix).ReplaceAllString(block, "${1}")
	return block
}

func (r *Runner) SplitPatchByRepoWithWorkspace(patchText string, ws *models.TaskWorkspace, role string) map[string]string {
	repos := make(map[string]string)
	parts := strings.Split(patchText, "diff --git ")
	if len(parts) <= 1 || (len(parts) == 2 && parts[0] == "" && !strings.Contains(patchText, "diff --git ")) {
		trimmed := strings.TrimSpace(patchText)
		if trimmed == "" {
			return repos
		}
		lines := strings.Split(trimmed, "\n")
		var firstPath string
		for _, line := range lines {
			if strings.HasPrefix(line, "--- a/") {
				firstPath = line[len("--- a/"):]
				break
			} else if strings.HasPrefix(line, "+++ b/") {
				firstPath = line[len("+++ b/"):]
				break
			}
		}
		if firstPath != "" {
			repoName, repoRelPath := r.NormalizePatchPath(firstPath, ws, role)
			if repoName != "" {
				cleanPatch := r.CleanPatchBlock(trimmed, repoName, repoRelPath, firstPath)
				repos[repoName] = cleanPatch
			} else {
				repos[""] = trimmed
			}
		} else {
			repos[""] = trimmed
		}
		return repos
	}

	header := parts[0]
	repoBlocks := make(map[string][]string)
	for i := 1; i < len(parts); i++ {
		block := parts[i]
		lineEnd := strings.Index(block, "\n")
		if lineEnd == -1 {
			lineEnd = len(block)
		}
		headerLine := block[:lineEnd]

		var firstPath string
		if strings.HasPrefix(headerLine, "a/") {
			sub := headerLine[2:]
			sepIdx := strings.Index(sub, " b/")
			if sepIdx != -1 {
				firstPath = sub[:sepIdx]
			}
		}

		if firstPath != "" {
			repoName, repoRelPath := r.NormalizePatchPath(firstPath, ws, role)
			if repoName != "" {
				cleanBlock := r.CleanPatchBlock(block, repoName, repoRelPath, firstPath)
				repoBlocks[repoName] = append(repoBlocks[repoName], "diff --git "+cleanBlock)
			} else {
				repoBlocks[""] = append(repoBlocks[""], "diff --git "+block)
			}
		} else {
			repoBlocks[""] = append(repoBlocks[""], "diff --git "+block)
		}
	}

	for repoName, blocks := range repoBlocks {
		repos[repoName] = header + strings.Join(blocks, "")
	}
	return repos
}
