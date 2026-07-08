package wkspace

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/auto-code-os/auto-code-os/server/pkg/models"
	"github.com/auto-code-os/auto-code-os/server/pkg/paths"
)

// GetTaskWorkspace returns the workspace layout for a task.
func (m *Manager) GetTaskWorkspace(task *models.Task) *models.TaskWorkspace {
	wp := paths.NewOSWorkspacePaths(m.WorkspaceRoot)
	return &models.TaskWorkspace{
		Root:         wp.TaskRoot(task.ID).String(),
		SpecsDir:     wp.SpecsDir(task.ID).String(),
		ContextDir:   wp.ContextDir(task.ID).String(),
		ArtifactsDir: wp.ArtifactsDir(task.ID).String(),
		LogsDir:      wp.LogsDir(task.ID).String(),
		PRDir:        wp.PRDir(task.ID).String(),
	}
}

// LoadTaskWorkspace loads workspace metadata from disk, auto-initializing if needed.
func (m *Manager) LoadTaskWorkspace(ctx context.Context, task *models.Task) (*models.TaskWorkspace, error) {
	ws := m.GetTaskWorkspace(task)
	metaJSONPath := filepath.Join(ws.Root, "metadata.json")
	metaBytes, err := os.ReadFile(metaJSONPath)
	if err != nil {
		if os.IsNotExist(err) {
			if _, statErr := os.Stat(ws.Root); statErr == nil {
				return m.InitTaskWorkspace(ctx, task)
			}
			return nil, fmt.Errorf("metadata.json not found and workspace does not exist: %w", err)
		}
		return nil, err
	}

	var meta models.TaskWorkspaceMetadata
	if err := json.Unmarshal(metaBytes, &meta); err != nil {
		return nil, fmt.Errorf("unmarshal metadata.json: %w", err)
	}

	if m.Repositories != nil {
		projectRepos, err := m.Repositories.ListByProjectID(ctx, task.ProjectID)
		if err == nil {
			repoMap := make(map[string]models.Repository)
			for _, r := range projectRepos {
				repoMap[r.ID] = r
			}

			updated := false
			for i, rWS := range meta.Repos {
				if r, ok := repoMap[rWS.RepoID]; ok {
					expectedBranch := r.Branch
					if expectedBranch == "" {
						expectedBranch = "main"
					}
					if rWS.DefaultBranch != expectedBranch {
						meta.Repos[i].DefaultBranch = expectedBranch
						meta.Repos[i].Paths.Main = paths.NewOSWorkspacePaths("").RepoMainRelative(rWS.Name, expectedBranch)
						updated = true
					}
				}
			}
			if updated {
				if saveErr := m.SaveTaskWorkspaceMetadata(task, &models.TaskWorkspace{Root: ws.Root, Repos: meta.Repos}); saveErr != nil {
					return nil, fmt.Errorf("failed to save reconciled workspace metadata: %w", saveErr)
				}
			}
		}
	}

	ws.Repos = meta.Repos
	return ws, nil
}

// SaveTaskWorkspaceMetadata persists workspace repo metadata to metadata.json.
func (m *Manager) SaveTaskWorkspaceMetadata(task *models.Task, ws *models.TaskWorkspace) error {
	meta := models.TaskWorkspaceMetadata{
		WorkspaceVersion: 1,
		Repos:            ws.Repos,
	}
	metaJSONPath := filepath.Join(ws.Root, "metadata.json")
	metaBytes, err := json.MarshalIndent(meta, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(metaJSONPath, metaBytes, 0o644)
}

// FindRepoWorkspaceByPath matches a filesystem path to a repo workspace.
func (m *Manager) FindRepoWorkspaceByPath(ctx context.Context, task *models.Task, path string) (*models.RepoWorkspace, error) {
	ws, err := m.LoadTaskWorkspace(ctx, task)
	if err != nil {
		return nil, err
	}
	absPath, err := filepath.Abs(path)
	if err != nil {
		absPath = path
	}

	for i := range ws.Repos {
		rWS := &ws.Repos[i]
		mainAbs := filepath.Join(ws.Root, rWS.Paths.Main)
		if rel, errRel := filepath.Rel(mainAbs, absPath); errRel == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
			return rWS, nil
		}
		for _, wtRel := range rWS.Paths.Worktrees {
			wtAbs := filepath.Join(ws.Root, wtRel)
			if rel, errRel := filepath.Rel(wtAbs, absPath); errRel == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
				return rWS, nil
			}
		}
		repoRootAbs := paths.NewOSWorkspacePaths(m.WorkspaceRoot).RepoRoot(task.ID, rWS.Name).String()
		if rel, errRel := filepath.Rel(repoRootAbs, absPath); errRel == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
			return rWS, nil
		}
	}
	if len(ws.Repos) == 1 {
		return &ws.Repos[0], nil
	}
	return nil, fmt.Errorf("no repository matching path %s found in workspace", path)
}
