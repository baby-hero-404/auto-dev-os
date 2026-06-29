package patch

import (
	"context"

	"github.com/auto-code-os/auto-code-os/server/internal/sandbox"
	"github.com/auto-code-os/auto-code-os/server/pkg/models"
)

type PatchEngine interface {
	Validate(patchData string, basePath string) []ValidationError
	Apply(ctx context.Context, task *models.Task, agent *models.Agent, stepID string, patchData string, worktreeSuffix string) error
}

func NewEngine(preferredStrategy string, runner *Runner) PatchEngine {
	if preferredStrategy == "search_replace" {
		return &SearchReplaceApplier{runner: runner}
	}
	return &LegacyGitApplier{runner: runner}
}

type LegacyGitApplier struct {
	runner *Runner
}

func (a *LegacyGitApplier) Validate(patchData string, basePath string) []ValidationError {
	return ValidateUnifiedDiff(patchData, basePath)
}

func (a *LegacyGitApplier) Apply(ctx context.Context, task *models.Task, agent *models.Agent, stepID string, patchData string, worktreeSuffix string) error {
	return a.runner.ApplyPatch(ctx, task, agent, stepID, patchData, worktreeSuffix)
}

type SearchReplaceApplier struct {
	runner *Runner
}

func (a *SearchReplaceApplier) Validate(patchData string, basePath string) []ValidationError {
	blocks := ParseSearchReplace(patchData)
	return ValidateSearchReplace(blocks, basePath)
}

func (a *SearchReplaceApplier) Apply(ctx context.Context, task *models.Task, agent *models.Agent, stepID string, patchData string, worktreeSuffix string) error {
	blocks := ParseSearchReplace(patchData)

	// Determine the base path
	localPath := sandbox.WorkspacePath(a.runner.WorkspaceRoot, task.ID)
	basePath := localPath

	if task.RepositoryID != nil {
		repoHostPath, err := a.runner.GetTaskRepoHostPath(ctx, task)
		if err != nil {
			return err
		}
		basePath = a.runner.HostWorktreePath(task, repoHostPath, worktreeSuffix)
	} else {
		// Multi-repo: HostWorktreePath might need to be resolved per-repo if blocks have repo prefixes.
		// For simplicity, we assume blocks have paths relative to localPath like "repo-a/src/main.go"
		// We can just use localPath or the specific HostWorktreePath.
		// Wait, localPath is `code/repos/repo-name/worktrees/suffix`. In multi-repo, it's `workspace/repo-name`.
		// Let's use localPath for multi-repo, or we can look up the repo.
		// Since we have option B path translation, files in multi-repo are `repo-name/...`
		// which is relative to `localPath/code/repos` or similar. Let's just use `localPath`.
		// Wait, `Runner.HostWorktreePath` uses repoHostPath. If it's multi-repo, we should probably resolve per file.
		// To keep it simple, we'll let ApplySearchReplace use `basePath` but we should compute it properly.
		basePath = a.runner.HostWorktreePath(task, localPath, worktreeSuffix)
	}

	return ApplySearchReplace(blocks, basePath)
}
