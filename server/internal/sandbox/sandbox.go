package sandbox

import (
	"context"
	"fmt"
	"maps"
	"strings"
	"time"

	"go.opentelemetry.io/otel"
)

const (
	NetworkModeDefault = ""
	NetworkModeNone    = "none"
	NetworkModeBridge  = "bridge"
)

type CommandRequest struct {
	TaskID      string
	AgentID     string
	Workspace   string
	Command     []string
	Env         map[string]string
	SecretEnv   map[string]string
	NetworkMode string
	Timeout     time.Duration
}

type CommandResult struct {
	ExitCode int
	Stdout   string
	Stderr   string
}

type Runtime interface {
	Run(ctx context.Context, req CommandRequest) (*CommandResult, error)
	Prewarm(ctx context.Context) error
}

type StubRuntime struct{}

func NewStubRuntime() *StubRuntime {
	return &StubRuntime{}
}

func (r *StubRuntime) Prewarm(ctx context.Context) error {
	return nil
}

func (r *StubRuntime) Run(ctx context.Context, req CommandRequest) (*CommandResult, error) {
	ctx, span := otel.Tracer("auto-code-os/sandbox").Start(ctx, "sandbox.stub.run")
	defer span.End()
	if err := validateCommand(req.Command); err != nil {
		return nil, err
	}
	if err := validateExecutionPolicy(req); err != nil {
		return nil, err
	}
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
	}
	return &CommandResult{
		ExitCode: 0,
		Stdout:   fmt.Sprintf("stub sandbox executed: %s", strings.Join(req.Command, " ")),
		Stderr:   "",
	}, nil
}

func mergedEnv(req CommandRequest) map[string]string {
	env := make(map[string]string, len(req.Env)+len(req.SecretEnv))
	maps.Copy(env, req.Env)
	maps.Copy(env, req.SecretEnv)
	return env
}
