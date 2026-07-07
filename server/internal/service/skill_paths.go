package service

import "path/filepath"

// SkillPathManager centralizes path resolution for the skills system
type SkillPathManager struct {
	skillsRoot string
}

func NewSkillPathManager(skillsRoot string) *SkillPathManager {
	return &SkillPathManager{skillsRoot: skillsRoot}
}

// GitSourceRoot returns the root directory where all git skills are cloned.
func (m *SkillPathManager) GitSourceRoot() string {
	return filepath.Join(m.skillsRoot, "git")
}

// GitRepoRoot returns the path to a specific cloned git repository.
func (m *SkillPathManager) GitRepoRoot(repoName string) string {
	return filepath.Join(m.GitSourceRoot(), repoName)
}

// GitRegistryPath returns the path to the registry.json or registry.min.json for a cloned repo.
func (m *SkillPathManager) GitRegistryPath(repoName string, minified bool) string {
	if minified {
		return filepath.Join(m.GitRepoRoot(repoName), "registry.min.json")
	}
	return filepath.Join(m.GitRepoRoot(repoName), "registry.json")
}

// GitSkillPath returns the path to a specific skill inside a cloned repo.
func (m *SkillPathManager) GitSkillPath(repoName, relPath string) string {
	return filepath.Join(m.GitRepoRoot(repoName), relPath)
}

// GitSkillPathRelative returns the relative path of a skill starting from the "git" folder.
func (m *SkillPathManager) GitSkillPathRelative(repoName, relPath string) string {
	return filepath.ToSlash(filepath.Join("git", repoName, relPath))
}

// GlobalRegistryPath returns the path to the global registry JSON.
func (m *SkillPathManager) GlobalRegistryPath(minified bool) string {
	if minified {
		return filepath.Join(m.skillsRoot, "registry.min.json")
	}
	return filepath.Join(m.skillsRoot, "registry.json")
}
