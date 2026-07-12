package workflow

import (
	"fmt"
	"strings"

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
	return HardWorkflow(runners, nil)
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

func MediumWorkflow(runners map[string]StepFunc, subtasks map[string][]string) Definition {
	statusSchema := Schema{Fields: map[string]FieldSchema{
		"status": {Type: FieldString, Required: false},
	}}

	steps := []StepDefinition{
		{ID: StepContextLoad, Name: "Context", OutputSchema: statusSchema, Run: runners[StepContextLoad]},
		{ID: StepAnalyze, Name: "Analyze", DependsOn: []string{StepContextLoad}, OutputSchema: statusSchema, Run: runners[StepAnalyze]},
		{ID: StepPlan, Name: "Plan", DependsOn: []string{StepAnalyze}, OutputSchema: statusSchema, Run: runners[StepPlan]},
	}

	var backendDependencies []string
	if beTasks, ok := subtasks["backend"]; ok && len(beTasks) > 0 {
		prev := StepPlan
		for i := range beTasks {
			id := fmt.Sprintf("%s_%d", StepCodeBackend, i)
			steps = append(steps, StepDefinition{
				ID:           id,
				Name:         fmt.Sprintf("Code Backend %d", i+1),
				DependsOn:    []string{prev},
				OutputSchema: statusSchema,
				Run:          runners[StepCodeBackend],
			})
			prev = id // Make them sequential
		}
		backendDependencies = []string{prev}
	} else {
		steps = append(steps, StepDefinition{ID: StepCodeBackend, Name: "Code Backend", DependsOn: []string{StepPlan}, OutputSchema: statusSchema, Run: runners[StepCodeBackend]})
		backendDependencies = []string{StepCodeBackend}
	}

	var frontendDependencies []string
	if feTasks, ok := subtasks["frontend"]; ok && len(feTasks) > 0 {
		prev := StepPlan
		for i := range feTasks {
			id := fmt.Sprintf("%s_%d", StepCodeFrontend, i)
			steps = append(steps, StepDefinition{
				ID:           id,
				Name:         fmt.Sprintf("Code Frontend %d", i+1),
				DependsOn:    []string{prev},
				OutputSchema: statusSchema,
				Run:          runners[StepCodeFrontend],
			})
			prev = id // Make them sequential
		}
		frontendDependencies = []string{prev}
	} else {
		steps = append(steps, StepDefinition{ID: StepCodeFrontend, Name: "Code Frontend", DependsOn: []string{StepPlan}, OutputSchema: statusSchema, Run: runners[StepCodeFrontend]})
		frontendDependencies = []string{StepCodeFrontend}
	}

	mergeDependsOn := append(backendDependencies, frontendDependencies...)

	steps = append(steps, []StepDefinition{
		{ID: StepMerge, Name: "Merge", DependsOn: mergeDependsOn, OutputSchema: statusSchema, Run: runners[StepMerge]},
		{ID: StepReview, Name: "Review", DependsOn: []string{StepMerge}, OutputSchema: statusSchema, Run: runners[StepReview]},
		{ID: StepFix, Name: "Fix", DependsOn: []string{StepReview}, OutputSchema: statusSchema, Run: runners[StepFix]},
		{ID: StepTest, Name: "Test", DependsOn: []string{StepFix}, OutputSchema: statusSchema, Run: runners[StepTest]},
		{ID: StepPR, Name: "PR", DependsOn: []string{StepTest}, OutputSchema: statusSchema, Run: runners[StepPR]},
	}...)

	return Definition{Name: "auto-code-os-medium-workflow", Steps: steps}
}

func DynamicDAGWorkflow(runners map[string]StepFunc, units []models.ExecutionUnit) Definition {
	statusSchema := Schema{Fields: map[string]FieldSchema{
		"status": {Type: FieldString, Required: false},
	}}

	steps := []StepDefinition{
		{ID: StepContextLoad, Name: "Context", OutputSchema: statusSchema, Run: runners[StepContextLoad]},
		{ID: StepAnalyze, Name: "Analyze", DependsOn: []string{StepContextLoad}, OutputSchema: statusSchema, Run: runners[StepAnalyze]},
		{ID: StepPlan, Name: "Plan", DependsOn: []string{StepAnalyze}, OutputSchema: statusSchema, Run: runners[StepPlan]},
	}

	idToStepID := make(map[string]string)
	var beCount, feCount int

	for _, unit := range units {
		agent := strings.ToLower(unit.ExecutionProfile.Agent)
		if agent == "frontend" {
			stepID := fmt.Sprintf("%s_%d", StepCodeFrontend, feCount)
			idToStepID[unit.ID] = stepID
			feCount++
		} else {
			stepID := fmt.Sprintf("%s_%d", StepCodeBackend, beCount)
			idToStepID[unit.ID] = stepID
			beCount++
		}
	}

	idToIndex := make(map[string]int)
	for i, u := range units {
		idToIndex[u.ID] = i
	}

	var beIndex, feIndex int
	var allStepIDs []string

	for idx, unit := range units {
		agent := strings.ToLower(unit.ExecutionProfile.Agent)
		var stepID string
		var runFunc StepFunc
		var name string

		if agent == "frontend" {
			stepID = fmt.Sprintf("%s_%d", StepCodeFrontend, feIndex)
			runFunc = runners[StepCodeFrontend]
			name = fmt.Sprintf("Code Frontend %d: %s", feIndex+1, unit.Objective)
			feIndex++
		} else {
			stepID = fmt.Sprintf("%s_%d", StepCodeBackend, beIndex)
			runFunc = runners[StepCodeBackend]
			name = fmt.Sprintf("Code Backend %d: %s", beIndex+1, unit.Objective)
			beIndex++
		}

		allStepIDs = append(allStepIDs, stepID)

		var dependsOn []string
		for _, dep := range unit.Dependencies {
			if depIdx, ok := idToIndex[dep]; ok && depIdx < idx {
				if mapped, ok := idToStepID[dep]; ok {
					dependsOn = append(dependsOn, mapped)
				}
			}
		}

		// Enforce setup-project (first unit) dependency for all subsequent units
		if idx > 0 && len(units) > 0 {
			firstUnitStepID := idToStepID[units[0].ID]
			if len(dependsOn) == 0 {
				dependsOn = []string{firstUnitStepID}
			}
		}

		// Respect parallelizable: false constraint
		if idx > 0 && !unit.Constraints.Parallelizable && len(units) > 0 {
			prevStepID := idToStepID[units[idx-1].ID]
			found := false
			for _, d := range dependsOn {
				if d == prevStepID {
					found = true
					break
				}
			}
			if !found {
				if len(dependsOn) == 1 && dependsOn[0] == StepPlan {
					dependsOn = []string{prevStepID}
				} else {
					dependsOn = append(dependsOn, prevStepID)
				}
			}
		}

		if len(dependsOn) == 0 {
			dependsOn = []string{StepPlan}
		}

		steps = append(steps, StepDefinition{
			ID:           stepID,
			Name:         name,
			DependsOn:    dependsOn,
			OutputSchema: statusSchema,
			Run:          runFunc,
		})
	}

	if len(units) == 0 {
		steps = append(steps, StepDefinition{ID: StepCodeBackend, Name: "Code Backend", DependsOn: []string{StepPlan}, OutputSchema: statusSchema, Run: runners[StepCodeBackend]})
		steps = append(steps, StepDefinition{ID: StepCodeFrontend, Name: "Code Frontend", DependsOn: []string{StepPlan}, OutputSchema: statusSchema, Run: runners[StepCodeFrontend]})
		allStepIDs = []string{StepCodeBackend, StepCodeFrontend}
	}

	steps = append(steps, []StepDefinition{
		{ID: StepMerge, Name: "Merge", DependsOn: allStepIDs, OutputSchema: statusSchema, Run: runners[StepMerge]},
		{ID: StepReview, Name: "Review", DependsOn: []string{StepMerge}, OutputSchema: statusSchema, Run: runners[StepReview]},
		{ID: StepFix, Name: "Fix", DependsOn: []string{StepReview}, OutputSchema: statusSchema, Run: runners[StepFix]},
		{ID: StepTest, Name: "Test", DependsOn: []string{StepFix}, OutputSchema: statusSchema, Run: runners[StepTest]},
		{ID: StepPR, Name: "PR", DependsOn: []string{StepTest}, OutputSchema: statusSchema, Run: runners[StepPR]},
	}...)

	return Definition{Name: "auto-code-os-dynamic-dag-workflow", Steps: steps}
}

func HardWorkflow(runners map[string]StepFunc, subtasks map[string][]string) Definition {
	def := MediumWorkflow(runners, subtasks)
	def.Name = "auto-code-os-hard-workflow"
	return def
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
	}
	if d, ok := desc[name]; ok {
		return d
	}
	return name
}
