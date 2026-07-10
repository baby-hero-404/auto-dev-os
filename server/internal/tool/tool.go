package tool

import (
	"context"
	"encoding/json"
)

// Category defines the classification of a tool.
type Category string

const (
	CategoryFilesystem    Category = "filesystem"
	CategoryEditing       Category = "editing"
	CategoryGit           Category = "git"
	CategorySearch        Category = "search"
	CategoryBuild         Category = "build"
	CategoryContext       Category = "context"
	CategoryDocumentation Category = "documentation"
)

// Capability defines the operational boundary required to execute a tool.
type Capability string

const (
	CapRead       Capability = "read"
	CapEdit       Capability = "edit"
	CapCreate     Capability = "create"
	CapSearch     Capability = "search"
	CapBuild      Capability = "build"
	CapGit        Capability = "git"
	CapGitDiff    Capability = "git.diff"
	CapContext    Capability = "context"
	CapDocs       Capability = "docs"
	CapDependency Capability = "dependency"
)

// Diagnostic represents a compiler, linter, or test execution error/warning.
type Diagnostic struct {
	Severity string `json:"severity"` // "error" or "warning"
	File     string `json:"file"`
	Line     int    `json:"line"`
	Message  string `json:"message"`
}

// Call encapsulates the context and input arguments for a tool invocation.
type Call struct {
	Input     map[string]any `json:"input"`
	Workspace string         `json:"workspace"`
	TaskID    string         `json:"task_id"`
	AgentID   string         `json:"agent_id"`
	AgentRole string         `json:"agent_role"`
}

// Result captures the output and structured metadata from a tool execution.
type Result struct {
	Success      bool           `json:"success"`
	Message      string         `json:"message,omitempty"`
	Output       string         `json:"output,omitempty"`
	FilesChanged []string       `json:"files_changed,omitempty"`
	Diagnostics  []Diagnostic   `json:"diagnostics,omitempty"`
	Metadata     map[string]any `json:"metadata,omitempty"`
}

// Tool represents a standard agent capability.
type Tool interface {
	Name() string
	Description() string
	Schema() json.RawMessage
	Category() Category
	Capabilities() []Capability
	Execute(ctx context.Context, call Call) (Result, error)
}

// AffectedFilesProvider defines the interface to retrieve affected files for a task.
type AffectedFilesProvider interface {
	GetAffectedFiles(ctx context.Context, taskID string) ([]string, error)
}
