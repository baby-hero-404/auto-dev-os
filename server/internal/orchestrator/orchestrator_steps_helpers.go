package orchestrator

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/auto-code-os/auto-code-os/server/internal/sandbox"
	"github.com/auto-code-os/auto-code-os/server/internal/workflow"
	"github.com/auto-code-os/auto-code-os/server/pkg/llm"
	"github.com/auto-code-os/auto-code-os/server/pkg/models"
	"go.opentelemetry.io/otel"
)

func (o *Orchestrator) getSuccessfulCheckpoint(ctx context.Context, taskID string, step string) (map[string]any, bool) {
	checkpoints, err := o.workflows.ListCheckpoints(ctx, taskID)
	if err != nil {
		return nil, false
	}
	var latestSuccess *models.WorkflowCheckpoint
	for i := len(checkpoints) - 1; i >= 0; i-- {
		cp := checkpoints[i]
		if cp.Step == step {
			var state map[string]any
			if err := json.Unmarshal(cp.State, &state); err == nil {
				if state["status"] == "success" {
					latestSuccess = &cp
					break
				}
			}
		}
	}
	if latestSuccess != nil {
		var state map[string]any
		_ = json.Unmarshal(latestSuccess.State, &state)
		if out, ok := state["output"].(map[string]any); ok {
			return out, true
		}
		return map[string]any{}, true
	}
	return nil, false
}

func (o *Orchestrator) getSavedPatch(ctx context.Context, taskID string, step string) (string, error) {
	if o.artifacts == nil {
		return "", fmt.Errorf("artifacts repository is not configured")
	}
	arts, err := o.artifacts.ListByTaskID(ctx, taskID)
	if err != nil {
		return "", err
	}
	var latestPatch *models.WorkflowArtifact
	for i := len(arts) - 1; i >= 0; i-- {
		art := arts[i]
		if art.Step == step && art.Type == "patch" {
			latestPatch = &art
			break
		}
	}
	if latestPatch == nil {
		return "", fmt.Errorf("no patch artifact found for step %s", step)
	}
	var patch string
	if err := json.Unmarshal(latestPatch.Payload, &patch); err == nil {
		return patch, nil
	}
	return string(latestPatch.Payload), nil
}

func (o *Orchestrator) withCheckpointRecovery(task *models.Task, agent *models.Agent, stepID string, runner workflow.StepFunc) workflow.StepFunc {
	return func(ctx context.Context, sc workflow.StepContext) (map[string]any, error) {
		if stepID != workflow.StepAnalyze {
			if output, exists := o.getSuccessfulCheckpoint(ctx, task.ID, stepID); exists {
				o.log(ctx, task.ID, nil, "info", fmt.Sprintf("step %s: resuming from previous successful checkpoint", stepID))

				if stepID == workflow.StepCodeBackend || stepID == workflow.StepCodeFrontend || stepID == workflow.StepFix {
					if patch, err := o.getSavedPatch(ctx, task.ID, stepID); err == nil && patch != "" {
						o.log(ctx, task.ID, nil, "info", fmt.Sprintf("step %s: re-applying saved patch to workspace", stepID))

						worktreeSuffix := ""
						if stepID == workflow.StepCodeBackend {
							worktreeSuffix = "-be-worktree"
						} else if stepID == workflow.StepCodeFrontend {
							worktreeSuffix = "-fe-worktree"
						}

						if applyErr := o.applyPatch(ctx, task, agent, stepID, patch, worktreeSuffix); applyErr != nil {
							o.log(ctx, task.ID, nil, "warn", fmt.Sprintf("step %s: failed to re-apply patch (%v), rerunning step", stepID, applyErr))
							return runner(ctx, sc)
						}
					}
				}

				switch stepID {
				case workflow.StepPlan:
					_, _ = o.updateTaskStatus(ctx, task.ID, models.TaskStatusCoding)
				case workflow.StepMerge:
					_, _ = o.updateTaskStatus(ctx, task.ID, models.TaskStatusReviewing)
				case workflow.StepReview:
					nextStatus := models.TaskStatusTesting
					if parsed, ok := output["parsed"].(map[string]any); ok {
						if findings, exists := parsed["findings"]; exists {
							if slice, ok := findings.([]any); ok && len(slice) > 0 {
								nextStatus = models.TaskStatusFixing
							}
						}
					}
					_, _ = o.updateTaskStatus(ctx, task.ID, nextStatus)
				case workflow.StepFix:
					_, _ = o.updateTaskStatus(ctx, task.ID, models.TaskStatusReviewing)
				case workflow.StepTest:
					_, _ = o.updateTaskStatus(ctx, task.ID, models.TaskStatusTesting)
				case workflow.StepPR:
					_, _ = o.updateTaskStatus(ctx, task.ID, models.TaskStatusHumanReview)
				}

				return output, nil
			}
		}
		return runner(ctx, sc)
	}
}

