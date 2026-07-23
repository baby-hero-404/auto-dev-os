package patch

import (
	"context"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/auto-code-os/auto-code-os/server/pkg/models"
	"github.com/auto-code-os/auto-code-os/server/pkg/paths"
)

func (r *Runner) appendNewAffectedFiles(ctx context.Context, task *models.Task, files map[string]bool) error {
	if len(files) == 0 || r.UpdateTaskAnalysis == nil {
		return nil
	}

	// Merge into the freshest analysis via the safe read-modify-write
	// callback rather than the possibly-stale in-memory task.Analysis, so
	// concurrent step updates (e.g. the other agent's affected-files write)
	// are never clobbered.
	return r.UpdateTaskAnalysis(ctx, task, func(analysis *models.TaskAnalysis) bool {
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
		return changed
	})
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
				return repo.Name, stripWorktreeAndBranchPrefix(rem, repo, role)
			}
		}
	}

	// 3. Fallback: if we only have 1 repository in the workspace, it must belong to it!
	if ws != nil && len(ws.Repos) == 1 {
		repo := ws.Repos[0]
		rem := firstPath
		if rem2, ok2 := stripPrefix(rem, role); ok2 {
			rem = rem2
		}
		return repo.Name, stripWorktreeAndBranchPrefix(rem, repo, role)
	}

	// 4. Fallback: parse first component as repoName. Only trustworthy when the workspace has
	// no repo metadata at all (ws == nil) — with >1 known repos, a bare path (e.g. "main.go" or
	// "internal/x.go") carries no repo identity of its own; the caller must instead resolve
	// identity out-of-band (see repoRelPathForKnownRepo / the "--- Repository:" header path in
	// SplitPatchByRepoWithWorkspace) rather than have this guess a subdirectory as a repo name.
	if ws == nil || len(ws.Repos) == 0 {
		idx := strings.Index(firstPath, "/")
		if idx != -1 {
			return firstPath[:idx], firstPath[idx+1:]
		}
	}
	return "", firstPath
}

// mainBranchDirFor returns the on-disk directory name for repo's main-branch checkout: its
// explicit Paths.Main basename, else its DefaultBranch, else the "main" convention used by
// paths.OSWorkspacePaths.RepoMain.
func mainBranchDirFor(repo models.RepoWorkspace) string {
	if repo.Paths.Main != "" {
		return filepath.Base(repo.Paths.Main)
	}
	if repo.DefaultBranch != "" {
		return repo.DefaultBranch
	}
	return "main"
}

