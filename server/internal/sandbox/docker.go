package sandbox

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/mount"
	"github.com/docker/docker/client"
	"github.com/docker/docker/pkg/stdcopy"
)

type DockerConfig struct {
	Image             string
	WorkspaceRoot     string
	MemoryBytes       int64
	NanoCPUs          int64
	DisableNetworking bool
}

type DockerRuntime struct {
	client *client.Client
	config DockerConfig
}

func NewDockerRuntime(config DockerConfig) (*DockerRuntime, error) {
	if config.Image == "" {
		config.Image = "auto-code-os-sandbox:latest"
	}
	if config.WorkspaceRoot == "" {
		config.WorkspaceRoot = "/tmp/auto-code-os/workspaces"
	}
	cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		return nil, fmt.Errorf("create docker client: %w", err)
	}
	return &DockerRuntime{client: cli, config: config}, nil
}

func (r *DockerRuntime) Run(ctx context.Context, req CommandRequest) (*CommandResult, error) {
	if err := validateCommand(req.Command); err != nil {
		return nil, err
	}
	if err := validateExecutionPolicy(req); err != nil {
		return nil, err
	}
	if len(req.Command) == 0 {
		return nil, fmt.Errorf("docker command is required")
	}
	if req.Timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, req.Timeout)
		defer cancel()
	}

	workspace := req.Workspace
	if workspace == "" {
		workspace = WorkspacePath(r.config.WorkspaceRoot, req.TaskID)
	}
	if err := os.MkdirAll(workspace, 0o755); err != nil {
		return nil, fmt.Errorf("create sandbox workspace: %w", err)
	}

	envMap := mergedEnv(req)
	env := make([]string, 0, len(envMap))
	for key, value := range envMap {
		env = append(env, key+"="+value)
	}

	networkMode := container.NetworkMode("none")
	if req.NetworkMode == NetworkModeBridge || (req.NetworkMode == NetworkModeDefault && !r.config.DisableNetworking) {
		networkMode = "bridge"
	}

	createResp, err := r.client.ContainerCreate(ctx, &container.Config{
		Image:      r.config.Image,
		Cmd:        req.Command,
		Env:        env,
		WorkingDir: "/workspace",
	}, &container.HostConfig{
		NetworkMode: networkMode,
		Resources: container.Resources{
			Memory:   r.config.MemoryBytes,
			NanoCPUs: r.config.NanoCPUs,
		},
		Mounts: []mount.Mount{
			{
				Type:   mount.TypeBind,
				Source: workspace,
				Target: "/workspace",
			},
		},
	}, nil, nil, "")
	if err != nil {
		return nil, fmt.Errorf("create docker container: %w", err)
	}
	containerID := createResp.ID
	defer func() {
		timeout := 5
		_ = r.client.ContainerStop(context.Background(), containerID, container.StopOptions{Timeout: &timeout})
		_ = r.client.ContainerRemove(context.Background(), containerID, container.RemoveOptions{Force: true})
	}()

	if err := r.client.ContainerStart(ctx, containerID, container.StartOptions{}); err != nil {
		return nil, fmt.Errorf("start docker container: %w", err)
	}

	waitCh, errCh := r.client.ContainerWait(ctx, containerID, container.WaitConditionNotRunning)
	var statusCode int64
	select {
	case err := <-errCh:
		if err != nil {
			return nil, fmt.Errorf("wait for docker container: %w", err)
		}
	case waitResp := <-waitCh:
		statusCode = waitResp.StatusCode
	case <-ctx.Done():
		return nil, ctx.Err()
	}

	logReader, err := r.client.ContainerLogs(ctx, containerID, container.LogsOptions{
		ShowStdout: true,
		ShowStderr: true,
	})
	if err != nil {
		return nil, fmt.Errorf("read docker container logs: %w", err)
	}
	defer logReader.Close()

	stdout, stderr, err := splitDockerLogs(logReader)
	if err != nil {
		return nil, err
	}
	return &CommandResult{ExitCode: int(statusCode), Stdout: stdout, Stderr: stderr}, nil
}

func splitDockerLogs(reader io.Reader) (string, string, error) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	if _, err := stdcopy.StdCopy(&stdout, &stderr, reader); err != nil {
		return "", "", fmt.Errorf("copy docker logs: %w", err)
	}
	return strings.TrimSpace(stdout.String()), strings.TrimSpace(stderr.String()), nil
}
