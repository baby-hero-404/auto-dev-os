package gitops

import (
	"context"
	"encoding/base64"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

func (p *GitHubProvider) CloneRepo(ctx context.Context, repoURL, token, branch, localPath string) (string, error) {
	var authArgs []string
	if token != "" {
		b64 := base64.StdEncoding.EncodeToString([]byte("x-access-token:" + token))
		authArgs = append(authArgs, "-c", "http.extraHeader=AUTHORIZATION: basic "+b64)
	}

	var cloneCmd *exec.Cmd
	if branch != "" {
		args := append(authArgs, "clone", "--branch", branch, "--single-branch", repoURL, localPath)
		cloneCmd = gitCommand(ctx, args...)
		if output, err := cloneCmd.CombinedOutput(); err != nil {
			outStr := string(output)
			if strings.Contains(outStr, "Remote branch") || strings.Contains(outStr, "Could not find remote branch") {
				// Retry with the default branch
				os.RemoveAll(localPath)
				fallbackArgs := append(authArgs, "clone", "--single-branch", repoURL, localPath)
				fallbackCmd := gitCommand(ctx, fallbackArgs...)
				if fallbackOutput, fallbackErr := fallbackCmd.CombinedOutput(); fallbackErr != nil {
					return "", fmt.Errorf("git clone: %w: %s (fallback failed: %s)", err, sanitizeToken(outStr, token), sanitizeToken(string(fallbackOutput), token))
				}
			} else {
				return "", fmt.Errorf("git clone: %w: %s", err, sanitizeToken(outStr, token))
			}
		}
	} else {
		args := append(authArgs, "clone", "--single-branch", repoURL, localPath)
		cloneCmd = gitCommand(ctx, args...)
		if output, err := cloneCmd.CombinedOutput(); err != nil {
			return "", fmt.Errorf("git clone: %w: %s", err, sanitizeToken(string(output), token))
		}
	}

	actualBranchCmd := gitCommand(ctx, "-C", localPath, "rev-parse", "--abbrev-ref", "HEAD")
	actualBranchOutput, err := actualBranchCmd.CombinedOutput()
	if err != nil {
		// Fallback for newly created empty repository where HEAD does not point to any commit yet
		targetBranch := branch
		if targetBranch == "" {
			fallbackCmd := gitCommand(ctx, "-C", localPath, "symbolic-ref", "--short", "HEAD")
			fallbackOutput, fallbackErr := fallbackCmd.CombinedOutput()
			if fallbackErr == nil {
				targetBranch = strings.TrimSpace(string(fallbackOutput))
			}
		}
		if targetBranch == "" {
			targetBranch = "main" // absolute fallback
		}

		// Initialize empty repository so that it is no longer unborn and worktrees can be created.
		// Use user config to avoid "Please tell me who you are" git commit errors.
		_ = gitCommand(ctx, "-C", localPath, "config", "user.name", "AutoCodeOS Initializer").Run()
		_ = gitCommand(ctx, "-C", localPath, "config", "user.email", "init@autocodeos.local").Run()

		// Rename/checkout to the target branch.
		checkoutCmd := gitCommand(ctx, "-C", localPath, "checkout", "-B", targetBranch)
		_ = checkoutCmd.Run()

		// Create an initial commit with an empty README.md.
		_ = os.WriteFile(filepath.Join(localPath, "README.md"), []byte("# "+targetBranch+"\n"), 0644)
		_ = gitCommand(ctx, "-C", localPath, "add", "README.md").Run()

		commitCmd := gitCommand(ctx, "-C", localPath, "commit", "-m", "Initial commit")
		if commitOut, commitErr := commitCmd.CombinedOutput(); commitErr != nil {
			return "", fmt.Errorf("failed to create initial commit for empty repo: %w: %s", commitErr, string(commitOut))
		}

		actualBranchOutput = []byte(targetBranch)
	}
	actualBranch := strings.TrimSpace(string(actualBranchOutput))
	return actualBranch, nil
}

func (p *GitHubProvider) CreateBranch(ctx context.Context, localPath, branchName string) error {
	// Detach HEAD so we are not on the branch we want to delete.
	detachCmd := gitCommand(ctx, "-C", localPath, "checkout", "--detach")
	_ = detachCmd.Run()

	// Force delete the branch if it already exists.
	deleteCmd := gitCommand(ctx, "-C", localPath, "branch", "-D", branchName)
	_ = deleteCmd.Run()

	// Create and checkout the clean branch.
	cmd := gitCommand(ctx, "-C", localPath, "checkout", "-b", branchName)
	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("git checkout branch: %w: %s", err, string(output))
	}
	return nil
}