func (o *Orchestrator) runLLMStep(ctx context.Context, task *models.Task, agent *models.Agent, jobID, stepID, instruction string) (map[string]any, error) {
	if o.llm == nil {
		return nil, fmt.Errorf("llm provider is not configured")
	}
	var messages []llm.Message
	var err error
	if o.prompts != nil {
		messages, _, err = o.prompts.AssembleForAgent(ctx, *task, agent, nil)
		if err != nil {
			return nil, err
		}
	} else {
		messages = []llm.Message{{Role: "user", Content: task.Title + "\n\n" + task.Description}}
	}
	fullInstruction := instruction

	var analysis models.TaskAnalysis
	if len(task.Analysis) > 0 {
		_ = json.Unmarshal(task.Analysis, &analysis)
	}
	if len(analysis.AffectedFiles) > 0 && (stepID == workflow.StepCodeBackend || stepID == workflow.StepCodeFrontend || stepID == workflow.StepFix || stepID == workflow.StepReview) {
		var b strings.Builder
		b.WriteString("\n\n### Workspace Affected Files ###\n")
		localPath := sandbox.WorkspacePath(o.workspaceRoot, task.ID)
		for _, file := range analysis.AffectedFiles {
			content, err := os.ReadFile(filepath.Join(localPath, file))
			if err == nil {
				b.WriteString(fmt.Sprintf("\n--- %s ---\n```\n%s\n```\n", file, string(content)))
			}
		}
		fullInstruction += b.String()
	}

	if stepID == workflow.StepCodeBackend || stepID == workflow.StepCodeFrontend || stepID == workflow.StepFix || stepID == workflow.StepPlan || stepID == workflow.StepAnalyze {
		fullInstruction += "\n\nCRITICAL REQUIREMENT: Do NOT output any tool calls, function calls, or markdown block thoughts. You do NOT have tool execution capabilities in this single-shot step. You MUST output ONLY a valid JSON object matching the requested format directly (or inside a ```json ``` block)."
	}
	if stepID == workflow.StepCodeBackend || stepID == workflow.StepCodeFrontend || stepID == workflow.StepFix {
		fullInstruction += "\n\nCRITICAL REQUIREMENT: The patch/diff field MUST contain a valid Unified Git Diff (starting with 'diff --git') representing all source code changes. Do NOT output raw file contents. Do NOT include any text outside the JSON structure."
	}
	messages = append(messages, llm.Message{Role: "user", Content: "Workflow step: " + stepID + "\n\n" + fullInstruction})

	// Save prompt artifact
	_ = o.saveArtifact(ctx, jobID, task.ID, stepID, "prompt", messages)

	routeName := agent.ModelLevelGroup
	if o.projects != nil {
		if p, err := o.projects.GetByID(ctx, task.ProjectID); err == nil {
			if agent.Role == models.AgentRolePlanner && p.DefaultModelLevel != "" {
				// Spec: DefaultModelLevel is specifically the default for the Planner agent
				routeName = p.DefaultModelLevel
			} else if (routeName == "" || routeName == "default") && p.DefaultModelLevel != "" {
				// For other agents, act as fallback
				routeName = p.DefaultModelLevel
			}
		}
	}

	ctx = llm.WithRouteOptions(ctx, llm.RouteOptions{
		Complexity: task.Complexity,
		OrgID:      agent.OrgID,
		ProjectID:  task.ProjectID,
		AgentID:    agent.ID,
		TaskID:     task.ID,
		RouteName:  routeName,
	})
	resp, err := o.llm.Chat(ctx, messages)
	if err != nil {
		return nil, fmt.Errorf("llm call failed: %w", err)
	}
	o.log(ctx, task.ID, nil, "info", fmt.Sprintf("%s: llm response from %s", stepID, resp.Model))
	var parsed map[string]any
	if parsedJSON, err := parseJSONMarkdown(resp.Content); err == nil {
		parsed = parsedJSON
	} else {
		parsed = map[string]any{"raw_content": resp.Content}
	}

	// Save llm_response artifact
	_ = o.saveArtifact(ctx, jobID, task.ID, stepID, "llm_response", parsed)

	o.writeLLMCallTrace(ctx, task, agent, stepID, messages, resp, parsed)

	return map[string]any{
		"status":        "llm_completed",
		"model":         resp.Model,
		"content":       resp.Content,
		"parsed":        parsed,
		"prompt_tokens": resp.PromptTokens,
		"output_tokens": resp.OutputTokens,
	}, nil
}

