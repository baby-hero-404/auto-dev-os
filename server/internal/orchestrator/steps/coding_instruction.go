package steps

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"text/template"

	"github.com/auto-code-os/auto-code-os/server/internal/workflow"
	"github.com/auto-code-os/auto-code-os/server/pkg/models"
	"github.com/auto-code-os/auto-code-os/server/pkg/paths"
)

// codingInstructionParams parameterizes the shared instruction-building logic used by
// code_backend and code_frontend (Issue 3: these two files duplicated the same ~80 lines of
// repo-context/tree/subtask/prior-files construction, only varying by role strings).
type codingInstructionParams struct {
	Task            *models.Task
	Workspace       WorkspaceLoader
	IsEasy          bool
	Role            string // "backend" | "frontend"
	SubtaskKey      string // "backend" | "frontend"
	InstructionVerb  string // e.g. "Implement the backend changes."
	PRFeedback       string
	PreHydratedFiles string
}

type codingTemplateData struct {
	InstructionVerb  string
	RepoContext      string
	PRFeedback       string
	Tree             string
	AssignedSubtask  string
	AssignedSubtasks []string
	PriorFiles       []string
	PreHydratedFiles string
}

// buildCodingInstruction assembles the base instruction text (role framing, repo path
// conventions, PR feedback, repository structure, assigned subtask, and prior-step files)
// shared by the backend and frontend coding steps. It also attaches an AgentPathContext to
// ctx when a workspace is resolved, so the returned ctx must be used for the rest of the step.
func buildCodingInstruction(ctx context.Context, stepCtx workflow.StepContext, p codingInstructionParams) (instruction string, physicalRoot string, outCtx context.Context) {
	var pathCtx *paths.AgentPathContext
	repoContext := ""
	if p.Workspace != nil {
		if ws, _ := p.Workspace.LoadTaskWorkspace(ctx, p.Task); ws != nil {
			var useRepoPrefix bool
			var repoName string

			if len(ws.Repos) == 1 {
				repoName = ws.Repos[0].Name
				useRepoPrefix = false
				if p.IsEasy {
					physicalRoot = paths.NewOSWorkspacePaths(filepath.Dir(ws.Root)).RepoMain(p.Task.ID, repoName).String()
				} else {
					physicalRoot = paths.NewOSWorkspacePaths(filepath.Dir(ws.Root)).RepoWorktreeDir(p.Task.ID, repoName, p.Role).String()
				}
				repoContext = "\nIMPORTANT: Your workspace root IS the repository root.\nAll file paths MUST be relative (e.g., internal/model/commit.go).\nDo NOT prefix with the repository name.\nYour diff paths MUST be relative to the repository root, e.g., --- a/filepath. DO NOT include the repository name in the path."
			} else {
				useRepoPrefix = true
				physicalRoot = paths.NewOSWorkspacePaths(filepath.Dir(ws.Root)).CodeRoot(p.Task.ID).String()
				var names []string
				for _, r := range ws.Repos {
					names = append(names, r.Name)
				}
				repoContext = fmt.Sprintf(" You are working on multiple repositories: %s. Your diff paths MUST include the repository name prefix (e.g., --- a/repo-name/filepath).", strings.Join(names, ", "))
			}
			pathCtx = paths.NewAgentPathContext(physicalRoot, useRepoPrefix, repoName, p.Role)
			ctx = context.WithValue(ctx, paths.AgentPathContextKey, pathCtx)
		}
	}
	if repoContext == "" {
		repoContext = " Your diff paths MUST include the repository name prefix (e.g., --- a/repo-name/filepath)."
	}

	data := codingTemplateData{
		InstructionVerb:  p.InstructionVerb,
		RepoContext:      repoContext,
		PRFeedback:       p.PRFeedback,
		PreHydratedFiles: p.PreHydratedFiles,
	}

	// Perform repository structure scan
	var tree string
	var contextCache models.ContextCache
	haveContextCache := false
	if contextLoadOut, ok := stepCtx.Inputs[workflow.StepContextLoad]; ok {
		if cacheJSON, ok := contextLoadOut["context_cache"].(string); ok && cacheJSON != "" {
			if err := json.Unmarshal([]byte(cacheJSON), &contextCache); err == nil {
				haveContextCache = true
			}
		}
	}
	if !haveContextCache || contextCache.RepoMap == "" {
		if haveContextCache && contextCache.DirectoryTree != "" {
			tree = contextCache.DirectoryTree
		} else if physicalRoot != "" {
			if t, err := ScanDirectory(physicalRoot, 3, 200); err == nil && t != "" {
				tree = t
			}
		}
	}
	data.Tree = tree

	// Inject role-specific subtasks from Plan output
	if planOut, ok := stepCtx.Inputs[workflow.StepPlan]; ok {
		if subtasks, ok := planOut["subtasks"].(map[string]any); ok {
			if roleTasks, ok := subtasks[p.SubtaskKey].([]any); ok && len(roleTasks) > 0 {
				var taskIdx = -1
				if idx := strings.LastIndex(stepCtx.StepID, "_"); idx != -1 {
					if parsedIdx, err := strconv.Atoi(stepCtx.StepID[idx+1:]); err == nil {
						taskIdx = parsedIdx
					}
				}

				if taskIdx >= 0 && taskIdx < len(roleTasks) {
					data.AssignedSubtask = fmt.Sprintf("%v", roleTasks[taskIdx])
				} else {
					for _, t := range roleTasks {
						data.AssignedSubtasks = append(data.AssignedSubtasks, fmt.Sprintf("%v", t))
					}
				}
			}
		}
	}

	// Inject files created/modified by prior steps
	var priorFiles []string
	seenPriorFiles := make(map[string]bool)
	for inputStepID, stepOut := range stepCtx.Inputs {
		if strings.HasPrefix(inputStepID, workflow.StepCodeBackend) || strings.HasPrefix(inputStepID, workflow.StepCodeFrontend) {
			if fc, ok := stepOut["files_changed"]; ok {
				var filesList []string
				if fl, ok := fc.([]any); ok {
					for _, f := range fl {
						if str, ok := f.(string); ok {
							filesList = append(filesList, str)
						}
					}
				} else if fl, ok := fc.([]string); ok {
					filesList = fl
				}
				for _, f := range filesList {
					if !seenPriorFiles[f] {
						seenPriorFiles[f] = true
						priorFiles = append(priorFiles, f)
					}
				}
			}
		}
	}
	data.PriorFiles = priorFiles

	tmplPath := filepath.Join("internal", "prompts", "templates", "coding_instruction.tmpl")
	tmplBytes, err := os.ReadFile(tmplPath)
	if err != nil {
		// Fallback for unit tests running in internal/orchestrator/steps
		tmplPath = filepath.Join("..", "..", "prompts", "templates", "coding_instruction.tmpl")
		tmplBytes, err = os.ReadFile(tmplPath)
	}
	if err != nil {
		// Fallback if template is still missing
		instruction = p.InstructionVerb + repoContext + " DO NOT rewrite the entire file unless creating a new file."
	} else {
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
	if instruction == "" {
		instruction = p.InstructionVerb + repoContext + " DO NOT rewrite the entire file unless creating a new file."
	}

	return instruction, physicalRoot, ctx
}

func buildPreHydratedContext(ctx context.Context, task *models.Task, fileReader AffectedFileReader, frozenCtx *models.FrozenContext) string {
	if frozenCtx == nil || fileReader == nil {
		return ""
	}
	var sb strings.Builder
	tokens := 0
	const maxTokens = 4000 // roughly 16000 chars

	for _, af := range frozenCtx.AffectedFiles {
		content, ok := fileReader.ReadAffectedFileContent(ctx, task, af.File)
		if !ok || content == "" {
			continue
		}
		
		lines := strings.Split(content, "\n")
		if len(lines) > 200 {
			lines = lines[:200]
			lines = append(lines, "... (file truncated due to length)")
			content = strings.Join(lines, "\n")
		}

		estTokens := len(content) / 4
		if tokens+estTokens > maxTokens {
			break
		}
		
		if sb.Len() == 0 {
			sb.WriteString("\n\n## Pre-Read Files (do NOT re-read these)\n")
		}
		sb.WriteString(fmt.Sprintf("\n### %s\n```\n%s\n```\n", af.File, content))
		tokens += estTokens
	}
	return sb.String()
}
