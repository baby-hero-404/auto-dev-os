package wkspace

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
)

func ResetExistingWorkspace(ctx context.Context, localPath string) error {
	commands := [][]string{
		{"git", "-C", localPath, "reset", "--hard"},
		{"git", "-C", localPath, "clean", "-fdx"},
	}
	for _, args := range commands {
		cmd := exec.CommandContext(ctx, args[0], args[1:]...)
		if output, err := cmd.CombinedOutput(); err != nil {
			return fmt.Errorf("%s: %w: %s", strings.Join(args, " "), err, string(output))
		}
	}
	return nil
}

func GetRoleFromSuffix(suffix string) string {
	suffix = strings.TrimPrefix(suffix, "-")
	suffix = strings.TrimSuffix(suffix, "-worktree")
	switch suffix {
	case "be", "backend":
		return "backend"
	case "fe", "frontend":
		return "frontend"
	case "fix":
		return "fix"
	default:
		return suffix
	}
}