func (o *Orchestrator) runSandboxStep(ctx context.Context, task *models.Task, agent *models.Agent, stepID, command string) (map[string]any, error) {
	ctx, span := otel.Tracer("auto-code-os/orchestrator").Start(ctx, "orchestrator.sandbox_step")
	defer span.End()
	result, err := o.runtime.Run(ctx, sandbox.CommandRequest{
		TaskID:      task.ID,
		AgentID:     agent.ID,
		Command:     []string{"bash", "-lc", command},
		NetworkMode: sandbox.NetworkModeNone,
		Timeout:     5 * time.Minute,
	})
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(result.Stdout) != "" {
		o.log(ctx, task.ID, nil, "info", fmt.Sprintf("%s: %s", stepID, strings.TrimSpace(result.Stdout)))
	}
	if strings.TrimSpace(result.Stderr) != "" {
		o.log(ctx, task.ID, nil, "warn", fmt.Sprintf("%s: %s", stepID, strings.TrimSpace(result.Stderr)))
	}
	if result.ExitCode != 0 {
		return nil, fmt.Errorf("%s failed with exit code %d", stepID, result.ExitCode)
	}
	return map[string]any{"status": "ok", "stdout": result.Stdout}, nil
}

func deriveWorkflowAnalysis(task *models.Task) models.TaskAnalysis {
	text := strings.ToLower(task.Title + " " + task.Description)
	complexity := task.Complexity
	if complexity == "" {
		complexity = models.TaskComplexityEasy
	}
	hardSignals := []string{"architecture", "security", "auth", "permission", "rbac", "payment", "migration", "distributed"}
	mediumSignals := []string{"feature", "refactor", "api", "database", "ui", "workflow", "integration"}
	for _, signal := range hardSignals {
		if strings.Contains(text, signal) {
			complexity = models.TaskComplexityHard
			break
		}
	}
	if complexity != models.TaskComplexityHard {
		for _, signal := range mediumSignals {
			if strings.Contains(text, signal) {
				complexity = models.TaskComplexityMedium
				break
			}
		}
	}
	questions := []string{}
	if len(strings.TrimSpace(task.Description)) < 30 {
		questions = append(questions, "Please provide more implementation context, affected module names, and expected behavior.")
	}
	return models.TaskAnalysis{
		Complexity:    complexity,
		Scope:         "Generated by the Phase 3b workflow analyze step.",
		AffectedFiles: []string{},
		Risks:         []string{"Workflow uses deterministic planning until full LLM step execution is enabled."},
		ExecutionPlan: []string{
			"Assemble prompt with role, rules, and retrieved context.",
			"Decompose work into typed subtasks.",
			"Run backend and frontend coding tracks in parallel sandboxes.",
			"Merge, review, fix, test, and prepare PR approval checkpoint.",
		},
		ClarificationQuestions: questions,
	}
}

func parseJSONMarkdown(content string) (map[string]any, error) {
	trimmed := strings.TrimSpace(content)
	if strings.HasPrefix(trimmed, "```") {
		lines := strings.Split(trimmed, "\n")
		if len(lines) >= 2 {
			if strings.HasPrefix(lines[0], "```") {
				lines = lines[1:]
			}
			if strings.HasSuffix(lines[len(lines)-1], "```") {
				lines = lines[:len(lines)-1]
			}
			trimmed = strings.TrimSpace(strings.Join(lines, "\n"))
		}
	}
	var res map[string]any
	if err := json.Unmarshal([]byte(trimmed), &res); err != nil {
		start := strings.Index(trimmed, "{")
		end := strings.LastIndex(trimmed, "}")
		if start != -1 && end != -1 && end > start {
			trimmed = trimmed[start : end+1]
			if err := json.Unmarshal([]byte(trimmed), &res); err == nil {
				return res, nil
			}
		}
		return nil, err
	}
	return res, nil
}

