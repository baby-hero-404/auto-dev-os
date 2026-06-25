package gitops

import (
	"context"
	"fmt"
	"strings"

	"github.com/auto-code-os/auto-code-os/server/internal/orchestrator/workspace"
	"github.com/auto-code-os/auto-code-os/server/pkg/models"
)

type SandboxGitClient interface {
	CheckoutBranch(ctx context.Context, task *models.Task, agent *models.Agent, containerPath, branch string) error
	CheckoutNewBranch(ctx context.Context, task *models.Task, agent *models.Agent, containerPath, branch string) error
	HasBranch(ctx context.Context, task *models.Task, agent *models.Agent, containerPath, branch string) bool
	MergeBranch(ctx context.Context, task *models.Task, agent *models.Agent, containerPath, branch string) (string, error)
	CommitChanges(ctx context.Context, task *models.Task, agent *models.Agent, containerPath, message string) error
	GetDiff(ctx context.Context, task *models.Task, agent *models.Agent, containerPath string) (string, error)
	GetPRDiff(ctx context.Context, task *models.Task, agent *models.Agent, containerPath, baseBranch string) (string, error)
	GetChangedFiles(ctx context.Context, task *models.Task, agent *models.Agent, containerPath string) ([]string, error)
	GetWorkspaceDiff(ctx context.Context, task *models.Task, agent *models.Agent, containerPath, worktreeSuffix string) (string, error)
	GetWorkspaceChangedFiles(ctx context.Context, task *models.Task, agent *models.Agent, containerPath, worktreeSuffix string) ([]string, error)
}

type DefaultSandboxGitClient struct {
	RunSandboxStep func(ctx context.Context, task *models.Task, agent *models.Agent, stepID, command string) (map[string]any, error)
	Log            func(ctx context.Context, taskID string, jobID *string, level string, message string)
}

func NewSandboxGitClient(runStep func(context.Context, *models.Task, *models.Agent, string, string) (map[string]any, error), log func(context.Context, string, *string, string, string)) SandboxGitClient {
	return &DefaultSandboxGitClient{
		RunSandboxStep: runStep,
		Log:            log,
	}
}

func (c *DefaultSandboxGitClient) CheckoutBranch(ctx context.Context, task *models.Task, agent *models.Agent, containerPath, branch string) error {
	cmd := fmt.Sprintf("git -C %s checkout %s", workspace.QuoteShellArg(containerPath), workspace.QuoteShellArg(branch))
	_, err := c.RunSandboxStep(ctx, task, agent, "git_checkout_"+branch, cmd)
	return err
}

func (c *DefaultSandboxGitClient) CheckoutNewBranch(ctx context.Context, task *models.Task, agent *models.Agent, containerPath, branch string) error {
	cmd := fmt.Sprintf("git -C %s checkout -B %s", workspace.QuoteShellArg(containerPath), workspace.QuoteShellArg(branch))
	_, err := c.RunSandboxStep(ctx, task, agent, "git_checkout_new_"+branch, cmd)
	return err
}

func (c *DefaultSandboxGitClient) HasBranch(ctx context.Context, task *models.Task, agent *models.Agent, containerPath, branch string) bool {
	cmd := fmt.Sprintf("git -C %s show-ref --verify --quiet refs/heads/%s", workspace.QuoteShellArg(containerPath), workspace.QuoteShellArg(branch))
	_, err := c.RunSandboxStep(ctx, task, agent, "git_check_branch_"+branch, cmd)
	return err == nil
}

