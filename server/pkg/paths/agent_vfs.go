package paths

import (
	"fmt"
	"path/filepath"
	"strings"
)

// AgentPathContextKey is the context key for passing AgentPathContext between components.
const AgentPathContextKey = "agent_path_context"

// AgentPathContext provides sandboxed path resolution for agents executing tasks on worktrees.
type AgentPathContext struct {
	physicalRoot  string // for single repo: ".../worktrees/backend", for multi-repo: ".../code/repos"
	UseRepoPrefix bool
	RepoName      string
	Role          string
}

// NewAgentPathContext creates a new AgentPathContext bound to a physical worktree root.
func NewAgentPathContext(physicalRoot string, useRepoPrefix bool, repoName string, role string) *AgentPathContext {
	physicalRoot = filepath.Clean(physicalRoot)
	if abs, err := filepath.Abs(physicalRoot); err == nil {
		physicalRoot = abs
	}
	return &AgentPathContext{
		physicalRoot:  filepath.ToSlash(physicalRoot),
		UseRepoPrefix: useRepoPrefix,
		RepoName:      repoName,
		Role:          role,
	}
}

// StripGitDiffArtifacts cleans common git diff path decorations (e.g. "a/", "b/").
func (v *AgentPathContext) StripGitDiffArtifacts(path string) string {
	path = filepath.ToSlash(filepath.Clean(path))
	path = strings.TrimPrefix(path, "/")
	
	// Remove standard git diff prefixes
	if strings.HasPrefix(path, "a/") {
		path = strings.TrimPrefix(path, "a/")
	} else if strings.HasPrefix(path, "b/") {
		path = strings.TrimPrefix(path, "b/")
	}
	
	return filepath.ToSlash(filepath.Clean(path))
}

// ToLogical translates a physical absolute path to a clean relative logical path.
func (v *AgentPathContext) ToLogical(physical string) (string, error) {
	physical = filepath.Clean(physical)
	if abs, err := filepath.Abs(physical); err == nil {
		physical = abs
	}
	physical = filepath.ToSlash(physical)

	if !strings.HasPrefix(physical, v.physicalRoot) {
		return "", fmt.Errorf("path %s is outside the worktree root %s", physical, v.physicalRoot)
	}

	rel, err := filepath.Rel(v.physicalRoot, physical)
	if err != nil {
		return "", err
	}
	rel = filepath.ToSlash(rel)
	if rel == "." {
		rel = ""
	}

	if v.UseRepoPrefix {
		// Handle multi-repo paths like "repoName/worktrees/role/path"
		parts := strings.Split(rel, "/")
		if len(parts) >= 4 && parts[1] == "worktrees" {
			// Strip "worktrees/role"
			rel = parts[0] + "/" + strings.Join(parts[3:], "/")
		} else if v.RepoName != "" && rel != "" {
			rel = filepath.ToSlash(filepath.Join(v.RepoName, rel))
		}
	}

	return rel, nil
}

// ToPhysical translates a relative logical path back to a physical absolute path, enforcing sandbox rules.
func (v *AgentPathContext) ToPhysical(logical string) (string, error) {
	if filepath.IsAbs(logical) || strings.HasPrefix(logical, "/") || strings.HasPrefix(logical, "\\") {
		return "", fmt.Errorf("absolute path not allowed: %s", logical)
	}

	logical = v.StripGitDiffArtifacts(logical)

	if v.UseRepoPrefix {
		parts := strings.Split(logical, "/")
		if len(parts) >= 2 {
			repoName := parts[0]
			relPath := strings.Join(parts[1:], "/")
			roleFolder := "main"
			if v.Role != "" {
				roleFolder = filepath.Join("worktrees", v.Role)
			}
			logical = filepath.Join(repoName, roleFolder, relPath)
		} else if v.RepoName != "" {
			roleFolder := "main"
			if v.Role != "" {
				roleFolder = filepath.Join("worktrees", v.Role)
			}
			logical = filepath.Join(v.RepoName, roleFolder, logical)
		}
	} else if v.RepoName != "" {
		repoPrefix := v.RepoName + "/"
		if strings.HasPrefix(logical, repoPrefix) {
			logical = logical[len(repoPrefix):]
		} else if logical == v.RepoName {
			logical = ""
		}
	}

	// Prevent directory traversal attacks
	cleanLogical := filepath.Clean(logical)
	if strings.HasPrefix(cleanLogical, "..") || strings.HasPrefix(cleanLogical, "/") {
		return "", fmt.Errorf("path traversal violation: %s is outside the sandbox", logical)
	}

	physical := filepath.Join(v.physicalRoot, cleanLogical)
	physical = filepath.Clean(physical)
	if abs, err := filepath.Abs(physical); err == nil {
		physical = abs
	}
	physical = filepath.ToSlash(physical)

	// Ensure the physical path is strictly within the physicalRoot
	if !strings.HasPrefix(physical, v.physicalRoot) && physical != v.physicalRoot {
		return "", fmt.Errorf("security boundary violation: %s resolved outside %s", logical, v.physicalRoot)
	}

	return physical, nil
}

// PhysicalRoot returns the underlying physical root directory of this context.
func (v *AgentPathContext) PhysicalRoot() string {
	return v.physicalRoot
}
