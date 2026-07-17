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

// BackendAgentAssigner defines the optional hook to assign a role-specific backend agent.
type BackendAgentAssigner interface {
	AssignBackendAgent(ctx context.Context, task *models.Task) (*models.Agent, error)
}

// CodeBackendStep implements Step for the backend coding track.
type CodeBackendStep struct {
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

func NewCodeBackendStep(
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
) *CodeBackendStep {
	return &CodeBackendStep{
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

func (s *CodeBackendStep) ID() string                         { return workflow.StepCodeBackend }
func (s *CodeBackendStep) StatusOnResume(_ StepResult) string { return models.TaskStatusCoding }

func (s *CodeBackendStep) Execute(ctx context.Context, stepCtx workflow.StepContext) (StepResult, error) {
	var t *models.Task
	if s.tasks != nil {
		t, _ = s.tasks.GetByID(ctx, s.rt.Task.ID)
	}
	isEasy := t != nil && t.Complexity == models.TaskComplexityEasy

	backendAgent := s.rt.Agent
	assignedAgentID := ""

	// Finding 6: Role-Specialization Bypassed in EasyWorkflow
	if !isEasy {
		if backendAgent == nil || backendAgent.Role != models.AgentRoleBackend {
			assigner, ok := s.agents.(BackendAgentAssigner)
			if !ok {
				roleStr := "nil"
				if backendAgent != nil {
					roleStr = backendAgent.Role
				}
				return nil, fmt.Errorf("backend coding step requires a backend agent, but got role %s", roleStr)
			}
			bg, err := assigner.AssignBackendAgent(ctx, s.rt.Task)
			if bg != nil {
				backendAgent = bg
				assignedAgentID = bg.ID
				s.log.Log(ctx, s.rt.Task.ID, &s.rt.JobID, "info", fmt.Sprintf("assigned backend agent %s for backend coding step", backendAgent.Name))
			}
			if err != nil {
				if assignedAgentID != "" {
					if releaser, ok := s.agents.(AgentReleaser); ok {
						_ = releaser.Release(context.WithoutCancel(ctx), assignedAgentID)
					}
				}
				return nil, fmt.Errorf("failed to assign backend agent for backend coding step: %w", err)
			}
		}
		if assignedAgentID != "" && (s.rt.Agent == nil || assignedAgentID != s.rt.Agent.ID) {
			defer func() {
				if releaser, ok := s.agents.(AgentReleaser); ok {
					if err := releaser.Release(context.WithoutCancel(ctx), assignedAgentID); err != nil {
						s.log.Log(ctx, s.rt.Task.ID, &s.rt.JobID, "warn", fmt.Sprintf("release backend agent failed: %v", err))
					}
				}
			}()
		}
		if backendAgent == nil || backendAgent.Role != models.AgentRoleBackend {
			roleStr := "nil"
			if backendAgent != nil {
				roleStr = backendAgent.Role
			}
			return nil, fmt.Errorf("backend coding step requires a backend agent, but got role %s", roleStr)
		}
	} else if backendAgent == nil {
		return nil, fmt.Errorf("coding step requires an agent")
	}

	worktreeSuffix := ""
	if !isEasy {
		worktreeSuffix = models.WorktreeSuffixBackend
		if err := setupSandbox(ctx, s.rt.Task, backendAgent, s.worktree, s.workspace, models.RoleBackend, "backend", worktreeSuffix); err != nil {
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

	var analysis models.TaskAnalysis
	if t != nil {
		_ = json.Unmarshal(t.Analysis, &analysis)
	}
	frozen := LoadFrozenContext(stepCtx, &analysis)
	preHydratedCtx := buildPreHydratedContext(ctx, s.rt.Task, s.fileReader, frozen)

	instruction, _, ctx := buildCodingInstruction(ctx, stepCtx, codingInstructionParams{
		Task:             s.rt.Task,
		Workspace:        s.workspace,
		IsEasy:           isEasy,
		Role:             "backend",
		SubtaskKey:       "backend",
		InstructionVerb:  "Implement the backend changes using the available tools (e.g. search_replace, create_file) to edit files directly. Use run_tests/run_build/run_lint to verify your work before finishing.",
		PRFeedback:       prFeedback,
		PreHydratedFiles: preHydratedCtx,
	})

	out, _, err := runPatchRetryLoop(ctx, patchRetryConfig{
		Task:           s.rt.Task,
		Agent:          backendAgent,
		JobID:          s.rt.JobID,
		StepID:         stepCtx.StepID,
		WorktreeSuffix: worktreeSuffix,
		TestLabel:      "code_backend_test",
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
		Checkpoints:    s.checkpoints,
	}, instruction)
	if err != nil {
		return nil, err
	}
	if s.diff != nil {
		if diffText, diffErr := s.diff.CaptureWorkspaceDiff(ctx, s.rt.Task, backendAgent, stepCtx.StepID, worktreeSuffix); diffErr == nil && diffText != "" {
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
			changedFiles, diffErr = s.diff.GetChangedFiles(ctx, s.rt.Task, backendAgent, repoHostPath, worktreeSuffix)
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
		if err := commitSandbox(ctx, s.rt.Task, backendAgent, s.worktree, s.workspace, models.RoleBackend, "backend", worktreeSuffix); err != nil {
			return nil, err
		}
	}

	// Mark subtask as complete in the database if applicable
	if planOut, ok := stepCtx.Inputs[workflow.StepPlan]; ok {
		if subtasks, ok := planOut["subtasks"].(map[string]any); ok {
			if beTasks, ok := subtasks["backend"].([]any); ok && len(beTasks) > 0 {
				var taskIdx = -1
				if idx := strings.LastIndex(stepCtx.StepID, "_"); idx != -1 {
					if parsedIdx, err := strconv.Atoi(stepCtx.StepID[idx+1:]); err == nil {
						taskIdx = parsedIdx
					}
				}
				if taskIdx >= 0 && taskIdx < len(beTasks) {
					if taskText, ok := beTasks[taskIdx].(string); ok {
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
					} else {
						s.log.Log(ctx, s.rt.Task.ID, &s.rt.JobID, "warn", fmt.Sprintf("plan subtask at index %d is not a string, skipping tasks_md update", taskIdx))
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
