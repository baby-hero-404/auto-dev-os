package workflow

import (
	"github.com/auto-code-os/auto-code-os/server/pkg/models"
)

// Step ID constants — canonical names used across the workflow engine and orchestrator.
const (
	StepContextLoad  = "context_load"
	StepAnalyze      = "analyze"
	StepPlan         = "plan"
	StepCodeBackend  = "code_backend"
	StepCodeFrontend = "code_frontend"
	StepMerge        = "merge"
	StepReview       = "review"
	StepFix          = "fix"
	StepTest         = "test"
	StepPR           = "pr"

	// CLI spec-first pipeline (engine=cli): cli_analyze -> cli_spec -> cli_implement -> [cross_review] -> cli_mr.
	StepCLIAnalyze   = "cli_analyze"
	StepCLISpec      = "cli_spec"
	StepCLIImplement = "cli_implement"
	// StepCLIImplementBackend/Frontend are the parallel CLI implement tracks
	// (Phase 4): two CLI agents implement against the same approved spec set
	// in their own git worktrees (models.WorktreeSuffixBackend/Frontend),
	// then StepMerge reconciles both — mirroring StepCodeBackend/Frontend's
	// existing pattern in MediumWorkflow.
	StepCLIImplementBackend  = "cli_implement_backend"
	StepCLIImplementFrontend = "cli_implement_frontend"
	StepCrossReview          = "cross_review"
	StepCLIMR                = "cli_mr"
)

// Step status constants used across the engine and API.
const (
	StepStatusPending         = "pending"
	StepStatusRunning         = "running"
	StepStatusSuccess         = "success"
	StepStatusFailed          = "failed"
	StepStatusSkipped         = "skipped"
	StepStatusPaused          = "paused"
	StepStatusWaitingApproval = "waiting_approval"
)

// CLISpecFirstWorkflow builds the 4-step pipeline used when a task resolves
// to engine "cli": the black-box CLI agent owns its own tool-loop, context
// loading, and planning, so the server's job is reduced to spec-first
// gating (analyze -> author spec -> implement against it -> open the MR)
// rather than the API-native DAG's per-tool-call orchestration.
// includeCrossReview inserts an optional cross_review node between
// cli_implement and cli_mr — used when the project's review_harness_policy
// is not "same", so CLI-coded work still gets an independent AI review pass
// before the MR is opened (cross-harness-review REQ-M02).
func CLISpecFirstWorkflow(runners map[string]StepFunc, includeCrossReview bool) Definition {
	statusSchema := Schema{Fields: map[string]FieldSchema{
		"status": {Type: FieldString, Required: false},
	}}

	steps := []StepDefinition{
		{ID: StepCLIAnalyze, Name: "Analyze", OutputSchema: statusSchema, Run: runners[StepCLIAnalyze]},
		{ID: StepCLISpec, Name: "Author Spec", DependsOn: []string{StepCLIAnalyze}, OutputSchema: statusSchema, Run: runners[StepCLISpec]},
		{ID: StepCLIImplement, Name: "Implement", DependsOn: []string{StepCLISpec}, OutputSchema: statusSchema, Run: runners[StepCLIImplement]},
	}

	lastStep := StepCLIImplement
	if includeCrossReview {
		steps = append(steps, StepDefinition{ID: StepCrossReview, Name: "Cross-Harness Review", DependsOn: []string{StepCLIImplement}, OutputSchema: statusSchema, Run: runners[StepCrossReview]})
		lastStep = StepCrossReview
	}
	steps = append(steps, StepDefinition{ID: StepCLIMR, Name: "Merge Request", DependsOn: []string{lastStep}, OutputSchema: statusSchema, Run: runners[StepCLIMR]})

	return Definition{Name: "auto-code-os-cli-spec-first-workflow", Steps: steps}
}

