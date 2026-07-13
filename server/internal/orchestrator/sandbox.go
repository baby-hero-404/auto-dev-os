package orchestrator

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"github.com/auto-code-os/auto-code-os/server/internal/sandbox"
	"github.com/auto-code-os/auto-code-os/server/pkg/models"
	"github.com/auto-code-os/auto-code-os/server/pkg/paths"
	"go.opentelemetry.io/otel"
)

func (o *Orchestrator) runSandboxStep(ctx context.Context, task *models.Task, agent *models.Agent, stepID, command string) (map[string]any, error) {
	ctx, span := otel.Tracer("auto-code-os/orchestrator").Start(ctx, "orchestrator.sandbox_step")
	defer span.End()

	localPath := sandbox.WorkspacePath(o.workspaceRoot, task.ID)
	networkMode := sandbox.NetworkModeNone
	if !o.disableNetworking {
		networkMode = sandbox.NetworkModeBridge
	}

	result, err := o.runtime.Run(ctx, sandbox.CommandRequest{
		TaskID:      task.ID,
		AgentID:     agent.ID,
		Workspace:   localPath,
		Command:     []string{"bash", "-lc", command},
		NetworkMode: networkMode,
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
		errMsg := fmt.Sprintf("%s failed with exit code %d", stepID, result.ExitCode)
		if strings.TrimSpace(result.Stderr) != "" {
			errMsg += ": " + strings.TrimSpace(result.Stderr)
		}
		return nil, fmt.Errorf("%s", errMsg)
	}
	return map[string]any{"status": "ok", "stdout": result.Stdout}, nil
}

func (o *Orchestrator) runSandboxStepInWorktree(ctx context.Context, task *models.Task, agent *models.Agent, stepID, command string, worktreeSuffix string) (map[string]any, error) {
	o.initRepoutil()
	localPath := sandbox.WorkspacePath(o.workspaceRoot, task.ID)
	hostWorkspacePath := localPath
	if worktreeSuffix != "" {
		hostWorkspacePath = o.repoutil.HostWorktreePath(task, localPath, worktreeSuffix)
	}

	containerWorkDir := o.containerPathForHostPath(task, hostWorkspacePath, "")
	wrappedCommand := fmt.Sprintf("cd %s && %s", paths.QuoteShellArg(containerWorkDir), command)

	ctx, span := otel.Tracer("auto-code-os/orchestrator").Start(ctx, "orchestrator.sandbox_step")
	defer span.End()

	networkMode := sandbox.NetworkModeNone
	if !o.disableNetworking {
		networkMode = sandbox.NetworkModeBridge
	}

	result, err := o.runtime.Run(ctx, sandbox.CommandRequest{
		TaskID:      task.ID,
		AgentID:     agent.ID,
		Workspace:   localPath,
		Command:     []string{"bash", "-lc", wrappedCommand},
		NetworkMode: networkMode,
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
		errMsg := fmt.Sprintf("%s failed with exit code %d", stepID, result.ExitCode)
		if strings.TrimSpace(result.Stderr) != "" {
			errMsg += ": " + strings.TrimSpace(result.Stderr)
		}
		return nil, fmt.Errorf("%s", errMsg)
	}
	return map[string]any{"status": "ok", "stdout": result.Stdout}, nil
}

func (o *Orchestrator) containerPathForHostPath(task *models.Task, hostPath string, worktreeSuffix string) string {
	o.initRepoutil()
	localPath := sandbox.WorkspacePath(o.workspaceRoot, task.ID)
	activeWorkspaceHostPath := localPath
	if worktreeSuffix != "" {
		activeWorkspaceHostPath = o.repoutil.HostWorktreePath(task, localPath, worktreeSuffix)
	}
	return paths.ContainerPathForHostPath(localPath, activeWorkspaceHostPath, hostPath)
}

func (o *Orchestrator) readAffectedFileContent(ctx context.Context, task *models.Task, file string) (string, bool) {
	file = strings.TrimSpace(file)
	if file == "" {
		return "", false
	}

	var role string
	if task.AgentID != nil && o.agents != nil {
		if agent, err := o.agents.GetByID(ctx, *task.AgentID); err == nil && agent != nil {
			role = agent.Role
		}
	}

	o.initWkspace()
	ws, err := o.wkspace.LoadTaskWorkspace(ctx, task)
	if err == nil && ws != nil {
		for _, repo := range ws.Repos {
			prefix := repo.Name + string(filepath.Separator)
			if strings.HasPrefix(filepath.Clean(file), prefix) {
				rel := strings.TrimPrefix(filepath.Clean(file), prefix)
				if role != "" && repo.Paths.Worktrees != nil {
					if wtPath, ok := repo.Paths.Worktrees[role]; ok && wtPath != "" {
						root := filepath.Join(ws.Root, wtPath)
						safePath, err := paths.ResolveSafePath(root, rel)
						if err == nil {
							if content, readErr := paths.ReadLimitedFile(safePath, 20_000); readErr == nil {
								return content, true
							}
						}
					}
				}
				root := filepath.Join(ws.Root, repo.Paths.Main)
				safePath, err := paths.ResolveSafePath(root, rel)
				if err == nil {
					if content, readErr := paths.ReadLimitedFile(safePath, 20_000); readErr == nil {
						return content, true
					}
				}
			}
		}
		// Fallback for single repository workspace if file does not have a repo prefix
		if len(ws.Repos) == 1 {
			repo := ws.Repos[0]
			if role != "" && repo.Paths.Worktrees != nil {
				if wtPath, ok := repo.Paths.Worktrees[role]; ok && wtPath != "" {
					root := filepath.Join(ws.Root, wtPath)
					safePath, err := paths.ResolveSafePath(root, file)
					if err == nil {
						if content, readErr := paths.ReadLimitedFile(safePath, 20_000); readErr == nil {
							return content, true
						}
					}
				}
			}
			root := filepath.Join(ws.Root, repo.Paths.Main)
			safePath, err := paths.ResolveSafePath(root, file)
			if err == nil {
				if content, readErr := paths.ReadLimitedFile(safePath, 20_000); readErr == nil {
					return content, true
				}
			}
		}
	}

	for _, root := range o.affectedFileRoots(ctx, task, file) {
		safePath, err := paths.ResolveSafePath(root, file)
		if err == nil {
			if content, readErr := paths.ReadLimitedFile(safePath, 20_000); readErr == nil {
				return content, true
			}
		}
	}

	return "", false
}

func (o *Orchestrator) affectedFileRoots(ctx context.Context, task *models.Task, file string) []string {
	o.initRepoutil()
	localPath := sandbox.WorkspacePath(o.workspaceRoot, task.ID)
	roots := []string{localPath}
	if repoHostPath, err := o.repoutil.GetTaskRepoHostPath(ctx, task); err == nil && repoHostPath != localPath {
		roots = append([]string{repoHostPath}, roots...)
	}
	return roots
}
