# Orchestrator Step Decoupling — Implementation Plan (v2)

> **For agentic workers:** Use subagent-driven-development or executing-plans
> to implement this plan task-by-task. Steps use checkbox syntax for tracking.

**Goal:** Refactor `server/internal/orchestrator` to decouple workflow steps from the monolithic `steps.Deps` struct, making each step independently testable, swappable, and maintainable without touching the orchestrator core.

**Architecture:** Steps become job-scoped structs that store `Task`, `Agent`, and `JobID` at construction time. Each step declares only the narrow interfaces it actually uses (e.g. `PatchApplier`, `DiffCapturer`) instead of receiving the 20+ field `Deps` god-struct. Service interfaces are derived bottom-up from actual step usage. Tests are written alongside each step conversion, not deferred.

**Tech Stack:** Go 1.23, `testing` stdlib, existing `workflow.StepFunc` signature, existing GORM repositories.

---

## Current Problems

| Problem | Location | Impact |
| :--- | :--- | :--- |
| `steps.Deps` is a god-struct with 20+ fields | `steps/deps.go` | Every step receives ALL deps, even unused ones |
| Steps duplicate interface declarations | `steps/deps.go` vs `interfaces.go` | Maintenance drift between two canonical locations |
| `orchestrator.go` has 30+ thin wrapper methods | `orchestrator.go:321-553` | 230 lines of pure indirection |
| No unit tests for 8 of 10 steps | `steps/` | Only `analyze_test.go` and `context_load_test.go` exist |
| `checkpoint/recovery.go` hardcodes step→status map | `recovery.go:49-70` | Adding a step requires editing recovery middleware |

---

## Migration Shape (Review-Adjusted)

```
1. Define Step interface + StepRuntime + StepResult              → Task 1
2. Convert ONE simple step end-to-end (plan.go) with tests       → Task 2
3. Derive focused interfaces from that step's actual usage       → Task 2 (inline)
4. Convert 2 more steps to validate the pattern (review, fix)    → Task 3
5. Convert remaining 7 steps, each with unit tests alongside     → Task 4
6. Refactor step registry + service adapters + checkpoint        → Task 5
   (checkpoint uses ResumeStatusFunc to avoid importing steps)
7. Clean wrappers (hard-gate audit, step-only vs worker split)   → Task 6
8. Remove legacy Deps struct and ExecuteXXX free functions       → Task 7
```

---

## Task 1: Define `Step` Interface, `StepResult`, and `StepRuntime`

> Addresses: Finding 1 (blocking — Execute lacks task/agent/jobID context), Finding 7 (StepResult)

**Files:**
- Create: `server/internal/orchestrator/steps/step.go`

- [x] **Step 1: Create the Step interface file**

```go
// server/internal/orchestrator/steps/step.go
package steps

import (
    "context"
    "github.com/auto-code-os/auto-code-os/server/internal/workflow"
    "github.com/auto-code-os/auto-code-os/server/pkg/models"
)

// StepResult is the canonical return type from step execution.
type StepResult = map[string]any

// StepRuntime carries job-scoped context that every step needs.
// This is injected at construction time (steps are job-scoped),
// so Execute does not need task/agent/jobID parameters.
type StepRuntime struct {
    Task  *models.Task
    Agent *models.Agent
    JobID string
}

// Step is the contract every workflow step must implement.
// Steps are constructed per-job with their specific dependencies
// and StepRuntime, so Execute only needs ctx and workflow context.
type Step interface {
    // ID returns the workflow step identifier (e.g. "context_load").
    ID() string

    // Execute runs the step logic and returns a result.
    Execute(ctx context.Context, stepCtx workflow.StepContext) (StepResult, error)

    // StatusOnResume returns the task status to restore when this step
    // is skipped during checkpoint recovery. It receives the cached
    // checkpoint output so steps like review can choose between
    // "testing" and "fixing" based on findings. Empty string means
    // no status transition needed.
    StatusOnResume(output StepResult) string
}
```

- [x] **Step 2: Verify the file compiles**