func (o *Orchestrator) applyPatch(ctx context.Context, task *models.Task, agent *models.Agent, stepID string, patchText string, worktreeSuffix string) error {
	if patchText == "" {
		return nil
	}

	// Scan lines of patch to extract modified files
	lines := strings.Split(patchText, "\n")
	var modifiedFiles []string
	for _, line := range lines {
		if strings.HasPrefix(line, "+++ b/") {
			file := strings.TrimPrefix(line, "+++ b/")
			file = strings.TrimSpace(file)
			if file != "/dev/null" {
				modifiedFiles = append(modifiedFiles, file)
			}
		} else if strings.HasPrefix(line, "--- a/") {
			file := strings.TrimPrefix(line, "--- a/")
			file = strings.TrimSpace(file)
			if file != "/dev/null" {
				modifiedFiles = append(modifiedFiles, file)
			}
		}
	}

	// Enforce affected files if specified
	if task.Analysis != nil {
		var analysis models.TaskAnalysis
		if err := json.Unmarshal(task.Analysis, &analysis); err == nil && len(analysis.AffectedFiles) > 0 {
			// Validate all files modified in the patch against the allowed pattern list
			for _, file := range modifiedFiles {
				isAllowed := false
				for _, pattern := range analysis.AffectedFiles {
					if matchAffectedFile(pattern, file) {
						isAllowed = true
						break
					}
				}
				if !isAllowed {
					return fmt.Errorf("security violation: patch attempts to modify file %q which is not in the approved affected_files spec %v", file, analysis.AffectedFiles)
				}
			}
		}
	}

	localPath := sandbox.WorkspacePath(o.workspaceRoot, task.ID)

	if task.RepositoryID != nil {
		// Single repo
		targetPath := o.hostWorktreePath(task, localPath, worktreeSuffix)
		fullPath := filepath.Join(targetPath, "patch.diff")
		if err := os.MkdirAll(filepath.Dir(fullPath), 0o755); err != nil {
			return err
		}
		if err := os.WriteFile(fullPath, []byte(patchText), 0o644); err != nil {
			return err
		}

		containerTargetPath := o.containerPathForHostPath(task, targetPath, worktreeSuffix)
		containerPatchPath := filepath.Join(containerTargetPath, "patch.diff")

		cmd := fmt.Sprintf("git -C %[1]s apply --recount --whitespace=nowarn %[2]s || patch -d %[1]s -p1 < %[2]s || patch -d %[1]s -p0 < %[2]s",
			quoteShellArg(containerTargetPath),
			quoteShellArg(containerPatchPath),
		)
		_, err := o.runSandboxStepInWorktree(ctx, task, agent, stepID+"_apply_patch", cmd, worktreeSuffix)
		if err != nil {
			return fmt.Errorf("git apply patch: %w", err)
		}
		_, _ = o.runSandboxStepInWorktree(ctx, task, agent, stepID+"_clean_patch", fmt.Sprintf("rm %s", quoteShellArg(containerPatchPath)), worktreeSuffix)
		return nil
	}

	// Multi-repo: split patch by repository
	repoPatches := splitPatchByRepo(patchText)
	for repoName, repoPatchText := range repoPatches {
		repoHostPath := filepath.Join(localPath, repoName)
		repoWorktreeHostPath := o.hostWorktreePath(task, repoHostPath, worktreeSuffix)

		fullPath := filepath.Join(repoWorktreeHostPath, "patch.diff")
		if err := os.MkdirAll(filepath.Dir(fullPath), 0o755); err != nil {
			return err
		}
		if err := os.WriteFile(fullPath, []byte(repoPatchText), 0o644); err != nil {
			return err
		}

		containerRepoWorktreePath := o.containerPathForHostPath(task, repoWorktreeHostPath, worktreeSuffix)
		containerPatchPath := filepath.Join(containerRepoWorktreePath, "patch.diff")

		// Use -p2 because splitPatchByRepo keeps a/repoName/ prefix, which needs to be stripped relative to repoWorktree root.
		cmd := fmt.Sprintf("git -C %[1]s apply -p2 --recount --whitespace=nowarn %[2]s || patch -d %[1]s -p2 < %[2]s",
			quoteShellArg(containerRepoWorktreePath),
			quoteShellArg(containerPatchPath),
		)
		_, err := o.runSandboxStepInWorktree(ctx, task, agent, stepID+"_apply_patch_"+repoName, cmd, worktreeSuffix)
		if err != nil {
			return fmt.Errorf("git apply patch failed for repo %s: %w", repoName, err)
		}
		_, _ = o.runSandboxStepInWorktree(ctx, task, agent, stepID+"_clean_patch_"+repoName, fmt.Sprintf("rm %s", quoteShellArg(containerPatchPath)), worktreeSuffix)
	}
	return nil
}

func (o *Orchestrator) captureWorkspaceDiff(ctx context.Context, task *models.Task, agent *models.Agent, stepID string, worktreeSuffix string) (string, error) {
	localPath := sandbox.WorkspacePath(o.workspaceRoot, task.ID)
	targetPath := o.hostWorktreePath(task, localPath, worktreeSuffix)
	containerTargetPath := o.containerPathForHostPath(task, targetPath, worktreeSuffix)

	if task.RepositoryID != nil {
		out, err := o.runSandboxStepInWorktree(ctx, task, agent, stepID+"_git_diff", fmt.Sprintf("git -C %s diff", quoteShellArg(containerTargetPath)), worktreeSuffix)
		if err != nil {
			return "", fmt.Errorf("git diff failed: %w", err)
		}
		diffText, _ := out["stdout"].(string)
		return diffText, nil
	}

	// Multi-repo diff
	var pattern string
	if worktreeSuffix != "" {
		pattern = fmt.Sprintf("*%s", worktreeSuffix)
	} else {
		pattern = "*"
	}
	out, err := o.runSandboxStepInWorktree(ctx, task, agent, stepID+"_git_diff_multi", fmt.Sprintf(`
		DIFF_OUT=""
		for d in %[1]s/%[2]s/ ; do
			if [ -d "$d/.git" ]; then
				pushd "$d" > /dev/null
				REPO_DIFF=$(git diff)
				if [ -n "$REPO_DIFF" ]; then
					d_name=$(basename "$d")
					repo_display="${d_name%%%[3]s}"
					DIFF_OUT="${DIFF_OUT}--- Repository: ${repo_display}\n${REPO_DIFF}\n\n"
				fi
				popd > /dev/null
			fi
		done
		echo -e "$DIFF_OUT"
	`, quoteShellArg(containerTargetPath), pattern, worktreeSuffix), worktreeSuffix)
	if err != nil {
		return "", fmt.Errorf("multi-repo git diff failed: %w", err)
	}
	diffText, _ := out["stdout"].(string)
	return diffText, nil
}

