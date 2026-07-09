package paths

// WorkspacePaths defines interface for workspace path queries.
type WorkspacePaths interface {
	TaskRoot(taskID string) Directory
	SpecsDir(taskID string) Directory
	ContextDir(taskID string) Directory
	ArtifactsDir(taskID string) Directory
	LogsDir(taskID string) Directory
	PRDir(taskID string) Directory
	OpenSpecDir(taskID, changeName string) Directory
	CodeRoot(taskID string) Directory
	RepoRoot(taskID, repoName string) Directory
	RepoMain(taskID, repoName string) Directory
	RepoMainRelative(repoName string) string // returns relative string for metadata
	RepoWorktreeDir(taskID, repoName, role string) Directory
	RepoWorktreeRelative(repoName, role string) string // returns relative string
}

// PromptPaths defines interface for system, step, and role prompts.
type PromptPaths interface {
	Root() Directory
	RolePrompt(role string) File
	StepPrompt(step string) File
	CorePrompt(name string) File
}

// SkillPaths defines interface for the skill system directory structure.
type SkillPaths interface {
	Root() Directory
	GitSourceRoot() Directory
	GitRepoRoot(repoName string) Directory
	GitRegistryPath(repoName string, minified bool) File
	GitSkillPath(repoName, relPath string) File
	GitSkillPathRelative(repoName, relPath string) string // returns relative string
	GlobalRegistryPath(minified bool) File
}

// DatabasePaths defines interface for cache/data SQLite paths.
type DatabasePaths interface {
	CacheDB() File
}

// MigrationSource defines interface for DB migration script root.
type MigrationSource interface {
	Root() Directory
}

// LogPaths defines interface for global logging roots.
type LogPaths interface {
	Root() Directory
	LogFile(name string) File
}
