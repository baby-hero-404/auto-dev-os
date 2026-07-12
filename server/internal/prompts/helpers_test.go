package prompts

import (
	"strings"
	"testing"

	"github.com/auto-code-os/auto-code-os/server/internal/workflow"
	"github.com/auto-code-os/auto-code-os/server/pkg/models"
)

func TestDeduplicateSnippets_OverlappingRanges(t *testing.T) {
	snippets := []models.ContextSnippet{
		{Path: "pkg/handler.go", StartLine: 10, EndLine: 30, Content: "func A() {}", Relevance: 9.0, Retriever: "semantic"},
		{Path: "pkg/handler.go", StartLine: 17, EndLine: 37, Content: "func A() {} // overlap", Relevance: 8.5, Retriever: "semantic"},
		{Path: "pkg/service.go", StartLine: 1, EndLine: 20, Content: "func B() {}", Relevance: 8.0, Retriever: "keyword"},
	}
	got := deduplicateSnippets(snippets)
	if len(got) != 2 {
		t.Fatalf("expected 2 snippets after dedup, got %d", len(got))
	}
	if got[0].Path != "pkg/handler.go" || got[1].Path != "pkg/service.go" {
		t.Fatalf("unexpected dedup result: %#v", got)
	}
}

func TestDeduplicateSnippets_DifferentFiles_Kept(t *testing.T) {
	snippets := []models.ContextSnippet{
		{Path: "a.go", StartLine: 1, EndLine: 20, Content: "file a"},
		{Path: "b.go", StartLine: 1, EndLine: 20, Content: "file b"},
	}
	got := deduplicateSnippets(snippets)
	if len(got) != 2 {
		t.Fatalf("expected 2 snippets (different files), got %d", len(got))
	}
}

func TestDeduplicateSnippets_NoOverlap_Kept(t *testing.T) {
	snippets := []models.ContextSnippet{
		{Path: "a.go", StartLine: 1, EndLine: 10, Content: "first"},
		{Path: "a.go", StartLine: 50, EndLine: 60, Content: "second"},
	}
	got := deduplicateSnippets(snippets)
	if len(got) != 2 {
		t.Fatalf("expected 2 snippets (no overlap), got %d", len(got))
	}
}

func TestLineOverlap_Full(t *testing.T) {
	a := models.ContextSnippet{StartLine: 10, EndLine: 30}
	b := models.ContextSnippet{StartLine: 10, EndLine: 30}
	if o := lineOverlap(a, b); o != 1.0 {
		t.Fatalf("expected overlap 1.0, got %f", o)
	}
}

func TestLineOverlap_None(t *testing.T) {
	a := models.ContextSnippet{StartLine: 10, EndLine: 20}
	b := models.ContextSnippet{StartLine: 30, EndLine: 40}
	if o := lineOverlap(a, b); o != 0 {
		t.Fatalf("expected overlap 0, got %f", o)
	}
}

func TestExtractSubtaskIndex(t *testing.T) {
	idx, ok := extractSubtaskIndex("code_backend_0")
	if !ok || idx != 0 {
		t.Errorf("Expected 0, true; got %d, %v", idx, ok)
	}

	idx, ok = extractSubtaskIndex("code_frontend_12")
	if !ok || idx != 12 {
		t.Errorf("Expected 12, true; got %d, %v", idx, ok)
	}

	_, ok = extractSubtaskIndex("code_backend")
	if ok {
		t.Errorf("Expected false for missing index")
	}
}

func TestExtractSpecsSectionForSubtask(t *testing.T) {
	tasksMD := `
## 1. Init
- [ ] Task 1
## 2. Sync
- [ ] Task 2
`
	specsMD := `
## ADDED Requirements
### Requirement: 1. Init
The init must do X.
### Requirement: 2. Sync
The sync must do Y.
`

	section := extractSpecsSectionForSubtask(specsMD, tasksMD, 0, "code_backend_0")
	if !strings.Contains(section, "1. Init") || strings.Contains(section, "2. Sync") {
		t.Errorf("Expected section 1, got %q", section)
	}

	section2 := extractSpecsSectionForSubtask(specsMD, tasksMD, 1, "code_backend_1")
	if !strings.Contains(section2, "2. Sync") || strings.Contains(section2, "1. Init") {
		t.Errorf("Expected section 2, got %q", section2)
	}
}

// TestExtractSpecsSectionForSubtask_ClassificationMatchesParseTasksMD guards against the
// isRole/classifyHeading drift from REQ-M01: a heading like "Build Login Page" is bucketed
// as frontend by ParseTasksMD (via the "page" signal) but the old local isRole closure in
// extractSpecsSectionForSubtask didn't recognize "page" and defaulted it to backend,
// desynchronizing the backend heading index from the one ParseTasksMD actually used.
func TestExtractSpecsSectionForSubtask_ClassificationMatchesParseTasksMD(t *testing.T) {
	tasksMD := `
## 1. Setup Database Handler
- [ ] Task 1
## 2. Build Login Page
- [ ] Task 2
## 3. Create API Endpoint
- [ ] Task 3
`
	specsMD := `
## ADDED Requirements
### Requirement: 1. Setup Database Handler
Backend subtask one.
### Requirement: 2. Build Login Page
Frontend subtask.
### Requirement: 3. Create API Endpoint
Backend subtask two.
`

	subtasks := workflow.ParseTasksMD(tasksMD)
	if len(subtasks["backend"]) != 2 || len(subtasks["frontend"]) != 1 {
		t.Fatalf("expected ParseTasksMD to bucket 2 backend + 1 frontend, got backend=%d frontend=%d", len(subtasks["backend"]), len(subtasks["frontend"]))
	}

	// The 2nd backend subtask (index 1) is "Create API Endpoint" (heading #3), NOT
	// "Build Login Page" (heading #2), which belongs to frontend.
	section := extractSpecsSectionForSubtask(specsMD, tasksMD, 1, "code_backend_1")
	if !strings.Contains(section, "3. Create API Endpoint") {
		t.Errorf("expected 2nd backend subtask to resolve to heading 3, got %q", section)
	}
	if strings.Contains(section, "2. Build Login Page") {
		t.Errorf("backend subtask incorrectly matched the frontend-classified heading: %q", section)
	}

	// The 1st frontend subtask (index 0) is "Build Login Page" (heading #2).
	feSection := extractSpecsSectionForSubtask(specsMD, tasksMD, 0, "code_frontend_0")
	if !strings.Contains(feSection, "2. Build Login Page") {
		t.Errorf("expected 1st frontend subtask to resolve to heading 2, got %q", feSection)
	}
}
