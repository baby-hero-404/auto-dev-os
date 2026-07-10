package skills

// SkillCall encapsulates the legacy tool execution request.
type SkillCall struct {
	Name         string         `json:"name"`
	Input        map[string]any `json:"input"`
	Workspace    string         `json:"workspace"`
	TaskID       string         `json:"task_id"`
	AgentID      string         `json:"agent_id"`
	AgentName    string         `json:"agent_name,omitempty"`
	AllowedTools []string       `json:"allowed_tools,omitempty"`
}

// SkillResult encapsulates the legacy tool execution response.
type SkillResult struct {
	Name    string `json:"name"`
	Output  string `json:"output"`
	Success bool   `json:"success"`
	Error   string `json:"error,omitempty"`
}
