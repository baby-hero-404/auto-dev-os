package paths

import (
	"path/filepath"
)

// OSSkillPaths implements SkillPaths for the local OS filesystem.
type OSSkillPaths struct {
	skillsRoot string
}

// NewOSSkillPaths creates a new OSSkillPaths provider.
func NewOSSkillPaths(skillsRoot string) *OSSkillPaths {
	return &OSSkillPaths{skillsRoot: filepath.ToSlash(filepath.Clean(skillsRoot))}
}

// Root returns the skills base root directory.
func (s *OSSkillPaths) Root() Directory {
	return NewDirectory(s.skillsRoot)
}

// GitSourceRoot returns the root directory where all git skills are cloned.
func (s *OSSkillPaths) GitSourceRoot() Directory {
	return s.Root().Child("git")
}

// GitRepoRoot returns the path to a specific cloned git repository.
func (s *OSSkillPaths) GitRepoRoot(repoName string) Directory {
	return s.GitSourceRoot().Child(repoName)
}

// GitRegistryPath returns the path to the registry.json or registry.min.json for a cloned repo.
func (s *OSSkillPaths) GitRegistryPath(repoName string, minified bool) File {
	if minified {
		return s.GitRepoRoot(repoName).File("registry.min.json")
	}
	return s.GitRepoRoot(repoName).File("registry.json")
}

// GitSkillPath returns the path to a specific skill inside a cloned repo.
func (s *OSSkillPaths) GitSkillPath(repoName, relPath string) File {
	return s.GitRepoRoot(repoName).File(relPath)
}

// GitSkillPathRelative returns the relative path of a skill starting from the "git" folder.
func (s *OSSkillPaths) GitSkillPathRelative(repoName, relPath string) string {
	return filepath.ToSlash(filepath.Join("git", repoName, relPath))
}

// GlobalRegistryPath returns the path to the global registry JSON.
func (s *OSSkillPaths) GlobalRegistryPath(minified bool) File {
	if minified {
		return s.Root().File("registry.min.json")
	}
	return s.Root().File("registry.json")
}