Run: `cd server && go build ./internal/orchestrator/steps/`
Expected: Build succeeds with no errors.

- [x] **Step 3: Commit**

```bash
git add server/internal/orchestrator/steps/step.go
git commit -m "refactor(orchestrator): define Step interface with StepRuntime and StepResult"
```

---

## Task 2: Convert `plan.go` End-to-End (Pattern Proof)

> Addresses: Finding 2 (derive interfaces from actual usage), Finding 3 (meaningful tests)

`plan.go` is the simplest step (56 lines). We convert it fully without `toDeps()`, deriving focused interfaces from its actual code dependencies.

**Actual dependencies of `plan.go`** (derived from source):
- `deps.Tasks.GetByID` → `TaskReader`
- `deps.LLM` (via `deps.RunLLMStep`) → `LLMRunner`
- `deps.RepoUtil.LoadTargetRepositories` → `WorktreeManager.LoadTargetRepositories`
- `deps.RepoUtil.SetupRoleBranches` → `WorktreeManager.SetupRoleBranches`
- `deps.Wkspace.LoadTaskWorkspace` → `WorkspaceLoader`
- `deps.UpdateTaskStatus` → `StatusUpdater`

> **Nil Fallback Rule:** Existing `ExecutePlan` handles `deps.LLM == nil` and
> `deps.RepoUtil == nil` gracefully. ALL converted steps MUST preserve these
> nil-safety semantics. Constructors accept nil for optional dependencies;
> `Execute` must check for nil before calling optional interfaces.

**Files:**
- Create: `server/internal/orchestrator/steps/services.go`
- Modify: `server/internal/orchestrator/steps/plan.go`
- Create: `server/internal/orchestrator/steps/mocks_test.go`
- Create: `server/internal/orchestrator/steps/plan_test.go`

- [x] **Step 1: Create `services.go` with focused interfaces derived from `plan.go`**

