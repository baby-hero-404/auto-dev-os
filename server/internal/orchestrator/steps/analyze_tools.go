package steps

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/auto-code-os/auto-code-os/server/internal/tool"
	"github.com/auto-code-os/auto-code-os/server/pkg/paths"
)

// listAnalyzeFiles lists all files in the task workspace.
func (s *AnalyzeStep) listAnalyzeFiles(ctx context.Context) (string, error) {
	if s.registry == nil {
		return "", fmt.Errorf("registry not configured")
	}

	osPaths := paths.NewOSWorkspacePaths(s.workspaceRoot)
	localPath := osPaths.TaskRoot(s.rt.Task.ID).String()

	var roots []struct {
		path   string
		prefix string
	}
	if s.wkspace == nil {
		roots = append(roots, struct{ path, prefix string }{localPath, ""})
	} else {
		ws, err := s.wkspace.LoadTaskWorkspace(ctx, s.rt.Task)
		if err != nil || ws == nil || len(ws.Repos) == 0 {
			roots = append(roots, struct{ path, prefix string }{localPath, ""})
		} else {
			targetCount := 0
			for _, repo := range ws.Repos {
				if s.rt.Task.RepositoryID != nil && repo.RepoID != *s.rt.Task.RepositoryID {
					continue
				}
				if repo.Paths.Main == "" {
					continue
				}
				targetCount++
			}
			for _, repo := range ws.Repos {
				if s.rt.Task.RepositoryID != nil && repo.RepoID != *s.rt.Task.RepositoryID {
					continue
				}
				if repo.Paths.Main == "" {
					continue
				}
				prefix := ""
				if s.rt.Task.RepositoryID == nil && targetCount > 1 {
					prefix = repo.Name
				}
				roots = append(roots, struct{ path, prefix string }{
					path:   osPaths.RepoMain(s.rt.Task.ID, repo.Name).String(),
					prefix: prefix,
				})
			}
			if len(roots) == 0 {
				roots = append(roots, struct{ path, prefix string }{localPath, ""})
			}
		}
	}

	var allFiles []string
	for _, root := range roots {
		res, err := s.registry.Execute(ctx, "list_files", tool.Call{
			Input:     map[string]any{"max_depth": 3, "max_files": 200},
			Workspace: root.path,
			TaskID:    s.rt.Task.ID,
			AgentID:   s.rt.Agent.ID,
			AgentRole: s.rt.Agent.Role,
		})
		if err != nil {
			return "", err
		}
		if !res.Success {
			return "", fmt.Errorf("list_files failed for %s: %s", root.path, res.Message)
		}

		if filesVal, exists := res.Metadata["files"]; exists {
			if filesSlice, ok := filesVal.([]string); ok {
				for _, f := range filesSlice {
					joined := f
					if root.prefix != "" {
						joined = filepath.ToSlash(filepath.Join(root.prefix, f))
					}
					allFiles = append(allFiles, joined)
				}
			} else if anySlice, ok := filesVal.([]any); ok {
				for _, fVal := range anySlice {
					if f, ok := fVal.(string); ok {
						joined := f
						if root.prefix != "" {
							joined = filepath.ToSlash(filepath.Join(root.prefix, f))
						}
						allFiles = append(allFiles, joined)
					}
				}
			}
		}
	}

	if len(allFiles) == 0 {
		return "No files found in workspace.", nil
	}
	return strings.Join(allFiles, "\n"), nil
}