func (c *DefaultSandboxGitClient) MergeBranch(ctx context.Context, task *models.Task, agent *models.Agent, containerPath, branch string) (string, error) {
	cmd := fmt.Sprintf("git -C %s merge --no-commit %s", workspace.QuoteShellArg(containerPath), workspace.QuoteShellArg(branch))
	_, err := c.RunSandboxStep(ctx, task, agent, "git_merge_"+branch, cmd)
	if err != nil {
		conflictCheckCmd := fmt.Sprintf("git -C %s diff --name-only --diff-filter=U", workspace.QuoteShellArg(containerPath))
		conflictCheck, errCC := c.RunSandboxStep(ctx, task, agent, "git_conflict_check_"+branch, conflictCheckCmd)
		if errCC != nil {
			c.Log(ctx, task.ID, nil, "warn", fmt.Sprintf("failed to run conflict check: %v", errCC))
		}
		conflictOut, _ := conflictCheck["stdout"].(string)
		if strings.TrimSpace(conflictOut) != "" {
			abortCmd := fmt.Sprintf("git -C %s merge --abort", workspace.QuoteShellArg(containerPath))
			if _, errAbort := c.RunSandboxStep(ctx, task, agent, "git_merge_abort_"+branch, abortCmd); errAbort != nil {
				c.Log(ctx, task.ID, nil, "warn", fmt.Sprintf("failed to abort merge: %v", errAbort))
			}
			return models.MergeStatusConflict, fmt.Errorf("%s", strings.TrimSpace(conflictOut))
		}
		return models.MergeStatusFailed, fmt.Errorf("merge failed: %w", err)
	}
	return models.MergeStatusMerged, nil
}

func (c *DefaultSandboxGitClient) CommitChanges(ctx context.Context, task *models.Task, agent *models.Agent, containerPath, message string) error {
	cmdName := fmt.Sprintf("git -C %s config user.name %s", workspace.QuoteShellArg(containerPath), workspace.QuoteShellArg(agent.Name))
	if _, err := c.RunSandboxStep(ctx, task, agent, "git_config_name", cmdName); err != nil {
		c.Log(ctx, task.ID, nil, "warn", fmt.Sprintf("failed to configure git user name: %v", err))
	}

	cmdEmail := fmt.Sprintf("git -C %s config user.email %s", workspace.QuoteShellArg(containerPath), workspace.QuoteShellArg(fmt.Sprintf("%s@autocodeos.com", agent.Name)))
	if _, err := c.RunSandboxStep(ctx, task, agent, "git_config_email", cmdEmail); err != nil {
		c.Log(ctx, task.ID, nil, "warn", fmt.Sprintf("failed to configure git user email: %v", err))
	}

	cmdStatus := fmt.Sprintf("git -C %s diff --cached --quiet", workspace.QuoteShellArg(containerPath))
	_, err := c.RunSandboxStep(ctx, task, agent, "git_diff_cached", cmdStatus)
	if err != nil {
		cmdCommit := fmt.Sprintf("git -C %s commit -m %s", workspace.QuoteShellArg(containerPath), workspace.QuoteShellArg(message))
		_, errCommit := c.RunSandboxStep(ctx, task, agent, "git_commit", cmdCommit)
		return errCommit
	}
	return nil
}

func (c *DefaultSandboxGitClient) GetDiff(ctx context.Context, task *models.Task, agent *models.Agent, containerPath string) (string, error) {
	cmd := fmt.Sprintf("git -C %s diff", workspace.QuoteShellArg(containerPath))
	out, err := c.RunSandboxStep(ctx, task, agent, "git_diff", cmd)
	if out != nil {
		if stdout, ok := out["stdout"].(string); ok {
			return stdout, err
		}
	}
	return "", err
}

func (c *DefaultSandboxGitClient) GetPRDiff(ctx context.Context, task *models.Task, agent *models.Agent, containerPath, baseBranch string) (string, error) {
	diffCmd := fmt.Sprintf("cd %[1]s && (git diff %[2]s...HEAD 2>/dev/null || git diff master...HEAD 2>/dev/null || git diff HEAD~1 2>/dev/null || git diff)",
		workspace.QuoteShellArg(containerPath),
		workspace.QuoteShellArg(baseBranch),
	)
	out, err := c.RunSandboxStep(ctx, task, agent, "git_pr_diff", diffCmd)
	if out != nil {
		if stdout, ok := out["stdout"].(string); ok {
			return stdout, err
		}
	}
	return "", err
}

