package workflow

import (
	"context"
	"testing"

	"github.com/auto-code-os/auto-code-os/server/pkg/models"
)

func TestDynamicDAGWorkflow_ConstraintsAndDependencies(t *testing.T) {
	runners := map[string]StepFunc{
		StepContextLoad:  func(_ context.Context, _ StepContext) (map[string]any, error) { return nil, nil },
		StepAnalyze:      func(_ context.Context, _ StepContext) (map[string]any, error) { return nil, nil },
		StepPlan:         func(_ context.Context, _ StepContext) (map[string]any, error) { return nil, nil },
		StepCodeBackend:  func(_ context.Context, _ StepContext) (map[string]any, error) { return nil, nil },
		StepCodeFrontend: func(_ context.Context, _ StepContext) (map[string]any, error) { return nil, nil },
		StepMerge:        func(_ context.Context, _ StepContext) (map[string]any, error) { return nil, nil },
		StepReview:       func(_ context.Context, _ StepContext) (map[string]any, error) { return nil, nil },
		StepFix:          func(_ context.Context, _ StepContext) (map[string]any, error) { return nil, nil },
		StepTest:         func(_ context.Context, _ StepContext) (map[string]any, error) { return nil, nil },
		StepPR:           func(_ context.Context, _ StepContext) (map[string]any, error) { return nil, nil },
	}

	units := []models.ExecutionUnit{
		{
			ID:        "setup-project",
			Objective: "Setup project structure",
			ExecutionProfile: models.ExecutionProfile{
				Agent: "backend",
			},
			Constraints: models.ExecutionConstraints{
				Parallelizable: false,
			},
		},
		{
			ID:        "unit-backend-1",
			Objective: "Create backend service",
			ExecutionProfile: models.ExecutionProfile{
				Agent: "backend",
			},
			Constraints: models.ExecutionConstraints{
				Parallelizable: true,
			},
		},
		{
			ID:        "unit-frontend-1",
			Objective: "Create frontend dashboard",
			ExecutionProfile: models.ExecutionProfile{
				Agent: "frontend",
			},
			Constraints: models.ExecutionConstraints{
				Parallelizable: false, // Not parallelizable
			},
		},
	}

	def := DynamicDAGWorkflow(runners, units)

	// Validate steps length
	// Standard: Context (0), Analyze (1), Plan (2)
	// Units: setup-project (code_backend_0) (3), unit-backend-1 (code_backend_1) (4), unit-frontend-1 (code_frontend_0) (5)
	// Remaining: Merge, Review, Fix, Test, PR (6-10)
	expectedSteps := 11 // 3 base + 3 units + 5 end steps
	if len(def.Steps) != expectedSteps {
		t.Fatalf("expected %d steps, got %d", expectedSteps, len(def.Steps))
	}

	// Map steps by ID for easy lookup
	stepMap := make(map[string]StepDefinition)
	for _, s := range def.Steps {
		stepMap[s.ID] = s
	}

	// 1. Verify unit-backend-1 (code_backend_1) depends on setup-project (code_backend_0)
	unit1, ok := stepMap["code_backend_1"]
	if !ok {
		t.Fatal("step code_backend_1 not found")
	}
	hasSetupDep := false
	for _, dep := range unit1.DependsOn {
		if dep == "code_backend_0" {
			hasSetupDep = true
		}
	}
	if !hasSetupDep {
		t.Errorf("expected code_backend_1 to depend on code_backend_0 (setup-project), got %v", unit1.DependsOn)
	}

	// 2. Verify unit-frontend-1 (code_frontend_0) which has parallelizable: false
	// It should depend on the immediately preceding unit (code_backend_1)
	unit2, ok := stepMap["code_frontend_0"]
	if !ok {
		t.Fatal("step code_frontend_0 not found")
	}
	hasPrevDep := false
	for _, dep := range unit2.DependsOn {
		if dep == "code_backend_1" {
			hasPrevDep = true
		}
	}
	if !hasPrevDep {
		t.Errorf("expected code_frontend_0 to depend on code_backend_1 due to parallelizable: false, got %v", unit2.DependsOn)
	}
}
