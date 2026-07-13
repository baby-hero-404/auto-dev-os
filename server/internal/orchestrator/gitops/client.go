package gitops

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/auto-code-os/auto-code-os/server/pkg/models"
	"github.com/auto-code-os/auto-code-os/server/pkg/paths"
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
	cmd := fmt.Sprintf("git -C %s checkout %s", paths.QuoteShellArg(containerPath), paths.QuoteShellArg(branch))
	_, err := c.RunSandboxStep(ctx, task, agent, "git_checkout_"+branch, cmd)
	return err
}

func (c *DefaultSandboxGitClient) CheckoutNewBranch(ctx context.Context, task *models.Task, agent *models.Agent, containerPath, branch string) error {
	cmd := fmt.Sprintf("git -C %s checkout -B %s", paths.QuoteShellArg(containerPath), paths.QuoteShellArg(branch))
	_, err := c.RunSandboxStep(ctx, task, agent, "git_checkout_new_"+branch, cmd)
	return err
}

func (c *DefaultSandboxGitClient) HasBranch(ctx context.Context, task *models.Task, agent *models.Agent, containerPath, branch string) bool {
	cmd := fmt.Sprintf("git -C %s show-ref --verify --quiet refs/heads/%s", paths.QuoteShellArg(containerPath), paths.QuoteShellArg(branch))
	_, err := c.RunSandboxStep(ctx, task, agent, "git_check_branch_"+branch, cmd)
	return err == nil
}

