package sandbox

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/image"
	"github.com/docker/docker/api/types/mount"
	"github.com/docker/docker/client"
	"github.com/docker/docker/pkg/stdcopy"
	"go.opentelemetry.io/otel"
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

func (r *DockerRuntime) Health(ctx context.Context) error {
	if _, err := r.client.Ping(ctx); err != nil {
		return fmt.Errorf("ping docker daemon: %w", err)
	}
	return nil
}

func (r *DockerRuntime) Prewarm(ctx context.Context) error {
	// First check if the image already exists locally.
	_, err := r.client.ImageInspect(ctx, r.config.Image)
	if err == nil {
		return nil
	}

	reader, err := r.client.ImagePull(ctx, r.config.Image, image.PullOptions{})
	if err != nil {
		return fmt.Errorf("pull sandbox image: %w", err)
	}
	defer reader.Close()
	_, _ = io.Copy(io.Discard, reader)
	return nil
}

// resolveNetworkMode maps the requested mode to the mode the container will
// actually run with, resolving NetworkModeDefault against the runtime's
// DisableNetworking config. This single resolution point feeds both policy
// validation and ContainerCreate, so the two can never disagree about
// whether the container has network access.
//
// "bridge"/default-with-networking resolve to Docker's own default bridge
// network ("bridge") — NAT'd egress, no host-namespace sharing. This used
// to literally return "host" (full host network-namespace access: every
// interface, every 127.0.0.1 service on the host, visible to the
// container), which contradicted both the NetworkModeBridge name and
// policy.go's SecretEnv exposure gating, which assumes bridge is a lower-
// exposure mode than host. NetworkModeNone is unaffected.
func (r *DockerRuntime) resolveNetworkMode(requested string) string {
	if requested == NetworkModeBridge || (requested == NetworkModeDefault && !r.config.DisableNetworking) {
		return "bridge"
	}
	return NetworkModeNone
}

// credentialFilesOverlap reports whether authDirTarget (a directory or file
// path mounted by the host-session fallback) equals or contains any target
// path in credentialFiles (the DB-linked credential's per-file mounts). A
// per-org CredentialFiles entry targeting a path always wins over the
// authDirs fallback — but if authDirTarget is a parent directory of a
// CredentialFiles path (e.g. authDirFiles mounts ".claude/.credentials.json"
// as a whole read-only file while CredentialFiles mounts a path inside the
// same directory), both mounts being added makes Docker try to bind-mount a
// file inside an already read-only-mounted directory, which runc rejects at
// container start ("make mountpoint ...: read-only file system"). Checking
// only exact-path equality missed this containment case.
//
// Only authDirFiles (still read-only) needs this check. authDirTrees no
// longer does: it mounts a writable staged copy (see copyDirTree), and
// nesting a CredentialFiles bind mount inside an already-writable parent
// mount is not rejected by runc the way read-only parents are — so the real
// per-org credential file and the writable staged home directory it lives
// alongside (giving the CLI somewhere to write "brain"/"conversations"/etc)
// can now coexist instead of the credential forcing the whole tree mount to
// be skipped.
func credentialFilesOverlap(authDirTarget string, credentialFiles map[string]string) bool {
	for credTarget := range credentialFiles {
		if credTarget == authDirTarget || strings.HasPrefix(credTarget, authDirTarget+"/") {
			return true
		}
	}
	return false
}

// copyDirTree recursively copies src into dst, preserving file modes and
// symlinks. Used to stage a host CLI config directory (e.g. ~/.gemini) into
// a throwaway writable location before mounting it into the sandbox — see
// the authDirTrees comment in Run for why a plain read-only bind mount of
// the host directory doesn't work here.
func copyDirTree(src, dst string) error {
	return filepath.WalkDir(src, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		target := filepath.Join(dst, rel)
		if rel == "." {
			// dst itself (created by the caller via os.MkdirTemp, which
			// defaults to 0700) is the bind mount's target root — leaving
			// it owner-only would block the container's "agent" UID from
			// even entering it, regardless of how permissive the entries
			// inside it are.
			return os.Chmod(dst, 0777)
		}

		switch {
		case d.Type()&os.ModeSymlink != 0:
			linkTarget, err := os.Readlink(path)
			if err != nil {
				return err
			}
			return os.Symlink(linkTarget, target)
		case d.IsDir():
			// 0777 (not just owner-writable): bind mounts carry host-side
			// permission bits as-is, with no UID remapping. The staging
			// copy is written by the host server process's UID, but read
			// inside the container by the fixed "agent" UID from
			// docker/Dockerfile.sandbox (1000 by default) — a different
			// UID entirely. Owner-only bits (e.g. 0700) grant that UID
			// nothing, so mkdir/write still fail with the exact same
			// "permission denied" this staging copy exists to fix. The
			// directory is a throwaway per-run copy anyway (see the
			// authDirTrees comment above), so there's no confidentiality
			// reason to restrict it beyond the temp dir itself.
			return os.MkdirAll(target, 0777)
		default:
			data, err := os.ReadFile(path)
			if err != nil {
				return err
			}
			return os.WriteFile(target, data, 0666)
		}
	})
}