// CLISpecFirstParallelWorkflow is CLISpecFirstWorkflow's dual-agent variant
// (Phase 4): after the spec is authored, two CLI agents implement in
// parallel — one in the backend role worktree, one in the frontend role
// worktree (mirrors MediumWorkflow's code_backend/code_frontend split) —
// then StepMerge reconciles both role branches into the integration branch
// before the optional cross_review and cli_mr steps.
func CLISpecFirstParallelWorkflow(runners map[string]StepFunc, includeCrossReview bool) Definition {
	statusSchema := Schema{Fields: map[string]FieldSchema{
		"status": {Type: FieldString, Required: false},
	}}

	steps := []StepDefinition{
		{ID: StepCLIAnalyze, Name: "Analyze", OutputSchema: statusSchema, Run: runners[StepCLIAnalyze]},
		{ID: StepCLISpec, Name: "Author Spec", DependsOn: []string{StepCLIAnalyze}, OutputSchema: statusSchema, Run: runners[StepCLISpec]},
		{ID: StepCLIImplementBackend, Name: "Implement (Backend)", DependsOn: []string{StepCLISpec}, OutputSchema: statusSchema, Run: runners[StepCLIImplementBackend]},
		{ID: StepCLIImplementFrontend, Name: "Implement (Frontend)", DependsOn: []string{StepCLISpec}, OutputSchema: statusSchema, Run: runners[StepCLIImplementFrontend]},
		{ID: StepMerge, Name: "Merge", DependsOn: []string{StepCLIImplementBackend, StepCLIImplementFrontend}, OutputSchema: statusSchema, Run: runners[StepMerge]},
	}

	lastStep := StepMerge
	if includeCrossReview {
		steps = append(steps, StepDefinition{ID: StepCrossReview, Name: "Cross-Harness Review", DependsOn: []string{StepMerge}, OutputSchema: statusSchema, Run: runners[StepCrossReview]})
		lastStep = StepCrossReview
	}
	steps = append(steps, StepDefinition{ID: StepCLIMR, Name: "Merge Request", DependsOn: []string{lastStep}, OutputSchema: statusSchema, Run: runners[StepCLIMR]})

	return Definition{Name: "auto-code-os-cli-spec-first-parallel-workflow", Steps: steps}
}

// DescribeStep returns a human-readable description for a step name.
func DescribeStep(name string) string {
	desc := map[string]string{
		"context_load":  "Load repository context and conventions",
		"analyze":       "Analyze task complexity & scope",
		"plan":          "Parse OpenSpec & prepare subtask assignments",
		"code_backend":  "Execute backend code changes in sandbox",
		"code_frontend": "Execute frontend code changes in sandbox",
		"merge":         "Merge parallel code & resolve conflicts",
		"review":        "AI code review",
		"fix":           "Fix review feedback",
		"test":          "Run test suite",
		"pr":            "Create pull request",
		"cli_analyze":   "Analyze repo & task with the CLI agent",
		"cli_spec":      "Author OpenSpec set with the CLI agent",
		"cli_implement": "Implement against the approved spec",
		"cross_review":  "Independent cross-harness AI review of CLI-coded changes",
		"cli_mr":        "Create merge request",
	}
	if d, ok := desc[name]; ok {
		return d
	}
	return name
}

// StatusForStep returns the task status a paused-and-resumed job should be
// set to, keyed by which step raised the pause (docs/openspecs/
// cli-execution-reliability, REQ-006). Mirrors each CLI spec-first step's
// own StatusOnResume without importing the orchestrator/steps package
// (workflow sits below it in the dependency graph) — TaskService.Clarify
// uses this to resume a paused CLI step at its own status instead of always
// jumping back to TaskStatusAnalyzing (the correct behavior only for the
// legacy API-native flow, where clarification always originates at, and
// resumes to, the "analyze" step).
func StatusForStep(step string) string {
	switch step {
	case StepCLIImplement, StepCLIImplementBackend, StepCLIImplementFrontend:
		return models.TaskStatusCoding
	case StepCLIAnalyze, StepCLISpec:
		return models.TaskStatusAnalyzing
	default:
		return models.TaskStatusAnalyzing
	}
}
