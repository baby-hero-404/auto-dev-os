package evals

import (
	"context"
	"testing"
)

func TestEvaluator_RunPassesWhenAverageMeetsThreshold(t *testing.T) {
	store := NewMemoryDatasetStore(Dataset{
		Name: "skills",
		Cases: []GoldenCase{
			{ID: "1", Name: "patch", Input: "edit file", ExpectedBehavior: "apply_patch"},
			{ID: "2", Name: "tests", Input: "verify", ExpectedBehavior: "run_tests"},
		},
	})
	evaluator := NewEvaluator(store, KeywordJudge{})

	result, err := evaluator.Run(context.Background(), "skills", 0.9, func(_ context.Context, input string) (string, error) {
		if input == "edit file" {
			return "use apply_patch", nil
		}
		return "use run_tests", nil
	})
	if err != nil {
		t.Fatalf("Run returned error: %v", err)
	}
	if !result.Passing {
		t.Fatalf("expected passing result: %+v", result)
	}
	if result.AverageScore != 1 {
		t.Fatalf("expected perfect score, got %f", result.AverageScore)
	}
}

func TestEvaluator_RunFailsBelowThreshold(t *testing.T) {
	store := NewMemoryDatasetStore(Dataset{
		Name:  "skills",
		Cases: []GoldenCase{{ID: "1", Name: "patch", Input: "edit file", ExpectedBehavior: "apply_patch"}},
	})
	evaluator := NewEvaluator(store, KeywordJudge{})

	result, err := evaluator.Run(context.Background(), "skills", 0.9, func(context.Context, string) (string, error) {
		return "rewrite the full file", nil
	})
	if err != nil {
		t.Fatalf("Run returned error: %v", err)
	}
	if result.Passing {
		t.Fatalf("expected failing result: %+v", result)
	}
}
