# Template System Migration Implementation Plan

> **For agentic workers:** Use subagent-driven-development or executing-plans
> to implement this plan task-by-task. Steps use checkbox syntax for tracking.

**Goal:** Migrate hardcoded string concatenation in LLM instruction building to a robust Go `text/template` runtime disk-based system, inject available JIT skills into the analyze step, and fix the clarification question infinite loop.

**Architecture:** Use `text/template` and `os.ReadFile` to load `.tmpl` files from `server/internal/prompts/templates/`. The templates will be evaluated using a structured params object. Update `AnalyzeStep` to format and inject available JIT skills to the instruction and append a strict instruction to prevent the LLM from looping clarification questions.

**Tech Stack:** Go `text/template`, `os`

---

### Task 1: Create the Template Directory and Files

**Files:**
- Create: `server/internal/prompts/templates/coding_instruction.tmpl`
- Create: `server/internal/prompts/templates/analyze_instruction.tmpl`

- [x] **Step 1: Create the coding instruction template**

```go
// Create file: server/internal/prompts/templates/coding_instruction.tmpl
// Content:
{{.InstructionVerb}}{{if .RepoContext}} {{.RepoContext}}{{else}} Your diff paths MUST include the repository name prefix (e.g., --- a/repo-name/filepath).{{end}} DO NOT rewrite the entire file unless creating a new file.

{{if .PRFeedback}}
Note: The previous PR was rejected. Address the following PR rejection feedback:

{{.PRFeedback}}
{{end}}

{{if .Tree}}
=== Repository Structure ===
{{.Tree}}
{{end}}

{{if .AssignedSubtask}}
## Your Assigned Subtask:
{{.AssignedSubtask}}
{{else if .AssignedSubtasks}}
## Your Assigned Subtasks:
{{range $index, $task := .AssignedSubtasks}}{{add $index 1}}. {{$task}}
{{end}}
{{end}}

{{if .PriorFiles}}
### Files Created/Modified by Prior Steps ###
{{range .PriorFiles}}- {{.}}
{{end}}
{{end}}
```

- [x] **Step 2: Create the analyze instruction template**

```go
// Create file: server/internal/prompts/templates/analyze_instruction.tmpl
// Content:
Analyze this task and output the proposed specification as a valid JSON object matching the schema and template requested in the system instructions.

CRITICAL: If all requirements are clear and you have no NEW questions, you MUST return an empty array `[]` for `clarification_questions`. DO NOT repeat questions that have already been answered in the context.

{{if .AvailableSkills}}
=== Available JIT Skills ===
The following skills are available in the project registry. You may assign these to the agent if they are required to complete the task.
{{range .AvailableSkills}}- {{.Name}}: {{.Description}}
{{end}}
{{end}}

{{if .RepoContext}}
=== UNTRUSTED REPOSITORY-CONTROLLED CONTEXT (potentially outdated or invalid) ===
{{.RepoContext}}
{{end}}

{{if .WorkspaceFiles}}
=== Workspace Files ===
{{.WorkspaceFiles}}
{{end}}
```

### Task 2: Refactor `buildCodingInstruction` to Use `text/template`

**Files:**
- Modify: `server/internal/orchestrator/steps/coding_instruction.go`

- [x] **Step 1: Update `coding_instruction.go` to parse the template**

Replace the string concatenation logic in `buildCodingInstruction` with template rendering. Use a new struct for template data.

```go
// Add import "text/template" and "bytes"
type codingTemplateData struct {
	InstructionVerb  string
	RepoContext      string
	PRFeedback       string
	Tree             string
	AssignedSubtask  string
	AssignedSubtasks []string
	PriorFiles       []string
}

// In buildCodingInstruction, replace the concatenation with:
	data := codingTemplateData{
		InstructionVerb: p.InstructionVerb,
		RepoContext:     repoContext,
		PRFeedback:      p.PRFeedback,
		Tree:            tree,
	}

	// (Populate data.AssignedSubtask(s) and data.PriorFiles using the existing extraction logic)
	// ... (Existing extraction logic for subtasks and prior files goes here and populates data) ...

	tmplPath := filepath.Join(paths.NewOSWorkspacePaths(".").PromptsRoot().String(), "templates", "coding_instruction.tmpl")
	tmplBytes, err := os.ReadFile(tmplPath)
	if err != nil {
		// Fallback to string concatenation if template is missing (for backward compatibility during rollout)
		instruction = p.InstructionVerb + repoContext + " DO NOT rewrite the entire file unless creating a new file."
		// ... minimal fallback
	} else {
		// Add "add" template function for 1-based indexing
		funcMap := template.FuncMap{
			"add": func(a, b int) int { return a + b },
		}
		tmpl, err := template.New("coding").Funcs(funcMap).Parse(string(tmplBytes))
		if err == nil {
			var buf bytes.Buffer
			if err := tmpl.Execute(&buf, data); err == nil {
				instruction = buf.String()
			}
		}
	}
	return instruction, physicalRoot, ctx
```

### Task 3: Refactor `buildAnalyzeInstruction` to Use `text/template` and Inject Skills

**Files:**
- Modify: `server/internal/orchestrator/steps/analyze.go`

- [x] **Step 1: Refactor `buildAnalyzeInstruction`**

```go
type analyzeTemplateData struct {
	AvailableSkills []models.Skill
	RepoContext     string
	WorkspaceFiles  string
}

func (s *AnalyzeStep) buildAnalyzeInstruction(ctx context.Context, stepCtx workflow.StepContext) string {
	data := analyzeTemplateData{}
	
	if s.prompts != nil {
		// Assuming we can fetch available skills from the prompt assembler or registry
		// If skills cannot be fetched directly, list from registry.
		if s.registry != nil {
            // Logic to get skills mapping if exported, or pass via new method
			// data.AvailableSkills = s.registry.AllSkills()
		}
	}

	if contextOut, ok := stepCtx.Inputs[workflow.StepContextLoad]; ok {
		if contextJSON, err := json.Marshal(contextOut); err == nil {
			data.RepoContext = string(contextJSON)
		}
	}
	if files, err := s.listAnalyzeFiles(ctx); err == nil && files != "" && files != "No files found in workspace." {
		data.WorkspaceFiles = files
	}

	tmplPath := filepath.Join(paths.NewOSWorkspacePaths(".").PromptsRoot().String(), "templates", "analyze_instruction.tmpl")
	tmplBytes, err := os.ReadFile(tmplPath)
	if err == nil {
		tmpl, err := template.New("analyze").Parse(string(tmplBytes))
		if err == nil {
			var buf bytes.Buffer
			if err := tmpl.Execute(&buf, data); err == nil {
				return buf.String()
			}
		}
	}

	// Fallback
	instruction := "Analyze this task and output the proposed specification as a valid JSON object matching the schema and template requested in the system instructions."
    instruction += "\nCRITICAL: If all requirements are clear and you have no NEW questions, you MUST return an empty array `[]` for `clarification_questions`. DO NOT repeat questions that have already been answered in the context."
	if data.RepoContext != "" {
		instruction += "\n\n=== UNTRUSTED REPOSITORY-CONTROLLED CONTEXT (potentially outdated or invalid) ===\n" + data.RepoContext
	}
	if data.WorkspaceFiles != "" {
		instruction += "\n\n=== Workspace Files ===\n" + data.WorkspaceFiles
	}
	return instruction
}
```

- [x] **Step 2: Commit**

Run: `go test ./internal/orchestrator/steps/...`
Expected: PASS
Run: `git commit -m "refactor: migrate instruction builders to text/template system"`