func (o *Orchestrator) saveArtifact(ctx context.Context, jobID string, taskID string, step string, artType string, payload any) error {
	if o.artifacts == nil {
		return nil
	}
	raw, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	artifact := &models.WorkflowArtifact{
		JobID:   jobID,
		TaskID:  taskID,
		Step:    step,
		Type:    artType,
		Payload: raw,
	}
	return o.artifacts.Create(ctx, artifact)
}

func extractPatch(parsed map[string]any) string {
	if parsed == nil {
		return ""
	}
	var p string
	if v, ok := parsed["patch"].(string); ok && v != "" {
		p = v
	} else if v, ok := parsed["patch_text"].(string); ok && v != "" {
		p = v
	} else if v, ok := parsed["diff"].(string); ok && v != "" {
		p = v
	}
	if p == "" {
		return ""
	}
	p = strings.TrimSpace(p)
	if strings.HasPrefix(p, "```") {
		lines := strings.Split(p, "\n")
		if len(lines) >= 2 {
			endIdx := len(lines) - 1
			for i := len(lines) - 1; i > 0; i-- {
				if strings.HasPrefix(strings.TrimSpace(lines[i]), "```") {
					endIdx = i
					break
				}
			}
			p = strings.Join(lines[1:endIdx], "\n")
		}
	}
	return strings.TrimSpace(p) + "\n"
}

func deriveChangeName(task *models.Task) string {
	slug := strings.ToLower(task.Title)
	reg := regexp.MustCompile(`[^a-z0-9]+`)
	slug = reg.ReplaceAllString(slug, "-")
	slug = strings.Trim(slug, "-")
	if len(slug) > 30 {
		slug = slug[:30]
	}
	slug = strings.Trim(slug, "-")
	if slug == "" {
		slug = "task-" + task.ID
		if len(slug) > 13 {
			slug = slug[:13]
		}
	}
	return slug
}

func taskReadyForExecution(task *models.Task) bool {
	switch task.SpecStatus {
	case models.TaskSpecStatusApproved, models.TaskSpecStatusAutoApproved:
		return true
	default:
		return false
	}
}

func matchAffectedFile(pattern, file string) bool {
	pattern = strings.TrimSpace(pattern)
	file = strings.TrimSpace(file)
	if pattern == "" || file == "" {
		return false
	}

	// 1. Exact or Clean match
	if pattern == file || filepath.Clean(pattern) == filepath.Clean(file) {
		return true
	}

	// 2. Glob match
	if strings.ContainsAny(pattern, "*?[]") {
		if matched, err := filepath.Match(pattern, file); err == nil && matched {
			return true
		}
	}

	// If the pattern has no spaces, it is considered a specific filename or path,
	// so we don't apply loose description-based heuristics below.
	if !strings.Contains(pattern, " ") {
		return false
	}

	lowerPattern := strings.ToLower(pattern)

	// 3. Documentation file heuristic
	if strings.Contains(lowerPattern, "documentation") {
		ext := strings.ToLower(filepath.Ext(file))
		if ext == ".md" || ext == ".txt" || ext == ".rst" || strings.Contains(strings.ToLower(file), "/docs/") || strings.HasPrefix(strings.ToLower(file), "docs/") {
			return true
		}
	}

	// 4. Script file heuristic
	if strings.Contains(lowerPattern, "script") {
		ext := strings.ToLower(filepath.Ext(file))
		if ext == ".sh" || ext == ".py" || ext == ".bash" || ext == ".bat" || strings.Contains(strings.ToLower(file), "/scripts/") || strings.HasPrefix(strings.ToLower(file), "scripts/") {
			return true
		}
	}

	// 5. Extract and check extensions (e.g. "(.go)", "GoLang source files (.go)")
	extRegex := regexp.MustCompile(`\.([a-zA-Z0-9]+)`)
	exts := extRegex.FindAllString(pattern, -1)
	fileExt := filepath.Ext(file)
	for _, ext := range exts {
		cleanExt := strings.TrimRight(ext, " )]}.,;")
		if strings.EqualFold(cleanExt, fileExt) {
			return true
		}
	}

	// 6. Catch-all descriptive patterns
	if strings.Contains(lowerPattern, "all relevant source files") ||
		strings.Contains(lowerPattern, "all source files") ||
		strings.Contains(lowerPattern, "any file") ||
		strings.Contains(lowerPattern, "project files") {
		return true
	}

	// 7. Check if the filename itself is explicitly mentioned in the description
	baseName := filepath.Base(file)
	return strings.Contains(lowerPattern, strings.ToLower(baseName))
}

