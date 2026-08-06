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
	"sync"
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

// resolveImage returns requestImage if set, falling back to the runtime's
// configured default image otherwise — the "empty = today's exact behavior"
// contract CommandRequest.Image documents, so a caller that never opts into
// per-runtime image selection (i.e. everything before SandboxManager
// existed) is unaffected.
func (r *DockerRuntime) resolveImage(requestImage string) string {
	if requestImage != "" {
		return requestImage
	}
	return r.config.Image
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

// sessionMountsOverlap checks if the target path is already covered by a SessionMount
// (e.g. SessionMounts mounts "/home/agent/.claude" and target is "/home/agent/.claude/.credentials.json")
func sessionMountsOverlap(target string, sessionMounts map[string]string) bool {
	for smTarget := range sessionMounts {
		if target == smTarget || strings.HasPrefix(target, smTarget+"/") {
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

	if req.LogsHostDir != "" {
		if err := os.MkdirAll(req.LogsHostDir, 0o755); err != nil {
			return nil, fmt.Errorf("create sandbox logs dir: %w", err)
		}
		mounts = append(mounts, mount.Mount{
			Type:   mount.TypeBind,
			Source: req.LogsHostDir,
			Target: LogsContainerDir,
		})
	}

	for targetContainerPath, hostPath := range req.SessionMounts {
		// Ensure the host path exists
		if err := os.MkdirAll(hostPath, 0o777); err != nil {
			return nil, fmt.Errorf("create session mount dir: %w", err)
		}
		mounts = append(mounts, mount.Mount{
			Type:   mount.TypeBind,
			Source: hostPath,
			Target: targetContainerPath,
		})
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
			filepath.Join(homeDir, ".npm"):               "/home/agent/.npm",
			filepath.Join(homeDir, ".cache", "pip"):      "/home/agent/.cache/pip",
			filepath.Join(homeDir, ".m2"):                "/home/agent/.m2",
			filepath.Join(homeDir, ".gradle"):            "/home/agent/.gradle",
			filepath.Join(homeDir, ".cargo", "registry"): "/home/agent/.cargo/registry",
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

		// req.ExtraCacheMounts (SandboxManager-resolved runtime manifest
		// caches, e.g. flutter's ~/.pub-cache) are additive to the hardcoded
		// cacheDirs above, not a replacement — same os.Stat-guarded,
		// writable-bind pattern, since these caches also need write access
		// to install anything not already present.
		for targetContainerPath, absHostPath := range req.ExtraCacheMounts {
			if stat, err := os.Stat(absHostPath); err == nil && stat.IsDir() {
				mounts = append(mounts, mount.Mount{
					Type:   mount.TypeBind,
					Source: absHostPath,
					Target: targetContainerPath,
				})
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
			filepath.Join(homeDir, ".claude.json"):                            filepath.Join(SandboxHomeDir, ".claude.json"),                 // Claude Code config
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
			if sessionMountsOverlap(targetContainerPath, req.SessionMounts) {
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
			if credentialFilesOverlap(targetContainerPath, req.CredentialFiles) {
				continue
			}
			if sessionMountsOverlap(targetContainerPath, req.SessionMounts) {
				continue
			}
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

	// For credential files, they are typically small files containing OAuth
	// tokens or API keys (e.g. .claude.json, .gemini/antigravity-oauth-token).
	// Because Docker cannot reliably bind-mount single files from the host
	// while supporting atomic renames (which these CLIs do when refreshing
	// tokens), we stage them in a temporary directory on the host and mount
	// the whole parent directories.
	
	credentialHostPaths := make(map[string]string, len(req.CredentialFiles))
	if len(req.CredentialFiles) > 0 {
		credDir, err := os.MkdirTemp("", "auto-code-os-cred-*")
		if err != nil {
			return nil, fmt.Errorf("create credential staging dir: %w", err)
		}
		defer os.RemoveAll(credDir)

		// Recreate the exact file layout inside credDir OR write directly to a SessionMount
		for targetContainerPath, content := range req.CredentialFiles {
			var smContainerPath, smHostPath string
			isCovered := false
			for smTarget, smHost := range req.SessionMounts {
				if targetContainerPath == smTarget || strings.HasPrefix(targetContainerPath, smTarget+"/") {
					isCovered = true
					smContainerPath = smTarget
					smHostPath = smHost
					break
				}
			}

			if isCovered {
				// The path is already mapped by a SessionMount (e.g. ~/.claude).
				// Write the credential directly into the host session directory.
				relPath := strings.TrimPrefix(targetContainerPath, smContainerPath)
				hostPath := filepath.Join(smHostPath, relPath)

				if err := os.MkdirAll(filepath.Dir(hostPath), 0o777); err != nil {
					return nil, fmt.Errorf("create credential subdirs in session: %w", err)
				}
				// 0o666, not owner-only: bind mounts carry host-side
				// permission bits as-is with no UID remapping (see
				// copyDirTree above), and the container's fixed "agent" UID
				// is not generally the same UID as the host server process
				// writing this file.
				if err := os.WriteFile(hostPath, []byte(content), 0o666); err != nil {
					return nil, fmt.Errorf("write credential file in session: %w", err)
				}
				credentialHostPaths[targetContainerPath] = hostPath
				continue
			}

			// e.g. "/home/agent/.claude/.credentials.json" -> "home/agent/.claude/.credentials.json"
			relPath := strings.TrimPrefix(targetContainerPath, "/")
			hostPath := filepath.Join(credDir, relPath)

			if err := os.MkdirAll(filepath.Dir(hostPath), 0o777); err != nil {
				return nil, fmt.Errorf("create credential subdirs: %w", err)
			}
			
			// 0o666 allows read/write from any UID inside the container
			if err := os.WriteFile(hostPath, []byte(content), 0o666); err != nil {
				return nil, fmt.Errorf("write credential file: %w", err)
			}
			
			// We need this map for extracting updated files later
			credentialHostPaths[targetContainerPath] = hostPath
		}

		// Figure out the highest-level directories to mount so that atomic renames work.
		// Mounting individual files breaks atomic rename (rename(2) returns EXDEV).
		mountTargets := make(map[string]bool)
		for targetContainerPath := range req.CredentialFiles {
			// If it was written to a SessionMount, it doesn't need its own mount target
			if sessionMountsOverlap(targetContainerPath, req.SessionMounts) {
				continue
			}
			
			rel, err := filepath.Rel(SandboxHomeDir, targetContainerPath)
			if err == nil && !strings.HasPrefix(rel, "..") && rel != "." && strings.Contains(rel, string(filepath.Separator)) {
				// E.g. target="/home/agent/.claude/.credentials.json", rel=".claude/.credentials.json"
				// Top-level child of /home/agent is ".claude"
				parts := strings.Split(rel, string(filepath.Separator))
				topLevel := filepath.Join(SandboxHomeDir, parts[0])
				mountTargets[topLevel] = true
			} else {
				// Either outside SandboxHomeDir, or it's a file right in the home directory
				// (e.g. "/home/agent/.claude.json"). In this case, we have to mount it directly.
				mountTargets[targetContainerPath] = true
			}
		}

		for targetContainerPath := range mountTargets {
			relPath := strings.TrimPrefix(targetContainerPath, "/")
			hostPath := filepath.Join(credDir, relPath)

			// If it's a directory, ensure it is fully writable by the agent user
			if stat, err := os.Stat(hostPath); err == nil && stat.IsDir() {
				os.Chmod(hostPath, 0o777)
			}

			mounts = append(mounts, mount.Mount{
				Type:   mount.TypeBind,
				Source: hostPath,
				Target: targetContainerPath,
			})
		}
	}

	env := make([]string, 0, len(envMap))
	for key, value := range envMap {
		env = append(env, key+"="+value)
	}

	networkMode := container.NetworkMode(resolvedNetworkMode)

	createResp, err := r.client.ContainerCreate(ctx, &container.Config{
		Image:      r.resolveImage(req.Image),
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

	// Stream logs with Follow:true starting right after the container
	// starts, rather than fetching the whole buffer with ContainerLogs after
	// ContainerWait returns (the old behavior): a container killed mid-run
	// (OOM, host restart, context cancellation) left no trace at all under
	// the old approach, since ContainerWait never resolved and the
	// after-the-fact ContainerLogs call was never reached. Streaming
	// concurrently with the wait means a real-time on-disk copy (via
	// req.LogFilePath, Phase 6 "Real-time Log Streaming") survives up to the
	// point of failure even when the container itself never exits cleanly.
	stdout, stderr, logsErrCh, logCloser, activity, loopDet, err := r.streamContainerLogs(ctx, containerID, req.LogFilePath)
	if err != nil {
		return nil, err
	}
	defer logCloser()

	// watchForStall (Phase 7, "Smart Idle Timeout & Loop Detection") polls
	// the same activity/loop trackers streamContainerLogs just wired up and
	// force-kills the container the first time either condition trips,
	// independent of — and normally well before — req.Timeout's absolute
	// cap. watchdogDone stops it once the container has exited on its own
	// (success, normal failure, or ctx cancellation below) so it never
	// races a kill against an already-finished run.
	idleTimeout := req.IdleTimeout
	if idleTimeout <= 0 {
		idleTimeout = DefaultIdleTimeout
	}
	watchdogDone := make(chan struct{})
	defer close(watchdogDone)
	var stallMu sync.Mutex
	var stallReason string
	go watchForStall(watchdogDone, activity, loopDet, idleTimeout, 5*time.Second, func(reason string) {
		stallMu.Lock()
		stallReason = reason
		stallMu.Unlock()
		_ = r.client.ContainerKill(context.Background(), containerID, "SIGKILL")
	})

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
		// Context cancellation (Phase 7, "Context Cancellation SIGKILL
		// cascade" — e.g. a user clicking Cancel in the UI, whose handler
		// calls the CancelFunc stored in Orchestrator.jobCancels) must reach
		// and kill the container immediately, not just stop polling it: the
		// deferred ContainerStop below still runs, but it's a graceful
		// SIGTERM-then-wait-5s-then-SIGKILL — an explicit SIGKILL here skips
		// that grace period so a stuck/looping agent dies instantly instead
		// of continuing to run (and burn tokens/cost) for another 5s after
		// the user asked it to stop. Best-effort: ContainerKill errors are
		// swallowed since ctx.Err() is already the error being returned, and
		// the deferred ContainerStop/ContainerRemove are the backstop either
		// way.
		_ = r.client.ContainerKill(context.Background(), containerID, "SIGKILL")
		return nil, ctx.Err()
	}

	// The container has stopped; the log stream should reach EOF on its own
	// shortly after (Follow:true ends once the container's output is fully
	// drained). Give it a bounded grace period rather than blocking forever
	// on a stream that, in principle, could hang.
	select {
	case <-logsErrCh:
	case <-time.After(5 * time.Second):
	case <-ctx.Done():
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

	stallMu.Lock()
	killed := stallReason != ""
	killReason := stallReason
	stallMu.Unlock()

	// stallReason only covers kills our own watchForStall watchdog
	// initiated. A container can also die from an external SIGKILL — most
	// commonly the kernel/cgroup OOM killer enforcing r.config.MemoryBytes —
	// which surfaces only as statusCode 137 via ContainerWait with no
	// stallReason set. Docker's inspect API carries the authoritative
	// OOMKilled flag for exactly this case, so check it rather than
	// guessing from the exit code alone.
	if !killed && statusCode == 137 {
		if inspect, err := r.client.ContainerInspect(context.Background(), containerID); err == nil && inspect.State != nil && inspect.State.OOMKilled {
			killed = true
			killReason = KillReasonOOM
		}
	}

	return &CommandResult{
		ExitCode:               int(statusCode),
		Stdout:                 strings.TrimSpace(stdout.String()),
		Stderr:                 strings.TrimSpace(stderr.String()),
		UpdatedCredentialFiles: updatedCredentialFiles,
		Killed:                 killed,
		KillReason:             killReason,
	}, nil
}

// streamContainerLogs attaches to containerID's log stream with Follow:true
// and copies it into in-memory stdout/stderr buffers as it arrives — plus,
// if logFilePath is non-empty, into a real-time combined on-disk copy via
// io.MultiWriter (Phase 6, "Real-time Log Streaming"). The returned error
// channel receives the copy goroutine's terminal error (nil on clean EOF)
// exactly once; the returned closer must be deferred by the caller to stop
// the stream and release the log file handle.
// streamContainerLogs additionally returns the activityWriter/lineLoopDetector
// (Phase 7, "Smart Idle Timeout & Loop Detection") that observed every byte
// written to either stream, so Run's watchdog can poll them without a second
// pass over the output.
func (r *DockerRuntime) streamContainerLogs(ctx context.Context, containerID, logFilePath string) (*bytes.Buffer, *bytes.Buffer, chan error, func(), *activityWriter, *lineLoopDetector, error) {
	logReader, err := r.client.ContainerLogs(ctx, containerID, container.LogsOptions{
		ShowStdout: true,
		ShowStderr: true,
		Follow:     true,
	})
	if err != nil {
		return nil, nil, nil, nil, nil, nil, fmt.Errorf("read docker container logs: %w", err)
	}

	var stdoutBuf, stderrBuf bytes.Buffer
	var stdoutW, stderrW io.Writer = &stdoutBuf, &stderrBuf

	var logFile *os.File
	if logFilePath != "" {
		if err := os.MkdirAll(filepath.Dir(logFilePath), 0o755); err != nil {
			logReader.Close()
			return nil, nil, nil, nil, nil, nil, fmt.Errorf("create run log dir: %w", err)
		}
		// O_APPEND, not O_TRUNC: a retried run reusing the same log file
		// path (e.g. task retry re-running the same step) should not erase
		// the previous attempt's trace, matching the checkpoint/artifact
		// append conventions used elsewhere in this package.
		logFile, err = os.OpenFile(logFilePath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
		if err != nil {
			logReader.Close()
			return nil, nil, nil, nil, nil, nil, fmt.Errorf("open run log file: %w", err)
		}
		stdoutW = io.MultiWriter(&stdoutBuf, logFile)
		stderrW = io.MultiWriter(&stderrBuf, logFile)
	}

	// activityWriter/lineLoopDetector both wrap the already-composed
	// stdout/stderr writers so they observe exactly what's buffered/logged,
	// then feed watchForStall in Run — a single shared pair across stdout
	// and stderr, since a hallucinating agent's repeated error can land on
	// either stream depending on the CLI.
	activity := newActivityWriter(io.Discard)
	loopDet := newLineLoopDetector(io.Discard)
	stdoutW = io.MultiWriter(stdoutW, activity, loopDet)
	stderrW = io.MultiWriter(stderrW, activity, loopDet)

	errCh := make(chan error, 1)
	go func() {
		_, copyErr := stdcopy.StdCopy(stdoutW, stderrW, logReader)
		errCh <- copyErr
	}()

	closer := func() {
		_ = logReader.Close()
		if logFile != nil {
			_ = logFile.Close()
		}
	}
	return &stdoutBuf, &stderrBuf, errCh, closer, activity, loopDet, nil
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
		Image:        r.resolveImage(req.Image),
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
