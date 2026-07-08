package patch

import (
	"context"
	"fmt"
	"strings"

	"github.com/auto-code-os/auto-code-os/server/internal/sandbox"
	"github.com/auto-code-os/auto-code-os/server/pkg/models"
	"github.com/auto-code-os/auto-code-os/server/pkg/paths"
)

type PatchEngine interface {
	Validate(ctx context.Context, patchData string, basePath string) []ValidationError
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

func (a *LegacyGitApplier) Validate(ctx context.Context, patchData string, basePath string) []ValidationError {
	var validationErrs []ValidationError

	// Check AgentPathContext security boundaries if present
	var pathCtx *paths.AgentPathContext
	if actx, ok := ctx.Value(paths.AgentPathContextKey).(*paths.AgentPathContext); ok {
		pathCtx = actx
	}
	if pathCtx != nil {
		lines := strings.Split(patchData, "\n")
		for _, line := range lines {
			var file string
			if strings.HasPrefix(line, "--- ") {
				file = strings.TrimPrefix(line, "--- ")
				file = strings.TrimSpace(file)
				file = strings.TrimPrefix(file, "a/")
			} else if strings.HasPrefix(line, "+++ ") {
				file = strings.TrimPrefix(line, "+++ ")
				file = strings.TrimSpace(file)
				file = strings.TrimPrefix(file, "b/")
			}
			if file != "" && file != "/dev/null" {
				if _, err := pathCtx.ToPhysical(file); err != nil {
					validationErrs = append(validationErrs, ValidationError{
						Filepath: file,
						Reason:   fmt.Sprintf("security boundary violation: %v", err),
						IsFatal:  true,
					})
				}
			}
		}
	}

	for _, vErr := range ValidateUnifiedDiff(patchData, basePath) {
		validationErrs = append(validationErrs, vErr)
	}
	return validationErrs
}

func (a *LegacyGitApplier) Apply(ctx context.Context, task *models.Task, agent *models.Agent, stepID string, patchData string, worktreeSuffix string) error {
	return a.runner.ApplyPatch(ctx, task, agent, stepID, patchData, worktreeSuffix)
}

type SearchReplaceApplier struct {
	runner *Runner
}

func (a *SearchReplaceApplier) Validate(ctx context.Context, patchData string, basePath string) []ValidationError {
	blocks := ParseSearchReplace(patchData)
	var validationErrs []ValidationError

	// Check AgentPathContext security boundaries if present
	var pathCtx *paths.AgentPathContext
	if actx, ok := ctx.Value(paths.AgentPathContextKey).(*paths.AgentPathContext); ok {
		pathCtx = actx
	}
	if pathCtx != nil {
		for _, b := range blocks {
			if b.Filepath != "" {
				if _, err := pathCtx.ToPhysical(b.Filepath); err != nil {
					validationErrs = append(validationErrs, ValidationError{
						Filepath: b.Filepath,
						Reason:   fmt.Sprintf("security boundary violation: %v", err),
						IsFatal:  true,
					})
				}
			}
		}
	}

	for _, vErr := range ValidateSearchReplace(blocks, basePath) {
		validationErrs = append(validationErrs, vErr)
	}
	return validationErrs
}

func (a *SearchReplaceApplier) Apply(ctx context.Context, task *models.Task, agent *models.Agent, stepID string, patchData string, worktreeSuffix string) error {
	blocks := ParseSearchReplace(patchData)

	var pathCtx *paths.AgentPathContext
	if actx, ok := ctx.Value(paths.AgentPathContextKey).(*paths.AgentPathContext); ok {
		pathCtx = actx
	}

	if pathCtx != nil {
		for i, b := range blocks {
			if b.Filepath == "" {
				return fmt.Errorf("missing filepath in edit block")
			}
			phys, err := pathCtx.ToPhysical(b.Filepath)
			if err != nil {
				return &PolicyViolationError{
					Severity:   SeverityCritical,
					ErrorMsg:   fmt.Sprintf("security boundary violation: %v", err),
					Reason:     "unauthorized_path",
					Violations: []string{b.Filepath},
				}
			}
			blocks[i].Filepath = phys
		}
		return ApplySearchReplace(blocks, "")
	}

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
		// Multi-repo: blocks use repo-relative paths (e.g. "repo-a/src/main.go").
		// Resolve against the task workspace root which contains all repo checkouts.
		basePath = a.runner.HostWorktreePath(task, localPath, worktreeSuffix)
	}

	return ApplySearchReplace(blocks, basePath)
}