func (o *Orchestrator) runTargetedTests(ctx context.Context, task *models.Task, agent *models.Agent, jobID, stepName string, changedFiles []string, worktreeSuffix string) (map[string]any, error) {
	if len(changedFiles) == 0 {
		return map[string]any{"status": "skipped", "reason": "no changed files"}, nil
	}

	localPath := sandbox.WorkspacePath(o.workspaceRoot, task.ID)

	// Determine the host directory that is mounted as /workspace
	mountedHostPath := localPath
	if worktreeSuffix != "" && task.RepositoryID != nil {
		mountedHostPath = localPath + worktreeSuffix
	}

	// Group changed files by their nearest module directory (relative to mountedHostPath)
	type moduleGroup struct {
		isGo       bool
		isJS       bool
		files      []string
		goPackages map[string]bool
	}

	groups := make(map[string]*moduleGroup)

	for _, file := range changedFiles {
		ext := filepath.Ext(file)
		var markers []string
		isGo := false
		isJS := false
		if ext == ".go" {
			markers = []string{"go.mod"}
			isGo = true
		} else if ext == ".ts" || ext == ".tsx" || ext == ".js" || ext == ".jsx" {
			markers = []string{"package.json"}
			isJS = true
		} else {
			if _, err := os.Stat(filepath.Join(mountedHostPath, filepath.Dir(file), "go.mod")); err == nil {
				markers = []string{"go.mod"}
				isGo = true
			} else if _, err := os.Stat(filepath.Join(mountedHostPath, filepath.Dir(file), "package.json")); err == nil {
				markers = []string{"package.json"}
				isJS = true
			}
		}

		modRelDir := ""
		relFile := file
		if len(markers) > 0 {
			if dir, rf, found := findModuleDir(mountedHostPath, file, markers); found {
				modRelDir = dir
				relFile = rf
			}
		}

		g, ok := groups[modRelDir]
		if !ok {
			g = &moduleGroup{
				goPackages: make(map[string]bool),
			}
			groups[modRelDir] = g
		}
		if isGo {
			g.isGo = true
			dir := filepath.Dir(relFile)
			if dir == "." {
				g.goPackages["."] = true
			} else {
				g.goPackages["./"+dir] = true
			}
		} else if isJS {
			g.isJS = true
			g.files = append(g.files, relFile)
		}
	}

	var testErrors []string
	var testResults []map[string]any

	for modRelDir, g := range groups {
		var cmd string
		var detectedType string

		containerModPath := "/workspace"
		if modRelDir != "" {
			containerModPath = filepath.Join("/workspace", modRelDir)
		}

		if g.isGo {
			detectedType = "go"
			pkgs := []string{}
			for pkg := range g.goPackages {
				pkgs = append(pkgs, pkg+"/...")
			}
			cmd = fmt.Sprintf("cd %s && go test -v %s", quoteShellArg(containerModPath), strings.Join(pkgs, " "))
		} else if g.isJS {
			detectedType = "javascript"
			var quotedFiles []string
			for _, f := range g.files {
				quotedFiles = append(quotedFiles, quoteShellArg(f))
			}
			cmd = fmt.Sprintf("cd %s && (npm test -- --findRelatedTests %s || npm test -- %s || npm test)", quoteShellArg(containerModPath), strings.Join(quotedFiles, " "), strings.Join(quotedFiles, " "))
		} else {
			absModPath := filepath.Join(mountedHostPath, modRelDir)
			if _, err := os.Stat(filepath.Join(absModPath, "go.mod")); err == nil {
				cmd = fmt.Sprintf("cd %s && go test ./...", quoteShellArg(containerModPath))
				detectedType = "go"
			} else if _, err := os.Stat(filepath.Join(absModPath, "package.json")); err == nil {
				cmd = fmt.Sprintf("cd %s && npm test", quoteShellArg(containerModPath))
				detectedType = "javascript"
			} else {
				continue
			}
		}

		o.log(ctx, task.ID, &jobID, "info", fmt.Sprintf("running targeted tests for %s in %s: %s", detectedType, containerModPath, cmd))

		out, err := o.runSandboxStepInWorktree(ctx, task, agent, stepName, cmd, worktreeSuffix)
		if err != nil {
			o.log(ctx, task.ID, &jobID, "warn", fmt.Sprintf("targeted tests execution failed in %s: %v", containerModPath, err))
			_ = o.saveArtifact(ctx, jobID, task.ID, stepName, "targeted_test", map[string]any{
				"status":  "failed",
				"error":   err.Error(),
				"command": cmd,
				"type":    detectedType,
				"module":  modRelDir,
			})
			testErrors = append(testErrors, fmt.Sprintf("module %s: %v", modRelDir, err))
		} else {
			stdout, _ := out["stdout"].(string)
			result := map[string]any{
				"status":  "passed",
				"stdout":  stdout,
				"command": cmd,
				"type":    detectedType,
				"module":  modRelDir,
			}
			_ = o.saveArtifact(ctx, jobID, task.ID, stepName, "targeted_test", result)
			testResults = append(testResults, result)
		}
	}

	if len(testErrors) > 0 {
		return nil, fmt.Errorf("targeted tests failed: %s", strings.Join(testErrors, "; "))
	}

	if len(testResults) == 0 {
		return map[string]any{"status": "skipped", "reason": "no tests ran"}, nil
	}

	return map[string]any{
		"status": "passed",
		"info":   fmt.Sprintf("%d test suites passed", len(testResults)),
	}, nil
}