```go
// server/internal/orchestrator/steps/services.go
package steps

import (
    "context"
    "github.com/auto-code-os/auto-code-os/server/pkg/llm"
    "github.com/auto-code-os/auto-code-os/server/pkg/models"
)

// --- Narrow interfaces derived from actual step usage ---

// TaskReader reads task state. Used by: all steps.
type TaskReader interface {
    GetByID(ctx context.Context, id string) (*models.Task, error)
}

// TaskUpdater writes task state. Used by: analyze, pr.
type TaskUpdater interface {
    Update(ctx context.Context, id string, input models.UpdateTaskInput) (*models.Task, error)
}

// StatusUpdater transitions task status with validation. Used by: most steps.
type StatusUpdater interface {
    UpdateTaskStatus(ctx context.Context, taskID string, newStatus string) (*models.Task, error)
}

// ProjectReader reads project config. Used by: analyze, review, fix, test, pr.
type ProjectReader interface {
    GetByID(ctx context.Context, id string) (*models.Project, error)
}

// LLMRunner executes a single-shot LLM step. Used by: plan, code_be, code_fe, review, fix.
type LLMRunner interface {
    RunLLMStep(ctx context.Context, task *models.Task, agent *models.Agent, jobID string, stepID string, instruction string) (StepResult, error)
}

// LLMChatter provides multi-turn LLM chat. Used by: analyze (tool loop), context_load (profiler).
type LLMChatter interface {
    Chat(ctx context.Context, messages []llm.Message) (*llm.Response, error)
    ChatWithOptions(ctx context.Context, messages []llm.Message, opts llm.ChatOptions) (*llm.Response, error)
}

// PatchApplier applies a unified diff patch. Used by: code_be, code_fe, fix.
type PatchApplier interface {
    ApplyPatch(ctx context.Context, task *models.Task, agent *models.Agent, stepID string, patchText string, worktreeSuffix string) error
}

// DiffCapturer captures workspace diffs. Used by: code_be, code_fe, fix, merge, pr.
type DiffCapturer interface {
    CaptureWorkspaceDiff(ctx context.Context, task *models.Task, agent *models.Agent, stepID string, worktreeSuffix string) (string, error)
    CapturePRDiff(ctx context.Context, task *models.Task, agent *models.Agent, baseBranch string) (string, error)
    GetChangedFiles(ctx context.Context, task *models.Task, agent *models.Agent, targetPath string, worktreeSuffix string) ([]string, error)
    GetTaskRepoHostPath(ctx context.Context, task *models.Task) (string, error)
}

// WorktreeManager manages git worktrees for parallel coding. Used by: code_be, code_fe, plan.
type WorktreeManager interface {
    LoadTargetRepositories(ctx context.Context, task *models.Task) ([]models.Repository, error)
    SetupRoleBranches(ctx context.Context, task *models.Task, agent *models.Agent, jobID string, repos []models.Repository, ws *models.TaskWorkspace)
    SetupRoleWorktrees(ctx context.Context, task *models.Task, agent *models.Agent, repos []models.Repository, ws *models.TaskWorkspace, roleName string, roleLabel string, worktreeSuffix string) error
    CommitRoleWorktrees(ctx context.Context, task *models.Task, agent *models.Agent, repos []models.Repository, ws *models.TaskWorkspace, roleName string, roleLabel string, worktreeSuffix string) error
    RepoHostPath(task *models.Task, ws *models.TaskWorkspace, repo models.Repository) string
    ContainerPathForHostPath(task *models.Task, hostPath string, worktreeSuffix string) string
}

// SandboxRunner executes commands in Docker sandbox. Used by: context_load, test, analyze.
type SandboxRunner interface {
    RunCommand(ctx context.Context, task *models.Task, agent *models.Agent, stepID string, command string) (StepResult, error)
}

// TestRunner runs targeted tests on changed files. Used by: code_be, code_fe, fix.
type TestRunner interface {
    RunTargetedTests(ctx context.Context, task *models.Task, agent *models.Agent, jobID string, stepName string, changedFiles []string, worktreeSuffix string) (StepResult, error)
}

// ArtifactSaver persists step artifacts. Used by: all steps.
type ArtifactSaver interface {
    SaveArtifact(ctx context.Context, jobID string, taskID string, step string, artType string, payload any) error
}

// ArtifactLister reads stored artifacts. Used by: pr.
type ArtifactLister interface {
    ListByJobID(ctx context.Context, jobID string) ([]models.WorkflowArtifact, error)
}

// Logger writes task logs. Used by: all steps.
type Logger interface {
    Log(ctx context.Context, taskID string, jobID *string, level string, message string)
}

// SandboxGitClient executes git operations inside sandbox containers.
// Used by: merge (CheckoutBranch, MergeBranch, HasBranch, CommitChanges),
//          pr (GetChangedFiles, CheckoutNewBranch, GetPRDiff).
// NOTE: PatchApplier and TestRunner adapters internally wrap
// RunSandboxStepInWorktree — steps do not call it directly.
type SandboxGitClient interface {
    CheckoutBranch(ctx context.Context, task *models.Task, agent *models.Agent, containerPath string, branch string) error
    CheckoutNewBranch(ctx context.Context, task *models.Task, agent *models.Agent, containerPath string, branch string) error
    HasBranch(ctx context.Context, task *models.Task, agent *models.Agent, containerPath string, branch string) bool
    MergeBranch(ctx context.Context, task *models.Task, agent *models.Agent, containerPath string, branch string) (string, error)
    CommitChanges(ctx context.Context, task *models.Task, agent *models.Agent, containerPath string, message string) error
    GetChangedFiles(ctx context.Context, task *models.Task, agent *models.Agent, containerPath string) ([]string, error)
    GetPRDiff(ctx context.Context, task *models.Task, agent *models.Agent, containerPath string, baseBranch string) (string, error)
}

// WorkspaceLoader loads/saves workspace metadata. Used by: context_load, plan, code, merge, test, pr.
type WorkspaceLoader interface {
    LoadTaskWorkspace(ctx context.Context, task *models.Task) (*models.TaskWorkspace, error)
    SaveTaskWorkspaceMetadata(task *models.Task, ws *models.TaskWorkspace) error
}

// CheckpointReader counts completed step cycles. Used by: review, test.
type CheckpointReader interface {
    CountSuccessful(ctx context.Context, taskID string, step string) int
}

// CheckpointLister lists all checkpoints. Used by: fix (pr_rejection feedback).
type CheckpointLister interface {
    ListCheckpoints(ctx context.Context, taskID string) ([]models.WorkflowCheckpoint, error)
}

// PromptAssembler builds LLM prompts. Used by: analyze, context_load.
type PromptAssembler interface {
    AssembleForAgent(ctx context.Context, task models.Task, agent *models.Agent, history []llm.Message) ([]llm.Message, []llm.ToolDefinition, error)
}

// TraceRecorder writes LLM call traces. Used by: analyze.
type TraceRecorder interface {
    WriteLLMCallTrace(ctx context.Context, task *models.Task, agent *models.Agent, stepID string, messages []llm.Message, resp *llm.Response, parsed StepResult)
}

// ReviewerAssigner optionally assigns a specialized reviewer agent. Used by: review.
type ReviewerAssigner interface {
    AssignReviewer(ctx context.Context, task *models.Task) (*models.Agent, error)
}

// BackendAssigner optionally assigns a specialized backend agent. Used by: code_be.
type BackendAssigner interface {
    AssignBackendAgent(ctx context.Context, task *models.Task) (*models.Agent, error)
}

// FrontendAssigner optionally assigns a specialized frontend agent. Used by: code_fe.
type FrontendAssigner interface {
    AssignFrontendAgent(ctx context.Context, task *models.Task) (*models.Agent, error)
}

// GitOpsClient handles remote git operations. Used by: pr.
type GitOpsClient interface {
    CommitAndPush(ctx context.Context, localPath, repoURL, branchName, message string, files map[string]string, agentRole string) error
    CreatePullRequest(ctx context.Context, repoURL, branchName, title, body string) (string, error)
}
```