func (c *DefaultSandboxGitClient) GetChangedFiles(ctx context.Context, task *models.Task, agent *models.Agent, containerPath string) ([]string, error) {
	cmd := fmt.Sprintf("cd %s && git status --porcelain", workspace.QuoteShellArg(containerPath))
	out, err := c.RunSandboxStep(ctx, task, agent, "git_status", cmd)
	if err != nil {
		return nil, err
	}
	statusOut, _ := out["stdout"].(string)
	var repoChangedFiles []string
	for _, line := range strings.Split(statusOut, "\n") {
		line = strings.TrimSpace(line)
		if len(line) > 2 {
			repoChangedFiles = append(repoChangedFiles, strings.TrimSpace(line[2:]))
		}
	}
	return repoChangedFiles, nil
}

func (c *DefaultSandboxGitClient) GetWorkspaceDiff(ctx context.Context, task *models.Task, agent *models.Agent, containerPath, worktreeSuffix string) (string, error) {
	var pattern string
	if worktreeSuffix != "" {
		pattern = fmt.Sprintf("*%s", worktreeSuffix)
	} else {
		pattern = "*"
	}
	stepName := "git_diff_multi"
	if worktreeSuffix != "" {
		stepName += "_" + worktreeSuffix
	}
	out, err := c.RunSandboxStep(ctx, task, agent, stepName, fmt.Sprintf(`
		DIFF_OUT=""
		for d in %[1]s/%[2]s/ ; do
			if [ -e "$d/.git" ]; then
				pushd "$d" > /dev/null
				REPO_DIFF=$(git diff)
				if [ -n "$REPO_DIFF" ]; then
					d_name=$(basename "$d")
					repo_display="${d_name%%%[3]s}"
					DIFF_OUT="${DIFF_OUT}--- Repository: ${repo_display}\n${REPO_DIFF}\n\n"
				fi
				popd > /dev/null
			fi
		done
		echo -e "$DIFF_OUT"
	`, workspace.QuoteShellArg(containerPath), pattern, worktreeSuffix))
	if err != nil {
		return "", fmt.Errorf("multi-repo git diff failed: %w", err)
	}
	diffText, _ := out["stdout"].(string)
	return diffText, nil
}

func (c *DefaultSandboxGitClient) GetWorkspaceChangedFiles(ctx context.Context, task *models.Task, agent *models.Agent, containerPath, worktreeSuffix string) ([]string, error) {
	var pattern string
	if worktreeSuffix != "" {
		pattern = fmt.Sprintf("*%s", worktreeSuffix)
	} else {
		pattern = "*"
	}
	stepName := "git_status_multi"
	if worktreeSuffix != "" {
		stepName += "_" + worktreeSuffix
	}
	out, err := c.RunSandboxStep(ctx, task, agent, stepName, fmt.Sprintf(`
		FILES_OUT=""
		for d in %[1]s/%[2]s/ ; do
			if [ -e "$d/.git" ]; then
				pushd "$d" > /dev/null
				REPO_STATUS=$(git status --porcelain)
				if [ -n "$REPO_STATUS" ]; then
					d_name=$(basename "$d")
					repo_display="${d_name%%%[3]s}"
					echo "$REPO_STATUS" | while read -r line; do
						if [ ${#line} -gt 2 ]; then
							file_path="${line:3}"
							# Prefix with repo name if needed, or just return relative paths
							FILES_OUT="${FILES_OUT}repos/${repo_display}/${file_path}\n"
						fi
					done
				fi
				popd > /dev/null
			fi
		done
		echo -e "$FILES_OUT"
	`, workspace.QuoteShellArg(containerPath), pattern, worktreeSuffix))
	if err != nil {
		return nil, fmt.Errorf("multi-repo git status failed: %w", err)
	}
	statusOut, _ := out["stdout"].(string)
	var changedFiles []string
	for _, line := range strings.Split(statusOut, "\n") {
		line = strings.TrimSpace(line)
		if line != "" {
			changedFiles = append(changedFiles, line)
		}
	}
	return changedFiles, nil
}