func (r *DockerRuntime) Run(ctx context.Context, req CommandRequest) (*CommandResult, error) {
	ctx, span := otel.Tracer("auto-code-os/sandbox").Start(ctx, "sandbox.docker.run")
	defer span.End()
	if err := validateCommand(req.Command); err != nil {
		return nil, err
	}
	resolvedNetworkMode := r.resolveNetworkMode(req.NetworkMode)
	if err := validateExecutionPolicy(req, resolvedNetworkMode); err != nil {
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

	mounts := []mount.Mount{
		{
			Type:   mount.TypeBind,
			Source: workspace,
			Target: "/workspace",
		},
	}

	if homeDir, err := os.UserHomeDir(); err == nil && homeDir != "" {
		goModCachePath := filepath.Join(homeDir, "go", "pkg", "mod")
		if gopath := os.Getenv("GOPATH"); gopath != "" {
			first := strings.Split(gopath, string(os.PathListSeparator))[0]
			goModCachePath = filepath.Join(first, "pkg", "mod")
		} else {
			cmd := exec.Command("go", "env", "GOPATH")
			if out, err := cmd.Output(); err == nil {
				gopathVal := strings.TrimSpace(string(out))
				if gopathVal != "" {
					first := strings.Split(gopathVal, string(os.PathListSeparator))[0]
					goModCachePath = filepath.Join(first, "pkg", "mod")
				}
			}
		}

		// Define common language cache mappings: host absolute path -> container target path
		cacheDirs := map[string]string{
			goModCachePath:                               "/go/pkg/mod",
			filepath.Join(homeDir, ".npm"):               "/root/.npm",
			filepath.Join(homeDir, ".cache", "pip"):      "/root/.cache/pip",
			filepath.Join(homeDir, ".m2"):                "/root/.m2",
			filepath.Join(homeDir, ".gradle"):            "/root/.gradle",
			filepath.Join(homeDir, ".cargo", "registry"): "/root/.cargo/registry",
		}

		for absHostPath, targetContainerPath := range cacheDirs {
			if stat, err := os.Stat(absHostPath); err == nil && stat.IsDir() {
				// Writable: npm (cacache) and maven/gradle must write into
				// their cache to install anything not already present — a
				// read-only mount breaks dependency installation outright.
				mounts = append(mounts, mount.Mount{
					Type:   mount.TypeBind,
					Source: absHostPath,
					Target: targetContainerPath,
				})
				// If we mounted Go cache, set GOPATH env var
				if targetContainerPath == "/go/pkg/mod" {
					envMap["GOPATH"] = "/go"
				}
			}
		}

		// Inject host CLI credentials into the sandbox so the container can
		// automatically utilize the host's existing OAuth sessions without
		// manual config. A per-org CredentialFiles entry targeting the same
		// path always wins (see below), since that reflects an explicit,
		// task-scoped credential rather than whichever operator happens to
		// be running the server process.
		//
		// authDirFiles are single credential files: bind-mounted read-only
		// straight from the host, since nothing inside the sandbox needs to
		// write into them.
		//
		// authDirTrees are whole config directories that a CLI treats as its
		// home: alongside the OAuth token they also contain, the CLI itself
		// writes runtime state into them (e.g. antigravity-cli's "brain" and
		// "conversations" dirs under .gemini). A read-only bind mount of the
		// whole tree lets the CLI read its token but makes every such write
		// fail with "permission denied" (see antigravity-cli mkdir failures
		// this fixes) — and container state writes must never land back on
		// the host's real credential directory anyway. So these are copied
		// into a throwaway per-run staging dir and mounted read-write from
		// there: the CLI gets a writable home, the host original is
		// untouched, and the staging copy is discarded after the run.
		authDirFiles := map[string]string{
			filepath.Join(homeDir, ".claude.json"):                 filepath.Join(SandboxHomeDir, ".claude.json"),                 // Claude Code config
			filepath.Join(homeDir, ".claude", ".credentials.json"): filepath.Join(SandboxHomeDir, ".claude", ".credentials.json"), // Claude Code OAuth session
		}
		authDirTrees := map[string]string{
			filepath.Join(homeDir, ".gemini"):          filepath.Join(SandboxHomeDir, ".gemini"),          // Antigravity CLI
			filepath.Join(homeDir, ".config", "codex"): filepath.Join(SandboxHomeDir, ".config", "codex"), // Codex CLI
		}

		for absHostPath, targetContainerPath := range authDirFiles {
			if credentialFilesOverlap(targetContainerPath, req.CredentialFiles) {
				continue
			}
			if _, err := os.Stat(absHostPath); err == nil {
				mounts = append(mounts, mount.Mount{
					Type:     mount.TypeBind,
					Source:   absHostPath,
					Target:   targetContainerPath,
					ReadOnly: true,
				})
			}
		}

		for absHostPath, targetContainerPath := range authDirTrees {
			stat, err := os.Stat(absHostPath)
			if err != nil || !stat.IsDir() {
				continue
			}
			stagingDir, err := os.MkdirTemp("", "auto-code-os-authdir-*")
			if err != nil {
				return nil, fmt.Errorf("create auth dir staging dir: %w", err)
			}
			defer os.RemoveAll(stagingDir)
			if err := copyDirTree(absHostPath, stagingDir); err != nil {
				return nil, fmt.Errorf("stage auth dir %s: %w", absHostPath, err)
			}
			mounts = append(mounts, mount.Mount{
				Type:   mount.TypeBind,
				Source: stagingDir,
				Target: targetContainerPath,
			})
		}
	}

	credentialHostPaths := make(map[string]string, len(req.CredentialFiles))
	if len(req.CredentialFiles) > 0 {
		credDir, err := os.MkdirTemp("", "auto-code-os-cred-*")
		if err != nil {
			return nil, fmt.Errorf("create credential staging dir: %w", err)
		}
		defer os.RemoveAll(credDir)

		i := 0
		for targetContainerPath, content := range req.CredentialFiles {
			hostPath := filepath.Join(credDir, fmt.Sprintf("f%d", i))
			i++
			// 0o666, not 0o600: this file is bind-mounted read-write (not
			// ReadOnly, see below) so the CLI can refresh/write back its own
			// credential (see UpdatedCredentialFiles). Owner-only bits are
			// written by the host server process's UID but read/written
			// inside the container by the fixed "agent" UID from
			// docker/Dockerfile.sandbox — a different UID that a bind mount
			// never remaps — so 0o600 would silently block that write-back
			// exactly like the authDirTrees permission-denied bug this
			// mirrors.
			if err := os.WriteFile(hostPath, []byte(content), 0o666); err != nil {
				return nil, fmt.Errorf("stage credential file: %w", err)
			}
			mounts = append(mounts, mount.Mount{
				Type:   mount.TypeBind,
				Source: hostPath,
				Target: targetContainerPath,
			})
			credentialHostPaths[targetContainerPath] = hostPath
		}
	}

	env := make([]string, 0, len(envMap))
	for key, value := range envMap {
		env = append(env, key+"="+value)
	}

	networkMode := container.NetworkMode(resolvedNetworkMode)

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
		Mounts: mounts,
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

	var updatedCredentialFiles map[string]string
	for targetContainerPath, hostPath := range credentialHostPaths {
		data, err := os.ReadFile(hostPath)
		if err != nil {
			continue
		}
		if string(data) != req.CredentialFiles[targetContainerPath] {
			if updatedCredentialFiles == nil {
				updatedCredentialFiles = make(map[string]string)
			}
			updatedCredentialFiles[targetContainerPath] = string(data)
		}
	}

	return &CommandResult{
		ExitCode:               int(statusCode),
		Stdout:                 stdout,
		Stderr:                 stderr,
		UpdatedCredentialFiles: updatedCredentialFiles,
	}, nil
}

func (r *DockerRuntime) RunInteractive(ctx context.Context, req CommandRequest, stdin io.Reader, stdout, stderr io.Writer) error {
	ctx, span := otel.Tracer("auto-code-os/sandbox").Start(ctx, "sandbox.docker.runInteractive")
	defer span.End()
	if err := validateCommand(req.Command); err != nil {
		return err
	}
	resolvedNetworkMode := r.resolveNetworkMode(req.NetworkMode)
	if err := validateExecutionPolicy(req, resolvedNetworkMode); err != nil {
		return err
	}
	if len(req.Command) == 0 {
		return fmt.Errorf("docker command is required")
	}

	workspace := req.Workspace
	if workspace == "" {
		workspace = WorkspacePath(r.config.WorkspaceRoot, req.TaskID)
	}
	if err := os.MkdirAll(workspace, 0o755); err != nil {
		return fmt.Errorf("create sandbox workspace: %w", err)
	}

	envMap := mergedEnv(req)
	mounts := []mount.Mount{
		{
			Type:   mount.TypeBind,
			Source: workspace,
			Target: "/workspace",
		},
	}
	env := make([]string, 0, len(envMap))
	for key, value := range envMap {
		env = append(env, key+"="+value)
	}
	networkMode := container.NetworkMode(resolvedNetworkMode)

	createResp, err := r.client.ContainerCreate(ctx, &container.Config{
		Image:        r.config.Image,
		Cmd:          req.Command,
		Env:          env,
		WorkingDir:   "/workspace",
		Tty:          true,
		OpenStdin:    true,
		AttachStdin:  true,
		AttachStdout: true,
		AttachStderr: true,
	}, &container.HostConfig{
		NetworkMode: networkMode,
		Resources: container.Resources{
			Memory:   r.config.MemoryBytes,
			NanoCPUs: r.config.NanoCPUs,
		},
		Mounts: mounts,
	}, nil, nil, "")
	if err != nil {
		return fmt.Errorf("create interactive docker container: %w", err)
	}
	containerID := createResp.ID
	defer func() {
		timeout := 5
		_ = r.client.ContainerStop(context.Background(), containerID, container.StopOptions{Timeout: &timeout})
		_ = r.client.ContainerRemove(context.Background(), containerID, container.RemoveOptions{Force: true})
	}()

	attachResp, err := r.client.ContainerAttach(ctx, containerID, container.AttachOptions{
		Stream: true,
		Stdin:  true,
		Stdout: true,
		Stderr: true,
	})
	if err != nil {
		return fmt.Errorf("attach to docker container: %w", err)
	}
	defer attachResp.Close()

	// Connect streams
	go func() {
		_, _ = io.Copy(attachResp.Conn, stdin)
	}()
	// Tty=true merges stdout and stderr, but we can copy it to both just in case
	stdoutDone := make(chan struct{})
	go func() {
		defer close(stdoutDone)
		_, _ = io.Copy(stdout, attachResp.Reader)
	}()

	if err := r.client.ContainerStart(ctx, containerID, container.StartOptions{}); err != nil {
		return fmt.Errorf("start docker container: %w", err)
	}

	if req.ResizeCh != nil {
		go func() {
			for {
				select {
				case size, ok := <-req.ResizeCh:
					if !ok {
						return
					}
					_ = r.client.ContainerResize(ctx, containerID, container.ResizeOptions{
						Height: size.Rows,
						Width:  size.Cols,
					})
				case <-ctx.Done():
					return
				}
			}
		}()
	}

	waitCh, waitErrCh := r.client.ContainerWait(ctx, containerID, container.WaitConditionNotRunning)
	var waitErr error
	select {
	case err := <-waitErrCh:
		if err != nil {
			waitErr = fmt.Errorf("wait for docker container: %w", err)
		}
	case <-waitCh:
		// Completed
	case <-ctx.Done():
		return ctx.Err()
	}

	// The container has stopped, but attachResp.Reader may still have buffered
	// output in flight (e.g. the final line printed right before exit). Wait
	// for the copy goroutine to drain it before the deferred attachResp.Close()
	// tears down the stream out from under it, otherwise trailing output is lost.
	select {
	case <-stdoutDone:
	case <-time.After(2 * time.Second):
	case <-ctx.Done():
	}

	return waitErr
}

func splitDockerLogs(reader io.Reader) (string, string, error) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	if _, err := stdcopy.StdCopy(&stdout, &stderr, reader); err != nil {
		return "", "", fmt.Errorf("copy docker logs: %w", err)
	}
	return strings.TrimSpace(stdout.String()), strings.TrimSpace(stderr.String()), nil
}
