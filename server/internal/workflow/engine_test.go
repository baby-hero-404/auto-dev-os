package workflow

import (
	"context"
	"reflect"
	"sync"
	"testing"
)

func TestEngine_RunExecutesParallelDAG(t *testing.T) {
	var mu sync.Mutex
	order := []string{}
	run := func(id string) StepFunc {
		return func(context.Context, StepContext) (map[string]any, error) {
			mu.Lock()
			order = append(order, id)
			mu.Unlock()
			return map[string]any{"status": "ok"}, nil
		}
	}
	def := Definition{Steps: []StepDefinition{
		{ID: "start", OutputSchema: Schema{Fields: map[string]FieldSchema{"status": {Type: FieldString, Required: true}}}, Run: run("start")},
		{ID: "left", DependsOn: []string{"start"}, OutputSchema: Schema{Fields: map[string]FieldSchema{"status": {Type: FieldString, Required: true}}}, Run: run("left")},
		{ID: "right", DependsOn: []string{"start"}, OutputSchema: Schema{Fields: map[string]FieldSchema{"status": {Type: FieldString, Required: true}}}, Run: run("right")},
		{ID: "end", DependsOn: []string{"left", "right"}, OutputSchema: Schema{Fields: map[string]FieldSchema{"status": {Type: FieldString, Required: true}}}, Run: run("end")},
	}}

	result, err := (&Engine{MaxParallel: 2}).Run(context.Background(), def, nil)
	if err != nil {
		t.Fatalf("Run returned error: %v", err)
	}
	if result.Status["end"] != StepStatusSuccess {
		t.Fatalf("expected end step success, got %q", result.Status["end"])
	}
	if order[0] != "start" || order[len(order)-1] != "end" {
		t.Fatalf("unexpected execution order: %v", order)
	}
}

func TestEngine_RunValidatesOutputSchema(t *testing.T) {
	def := Definition{Steps: []StepDefinition{{
		ID:           "bad",
		OutputSchema: Schema{Fields: map[string]FieldSchema{"status": {Type: FieldString, Required: true}}},
		Run: func(context.Context, StepContext) (map[string]any, error) {
			return map[string]any{"status": 12}, nil
		},
	}}}

	_, err := (&Engine{}).Run(context.Background(), def, nil)
	if err == nil {
		t.Fatal("expected schema validation error")
	}
}

func TestSchemaValidateAllowsStringSlices(t *testing.T) {
	schema := Schema{Fields: map[string]FieldSchema{"items": {Type: FieldArray, Required: true}}}
	err := schema.Validate(map[string]any{"items": []string{"a"}})
	if err != nil {
		t.Fatalf("expected []string to validate: %v", err)
	}
	if !reflect.DeepEqual(schema.Fields["items"].Type, FieldArray) {
		t.Fatal("schema mutated unexpectedly")
	}
}

func TestValidateDAG(t *testing.T) {
	def := Definition{Steps: []StepDefinition{
		{ID: "start"},
		{ID: "left", DependsOn: []string{"start"}},
		{ID: "right", DependsOn: []string{"start"}},
		{ID: "end", DependsOn: []string{"left", "right"}},
	}}

	order, err := ValidateDAG(def)
	if err != nil {
		t.Fatalf("expected valid DAG: %v", err)
	}
	if len(order) != 4 || order[0] != "start" || order[3] != "end" {
		t.Fatalf("unexpected order: %v", order)
	}
}

func TestValidateDAG_Cycle(t *testing.T) {
	def := Definition{Steps: []StepDefinition{
		{ID: "a", DependsOn: []string{"c"}},
		{ID: "b", DependsOn: []string{"a"}},
		{ID: "c", DependsOn: []string{"b"}},
	}}

	_, err := ValidateDAG(def)
	if err == nil {
		t.Fatal("expected cycle detection error")
	}
}

func TestEngine_ResumeFromCheckpoint(t *testing.T) {
	var mu sync.Mutex
	executed := []string{}
	run := func(id string) StepFunc {
		return func(context.Context, StepContext) (map[string]any, error) {
			mu.Lock()
			executed = append(executed, id)
			mu.Unlock()
			return map[string]any{"status": "ok"}, nil
		}
	}

	def := Definition{Steps: []StepDefinition{
		{ID: "analyze", Run: run("analyze")},
		{ID: "code", DependsOn: []string{"analyze"}, Run: run("code")},
		{ID: "test", DependsOn: []string{"code"}, Run: run("test")},
		{ID: "pr", DependsOn: []string{"test"}, Run: run("pr")},
	}}

	// Simulate: "analyze" already succeeded in a previous run.
	engine := &Engine{
		MaxParallel: 1,
		CompletedSteps: map[string]map[string]any{
			"analyze": {"status": "ok"},
		},
	}

	result, err := engine.Run(context.Background(), def, nil)
	if err != nil {
		t.Fatalf("Run returned error: %v", err)
	}
	if result.Status["pr"] != StepStatusSuccess {
		t.Fatalf("expected pr step success, got %q", result.Status["pr"])
	}
	// "analyze" should NOT have been re-executed
	for _, id := range executed {
		if id == "analyze" {
			t.Fatal("analyze step should have been skipped (was in CompletedSteps)")
		}
	}
	// code, test, pr should have run
	if len(executed) != 3 {
		t.Fatalf("expected 3 steps executed (code, test, pr), got %d: %v", len(executed), executed)
	}
}