func (o *Orchestrator) runSandboxStepInWorktree(ctx context.Context, task *models.Task, agent *models.Agent, stepID, command string, worktreeSuffix string) (map[string]any, error) {
	localPath := sandbox.WorkspacePath(o.workspaceRoot, task.ID)
	hostWorkspacePath := localPath
	if worktreeSuffix != "" {
		hostWorkspacePath = o.hostWorktreePath(task, localPath, worktreeSuffix)
	}

	ctx, span := otel.Tracer("auto-code-os/orchestrator").Start(ctx, "orchestrator.sandbox_step")
	defer span.End()
	result, err := o.runtime.Run(ctx, sandbox.CommandRequest{
		TaskID:      task.ID,
		AgentID:     agent.ID,
		Workspace:   hostWorkspacePath,
		Command:     []string{"bash", "-lc", command},
		NetworkMode: sandbox.NetworkModeNone,
		Timeout:     5 * time.Minute,
	})
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(result.Stdout) != "" {
		o.log(ctx, task.ID, nil, "info", fmt.Sprintf("%s: %s", stepID, strings.TrimSpace(result.Stdout)))
	}
	if strings.TrimSpace(result.Stderr) != "" {
		o.log(ctx, task.ID, nil, "warn", fmt.Sprintf("%s: %s", stepID, strings.TrimSpace(result.Stderr)))
	}
	if result.ExitCode != 0 {
		return nil, fmt.Errorf("%s failed with exit code %d", stepID, result.ExitCode)
	}
	return map[string]any{"status": "ok", "stdout": result.Stdout}, nil
}

func (o *Orchestrator) containerPathForHostPath(task *models.Task, hostPath string, worktreeSuffix string) string {
	localPath := sandbox.WorkspacePath(o.workspaceRoot, task.ID)
	activeWorkspaceHostPath := localPath

	if worktreeSuffix != "" {
		activeWorkspaceHostPath = o.hostWorktreePath(task, localPath, worktreeSuffix)
	}

	rel, err := filepath.Rel(activeWorkspaceHostPath, hostPath)
	if err == nil && !strings.HasPrefix(rel, "..") {
		if rel == "." {
			return "/workspace"
		}
		return filepath.Join("/workspace", rel)
	}

	relMain, errMain := filepath.Rel(localPath, hostPath)
	if errMain == nil && !strings.HasPrefix(relMain, "..") {
		if relMain == "." {
			return "/workspace"
		}
		return filepath.Join("/workspace", relMain)
	}

	return "/workspace"
}

func quoteShellArg(s string) string {
	return "'" + strings.ReplaceAll(s, "'", "'\\''") + "'"
}

func readLimitedFile(path string, maxBytes int64) (string, error) {
	file, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer file.Close()

	stat, err := file.Stat()
	if err != nil {
		return "", err
	}

	limit := maxBytes
	if stat.Size() < limit {
		limit = stat.Size()
	}

	buf := make([]byte, limit)
	n, errRead := file.Read(buf)
	if errRead != nil && n == 0 {
		return "", errRead
	}

	content := string(buf[:n])
	if stat.Size() > maxBytes {
		content += "\n[TRUNCATED: file exceeded size limit]"
	}
	return content, nil
}

func findModuleDir(targetPath string, relFilePath string, markers []string) (string, string, bool) {
	absStart := filepath.Join(targetPath, relFilePath)
	dir := filepath.Dir(absStart)

	for {
		for _, marker := range markers {
			markerPath := filepath.Join(dir, marker)
			if stat, err := os.Stat(markerPath); err == nil && !stat.IsDir() {
				relDir, errRel := filepath.Rel(targetPath, dir)
				if errRel == nil {
					relFile, errFile := filepath.Rel(dir, absStart)
					if errFile == nil {
						return relDir, relFile, true
					}
				}
			}
		}
		if dir == targetPath || dir == filepath.Dir(dir) {
			break
		}
		dir = filepath.Dir(dir)
	}
	return "", "", false
}

