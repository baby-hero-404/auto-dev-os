package engine

import (
	"context"
	"errors"
	"testing"
)

func TestAPINativeEngine_Passthrough(t *testing.T) {
	want := &CodeStepResult{Success: true, Output: "done"}
	var gotReq CodeStepRequest
	e := NewAPINativeEngine(func(ctx context.Context, req CodeStepRequest) (*CodeStepResult, error) {
		gotReq = req
		return want, nil
	})

	req := CodeStepRequest{Instruction: "do the thing"}
	got, err := e.RunCodeStep(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != want {
		t.Errorf("expected delegate result to pass through unchanged")
	}
	if gotReq.Instruction != "do the thing" {
		t.Errorf("delegate did not receive the original request")
	}
	if e.Name() != "api_native" {
		t.Errorf("Name() = %q, want api_native", e.Name())
	}
	if _, err := e.Preflight(context.Background(), req); err != nil {
		t.Errorf("api_native Preflight should always succeed, got %v", err)
	}
}

func TestAPINativeEngine_NoDelegate(t *testing.T) {
	e := NewAPINativeEngine(nil)
	_, err := e.RunCodeStep(context.Background(), CodeStepRequest{})
	if err == nil {
		t.Fatal("expected error when no delegate configured")
	}
}

func TestAPINativeEngine_DelegateError(t *testing.T) {
	wantErr := errors.New("boom")
	e := NewAPINativeEngine(func(ctx context.Context, req CodeStepRequest) (*CodeStepResult, error) {
		return nil, wantErr
	})
	_, err := e.RunCodeStep(context.Background(), CodeStepRequest{})
	if !errors.Is(err, wantErr) {
		t.Errorf("expected delegate error to propagate, got %v", err)
	}
}