- [x] **Step 2: Add `PlanStep` struct to `plan.go`**

```go
// PlanStep implements Step for the execution planning phase.
type PlanStep struct {
    rt        StepRuntime
    tasks     TaskReader
    llm       LLMRunner
    worktree  WorktreeManager
    workspace WorkspaceLoader
    status    StatusUpdater
    log       Logger
}

func NewPlanStep(rt StepRuntime, tasks TaskReader, llm LLMRunner, worktree WorktreeManager, workspace WorkspaceLoader, status StatusUpdater, log Logger) *PlanStep {
    return &PlanStep{rt: rt, tasks: tasks, llm: llm, worktree: worktree, workspace: workspace, status: status, log: log}
}

func (s *PlanStep) ID() string                              { return workflow.StepPlan }
func (s *PlanStep) StatusOnResume(_ StepResult) string        { return models.TaskStatusCoding }

func (s *PlanStep) Execute(ctx context.Context, stepCtx workflow.StepContext) (StepResult, error) {
    // Inline existing ExecutePlan logic using s.rt.Task, s.rt.Agent, s.rt.JobID
    // and narrow interfaces s.tasks, s.llm, s.worktree, etc.
    // No Deps adapter needed.
}
```

- [x] **Step 3: Create `mocks_test.go` with test helpers**

```go
// server/internal/orchestrator/steps/mocks_test.go
package steps

import (
    "context"
    "github.com/auto-code-os/auto-code-os/server/pkg/models"
)

type mockTaskReader struct {
    task *models.Task
    err  error
}

func (m *mockTaskReader) GetByID(ctx context.Context, id string) (*models.Task, error) {
    return m.task, m.err
}

type mockStatusUpdater struct {
    called   bool
    lastID   string
    lastStatus string
}

func (m *mockStatusUpdater) UpdateTaskStatus(ctx context.Context, taskID string, newStatus string) (*models.Task, error) {
    m.called = true
    m.lastID = taskID
    m.lastStatus = newStatus
    return &models.Task{ID: taskID, Status: newStatus}, nil
}

type mockLogger struct{ messages []string }

func (m *mockLogger) Log(_ context.Context, _ string, _ *string, _ string, msg string) {
    m.messages = append(m.messages, msg)
}
// ... additional mocks as needed
```

