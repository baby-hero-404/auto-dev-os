package paths

import (
	"path/filepath"
)

// OSPromptPaths implements PromptPaths for the local OS filesystem.
type OSPromptPaths struct {
	promptsRoot string
}

// NewOSPromptPaths creates a new OSPromptPaths provider.
func NewOSPromptPaths(promptsRoot string) *OSPromptPaths {
	return &OSPromptPaths{promptsRoot: filepath.ToSlash(filepath.Clean(promptsRoot))}
}

// Root returns the root prompts directory.
func (p *OSPromptPaths) Root() Directory {
	return NewDirectory(p.promptsRoot)
}

// RolePrompt returns the file path for a role-specific system prompt (e.g. roles/planner.md).
func (p *OSPromptPaths) RolePrompt(role string) File {
	return p.Root().Child("roles").File(role + ".md")
}

// StepPrompt returns the file path for a step-specific instructions file (e.g. analyze.md).
func (p *OSPromptPaths) StepPrompt(step string) File {
	return p.Root().Child("steps").File(step + ".md")
}

// CorePrompt returns the file path for a core file (e.g. rules.md, system_prompt.md).
func (p *OSPromptPaths) CorePrompt(name string) File {
	return p.Root().Child("core").File(name)
}
