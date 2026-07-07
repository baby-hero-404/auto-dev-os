package paths

// PathRegistry acts as the Dependency Injection container (Composition Root) for all path providers.
// Registry MUST NOT be injected into business services. Instead, business services depend only on interfaces.
type PathRegistry struct {
	Workspace WorkspacePaths
	Prompt    PromptPaths
	Skill     SkillPaths
	Log       LogPaths
	Migration MigrationSource
	Database  DatabasePaths
	FS        FileSystem
}

// NewRegistry instantiates the OS-backed concrete implementations for the PathRegistry.
func NewRegistry(dataRoot, workspaceRoot, skillsRoot, logRoot, promptsRoot, migrationsRoot string) *PathRegistry {
	return &PathRegistry{
		Workspace: NewOSWorkspacePaths(workspaceRoot),
		Prompt:    NewOSPromptPaths(promptsRoot),
		Skill:     NewOSSkillPaths(skillsRoot),
		Log:       NewOSLogPaths(logRoot),
		Migration: NewOSMigrationSource(migrationsRoot),
		Database:  NewOSDatabasePaths(dataRoot),
		FS:        NewOSFileSystem(),
	}
}