- [x] **Step 4: Write `plan_test.go` with meaningful execution tests**

```go
func TestPlanStep_SkipsEasyTask(t *testing.T) {
    task := &models.Task{ID: "t1", Complexity: models.TaskComplexityEasy}
    step := NewPlanStep(
        StepRuntime{Task: task, Agent: &models.Agent{ID: "a1"}, JobID: "j1"},
        &mockTaskReader{task: task},
        nil, nil, nil,
        &mockStatusUpdater{},
        &mockLogger{},
    )
    result, err := step.Execute(context.Background(), workflow.StepContext{})
    require.NoError(t, err)
    assert.Equal(t, "skipped", result["status"])
}

func TestPlanStep_TransitionsToCoding(t *testing.T) {
    task := &models.Task{ID: "t1", Complexity: models.TaskComplexityMedium}
    statusMock := &mockStatusUpdater{}
    step := NewPlanStep(
        StepRuntime{Task: task, Agent: &models.Agent{ID: "a1"}, JobID: "j1"},
        &mockTaskReader{task: task},
        &mockLLMRunner{result: StepResult{"subtasks": []any{}}},
        nil, nil, statusMock, &mockLogger{},
    )
    _, err := step.Execute(context.Background(), workflow.StepContext{})
    require.NoError(t, err)
    assert.True(t, statusMock.called)
    assert.Equal(t, models.TaskStatusCoding, statusMock.lastStatus)
}
```

- [x] **Step 5: Run tests**

Run: `cd server && go test ./internal/orchestrator/steps/ -run TestPlanStep -v`
Expected: PASS

- [x] **Step 6: Commit**

```bash
git add server/internal/orchestrator/steps/
git commit -m "refactor(orchestrator): convert plan step end-to-end with focused interfaces and tests"
```

---

## Task 3: Convert `review` and `fix` Steps (Validate Pattern)

> Validates the pattern works for steps with complex dependencies (cycle limits, PR feedback, loopback errors).

**Files:**
- Modify: `server/internal/orchestrator/steps/review.go`
- Modify: `server/internal/orchestrator/steps/fix.go`
- Create: `server/internal/orchestrator/steps/review_test.go`
- Create: `server/internal/orchestrator/steps/fix_test.go`

- [x] **Step 1: Add `ReviewStep` struct** — uses `TaskReader`, `ProjectReader`, `LLMRunner`, `DiffCapturer`, `ArtifactSaver`, `ReviewerAssigner`, `CheckpointReader`, `StatusUpdater`, `Logger`
- [x] **Step 2: Write `review_test.go`** — test easy skip, cycle limit, human_only policy, finding detection
- [x] **Step 3: Add `FixStep` struct** — uses `TaskReader`, `LLMRunner`, `PatchApplier`, `DiffCapturer`, `TestRunner`, `ArtifactSaver`, `CheckpointLister`, `StatusUpdater`, `Logger`
- [x] **Step 4: Write `fix_test.go`** — test PR feedback injection, no-findings skip, loopback trigger, cycle limit skip
- [x] **Step 5: Run tests and verify compilation**

Run: `cd server && go test ./internal/orchestrator/steps/ -run "TestReviewStep|TestFixStep" -v`
Expected: PASS

- [x] **Step 6: Commit**

```bash
git add server/internal/orchestrator/steps/{review,fix}*.go
git commit -m "refactor(orchestrator): convert review and fix steps with tests"
```

---

## Task 4: Convert Remaining 7 Steps (Each with Tests)

**Files:** All remaining step files in `server/internal/orchestrator/steps/`

