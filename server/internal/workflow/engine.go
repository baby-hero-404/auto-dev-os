package workflow

import (
	"context"
	"errors"
	"fmt"
	"sync"
)

var ErrPaused = errors.New("workflow paused")

type PauseError struct {
	Step   string
	Reason string
}

func (e PauseError) Error() string {
	if e.Reason == "" {
		return ErrPaused.Error()
	}
	return e.Reason
}

func (e PauseError) Is(target error) bool {
	return target == ErrPaused
}

type StepFunc func(context.Context, StepContext) (map[string]any, error)

type StepDefinition struct {
	ID           string
	Name         string
	DependsOn    []string
	InputSchema  Schema
	OutputSchema Schema
	Run          StepFunc
}

type Definition struct {
	Name  string
	Steps []StepDefinition
}

type StepContext struct {
	Workflow string
	StepID   string
	Inputs   map[string]map[string]any
	Initial  map[string]any
}

type Event struct {
	StepID string
	Status string
	Output map[string]any
	Error  string
}

type Engine struct {
	MaxParallel int
	OnEvent     func(context.Context, Event) error
}

type Result struct {
	Outputs map[string]map[string]any
	Status  map[string]string
	Paused  bool
}

func (e *Engine) Run(ctx context.Context, def Definition, initial map[string]any) (Result, error) {
	if e.MaxParallel <= 0 {
		e.MaxParallel = 4
	}
	if err := validateDefinition(def); err != nil {
		return Result{}, err
	}

	result := Result{Outputs: map[string]map[string]any{}, Status: map[string]string{}}
	steps := map[string]StepDefinition{}
	for _, step := range def.Steps {
		steps[step.ID] = step
		result.Status[step.ID] = StepStatusPending
	}

	for {
		ready := readySteps(def.Steps, result.Status)
		if len(ready) == 0 {
			break
		}
		if len(ready) > e.MaxParallel {
			ready = ready[:e.MaxParallel]
		}

		type stepRunResult struct {
			id     string
			output map[string]any
			err    error
		}
		ch := make(chan stepRunResult, len(ready))
		var wg sync.WaitGroup
		for _, step := range ready {
			step := step
			result.Status[step.ID] = StepStatusRunning
			if err := e.emit(ctx, Event{StepID: step.ID, Status: StepStatusRunning}); err != nil {
				return result, err
			}
			wg.Add(1)
			go func() {
				defer wg.Done()
				output, err := runStep(ctx, def.Name, step, initial, result.Outputs)
				ch <- stepRunResult{id: step.ID, output: output, err: err}
			}()
		}
		wg.Wait()
		close(ch)

		for item := range ch {
			if item.err != nil {
				status := StepStatusFailed
				if errors.Is(item.err, ErrPaused) {
					status = StepStatusPaused
					result.Paused = true
				}
				result.Status[item.id] = status
				_ = e.emit(ctx, Event{StepID: item.id, Status: status, Error: item.err.Error()})
				return result, item.err
			}
			result.Outputs[item.id] = item.output
			result.Status[item.id] = StepStatusSuccess
			if err := e.emit(ctx, Event{StepID: item.id, Status: StepStatusSuccess, Output: item.output}); err != nil {
				return result, err
			}
		}
	}

	for _, status := range result.Status {
		if status != StepStatusSuccess && status != StepStatusSkipped {
			return result, fmt.Errorf("workflow ended with incomplete steps")
		}
	}
	return result, nil
}

func runStep(ctx context.Context, workflowName string, step StepDefinition, initial map[string]any, outputs map[string]map[string]any) (map[string]any, error) {
	if step.Run == nil {
		return nil, fmt.Errorf("step %q has no runner", step.ID)
	}
	input := aggregateInputs(step.DependsOn, outputs, initial)
	if len(step.InputSchema.Fields) > 0 {
		if err := step.InputSchema.Validate(input); err != nil {
			return nil, fmt.Errorf("step %q input validation failed: %w", step.ID, err)
		}
	}
	output, err := step.Run(ctx, StepContext{Workflow: workflowName, StepID: step.ID, Inputs: outputs, Initial: initial})
	if err != nil {
		return nil, err
	}
	if output == nil {
		output = map[string]any{}
	}
	if len(step.OutputSchema.Fields) > 0 {
		if err := step.OutputSchema.Validate(output); err != nil {
			return nil, fmt.Errorf("step %q output validation failed: %w", step.ID, err)
		}
	}
	return output, nil
}

func aggregateInputs(dependsOn []string, outputs map[string]map[string]any, initial map[string]any) map[string]any {
	input := map[string]any{}
	for k, v := range initial {
		input[k] = v
	}
	for _, dep := range dependsOn {
		for k, v := range outputs[dep] {
			input[k] = v
		}
	}
	return input
}

func readySteps(steps []StepDefinition, status map[string]string) []StepDefinition {
	ready := []StepDefinition{}
	for _, step := range steps {
		if status[step.ID] != StepStatusPending {
			continue
		}
		ok := true
		for _, dep := range step.DependsOn {
			if status[dep] != StepStatusSuccess && status[dep] != StepStatusSkipped {
				ok = false
				break
			}
		}
		if ok {
			ready = append(ready, step)
		}
	}
	return ready
}

func validateDefinition(def Definition) error {
	seen := map[string]bool{}
	for _, step := range def.Steps {
		if step.ID == "" {
			return fmt.Errorf("workflow step id is required")
		}
		if seen[step.ID] {
			return fmt.Errorf("duplicate workflow step %q", step.ID)
		}
		seen[step.ID] = true
	}
	for _, step := range def.Steps {
		for _, dep := range step.DependsOn {
			if !seen[dep] {
				return fmt.Errorf("step %q depends on unknown step %q", step.ID, dep)
			}
		}
	}
	return nil
}

func (e *Engine) emit(ctx context.Context, event Event) error {
	if e.OnEvent == nil {
		return nil
	}
	return e.OnEvent(ctx, event)
}