func (o *Orchestrator) getChangedFiles(ctx context.Context, task *models.Task, agent *models.Agent, targetPath string, worktreeSuffix string) ([]string, error) {
	var repos []models.Repository
	var err error
	if o.repositories != nil {
		repos, err = o.repositories.ListByProjectID(ctx, task.ProjectID)
	}
	if o.repositories == nil || err != nil || len(repos) == 0 {
		containerTargetPath := o.containerPathForHostPath(task, targetPath, worktreeSuffix)
		out, err := o.runSandboxStepInWorktree(ctx, task, agent, "git_diff_names", fmt.Sprintf("git -C %s diff --name-only", quoteShellArg(containerTargetPath)), worktreeSuffix)
		if err != nil {
			return nil, err
		}
		stdout, _ := out["stdout"].(string)
		if stdout == "" {
			return nil, nil
		}
		return strings.Split(strings.TrimSpace(stdout), "\n"), nil
	}

	var targetRepos []models.Repository
	if task.RepositoryID != nil {
		for _, r := range repos {
			if r.ID == *task.RepositoryID {
				targetRepos = append(targetRepos, r)
				break
			}
		}
	} else {
		targetRepos = repos
	}

	ws, errWS := o.LoadTaskWorkspace(ctx, task)

	var allChanged []string
	for _, repo := range targetRepos {
		localRepoPath := targetPath
		prefix := ""
		if errWS == nil {
			for i := range ws.Repos {
				if ws.Repos[i].RepoID == repo.ID {
					if worktreeSuffix != "" {
						role := getRoleFromSuffix(worktreeSuffix)
						if relPath, exists := ws.Repos[i].Paths.Worktrees[role]; exists && relPath != "" {
							localRepoPath = filepath.Join(ws.Root, relPath)
						} else {
							localRepoPath = filepath.Join(ws.Root, ws.Repos[i].Paths.Main)
						}
					} else {
						localRepoPath = filepath.Join(ws.Root, ws.Repos[i].Paths.Main)
					}
					if task.RepositoryID == nil {
						prefix = ws.Repos[i].Name + "/"
					}
					break
				}
			}
		} else if task.RepositoryID == nil {
			parts := strings.Split(repo.URL, "/")
			repoName := parts[len(parts)-1]
			repoName = strings.TrimSuffix(repoName, ".git")
			localRepoPath = filepath.Join(targetPath, repoName)
			prefix = repoName + "/"
		}

		containerRepoPath := o.containerPathForHostPath(task, localRepoPath, worktreeSuffix)
		out, err := o.runSandboxStepInWorktree(ctx, task, agent, "git_diff_names_"+repo.ID, fmt.Sprintf("git -C %s diff --name-only", quoteShellArg(containerRepoPath)), worktreeSuffix)
		if err == nil {
			if stdout, ok := out["stdout"].(string); ok && stdout != "" {
				lines := strings.Split(strings.TrimSpace(stdout), "\n")
				for _, line := range lines {
					if line != "" {
						allChanged = append(allChanged, prefix+line)
					}
				}
			}
		}
	}
	return allChanged, nil
}

func (o *Orchestrator) hostWorktreePath(task *models.Task, repoPath string, worktreeSuffix string) string {
	if worktreeSuffix == "" {
		return repoPath
	}

	ctx := context.Background()
	rWS, err := o.FindRepoWorkspaceByPath(ctx, task, repoPath)
	if err != nil {
		clean := strings.TrimPrefix(worktreeSuffix, "-")
		clean = strings.TrimSuffix(clean, "-worktree")
		localPath := sandbox.WorkspacePath(o.workspaceRoot, task.ID)
		if task.RepositoryID != nil {
			return filepath.Join(localPath, clean)
		}
		if repoPath == localPath {
			return localPath
		}
		return repoPath + worktreeSuffix
	}

	role := getRoleFromSuffix(worktreeSuffix)
	ws := o.GetTaskWorkspace(task)

	if rWS.Paths.Worktrees == nil {
		rWS.Paths.Worktrees = make(map[string]string)
	}
	if rWS.Branches.Role == nil {
		rWS.Branches.Role = make(map[string]string)
	}

	if path, exists := rWS.Paths.Worktrees[role]; exists && path != "" {
		return filepath.Join(ws.Root, path)
	}

	relPath := filepath.Join("code", "repos", rWS.Name, "worktrees", role)
	rWS.Paths.Worktrees[role] = relPath
	rWS.Branches.Role[role] = fmt.Sprintf("feature/%s-%s", task.ID, role)

	if wsLoaded, errLoad := o.LoadTaskWorkspace(ctx, task); errLoad == nil {
		for i := range wsLoaded.Repos {
			if wsLoaded.Repos[i].RepoID == rWS.RepoID {
				wsLoaded.Repos[i] = *rWS
				break
			}
		}
		_ = o.SaveTaskWorkspaceMetadata(task, wsLoaded)
	}

	return filepath.Join(ws.Root, relPath)
}

func splitPatchByRepo(patchText string) map[string]string {
	repos := make(map[string]string)
	parts := strings.Split(patchText, "diff --git ")
	header := parts[0]

	repoBlocks := make(map[string][]string)
	for i := 1; i < len(parts); i++ {
		block := parts[i]
		if strings.HasPrefix(block, "a/") {
			sub := block[2:]
			idx := strings.Index(sub, "/")
			if idx != -1 {
				repoName := sub[:idx]
				repoBlocks[repoName] = append(repoBlocks[repoName], "diff --git "+block)
			}
		}
	}

	for repoName, blocks := range repoBlocks {
		repos[repoName] = header + strings.Join(blocks, "")
	}
	return repos
}
