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