// stripWorktreeAndBranchPrefix strips a leading "worktrees/<role>/" or main-branch-directory
// segment from rem (a path already known to be relative to repo's root), once repo identity is
// established by the caller. Shared by both the path-inference branches of NormalizePatchPath
// and the header-based split in SplitPatchByRepoWithWorkspace (REQ-002 removed repo-name path
// prefixes from diff output, so per-file paths alone can no longer carry repo identity — callers
// that already know the repo via a "--- Repository:" marker still need this same stripping).
func stripWorktreeAndBranchPrefix(rem string, repo models.RepoWorkspace, role string) string {
	stripPrefix := func(p string, prefix string) (string, bool) {
		if p == prefix {
			return "", true
		}
		if strings.HasPrefix(p, prefix+"/") {
			return p[len(prefix)+1:], true
		}
		return "", false
	}

	if role != "" {
		if rem2, ok := stripPrefix(rem, "worktrees/"+role); ok {
			return rem2
		}
	}
	if strings.HasPrefix(rem, "worktrees/") {
		parts := strings.Split(rem, "/")
		if len(parts) >= 3 {
			return strings.Join(parts[2:], "/")
		}
	}

	mainBranchDir := mainBranchDirFor(repo)
	if rem2, ok := stripPrefix(rem, mainBranchDir); ok {
		return rem2
	}
	if mainBranchDir != "main" {
		if rem2, ok := stripPrefix(rem, "main"); ok {
			return rem2
		}
	}
	return rem
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

// repoHeaderRE matches the "--- Repository: <name>" marker line emitted by GetWorkspaceDiff /
// CapturePRDiff to delimit per-repo diff blocks in a concatenated multi-repo patch. This is the
// primary repo-attribution signal for multi-repo workspaces: since REQ-002 removed repo-name
// path prefixes from diff output, per-file paths alone can no longer disambiguate which repo a
// hunk belongs to once a workspace has more than one repository.
var repoHeaderRE = regexp.MustCompile(`(?m)^--- Repository: (.+)$`)

// splitByRepositoryHeader splits patchText on "--- Repository: <name>" marker lines, if present,
// returning the raw (uncleaned) segment following each marker. ok is false when no marker is
// found, so the caller falls back to path-based inference (single-repo diffs from GetDiff/
// GetPRDiff never carry this marker).
func splitByRepositoryHeader(patchText string) (map[string]string, bool) {
	locs := repoHeaderRE.FindAllStringSubmatchIndex(patchText, -1)
	if len(locs) == 0 {
		return nil, false
	}
	segments := make(map[string]string, len(locs))
	for i, loc := range locs {
		name := patchText[loc[2]:loc[3]]
		segStart := loc[1]
		segEnd := len(patchText)
		if i+1 < len(locs) {
			segEnd = locs[i+1][0]
		}
		segment := strings.TrimPrefix(patchText[segStart:segEnd], "\n")
		if existing, ok := segments[name]; ok {
			segments[name] = existing + segment
		} else {
			segments[name] = segment
		}
	}
	return segments, true
}

// repoRelPathForKnownRepo strips worktree/branch path prefixes from firstPath for a repo
// identity that's already known (from a "--- Repository:" header), rather than inferring it
// from the path itself the way NormalizePatchPath's step 2/3 do. Mirrors the same stripping
// rules via the shared stripWorktreeAndBranchPrefix helper.
func repoRelPathForKnownRepo(firstPath string, repoName string, ws *models.TaskWorkspace, role string) string {
	firstPath = strings.ReplaceAll(firstPath, "\\", "/")
	firstPath = filepath.ToSlash(filepath.Clean(firstPath))
	firstPath = strings.TrimPrefix(firstPath, "/")

	if strings.HasPrefix(firstPath, "a/") || strings.HasPrefix(firstPath, "b/") {
		firstPath = firstPath[2:]
	}
	if rem, ok := strings.CutPrefix(firstPath, paths.ReposDirName+"/"); ok {
		firstPath = rem
	}
	if rem, ok := strings.CutPrefix(firstPath, repoName+"/"); ok {
		firstPath = rem
	}

	var repo models.RepoWorkspace
	found := false
	if ws != nil {
		for _, rw := range ws.Repos {
			if rw.Name == repoName {
				repo, found = rw, true
				break
			}
		}
	}
	if !found {
		repo = models.RepoWorkspace{Name: repoName}
	}
	return stripWorktreeAndBranchPrefix(firstPath, repo, role)
}

// cleanKnownRepoPatch cleans per-file path prefixes in a diff segment that's already known to
// belong to repoName (via a "--- Repository:" header), reusing CleanPatchBlock's regex-based
// prefix stripping per "diff --git" block without needing to (mis)infer repo identity from the
// path itself — the exact fallback that broke on multi-repo workspaces (see path_normalizer
// tests / task 8291a25e report).
func (r *Runner) cleanKnownRepoPatch(segment string, repoName string, ws *models.TaskWorkspace, role string) string {
	parts := strings.Split(segment, "diff --git ")
	if len(parts) <= 1 {
		return segment
	}
	header := parts[0]
	var blocks []string
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
			if sepIdx := strings.Index(sub, " b/"); sepIdx != -1 {
				firstPath = sub[:sepIdx]
			}
		}

		if firstPath != "" {
			repoRelPath := repoRelPathForKnownRepo(firstPath, repoName, ws, role)
			blocks = append(blocks, "diff --git "+r.CleanPatchBlock(block, repoName, repoRelPath, firstPath))
		} else {
			blocks = append(blocks, "diff --git "+block)
		}
	}
	return header + strings.Join(blocks, "")
}

func (r *Runner) SplitPatchByRepoWithWorkspace(patchText string, ws *models.TaskWorkspace, role string) map[string]string {
	if segments, ok := splitByRepositoryHeader(patchText); ok {
		repos := make(map[string]string, len(segments))
		for repoName, segment := range segments {
			repos[repoName] = r.cleanKnownRepoPatch(strings.TrimSpace(segment), repoName, ws, role)
		}
		return repos
	}

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
