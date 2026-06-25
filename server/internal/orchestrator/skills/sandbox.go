package skills

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/auto-code-os/auto-code-os/server/internal/sandbox"
)

func (e *SkillExecutor) runSandboxCommand(ctx context.Context, call SkillCall, command string) SkillResult {
	if e.runtime == nil {
		return skillError(call.Name, "sandbox runtime is not configured")
	}
	resp, err := e.runtime.Run(ctx, sandbox.CommandRequest{
		TaskID:      call.TaskID,
		AgentID:     call.AgentID,
		Workspace:   call.Workspace,
		Command:     []string{"bash", "-lc", command},
		NetworkMode: sandbox.NetworkModeNone,
	})
	if err != nil {
		return skillError(call.Name, err.Error())
	}
	output := strings.TrimSpace(resp.Stdout + "\n" + resp.Stderr)
	return SkillResult{Name: call.Name, Success: resp.ExitCode == 0, Output: output}
}

func (e *SkillExecutor) analyzeLogs(call SkillCall) SkillResult {
	target, err := safeWorkspacePath(call.Workspace, stringInput(call.Input, "path", ""))
	if err != nil {
		return skillError(call.Name, err.Error())
	}
	maxBytes := intInput(call.Input, "max_bytes", 4000)
	data, err := os.ReadFile(target)
	if err != nil {
		return skillError(call.Name, err.Error())
	}
	if len(data) > maxBytes {
		data = data[len(data)-maxBytes:]
	}
	return SkillResult{Name: call.Name, Success: true, Output: string(data)}
}

func (e *SkillExecutor) searchCode(call SkillCall) SkillResult {
	query := stringInput(call.Input, "query", "")
	if query == "" {
		return skillError(call.Name, "query is required")
	}
	limit := intInput(call.Input, "limit", 20)
	var matches []string
	err := filepath.WalkDir(call.Workspace, func(path string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() || len(matches) >= limit {
			return err
		}
		if shouldSkipSearchPath(path) {
			return nil
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return nil
		}
		if strings.Contains(string(data), query) {
			rel, _ := filepath.Rel(call.Workspace, path)
			matches = append(matches, rel)
		}
		return nil
	})
	if err != nil {
		return skillError(call.Name, err.Error())
	}
	return SkillResult{Name: call.Name, Success: true, Output: strings.Join(matches, "\n")}
}

func safeWorkspacePath(workspace, rel string) (string, error) {
	if rel == "" {
		return "", fmt.Errorf("path is required")
	}
	if filepath.IsAbs(rel) {
		return "", fmt.Errorf("absolute paths are not allowed")
	}
	cleanWorkspace, err := filepath.Abs(workspace)
	if err != nil {
		return "", err
	}
	target := filepath.Clean(filepath.Join(cleanWorkspace, rel))
	if !strings.HasPrefix(target, cleanWorkspace+string(os.PathSeparator)) && target != cleanWorkspace {
		return "", fmt.Errorf("path escapes workspace")
	}
	return target, nil
}

func shouldSkipSearchPath(path string) bool {
	parts := strings.Split(filepath.ToSlash(path), "/")
	for _, part := range parts {
		switch part {
		case ".git", "node_modules", "vendor", "dist", "build":
			return true
		}
	}
	return false
}