func (c *DefaultSandboxGitClient) MergeBranch(ctx context.Context, task *models.Task, agent *models.Agent, containerPath, branch string) (string, error) {
	cmd := fmt.Sprintf("git -C %s merge --no-commit %s", paths.QuoteShellArg(containerPath), paths.QuoteShellArg(branch))
	_, err := c.RunSandboxStep(ctx, task, agent, "git_merge_"+branch, cmd)
	if err != nil {
		conflictCheckCmd := fmt.Sprintf("git -C %s diff --name-only --diff-filter=U", paths.QuoteShellArg(containerPath))
		conflictCheck, errCC := c.RunSandboxStep(ctx, task, agent, "git_conflict_check_"+branch, conflictCheckCmd)
		if errCC != nil {
			c.Log(ctx, task.ID, nil, "warn", fmt.Sprintf("failed to run conflict check: %v", errCC))
		}
		conflictOut, _ := conflictCheck["stdout"].(string)
		if strings.TrimSpace(conflictOut) != "" {
			abortCmd := fmt.Sprintf("git -C %s merge --abort", paths.QuoteShellArg(containerPath))
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
	cmdName := fmt.Sprintf("git -C %s config user.name %s", paths.QuoteShellArg(containerPath), paths.QuoteShellArg(agent.Name))
	if _, err := c.RunSandboxStep(ctx, task, agent, "git_config_name", cmdName); err != nil {
		c.Log(ctx, task.ID, nil, "warn", fmt.Sprintf("failed to configure git user name: %v", err))
	}

	cmdEmail := fmt.Sprintf("git -C %s config user.email %s", paths.QuoteShellArg(containerPath), paths.QuoteShellArg(fmt.Sprintf("%s@autocodeos.com", agent.Name)))
	if _, err := c.RunSandboxStep(ctx, task, agent, "git_config_email", cmdEmail); err != nil {
		c.Log(ctx, task.ID, nil, "warn", fmt.Sprintf("failed to configure git user email: %v", err))
	}

	cmdStatus := fmt.Sprintf("git -C %s diff --cached --quiet", paths.QuoteShellArg(containerPath))
	_, err := c.RunSandboxStep(ctx, task, agent, "git_diff_cached", cmdStatus)
	if err != nil {
		cmdCommit := fmt.Sprintf("git -C %s commit -m %s", paths.QuoteShellArg(containerPath), paths.QuoteShellArg(message))
		_, errCommit := c.RunSandboxStep(ctx, task, agent, "git_commit", cmdCommit)
		return errCommit
	}
	return nil
}

func getDiffPrefixes(containerPath string) string {
	rel, err := filepath.Rel("/workspace", containerPath)
	if err != nil || rel == "." || rel == ".." || strings.HasPrefix(rel, "../") {
		return ""
	}
	rel = filepath.ToSlash(rel)
	return fmt.Sprintf(" --src-prefix=a/%s/ --dst-prefix=b/%s/", rel, rel)
}

func (c *DefaultSandboxGitClient) GetDiff(ctx context.Context, task *models.Task, agent *models.Agent, containerPath string) (string, error) {
	prefixes := getDiffPrefixes(containerPath)
	trackCmd := fmt.Sprintf("git -C %s add -N .", paths.QuoteShellArg(containerPath))
	_, _ = c.RunSandboxStep(ctx, task, agent, "git_track_untracked", trackCmd)

	cmd := fmt.Sprintf("git -C %s diff%s", paths.QuoteShellArg(containerPath), prefixes)
	out, err := c.RunSandboxStep(ctx, task, agent, "git_diff", cmd)
	if out != nil {
		if stdout, ok := out["stdout"].(string); ok {
			return stdout, err
		}
	}
	return "", err
}

func (c *DefaultSandboxGitClient) GetPRDiff(ctx context.Context, task *models.Task, agent *models.Agent, containerPath, baseBranch string) (string, error) {
	prefixes := getDiffPrefixes(containerPath)
	diffCmd := fmt.Sprintf("cd %[1]s && (git diff %[3]s %[2]s...HEAD 2>/dev/null || git diff %[3]s master...HEAD 2>/dev/null || git diff %[3]s HEAD~1 2>/dev/null || git diff %[3]s)",
		paths.QuoteShellArg(containerPath),
		paths.QuoteShellArg(baseBranch),
		prefixes,
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
	cmd := fmt.Sprintf("cd %s && git status --porcelain", paths.QuoteShellArg(containerPath))
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
	stepName := "git_diff_multi"
	if worktreeSuffix != "" {
		stepName += "_" + worktreeSuffix
	}
	pyScript := `import json, os, subprocess, sys
container_path = sys.argv[1]
suffix = sys.argv[2]
clean_suffix = suffix.lstrip("-").replace("-worktree", "") if suffix else ""
if clean_suffix in ("be", "backend"):
    role = "backend"
elif clean_suffix in ("fe", "frontend"):
    role = "frontend"
elif clean_suffix == "fix":
    role = "fix"
else:
    role = clean_suffix
meta_path = os.path.join(container_path, "metadata.json")
if not os.path.exists(meta_path):
    sys.exit(0)
with open(meta_path) as f:
    meta = json.load(f)
diff_out = []
for repo in meta.get("repos", []):
    name = repo.get("name")
    paths = repo.get("paths", {})
    rel_path = ""
    if role:
        rel_path = paths.get("worktrees", {}).get(role, "")
    if not rel_path:
        rel_path = paths.get("main", "")
    if not rel_path:
        continue
    full_path = os.path.join(container_path, rel_path)
    if os.path.exists(os.path.join(full_path, ".git")):
        subprocess.run(["git", "-C", full_path, "add", "-N", "."], capture_output=True)
        res = subprocess.run(["git", "-C", full_path, "diff", f"--src-prefix=a/{rel_path}/", f"--dst-prefix=b/{rel_path}/"], capture_output=True, text=True)
        if res.returncode == 0 and res.stdout.strip():
            diff_out.append(f"--- Repository: {name}\n{res.stdout}")
print("\n".join(diff_out))`

	cmd := fmt.Sprintf("python3 -c %[1]s %[2]s %[3]s",
		paths.QuoteShellArg(pyScript),
		paths.QuoteShellArg(containerPath),
		paths.QuoteShellArg(worktreeSuffix),
	)

	out, err := c.RunSandboxStep(ctx, task, agent, stepName, cmd)
	if err != nil {
		return "", fmt.Errorf("multi-repo git diff failed: %w", err)
	}
	diffText, _ := out["stdout"].(string)
	return diffText, nil
}

func (c *DefaultSandboxGitClient) GetWorkspaceChangedFiles(ctx context.Context, task *models.Task, agent *models.Agent, containerPath, worktreeSuffix string) ([]string, error) {
	stepName := "git_status_multi"
	if worktreeSuffix != "" {
		stepName += "_" + worktreeSuffix
	}
	pyScript := `import json, os, subprocess, sys
container_path = sys.argv[1]
suffix = sys.argv[2]
clean_suffix = suffix.lstrip("-").replace("-worktree", "") if suffix else ""
if clean_suffix in ("be", "backend"):
    role = "backend"
elif clean_suffix in ("fe", "frontend"):
    role = "frontend"
elif clean_suffix == "fix":
    role = "fix"
else:
    role = clean_suffix
meta_path = os.path.join(container_path, "metadata.json")
if not os.path.exists(meta_path):
    sys.exit(0)
with open(meta_path) as f:
    meta = json.load(f)
files_out = []
for repo in meta.get("repos", []):
    name = repo.get("name")
    paths = repo.get("paths", {})
    rel_path = ""
    if role:
        rel_path = paths.get("worktrees", {}).get(role, "")
    if not rel_path:
        rel_path = paths.get("main", "")
    if not rel_path:
        continue
    full_path = os.path.join(container_path, rel_path)
    if os.path.exists(os.path.join(full_path, ".git")):
        res = subprocess.run(["git", "-C", full_path, "status", "--porcelain"], capture_output=True, text=True)
        if res.returncode == 0:
            for line in res.stdout.splitlines():
                if len(line) > 2:
                    files_out.append(f"{rel_path}/{line[3:].strip()}")
print("\n".join(files_out))`

	cmd := fmt.Sprintf("python3 -c %[1]s %[2]s %[3]s",
		paths.QuoteShellArg(pyScript),
		paths.QuoteShellArg(containerPath),
		paths.QuoteShellArg(worktreeSuffix),
	)

	out, err := c.RunSandboxStep(ctx, task, agent, stepName, cmd)
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
