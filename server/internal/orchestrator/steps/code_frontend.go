package steps

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"github.com/auto-code-os/auto-code-os/server/internal/workflow"
	"github.com/auto-code-os/auto-code-os/server/pkg/models"
)

// FrontendAgentAssigner defines the optional hook to assign a role-specific frontend agent.
type FrontendAgentAssigner interface {
	AssignFrontendAgent(ctx context.Context, task *models.Task) (*models.Agent, error)
}

// CodeFrontendStep implements Step for the frontend coding track.
type CodeFrontendStep struct {
	rt          StepRuntime
	tasks       TaskRepository
	llm         LLMRunner
	agents      any
	worktree    WorktreeManager
	patcher     PatchApplier
	diff        DiffCapturer
	workspace   WorkspaceLoader
	artifacts   ArtifactSaver
	tester      TestRunner
	checkpoints CheckpointLister
	log         Logger
	fileReader  AffectedFileReader
}

func NewCodeFrontendStep(
	rt StepRuntime,
	tasks TaskRepository,
	llm LLMRunner,
	agents any,
	worktree WorktreeManager,
	patcher PatchApplier,
	diff DiffCapturer,
	workspace WorkspaceLoader,
	artifacts ArtifactSaver,
	tester TestRunner,
	checkpoints CheckpointLister,
	log Logger,
	fileReader AffectedFileReader,
) *CodeFrontendStep {
	return &CodeFrontendStep{
		rt:          rt,
		tasks:       tasks,
		llm:         llm,
		agents:      agents,
		worktree:    worktree,
		patcher:     patcher,
		diff:        diff,
		workspace:   workspace,
		artifacts:   artifacts,
		tester:      tester,
		checkpoints: checkpoints,
		log:         log,
		fileReader:  fileReader,
	}
}

func (s *CodeFrontendStep) ID() string                         { return workflow.StepCodeFrontend }
func (s *CodeFrontendStep) StatusOnResume(_ StepResult) string { return models.TaskStatusCoding }

