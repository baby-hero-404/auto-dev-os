package evals

import (
	"context"
	"fmt"
	"strings"
)

type CandidateFunc func(ctx context.Context, input string) (string, error)

type Judge interface {
	Grade(ctx context.Context, item GoldenCase, output string) (Grade, error)
}

type Grade struct {
	Score     float64 `json:"score"`
	Reasoning string  `json:"reasoning"`
}

type CaseResult struct {
	CaseID    string  `json:"case_id"`
	Name      string  `json:"name"`
	Output    string  `json:"output"`
	Score     float64 `json:"score"`
	Reasoning string  `json:"reasoning"`
	Error     string  `json:"error,omitempty"`
}

type Result struct {
	Dataset         string       `json:"dataset"`
	AverageScore    float64      `json:"average_score"`
	Passing         bool         `json:"passing"`
	Threshold       float64      `json:"threshold"`
	CaseResults     []CaseResult `json:"case_results"`
	FailedCaseCount int          `json:"failed_case_count"`
}

type Evaluator struct {
	store DatasetStore
	judge Judge
}

func NewEvaluator(store DatasetStore, judge Judge) *Evaluator {
	return &Evaluator{store: store, judge: judge}
}

func (e *Evaluator) Run(ctx context.Context, datasetName string, threshold float64, candidate CandidateFunc) (Result, error) {
	if e.store == nil {
		return Result{}, fmt.Errorf("dataset store is required")
	}
	if e.judge == nil {
		return Result{}, fmt.Errorf("judge is required")
	}
	if candidate == nil {
		return Result{}, fmt.Errorf("candidate is required")
	}
	dataset, err := e.store.Get(ctx, datasetName)
	if err != nil {
		return Result{}, err
	}
	result := Result{Dataset: dataset.Name, Threshold: threshold}
	for _, item := range dataset.Cases {
		output, err := candidate(ctx, item.Input)
		caseResult := CaseResult{CaseID: item.ID, Name: item.Name, Output: output}
		if err != nil {
			caseResult.Error = err.Error()
			result.FailedCaseCount++
			result.CaseResults = append(result.CaseResults, caseResult)
			continue
		}
		grade, err := e.judge.Grade(ctx, item, output)
		if err != nil {
			caseResult.Error = err.Error()
			result.FailedCaseCount++
			result.CaseResults = append(result.CaseResults, caseResult)
			continue
		}
		caseResult.Score = clampScore(grade.Score)
		caseResult.Reasoning = grade.Reasoning
		result.AverageScore += caseResult.Score
		result.CaseResults = append(result.CaseResults, caseResult)
	}
	if len(result.CaseResults) > 0 {
		result.AverageScore = result.AverageScore / float64(len(result.CaseResults))
	}
	result.Passing = result.FailedCaseCount == 0 && result.AverageScore >= threshold
	return result, nil
}

type KeywordJudge struct{}

func (KeywordJudge) Grade(_ context.Context, item GoldenCase, output string) (Grade, error) {
	expected := strings.TrimSpace(strings.ToLower(item.ExpectedBehavior))
	if expected == "" {
		return Grade{Score: 1, Reasoning: "no expected behavior configured"}, nil
	}
	if strings.Contains(strings.ToLower(output), expected) {
		return Grade{Score: 1, Reasoning: "output contains expected behavior keyword"}, nil
	}
	return Grade{Score: 0, Reasoning: "output does not contain expected behavior keyword"}, nil
}

func clampScore(score float64) float64 {
	if score < 0 {
		return 0
	}
	if score > 1 {
		return 1
	}
	return score
}