| Step | Struct | Key Interfaces | Test Focus |
| :--- | :--- | :--- | :--- |
| `context_load.go` | `ContextLoadStep` | `TaskReader`, `SandboxRunner`, `WorkspaceLoader`, `LLMChatter`, `ArtifactSaver`, `Logger` | status update, artifact save, profile cache |
| `analyze.go` | `AnalyzeStep` | `TaskReader`, `TaskUpdater`, `ProjectReader`, `LLMChatter`, `PromptAssembler`, `SandboxRunner`, `ArtifactSaver`, `StatusUpdater`, `TraceRecorder`, `Logger` | complexity detection, OpenSpec file writing, human gate pause |
| `code_backend.go` | `CodeBackendStep` | `TaskReader`, `LLMRunner`, `PatchApplier`, `DiffCapturer`, `WorktreeManager`, `TestRunner`, `ArtifactSaver`, `Logger`, `BackendAssigner` | worktree setup, patch apply, targeted tests |
| `code_frontend.go` | `CodeFrontendStep` | Same as backend + frontend skip logic, `FrontendAssigner` | skip when no frontend files |
| `merge.go` | `MergeStep` | `TaskReader`, `WorktreeManager`, `WorkspaceLoader`, `ArtifactSaver`, `DiffCapturer`, `StatusUpdater`, `Logger`, `SandboxGitClient` | easy skip, conflict pause, metadata save |
| `testing.go` | `TestStep` | `TaskReader`, `ProjectReader`, `SandboxRunner`, `WorkspaceLoader`, `CheckpointReader`, `ArtifactSaver`, `StatusUpdater`, `Logger` | pass/fail, loopback, lint/build status |
| `pr.go` | `PRStep` | `TaskReader`, `TaskUpdater`, `ProjectReader`, `WorktreeManager`, `WorkspaceLoader`, `DiffCapturer`, `ArtifactLister`, `GitOpsClient`, `CheckpointLister`, `StatusUpdater`, `Logger`, `SandboxGitClient` | multi-repo PR, no-changes, risk domains |

> **Adapter Wrapping Note:** `PatchApplier` and `TestRunner` adapters internally
> call `RunSandboxStepInWorktree` from the orchestrator — steps never call sandbox
> worktree execution directly. `SandboxGitClient` is passed through directly from
> the existing `gitops.SandboxGitClient` in `Deps`.

- [x] **Step 1: Convert + test `context_load.go`**
- [x] **Step 2: Convert + test `analyze.go`**
- [x] **Step 3: Convert + test `code_backend.go`**
- [x] **Step 4: Convert + test `code_frontend.go`**
- [x] **Step 5: Convert + test `merge.go`**
- [x] **Step 6: Convert + test `testing.go`**
- [x] **Step 7: Convert + test `pr.go`**
- [x] **Step 8: Verify full compilation**

Run: `cd server && go test ./internal/orchestrator/... -v -count=1`
Expected: All tests pass.

- [x] **Step 9: Commit**

```bash
git add server/internal/orchestrator/steps/
git commit -m "refactor(orchestrator): convert remaining 7 steps with tests"
```

---

## Task 5: Refactor Step Registry, Service Adapters, and Checkpoint Recovery

> Merged: registry wiring + checkpoint recovery refactor done together to avoid
> touching the registry twice.
>
> **Import Cycle Prevention:** `checkpoint` must NOT import `steps` (because
> `steps/deps.go` imports `checkpoint` until Task 7). Instead, checkpoint
> accepts a plain `ResumeStatusFunc` callback. The step registry bridges
> `step.StatusOnResume` → `ResumeStatusFunc` at wiring time.

**Files:**
- Modify: `server/internal/orchestrator/step_registry.go`
- Create: `server/internal/orchestrator/service_adapters.go`
- Modify: `server/internal/orchestrator/checkpoint/recovery.go`

- [x] **Step 1: Create `service_adapters.go`** — adapter structs that bridge Orchestrator fields to step interfaces (TaskReader, StatusUpdater, LLMRunner, SandboxRunner, etc.)

- [x] **Step 2: Add `ResumeStatusFunc` type and refactor `WithCheckpointRecovery`**