func (s *CodeFrontendStep) Execute(ctx context.Context, stepCtx workflow.StepContext) (StepResult, error) {
	var t *models.Task
	var err error
	if s.tasks != nil {
		t, err = s.tasks.GetByID(ctx, s.rt.Task.ID)
	}

	hasFrontendFiles := false
	if err == nil && t != nil {
		if t.Complexity == models.TaskComplexityEasy {
			return StepResult{"status": "skipped", "info": "skipped frontend step for easy task"}, nil
		}
		var analysis models.TaskAnalysis
		_ = json.Unmarshal(t.Analysis, &analysis)
		frozen := LoadFrozenContext(stepCtx, &analysis)
		if frozen != nil {
			for _, file := range frozen.AffectedFiles {
				if isFrontendFile(file.File) {
					hasFrontendFiles = true
					break
				}
			}
		}
	}

	// Check skip signal from Plan step (Phase 3)
	if !hasFrontendFiles {
		if planOut, ok := stepCtx.Inputs[workflow.StepPlan]; ok {
			if skip, _ := planOut["skip_frontend"].(bool); skip {
				return StepResult{"status": "skipped", "info": "skipped by plan: no frontend work needed"}, nil
			}
		} else if err == nil && t != nil {
			// Fallback if Plan inputs are missing (e.g. in tests)
			return StepResult{"status": "skipped", "info": "no frontend files affected"}, nil
		}
	}

	frontendAgent := s.rt.Agent
	assignedAgentID := ""
	if frontendAgent == nil || frontendAgent.Role != models.AgentRoleFrontend {
		assigner, ok := s.agents.(FrontendAgentAssigner)
		if !ok {
			roleStr := "nil"
			if frontendAgent != nil {
				roleStr = frontendAgent.Role
			}
			return nil, fmt.Errorf("frontend coding step requires a frontend agent, but got role %s", roleStr)
		}
		fg, err := assigner.AssignFrontendAgent(ctx, s.rt.Task)
		if fg != nil {
			frontendAgent = fg
			assignedAgentID = fg.ID
			s.log.Log(ctx, s.rt.Task.ID, &s.rt.JobID, "info", fmt.Sprintf("assigned frontend agent %s for frontend coding step", frontendAgent.Name))
		}
		if err != nil {
			if assignedAgentID != "" {
				if releaser, ok := s.agents.(AgentReleaser); ok {
					_ = releaser.Release(context.WithoutCancel(ctx), assignedAgentID)
				}
			}
			return nil, fmt.Errorf("failed to assign frontend agent for frontend coding step: %w", err)
		}
	}
	if assignedAgentID != "" && (s.rt.Agent == nil || assignedAgentID != s.rt.Agent.ID) {
		defer func() {
			if releaser, ok := s.agents.(AgentReleaser); ok {
				if err := releaser.Release(context.WithoutCancel(ctx), assignedAgentID); err != nil {
					s.log.Log(ctx, s.rt.Task.ID, &s.rt.JobID, "warn", fmt.Sprintf("release frontend agent failed: %v", err))
				}
			}
		}()
	}
	if frontendAgent == nil || frontendAgent.Role != models.AgentRoleFrontend {
		roleStr := "nil"
		if frontendAgent != nil {
			roleStr = frontendAgent.Role
		}
		return nil, fmt.Errorf("frontend coding step requires a frontend agent, but got role %s", roleStr)
	}

	worktreeSuffix := ""
	if t != nil && t.Complexity != models.TaskComplexityEasy {
		worktreeSuffix = "-fe-worktree"
		if err := setupSandbox(ctx, s.rt.Task, frontendAgent, s.worktree, s.workspace, "fe", "frontend", worktreeSuffix); err != nil {
			return nil, err
		}
	}

	var prFeedback string
	if s.checkpoints != nil {
		if checkpoints, cpErr := s.checkpoints.ListCheckpoints(ctx, s.rt.Task.ID); cpErr == nil {
			for _, cp := range checkpoints {
				if cp.Step == "pr_rejection" {
					var state map[string]any
					if json.Unmarshal(cp.State, &state) == nil {
						if f, _ := state["feedback"].(string); f != "" {
							prFeedback = f
						}
					}
				}
			}
		}
	}

	instruction, _, ctx := buildCodingInstruction(ctx, stepCtx, codingInstructionParams{
		Task:            s.rt.Task,
		Workspace:       s.workspace,
		IsEasy:          t != nil && t.Complexity == models.TaskComplexityEasy,
		Role:            "frontend",
		SubtaskKey:      "frontend",
		InstructionVerb: "Implement the frontend changes when applicable, using the available tools (e.g. search_replace, create_file) to edit files directly. Use run_tests/run_build/run_lint to verify your work before finishing.",
		PRFeedback:      prFeedback,
	})

	out, _, err := runPatchRetryLoop(ctx, patchRetryConfig{
		Task:           s.rt.Task,
		Agent:          frontendAgent,
		JobID:          s.rt.JobID,
		StepID:         stepCtx.StepID,
		WorktreeSuffix: worktreeSuffix,
		TestLabel:      "code_frontend_test",
		MaxRetries:     3,
		Agentic:        true,
		LLM:            s.llm,
		Worktree:       s.worktree,
		Patcher:        s.patcher,
		Diff:           s.diff,
		Artifacts:      s.artifacts,
		Tester:         s.tester,
		Tasks:          s.tasks,
		Log:            s.log,
	}, instruction)
	if err != nil {
		return nil, err
	}
	if s.diff != nil {
		if diffText, diffErr := s.diff.CaptureWorkspaceDiff(ctx, s.rt.Task, frontendAgent, stepCtx.StepID, worktreeSuffix); diffErr == nil && diffText != "" {
			if s.artifacts != nil {
				_ = s.artifacts.SaveArtifact(ctx, s.rt.JobID, s.rt.Task.ID, stepCtx.StepID, "diff", diffText)
			}
		}
	}

	var changedFiles []string
	if s.diff != nil {
		repoHostPath, err := s.diff.GetTaskRepoHostPath(ctx, s.rt.Task)
		if err == nil {
			var diffErr error
			changedFiles, diffErr = s.diff.GetChangedFiles(ctx, s.rt.Task, frontendAgent, repoHostPath, worktreeSuffix)
			if diffErr != nil {
				s.log.Log(ctx, s.rt.Task.ID, &s.rt.JobID, "warn", fmt.Sprintf("failed to get changed files: %v", diffErr))
			}
		}
	}

	if len(changedFiles) > 0 {
		err := updateTaskAnalysis(ctx, s.rt.Task.ID, s.tasks, s.rt.Task, func(analysis *models.TaskAnalysis) bool {
			updated := false
			for _, newFile := range changedFiles {
				exists := false
				for _, f := range analysis.AffectedFiles {
					if f.File == newFile {
						exists = true
						break
					}
				}
				if !exists {
					analysis.AffectedFiles = append(analysis.AffectedFiles, models.AffectedFile{File: newFile})
					updated = true
				}
			}
			return updated
		})
		if err != nil {
			s.log.Log(ctx, s.rt.Task.ID, &s.rt.JobID, "warn", fmt.Sprintf("failed to update analysis affected files: %v", err))
		}
	}

	if worktreeSuffix != "" {
		if err := commitSandbox(ctx, s.rt.Task, frontendAgent, s.worktree, s.workspace, "fe", "frontend", worktreeSuffix); err != nil {
			return nil, err
		}
	}

	// Mark subtask as complete in the database if applicable
	if planOut, ok := stepCtx.Inputs[workflow.StepPlan]; ok {
		if subtasks, ok := planOut["subtasks"].(map[string]any); ok {
			if feTasks, ok := subtasks["frontend"].([]any); ok && len(feTasks) > 0 {
				var taskIdx = -1
				if idx := strings.LastIndex(stepCtx.StepID, "_"); idx != -1 {
					if parsedIdx, err := strconv.Atoi(stepCtx.StepID[idx+1:]); err == nil {
						taskIdx = parsedIdx
					}
				}
				if taskIdx >= 0 && taskIdx < len(feTasks) {
					taskText := feTasks[taskIdx].(string)

					err := updateTaskAnalysis(ctx, s.rt.Task.ID, s.tasks, s.rt.Task, func(analysis *models.TaskAnalysis) bool {
						if updatedTasksMD, ok := updateTaskSubtaskMarkdown(analysis.TasksMD, taskText); ok {
							analysis.TasksMD = updatedTasksMD
							return true
						}
						return false
					})
					if err != nil {
						s.log.Log(ctx, s.rt.Task.ID, &s.rt.JobID, "warn", fmt.Sprintf("failed to update tasks_md status: %v", err))
					}
				}
			}
		}
	}

	if out == nil {
		out = make(map[string]any)
	}
	out["files_changed"] = changedFiles

	return out, nil
}
