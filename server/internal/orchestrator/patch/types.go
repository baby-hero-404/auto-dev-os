package patch

// ValidationError represents a structural or logical failure in a proposed patch.
type ValidationError struct {
	RepoName string
	Filepath string
	Reason   string
	IsFatal  bool
}

// EditBlock represents a single Search & Replace block extracted from LLM output.
type EditBlock struct {
	Filepath string
	Search   string
	Replace  string
}