```go
// server/internal/orchestrator/checkpoint/recovery.go
package checkpoint

// ResumeStatusFunc returns the task status to restore when a step is
// skipped during checkpoint recovery. It receives the cached checkpoint
// output so that steps like review can choose between "testing" and
// "fixing" based on findings. Return "" for no status transition.
type ResumeStatusFunc func(output map[string]any) string

func (s *Store) WithCheckpointRecovery(
    stepID string,
    statusOnResume ResumeStatusFunc,
    task *models.Task,
    agent *models.Agent,
    jobStep string,
    runner workflow.StepFunc,
    applyPatch func(...) error,
    updateTaskStatus func(...) (*models.Task, error),
) workflow.StepFunc {
    return func(ctx context.Context, sc workflow.StepContext) (map[string]any, error) {
        // ... existing checkpoint lookup ...
        if output, exists := s.GetSuccessful(ctx, task.ID, stepID); exists {
            // Use output-aware callback instead of hardcoded switch:
            if updateTaskStatus != nil && statusOnResume != nil {
                if resumeStatus := statusOnResume(output); resumeStatus != "" {
                    _, _ = updateTaskStatus(ctx, task.ID, resumeStatus)
                }
            }
            return output, nil
        }
        return runner(ctx, sc)
    }
}
```

- [x] **Step 3: Refactor `stepRunners` in `step_registry.go`**

Construct Step instances, then bridge to checkpoint via closure:

```go
for _, s := range allSteps {
    step := s // capture
    stepID := step.ID()
    resolver := func(output map[string]any) string {
        return step.StatusOnResume(output)
    }
    runner := func(ctx context.Context, sc workflow.StepContext) (map[string]any, error) {
        return step.Execute(ctx, sc)
    }
    runners[stepID] = o.checkpoints.WithCheckpointRecovery(
        stepID, resolver, task, agent, jobStep, runner,
        o.applyPatch, o.updateTaskStatus,
    )
}
```

- [x] **Step 4: Remove `makeStepsDeps` function**
- [x] **Step 5: Verify tests pass**

Run: `cd server && go test ./internal/orchestrator/... -v -count=1`
Expected: All tests pass.

- [x] **Step 6: Commit**

```bash
git add server/internal/orchestrator/step_registry.go server/internal/orchestrator/service_adapters.go server/internal/orchestrator/checkpoint/recovery.go
git commit -m "refactor(orchestrator): wire Step interface into registry with cycle-safe checkpoint recovery"
```

---

## Task 6: Clean Up Delegation Wrappers (Split: Step-Only vs Worker-Lifecycle)

**Files:**
- Modify: `server/internal/orchestrator/orchestrator.go`

### 6a: Audit call sites first (HARD GATE)

- [x] **Step 1: Run call-site audit before ANY removal**

```bash
cd server && grep -rn 'GetTaskWorkspace\|RemoveWorkspace\|pruneWorkspaces\|FindRepoWorkspaceByPath\|partialCleanupWorkspace' \
    --include='*.go' internal/orchestrator/ | grep -v '_test.go'
```

For each wrapper found in non-test code (e.g. `llm_trace.go` calls `o.GetTaskWorkspace`),
migrate the call site to use the sub-manager directly (`o.wkspace.LoadTaskWorkspace`)
**before** deleting the wrapper. For test-only references, update or delete the test.

### 6b: Remove wrappers used ONLY by steps (now accessed via service adapters)

- [x] **Step 2: Remove repoutil wrappers** — `getTaskRepoHostPath`, `hostWorktreePath`, `repoHostPath`, `applyPatch`, `captureWorkspaceDiff`, `capturePRDiff`, `getChangedFiles`, `loadTargetRepositories`, `setupRoleBranches`, `setupRoleWorktrees`, `commitRoleWorktrees`
- [x] **Step 3: Remove checkpoint wrappers** — `getSuccessfulCheckpoint`, `countSuccessfulCheckpoints`, `getSavedPatch`, `saveArtifact`

### 6c: Preserve worker-lifecycle wrappers

