package orchestrator

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	orchestratorworkspace "github.com/auto-code-os/auto-code-os/server/internal/orchestrator/workspace"
	"github.com/auto-code-os/auto-code-os/server/internal/sandbox"
	"github.com/auto-code-os/auto-code-os/server/pkg/models"
	"go.opentelemetry.io/otel"
)

func (o *Orchestrator) runSandboxStep(ctx context.Context, task *models.Task, agent *models.Agent, stepID, command string) (map[string]any, error) {
	ctx, span := otel.Tracer("auto-code-os/orchestrator").Start(ctx, "orchestrator.sandbox_step")
	defer span.End()
	result, err := o.runtime.Run(ctx, sandbox.CommandRequest{
		TaskID:      task.ID,
		AgentID:     agent.ID,
		Command:     []string{"bash", "-lc", command},
		NetworkMode: sandbox.NetworkModeNone,
		Timeout:     5 * time.Minute,
	})
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(result.Stdout) != "" {
		o.log(ctx, task.ID, nil, "info", fmt.Sprintf("%s: %s", stepID, strings.TrimSpace(result.Stdout)))
	}
	if strings.TrimSpace(result.Stderr) != "" {
		o.log(ctx, task.ID, nil, "warn", fmt.Sprintf("%s: %s", stepID, strings.TrimSpace(result.Stderr)))
	}
	if result.ExitCode != 0 {
		return nil, fmt.Errorf("%s failed with exit code %d", stepID, result.ExitCode)
	}
	return map[string]any{"status": "ok", "stdout": result.Stdout}, nil
}

func (o *Orchestrator) runSandboxStepInWorktree(ctx context.Context, task *models.Task, agent *models.Agent, stepID, command string, worktreeSuffix string) (map[string]any, error) {
	localPath := sandbox.WorkspacePath(o.workspaceRoot, task.ID)
	hostWorkspacePath := localPath
	if worktreeSuffix != "" {
		hostWorkspacePath = o.hostWorktreePath(task, localPath, worktreeSuffix)
	}

	containerWorkDir := o.containerPathForHostPath(task, hostWorkspacePath, "")
	wrappedCommand := fmt.Sprintf("cd %s && %s", orchestratorworkspace.QuoteShellArg(containerWorkDir), command)

	ctx, span := otel.Tracer("auto-code-os/orchestrator").Start(ctx, "orchestrator.sandbox_step")
	defer span.End()
	result, err := o.runtime.Run(ctx, sandbox.CommandRequest{
		TaskID:      task.ID,
		AgentID:     agent.ID,
		Workspace:   localPath,
		Command:     []string{"bash", "-lc", wrappedCommand},
		NetworkMode: sandbox.NetworkModeNone,
		Timeout:     5 * time.Minute,
	})
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(result.Stdout) != "" {
		o.log(ctx, task.ID, nil, "info", fmt.Sprintf("%s: %s", stepID, strings.TrimSpace(result.Stdout)))
	}
	if strings.TrimSpace(result.Stderr) != "" {
		o.log(ctx, task.ID, nil, "warn", fmt.Sprintf("%s: %s", stepID, strings.TrimSpace(result.Stderr)))
	}
	if result.ExitCode != 0 {
		return nil, fmt.Errorf("%s failed with exit code %d", stepID, result.ExitCode)
	}
	return map[string]any{"status": "ok", "stdout": result.Stdout}, nil
}

func (o *Orchestrator) containerPathForHostPath(task *models.Task, hostPath string, worktreeSuffix string) string {
	localPath := sandbox.WorkspacePath(o.workspaceRoot, task.ID)
	activeWorkspaceHostPath := localPath
	if worktreeSuffix != "" {
		activeWorkspaceHostPath = o.hostWorktreePath(task, localPath, worktreeSuffix)
	}
	return orchestratorworkspace.ContainerPathForHostPath(localPath, activeWorkspaceHostPath, hostPath)
}


func (o *Orchestrator) readAffectedFileContent(ctx context.Context, task *models.Task, file string) (string, bool) {
	file = strings.TrimSpace(file)
	if file == "" {
		return "", false
	}

	for _, root := range o.affectedFileRoots(ctx, task, file) {
		safePath, err := orchestratorworkspace.ResolveSafePath(root, file)
		if err == nil {
			if content, readErr := orchestratorworkspace.ReadLimitedFile(safePath, 20_000); readErr == nil {
				return content, true
			}
		}
	}

	ws, err := o.LoadTaskWorkspace(ctx, task)
	if err != nil || ws == nil {
		return "", false
	}
	for _, repo := range ws.Repos {
		prefix := repo.Name + string(filepath.Separator)
		if strings.HasPrefix(filepath.Clean(file), prefix) {
			rel := strings.TrimPrefix(filepath.Clean(file), prefix)
			root := filepath.Join(ws.Root, repo.Paths.Main)
			safePath, err := orchestratorworkspace.ResolveSafePath(root, rel)
			if err == nil {
				if content, readErr := orchestratorworkspace.ReadLimitedFile(safePath, 20_000); readErr == nil {
					return content, true
				}
			}
		}
	}
	return "", false
}

func (o *Orchestrator) affectedFileRoots(ctx context.Context, task *models.Task, file string) []string {
	localPath := sandbox.WorkspacePath(o.workspaceRoot, task.ID)
	roots := []string{localPath}
	if repoHostPath, err := o.getTaskRepoHostPath(ctx, task); err == nil && repoHostPath != localPath {
		roots = append([]string{repoHostPath}, roots...)
	}
	return roots
}