func (p *GitHubProvider) CommitAndPush(ctx context.Context, localPath, message, token, agentRole string) error {
	// Build git identity from the agent role performing this task.
	if agentRole == "" {
		agentRole = "agent"
	}
	gitName := fmt.Sprintf("AutoCodeOS [%s]", agentRole)
	gitEmail := fmt.Sprintf("%s@autocodeos.local", agentRole)
	identityCommands := [][]string{
		{"git", "-C", localPath, "config", "user.name", gitName},
		{"git", "-C", localPath, "config", "user.email", gitEmail},
	}
	for _, args := range identityCommands {
		cmd := gitCommand(ctx, args[1:]...)
		if output, err := cmd.CombinedOutput(); err != nil {
			return fmt.Errorf("%s: %w: %s", strings.Join(args, " "), err, string(output))
		}
	}

	// Stage all changes.
	addCmd := gitCommand(ctx, "-C", localPath, "add", ".")
	if output, err := addCmd.CombinedOutput(); err != nil {
		return fmt.Errorf("git add: %w: %s", err, sanitizeToken(string(output), token))
	}

	// Check if there is anything to commit (avoid crash on clean working tree).
	statusCmd := gitCommand(ctx, "-C", localPath, "diff", "--cached", "--quiet")
	hasChanges := true
	if err := statusCmd.Run(); err == nil {
		hasChanges = false
	}

	// Commit if there are changes.
	if hasChanges {
		commitCmd := gitCommand(ctx, "-C", localPath, "commit", "-m", message)
		if output, err := commitCmd.CombinedOutput(); err != nil {
			return fmt.Errorf("git commit: %w: %s", err, sanitizeToken(string(output), token))
		}
	}

	// If the remote is empty, we must push the default branch first so that the remote base branch exists
	// and a PR can be successfully created.
	var authArgs []string
	if token != "" {
		b64 := base64.StdEncoding.EncodeToString([]byte("x-access-token:" + token))
		authArgs = append(authArgs, "-c", "http.extraHeader=AUTHORIZATION: basic "+b64)
	}

	lsArgs := append([]string{"-C", localPath}, authArgs...)
	lsArgs = append(lsArgs, "ls-remote", "--heads", "origin")
	lsCmd := gitCommand(ctx, lsArgs...)
	lsOut, err := lsCmd.CombinedOutput()
	if err == nil && len(strings.TrimSpace(string(lsOut))) == 0 {
		// Remote is empty. Find and push local default branch (main or master) to origin first.
		for _, b := range []string{"main", "master"} {
			checkCmd := gitCommand(ctx, "-C", localPath, "show-ref", "--quiet", "refs/heads/"+b)
			if err := checkCmd.Run(); err == nil {
				pushArgs := append([]string{"-C", localPath}, authArgs...)
				pushArgs = append(pushArgs, "push", "-u", "origin", b)
				_ = gitCommand(ctx, pushArgs...).Run()
				break
			}
		}
	}

	// Always force-push origin HEAD to ensure the remote branch is created or updated.
	var pushArgs []string
	pushArgs = append(pushArgs, "-C", localPath)
	if token != "" {
		b64 := base64.StdEncoding.EncodeToString([]byte("x-access-token:" + token))
		pushArgs = append(pushArgs, "-c", "http.extraHeader=AUTHORIZATION: basic "+b64)
	}
	pushArgs = append(pushArgs, "push", "-f", "origin", "HEAD")
	pushCmd := gitCommand(ctx, pushArgs...)
	if output, err := pushCmd.CombinedOutput(); err != nil {
		return fmt.Errorf("git push: %w: %s", err, sanitizeToken(string(output), token))
	}
	return nil
}
