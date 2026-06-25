package workflow

// Step ID constants — canonical names used across the workflow engine and orchestrator.
const (
	StepContextLoad  = "context_load"
	StepAnalyze      = "analyze"
	StepPlan         = "plan"
	StepCode         = "code"
	StepCodeBackend  = "code_backend"
	StepCodeFrontend = "code_frontend"
	StepMerge        = "merge"
	StepReview       = "review"
	StepFix          = "fix"
	StepTest         = "test"
	StepPR           = "pr"
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

// StepNameOrder returns the canonical step names in execution order.
func StepNameOrder() []string {
	return []string{
		StepContextLoad, StepAnalyze, StepPlan, StepCodeBackend, StepCodeFrontend, StepMerge, StepReview, StepFix, StepTest, StepPR,
	}
}

// DefaultWorkflow creates the canonical 10-step DAG definition.
// It accepts a map of step runners keyed by step ID constants.
// The orchestrator provides these runners via its stepRunners method.
func DefaultWorkflow(runners map[string]StepFunc) Definition {
	return HardWorkflow(runners)
}

func EasyWorkflow(runners map[string]StepFunc) Definition {
	statusSchema := Schema{Fields: map[string]FieldSchema{
		"status": {Type: FieldString, Required: false},
	}}

	steps := []StepDefinition{
		{ID: StepContextLoad, Name: "Context", OutputSchema: statusSchema, Run: runners[StepContextLoad]},
		{ID: StepAnalyze, Name: "Analyze", DependsOn: []string{StepContextLoad}, OutputSchema: statusSchema, Run: runners[StepAnalyze]},
		{ID: StepCodeBackend, Name: "Code", DependsOn: []string{StepAnalyze}, OutputSchema: statusSchema, Run: runners[StepCodeBackend]},
		{ID: StepTest, Name: "Test", DependsOn: []string{StepCodeBackend}, OutputSchema: statusSchema, Run: runners[StepTest]},
		{ID: StepPR, Name: "PR", DependsOn: []string{StepTest}, OutputSchema: statusSchema, Run: runners[StepPR]},
	}

	return Definition{Name: "auto-code-os-easy-workflow", Steps: steps}
}

func MediumWorkflow(runners map[string]StepFunc) Definition {
	statusSchema := Schema{Fields: map[string]FieldSchema{
		"status": {Type: FieldString, Required: false},
	}}

	steps := []StepDefinition{
		{ID: StepContextLoad, Name: "Context", OutputSchema: statusSchema, Run: runners[StepContextLoad]},
		{ID: StepAnalyze, Name: "Analyze", DependsOn: []string{StepContextLoad}, OutputSchema: statusSchema, Run: runners[StepAnalyze]},
		{ID: StepPlan, Name: "Plan", DependsOn: []string{StepAnalyze}, OutputSchema: statusSchema, Run: runners[StepPlan]},
		{ID: StepCodeBackend, Name: "Code Backend", DependsOn: []string{StepPlan}, OutputSchema: statusSchema, Run: runners[StepCodeBackend]},
		{ID: StepCodeFrontend, Name: "Code Frontend", DependsOn: []string{StepPlan}, OutputSchema: statusSchema, Run: runners[StepCodeFrontend]},
		{ID: StepMerge, Name: "Merge", DependsOn: []string{StepCodeBackend, StepCodeFrontend}, OutputSchema: statusSchema, Run: runners[StepMerge]},
		{ID: StepReview, Name: "Review", DependsOn: []string{StepMerge}, OutputSchema: statusSchema, Run: runners[StepReview]},
		{ID: StepFix, Name: "Fix", DependsOn: []string{StepReview}, OutputSchema: statusSchema, Run: runners[StepFix]},
		{ID: StepTest, Name: "Test", DependsOn: []string{StepFix}, OutputSchema: statusSchema, Run: runners[StepTest]},
		{ID: StepPR, Name: "PR", DependsOn: []string{StepTest}, OutputSchema: statusSchema, Run: runners[StepPR]},
	}

	return Definition{Name: "auto-code-os-medium-workflow", Steps: steps}
}

func HardWorkflow(runners map[string]StepFunc) Definition {
	def := MediumWorkflow(runners)
	def.Name = "auto-code-os-hard-workflow"
	return def
}

// DescribeStep returns a human-readable description for a step name.
func DescribeStep(name string) string {
	desc := map[string]string{
		"context_load": "Load repository context and conventions",
		"analyze":      "Analyze task complexity & scope",
		"plan":         "Decompose into sub-tasks",
		"code":         "Execute code changes in sandbox",
		"merge":        "Merge parallel code & resolve conflicts",
		"review":       "AI code review",
		"fix":          "Fix review feedback",
		"test":         "Run test suite",
		"pr":           "Create pull request",
	}
	if d, ok := desc[name]; ok {
		return d
	}
	return name
}
