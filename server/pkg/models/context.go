package models

type ContextSnippet struct {
	Source    string  `json:"source"`
	Path      string  `json:"path"`
	StartLine int     `json:"start_line"`
	EndLine   int     `json:"end_line"`
	Content   string  `json:"content"`
	Relevance float64 `json:"relevance"`
	Retriever string  `json:"retriever"`
}

// ContextCache holds pre-computed context artifacts from ContextLoad.
type ContextCache struct {
	SemanticSnippets []ContextSnippet `json:"semantic_snippets"`
	RepoMap          string           `json:"repo_map"`
	DirectoryTree    string           `json:"directory_tree"`
	ActiveFiles      []string         `json:"active_files"`
}