Keep: `ensureWorkspaceCloned`, `cleanupWorkspaceAfterFinalState`, `releaseWorkspaceLock`, `StartWorkspacePruner`, `StartLogPruner`

- [x] **Step 4: Migrate call sites and remove public workspace wrappers** — `GetTaskWorkspace`, `InitTaskWorkspace`, `FindRepoWorkspaceByPath`, `RemoveWorkspace`, `partialCleanupWorkspace`, `pruneWorkspaces`
- [x] **Step 5: Verify compilation and ALL tests pass (including orchestrator_test.go)**

Run: `cd server && go test ./internal/orchestrator/... -v -count=1`
Expected: All tests pass.

- [x] **Step 6: Commit**

```bash
git add server/internal/orchestrator/
git commit -m "refactor(orchestrator): remove step-only wrappers, migrate call sites, preserve worker lifecycle"
```

---

## Task 7: Remove Legacy `Deps` Struct

**Files:**
- Modify: `server/internal/orchestrator/steps/deps.go`
- Modify: `server/internal/orchestrator/interfaces.go`

**Interface ownership (final state):**
- `steps/services.go` — step-facing interfaces (the ONLY contracts steps depend on)
- `orchestrator/interfaces.go` — orchestrator-facing/repository interfaces (used by Orchestrator struct, worker.go, and service adapters)
- `service_adapters.go` — bridges between the two layers

- [x] **Step 1: Verify no `ExecuteXXX` free function is called anywhere**

```bash
cd server && grep -rn 'ExecuteContextLoad\|ExecuteAnalyze\|ExecutePlan\|ExecuteCodeBackend\|ExecuteCodeFrontend\|ExecuteMerge\|ExecuteReview\|ExecuteFix\|ExecuteTest\|ExecutePR' \
    --include='*.go' internal/orchestrator/ | grep -v '_test.go'
```

Expected: Zero matches (all wired through Step.Execute now).

- [x] **Step 2: Delete `Deps` struct and its 7 duplicate interfaces from `deps.go`**
- [x] **Step 3: Delete the old `ExecuteXXX` free functions**
- [x] **Step 4: Run full test suite + vet**

Run: `cd server && go test ./internal/orchestrator/... -v -count=1 && go vet ./internal/orchestrator/...`
Expected: All pass.

- [x] **Step 5: Commit**

```bash
git add server/internal/orchestrator/
git commit -m "refactor(orchestrator): remove legacy Deps struct and ExecuteXXX free functions"
```

---

## Verification Checklist

- [x] All 10 steps implement `Step` interface (with `ID()`, `Execute()`, `StatusOnResume(output)`).
- [x] Each step declares only the interfaces it actually uses (no god-interfaces).
- [x] No step imports the `orchestrator` parent package (no circular dependency).
- [x] `steps.Deps` struct is fully removed.
- [x] All existing `orchestrator_test.go` tests still pass.
- [x] New unit tests exist for all 10 steps — each testing actual execution, not just `ID()`.
- [x] Nil-safety preserved: steps handle nil optional dependencies gracefully.
- [x] `orchestrator.go` has ≤350 lines (down from 554).
- [x] `checkpoint/recovery.go` has no hardcoded step→status switch.
- [x] `go vet ./internal/orchestrator/...` reports no issues.
- [x] `make dev` starts successfully.

---

## Risk Mitigation

| Risk | Mitigation |
| :--- | :--- |
| Breaking existing tests during migration | Keep `ExecuteXXX` free functions alongside new structs until Task 7 |
| Circular imports between `steps` and `orchestrator` | Steps depend only on `steps/services.go` interfaces, never on `orchestrator` package |
| Service adapter boilerplate | Adapters are thin wrappers (~5 lines each); grouped by domain |
| Checkpoint recovery coupling | `StatusOnResume(output)` method on Step eliminates hardcoded switch |
| Worker lifecycle wrappers wrongly removed | Task 6 has hard-gate audit before any removal |
| Nil dependency crashes | Nil Fallback Rule: constructors accept nil; Execute checks before calling |
