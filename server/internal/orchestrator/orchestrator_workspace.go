package orchestrator

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/auto-code-os/auto-code-os/server/internal/observability"
	"github.com/auto-code-os/auto-code-os/server/internal/sandbox"
	"github.com/auto-code-os/auto-code-os/server/internal/workflow"
	"github.com/auto-code-os/auto-code-os/server/pkg/llm"
	"github.com/auto-code-os/auto-code-os/server/pkg/models"
)

func (o *Orchestrator) GetTaskWorkspace(task *models.Task) *models.TaskWorkspace {
	root := sandbox.WorkspacePath(o.workspaceRoot, task.ID)
	return &models.TaskWorkspace{
		Root:         root,
		SpecsDir:     filepath.Join(root, "specs"),
		ContextDir:   filepath.Join(root, "context"),
		ArtifactsDir: filepath.Join(root, "artifacts"),
		LogsDir:      filepath.Join(root, "logs"),
		PRDir:        filepath.Join(root, "pr"),
	}
}

func (o *Orchestrator) InitTaskWorkspace(ctx context.Context, task *models.Task) (*models.TaskWorkspace, error) {
	ws := o.GetTaskWorkspace(task)
	
	dirs := []string{
		ws.Root,
		ws.SpecsDir,
		ws.ContextDir,
		ws.ArtifactsDir,
		filepath.Join(ws.ArtifactsDir, "checkpoints"),
		filepath.Join(ws.ArtifactsDir, "diffs"),
		filepath.Join(ws.ArtifactsDir, "tests"),
		ws.LogsDir,
		filepath.Join(ws.LogsDir, "llm"),
		ws.PRDir,
	}
	for _, d := range dirs {
		if err := os.MkdirAll(d, 0o755); err != nil {
			return nil, fmt.Errorf("create dir %s: %w", d, err)
		}
	}

	var projectRepos []models.Repository
	if o.repositories != nil {
		var err error
		projectRepos, err = o.repositories.ListByProjectID(ctx, task.ProjectID)
		if err != nil {
			return nil, fmt.Errorf("list project repositories: %w", err)
		}
	}

	repos := []models.RepoWorkspace{}
	for _, pr := range projectRepos {
		if task.RepositoryID != nil && *task.RepositoryID != pr.ID {
			continue
		}

		parts := strings.Split(pr.URL, "/")
		repoName := parts[len(parts)-1]
		repoName = strings.TrimSuffix(repoName, ".git")

		repoWS := models.RepoWorkspace{
			RepoID:        pr.ID,
			Name:          repoName,
			URL:           pr.URL,
			DefaultBranch: pr.Branch,
			Status: models.RepoWorkspaceStatus{
				MergeStatus: models.MergeStatusPending,
				TestStatus:  models.TestStatusPending,
			},
			Paths: models.RepoWorkspacePaths{
				Main:      filepath.Join("code", "repos", repoName, "main"),
				Worktrees: make(map[string]string),
			},
			Branches: models.RepoWorkspaceBranches{
				Integration: fmt.Sprintf("feature/%s", task.ID),
				Role:        make(map[string]string),
			},
		}

		repos = append(repos, repoWS)
	}
	ws.Repos = repos

	taskSnap := models.TaskStateSnapshot{
		TaskID:      task.ID,
		ProjectID:   task.ProjectID,
		Title:       task.Title,
		Description: task.Description,
		Status:      task.Status,
		Complexity:  task.Complexity,
		SpecStatus:  task.SpecStatus,
		Labels:      task.Labels,
	}
	taskJSONPath := filepath.Join(ws.Root, "task.json")
	taskBytes, err := json.MarshalIndent(taskSnap, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("marshal task snapshot: %w", err)
	}
	if err := os.WriteFile(taskJSONPath, taskBytes, 0o644); err != nil {
		return nil, fmt.Errorf("write task.json: %w", err)
	}

	meta := models.TaskWorkspaceMetadata{
		WorkspaceVersion: 1,
		Repos:            repos,
	}
	metaJSONPath := filepath.Join(ws.Root, "metadata.json")
	metaBytes, err := json.MarshalIndent(meta, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("marshal metadata: %w", err)
	}
	if err := os.WriteFile(metaJSONPath, metaBytes, 0o644); err != nil {
		return nil, fmt.Errorf("write metadata.json: %w", err)
	}

	return ws, nil
}

func (o *Orchestrator) LoadTaskWorkspace(ctx context.Context, task *models.Task) (*models.TaskWorkspace, error) {
	ws := o.GetTaskWorkspace(task)
	metaJSONPath := filepath.Join(ws.Root, "metadata.json")
	metaBytes, err := os.ReadFile(metaJSONPath)
	if err != nil {
		if os.IsNotExist(err) {
			if _, statErr := os.Stat(ws.Root); statErr == nil {
				return o.InitTaskWorkspace(ctx, task)
			}
			return nil, fmt.Errorf("metadata.json not found and workspace does not exist: %w", err)
		}
		return nil, err
	}

	var meta models.TaskWorkspaceMetadata
	if err := json.Unmarshal(metaBytes, &meta); err != nil {
		return nil, fmt.Errorf("unmarshal metadata.json: %w", err)
	}

	ws.Repos = meta.Repos
	return ws, nil
}

func (o *Orchestrator) SaveTaskWorkspaceMetadata(task *models.Task, ws *models.TaskWorkspace) error {
	meta := models.TaskWorkspaceMetadata{
		WorkspaceVersion: 1,
		Repos:            ws.Repos,
	}
	metaJSONPath := filepath.Join(ws.Root, "metadata.json")
	metaBytes, err := json.MarshalIndent(meta, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(metaJSONPath, metaBytes, 0o644)
}

func (o *Orchestrator) ResolveRepoWorkspace(ctx context.Context, task *models.Task, repoID string) (*models.RepoWorkspace, error) {
	ws, err := o.LoadTaskWorkspace(ctx, task)
	if err != nil {
		return nil, err
	}
	for i := range ws.Repos {
		if ws.Repos[i].RepoID == repoID {
			return &ws.Repos[i], nil
		}
	}
	return nil, fmt.Errorf("repo %s not found in workspace", repoID)
}

func (o *Orchestrator) StartWorkspacePruner(ctx context.Context) {
	if o.retention.Retention <= 0 {
		return
	}
	interval := o.retention.Interval
	if interval <= 0 {
		interval = time.Hour
	}

	if removed, err := o.pruneWorkspaces(ctx); err != nil {
		observability.Warn(ctx, "workspace prune failed", "error", err)
	} else if removed > 0 {
		observability.Info(ctx, "workspace prune completed", "removed", removed)
	}

	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if removed, err := o.pruneWorkspaces(ctx); err != nil {
				observability.Warn(ctx, "workspace prune failed", "error", err)
			} else if removed > 0 {
				observability.Info(ctx, "workspace prune completed", "removed", removed)
			}
		}
	}
}

func (o *Orchestrator) ensureWorkspaceCloned(ctx context.Context, task *models.Task, agent *models.Agent, jobID string) error {
	if o.repositories == nil {
		return fmt.Errorf("repositories lookup not configured")
	}
	if o.gitOps == nil {
		return fmt.Errorf("gitops client not configured")
	}

	ws, err := o.LoadTaskWorkspace(ctx, task)
	if err != nil {
		ws, err = o.InitTaskWorkspace(ctx, task)
		if err != nil {
			return fmt.Errorf("failed to init task workspace: %w", err)
		}
	}

	if err := o.acquireWorkspaceLock(ctx, task, jobID); err != nil {
		return fmt.Errorf("failed to acquire workspace lock: %w", err)
	}

	checkpoints, cpErr := o.workflows.ListCheckpoints(ctx, task.ID)
	hasSuccessfulCodeStep := false
	completedSteps := make(map[string]bool)
	if cpErr == nil && len(checkpoints) > 0 {
		for _, cp := range checkpoints {
			var state map[string]any
			if json.Unmarshal(cp.State, &state) == nil {
				if status, _ := state["status"].(string); status == workflow.StepStatusSuccess {
					completedSteps[cp.Step] = true
					if cp.Step == workflow.StepCodeBackend || cp.Step == workflow.StepCodeFrontend || cp.Step == workflow.StepFix || cp.Step == workflow.StepMerge {
						hasSuccessfulCodeStep = true
					}
				}
			}
		}
	}

	var workspaceRestored bool

	for i, rWS := range ws.Repos {
		repoAbsPath := filepath.Join(ws.Root, rWS.Paths.Main)
		gitDir := filepath.Join(repoAbsPath, ".git")

		workspaceExists := false
		if stat, err := os.Stat(gitDir); err == nil && stat.IsDir() {
			workspaceExists = true
		}

		if workspaceExists {
			if !hasSuccessfulCodeStep {
				if err := resetExistingWorkspace(ctx, repoAbsPath); err != nil {
					return fmt.Errorf("reset existing workspace at %s: %w", repoAbsPath, err)
				}
			}
		} else {
			os.RemoveAll(repoAbsPath)
			if err := os.MkdirAll(filepath.Dir(repoAbsPath), 0o755); err != nil {
				return fmt.Errorf("create repo parent dir: %w", err)
			}
			if _, err := o.gitOps.CloneForTask(ctx, rWS.URL, rWS.DefaultBranch, repoAbsPath); err != nil {
				return fmt.Errorf("clone repo %s: %w", rWS.Name, err)
			}
			workspaceRestored = true
		}

		ws.Repos[i].Branches.Integration = fmt.Sprintf("feature/%s", task.ID)
	}

	if err := o.SaveTaskWorkspaceMetadata(task, ws); err != nil {
		return fmt.Errorf("failed to save metadata: %w", err)
	}

	if hasSuccessfulCodeStep && workspaceRestored {
		if o.artifacts != nil {
			if arts, errArts := o.artifacts.ListByTaskID(ctx, task.ID); errArts == nil {
				for _, art := range arts {
					if !completedSteps[art.Step] {
						continue
					}
					if art.Type == "patch" {
						var patchText string
						if json.Unmarshal(art.Payload, &patchText) == nil && patchText != "" {
							o.log(ctx, task.ID, nil, "info", fmt.Sprintf("Restoring checkpoint patch for step %s...", art.Step))
							if errApply := o.applyPatch(ctx, task, agent, art.Step+"_restore", patchText, ""); errApply != nil {
								o.log(ctx, task.ID, nil, "warn", fmt.Sprintf("Failed to restore patch for step %s: %v", art.Step, errApply))
							}
						}
					}
				}
			}
		}
	}

	return nil
}

func (o *Orchestrator) cleanupWorkspaceAfterFinalState(ctx context.Context, taskID string) {
	if o.retention.Retention != 0 {
		return
	}
	if err := o.partialCleanupWorkspace(ctx, taskID); err != nil {
		observability.Warn(ctx, "workspace partial cleanup failed", "task_id", taskID, "error", err)
	} else {
		observability.Info(ctx, "workspace partially cleaned after final state", "task_id", taskID)
	}
}

func (o *Orchestrator) pruneWorkspaces(ctx context.Context) (int, error) {
	root := o.workspaceRoot
	if root == "" {
		root = "/tmp/auto-code-os/workspaces"
	}
	entries, err := os.ReadDir(root)
	if err != nil {
		if os.IsNotExist(err) {
			return 0, nil
		}
		return 0, err
	}

	cutoff := time.Now().Add(-o.retention.Retention)
	removed := 0
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		info, err := entry.Info()
		if err != nil {
			observability.Warn(ctx, "workspace prune stat failed", "name", entry.Name(), "error", err)
			continue
		}
		taskID := entry.Name()
		if o.tasks != nil {
			task, err := o.tasks.GetByID(ctx, taskID)
			if err != nil {
				if strings.Contains(strings.ToLower(err.Error()), "not found") || strings.Contains(strings.ToLower(err.Error()), "record not found") {
					if err := o.removeWorkspace(taskID); err == nil {
						removed++
					}
				}
				continue
			}
			if task.Status == models.TaskStatusMerged || task.Status == models.TaskStatusFailed {
				if err := o.partialCleanupWorkspace(ctx, taskID); err != nil {
					observability.Warn(ctx, "workspace prune failed", "name", taskID, "error", err)
					continue
				}
				removed++
			}
		} else {
			if info.ModTime().Before(cutoff) {
				if err := o.removeWorkspace(entry.Name()); err != nil {
					observability.Warn(ctx, "workspace prune failed", "name", entry.Name(), "error", err)
					continue
				}
				removed++
			}
		}
	}
	return removed, nil
}

type localWorkspaceLock struct {
	TaskID      string    `json:"task_id"`
	JobID       string    `json:"job_id"`
	WorkerPID   int       `json:"worker_pid"`
	Hostname    string    `json:"hostname"`
	AcquiredAt  time.Time `json:"acquired_at"`
	HeartbeatAt time.Time `json:"heartbeat_at"`
	TTLSeconds  int       `json:"ttl_seconds"`
}

func (o *Orchestrator) acquireWorkspaceLock(ctx context.Context, task *models.Task, jobID string) error {
	ws := o.GetTaskWorkspace(task)
	lockPath := filepath.Join(ws.Root, ".workspace.lock")

	// 1. Acquire DB advisory lock first (Postgres level authority)
	if o.workflows != nil {
		lockConn, locked, err := o.workflows.AcquireAdvisoryLock(ctx, task.ID)
		if err != nil {
			return fmt.Errorf("failed to acquire DB advisory lock: %w", err)
		}
		if !locked {
			return fmt.Errorf("workspace is locked in DB by another active process (advisory lock check failed)")
		}
		o.lockConns.Store(task.ID, lockConn)
	}

	// 2. Read and handle existing lock check
	if data, err := os.ReadFile(lockPath); err == nil {
		var existingLock localWorkspaceLock
		if json.Unmarshal(data, &existingLock) == nil {
			if existingLock.JobID != jobID {
				timeSinceLastHeartbeat := time.Since(existingLock.HeartbeatAt)
				ttl := time.Duration(existingLock.TTLSeconds) * time.Second
				if ttl == 0 {
					ttl = 300 * time.Second
				}
				if timeSinceLastHeartbeat < ttl {
					// Release DB lock since we can't acquire the local workspace lock
					if o.workflows != nil {
						if lockConn, loaded := o.lockConns.LoadAndDelete(task.ID); loaded {
							_ = o.workflows.ReleaseAdvisoryLock(ctx, lockConn, task.ID)
						}
					}
					return fmt.Errorf("workspace is locked by active job %s on %s (last heartbeat %v ago)", existingLock.JobID, existingLock.Hostname, timeSinceLastHeartbeat)
				}
				// Stale lock: remove it before trying to acquire
				_ = os.Remove(lockPath)
			} else {
				// Already locked by this job
				return nil
			}
		}
	}

	if err := os.MkdirAll(ws.Root, 0o755); err != nil {
		// Clean up DB lock on failure
		if o.workflows != nil {
			if lockConn, loaded := o.lockConns.LoadAndDelete(task.ID); loaded {
				_ = o.workflows.ReleaseAdvisoryLock(ctx, lockConn, task.ID)
			}
		}
		return err
	}

	// 3. Atomic O_EXCL check
	f, err := os.OpenFile(lockPath, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o644)
	if err != nil {
		// Clean up DB lock on failure
		if o.workflows != nil {
			if lockConn, loaded := o.lockConns.LoadAndDelete(task.ID); loaded {
				_ = o.workflows.ReleaseAdvisoryLock(ctx, lockConn, task.ID)
			}
		}
		if os.IsExist(err) {
			return fmt.Errorf("workspace lock file already exists (atomic O_EXCL check failed)")
		}
		return err
	}
	defer f.Close()

	hostname, _ := os.Hostname()
	lock := localWorkspaceLock{
		TaskID:      task.ID,
		JobID:       jobID,
		WorkerPID:   os.Getpid(),
		Hostname:    hostname,
		AcquiredAt:  time.Now(),
		HeartbeatAt: time.Now(),
		TTLSeconds:  300,
	}

	bytes, err := json.MarshalIndent(lock, "", "  ")
	if err != nil {
		return err
	}
	if _, err := f.Write(bytes); err != nil {
		return err
	}

	// 4. Start heartbeat loop
	hbCtx, hbCancel := context.WithCancel(context.Background())
	o.lockCancels.Store(task.ID, hbCancel)

	go func(tID string, lPath string) {
		ticker := time.NewTicker(30 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-hbCtx.Done():
				return
			case <-ticker.C:
				data, err := os.ReadFile(lPath)
				if err != nil {
					continue
				}
				var activeLock localWorkspaceLock
				if json.Unmarshal(data, &activeLock) == nil && activeLock.JobID == jobID {
					activeLock.HeartbeatAt = time.Now()
					if updatedBytes, errMarshal := json.MarshalIndent(activeLock, "", "  "); errMarshal == nil {
						_ = os.WriteFile(lPath, updatedBytes, 0o644)
					}
				}
			}
		}
	}(task.ID, lockPath)

	return nil
}

func (o *Orchestrator) releaseWorkspaceLock(taskID string) {
	if cancelVal, loaded := o.lockCancels.LoadAndDelete(taskID); loaded {
		if cancel, ok := cancelVal.(context.CancelFunc); ok {
			cancel()
		}
	}
	if o.workflows != nil {
		if lockConn, loaded := o.lockConns.LoadAndDelete(taskID); loaded {
			_ = o.workflows.ReleaseAdvisoryLock(context.Background(), lockConn, taskID)
		}
	}
	root := sandbox.WorkspacePath(o.workspaceRoot, taskID)
	lockPath := filepath.Join(root, ".workspace.lock")
	_ = os.Remove(lockPath)
}

func (o *Orchestrator) partialCleanupWorkspace(ctx context.Context, taskID string) error {
	o.releaseWorkspaceLock(taskID)

	root := sandbox.WorkspacePath(o.workspaceRoot, taskID)
	codeDir := filepath.Join(root, "code", "repos")
	
	repos, err := os.ReadDir(codeDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}

	for _, rEntry := range repos {
		if !rEntry.IsDir() {
			continue
		}
		repoName := rEntry.Name()
		wtParentDir := filepath.Join(codeDir, repoName, "worktrees")
		worktrees, err := os.ReadDir(wtParentDir)
		if err != nil {
			continue
		}

		mainAbs := filepath.Join(codeDir, repoName, "main")

		for _, wtEntry := range worktrees {
			if !wtEntry.IsDir() {
				continue
			}
			role := wtEntry.Name()
			wtAbs := filepath.Join(wtParentDir, role)

			// Check git status to see if there are uncommitted changes
			statusCmd := exec.CommandContext(ctx, "git", "-C", wtAbs, "status", "--porcelain")
			statusOut, statusErr := statusCmd.CombinedOutput()
			if statusErr == nil && len(strings.TrimSpace(string(statusOut))) > 0 {
				// Capture both staged and unstaged modifications
				diffCmd := exec.CommandContext(ctx, "git", "-C", wtAbs, "diff", "HEAD")
				diffOut, diffErr := diffCmd.CombinedOutput()
				if diffErr != nil {
					return fmt.Errorf("failed to capture diff for worktree %s: %w", wtAbs, diffErr)
				}
				
				statusClean := strings.TrimSpace(string(statusOut))
				fullDiffContent := []byte(fmt.Sprintf("=== Worktree Status ===\n%s\n\n=== Diffs ===\n%s", statusClean, string(diffOut)))
				
				diffDir := filepath.Join(root, "artifacts", "diffs")
				if err := os.MkdirAll(diffDir, 0o755); err != nil {
					return fmt.Errorf("failed to create diff dir: %w", err)
				}
				diffPath := filepath.Join(diffDir, fmt.Sprintf("cleanup-%s-%s.diff", repoName, role))
				if err := os.WriteFile(diffPath, fullDiffContent, 0o644); err != nil {
					return fmt.Errorf("failed to write cleanup diff: %w", err)
				}
			}

			// Prune worktree using git worktree remove
			pruneCmd := exec.CommandContext(ctx, "git", "-C", mainAbs, "worktree", "remove", wtAbs, "--force")
			if err := pruneCmd.Run(); err != nil {
				// Fallback to manual removal if git worktree remove fails
				if errRemove := os.RemoveAll(wtAbs); errRemove != nil {
					return fmt.Errorf("failed to remove worktree path %s: %w", wtAbs, errRemove)
				}
			}
		}
	}

	// Update metadata.json if it exists and can be loaded
	if o.tasks != nil {
		if task, err := o.tasks.GetByID(ctx, taskID); err == nil {
			if ws, errLoad := o.LoadTaskWorkspace(ctx, task); errLoad == nil {
				for i := range ws.Repos {
					ws.Repos[i].Paths.Worktrees = make(map[string]string)
					ws.Repos[i].Branches.Role = make(map[string]string)
				}
				_ = o.SaveTaskWorkspaceMetadata(task, ws)
			}
		}
	}

	return nil
}

func (o *Orchestrator) removeWorkspace(taskID string) error {
	if strings.TrimSpace(taskID) == "" {
		return fmt.Errorf("task id is required")
	}
	root := o.workspaceRoot
	if root == "" {
		root = "/tmp/auto-code-os/workspaces"
	}
	rootAbs, err := filepath.Abs(root)
	if err != nil {
		return err
	}
	targetAbs, err := filepath.Abs(sandbox.WorkspacePath(root, taskID))
	if err != nil {
		return err
	}
	if targetAbs == rootAbs {
		return fmt.Errorf("refusing to remove workspace root")
	}
	rootPrefix := rootAbs + string(os.PathSeparator)
	if !strings.HasPrefix(targetAbs, rootPrefix) {
		return fmt.Errorf("workspace path escapes root")
	}
	return os.RemoveAll(targetAbs)
}

func resetExistingWorkspace(ctx context.Context, localPath string) error {
	commands := [][]string{
		{"git", "-C", localPath, "reset", "--hard"},
		{"git", "-C", localPath, "clean", "-fdx"},
	}
	for _, args := range commands {
		cmd := exec.CommandContext(ctx, args[0], args[1:]...)
		if output, err := cmd.CombinedOutput(); err != nil {
			return fmt.Errorf("%s: %w: %s", strings.Join(args, " "), err, string(output))
		}
	}
	return nil
}

func (o *Orchestrator) StartLogPruner(ctx context.Context, retentionDays int, fileRoot string) {
	if retentionDays <= 0 || fileRoot == "" {
		return
	}
	interval := time.Hour
	if pruned, err := pruneLogFiles(ctx, retentionDays, fileRoot); err != nil {
		observability.Warn(ctx, "log files prune failed", "error", err)
	} else if pruned > 0 {
		observability.Info(ctx, "log files prune completed", "pruned", pruned)
	}

	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if pruned, err := pruneLogFiles(ctx, retentionDays, fileRoot); err != nil {
				observability.Warn(ctx, "log files prune failed", "error", err)
			} else if pruned > 0 {
				observability.Info(ctx, "log files prune completed", "pruned", pruned)
			}
		}
	}
}

func pruneLogFiles(ctx context.Context, retentionDays int, fileRoot string) (int, error) {
	entries, err := os.ReadDir(fileRoot)
	if err != nil {
		if os.IsNotExist(err) {
			return 0, nil
		}
		return 0, err
	}

	cutoff := time.Now().AddDate(0, 0, -retentionDays)
	pruned := 0
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		if !strings.HasSuffix(entry.Name(), ".jsonl") {
			continue
		}
		info, err := entry.Info()
		if err != nil {
			observability.Warn(ctx, "log prune stat failed", "name", entry.Name(), "error", err)
			continue
		}
		if info.ModTime().After(cutoff) {
			continue
		}
		filePath := filepath.Join(fileRoot, entry.Name())
		if err := os.Remove(filePath); err != nil {
			observability.Warn(ctx, "log prune remove failed", "path", filePath, "error", err)
			continue
		}
		pruned++
	}
	return pruned, nil
}

func (o *Orchestrator) FindRepoWorkspaceByPath(ctx context.Context, task *models.Task, path string) (*models.RepoWorkspace, error) {
	ws, err := o.LoadTaskWorkspace(ctx, task)
	if err != nil {
		return nil, err
	}
	absPath, err := filepath.Abs(path)
	if err != nil {
		absPath = path
	}

	for i := range ws.Repos {
		rWS := &ws.Repos[i]
		mainAbs := filepath.Join(ws.Root, rWS.Paths.Main)
		if mainAbs == absPath || strings.HasPrefix(absPath, mainAbs) {
			return rWS, nil
		}
		for _, wtRel := range rWS.Paths.Worktrees {
			wtAbs := filepath.Join(ws.Root, wtRel)
			if wtAbs == absPath || strings.HasPrefix(absPath, wtAbs) {
				return rWS, nil
			}
		}
		if strings.Contains(absPath, filepath.Join("code", "repos", rWS.Name)) {
			return rWS, nil
		}
	}
	if len(ws.Repos) == 1 {
		return &ws.Repos[0], nil
	}
	return nil, fmt.Errorf("no repository matching path %s found in workspace", path)
}

func getRoleFromSuffix(suffix string) string {
	suffix = strings.TrimPrefix(suffix, "-")
	suffix = strings.TrimSuffix(suffix, "-worktree")
	switch suffix {
	case "be", "backend":
		return "backend"
	case "fe", "frontend":
		return "frontend"
	case "fix":
		return "fix"
	default:
		return suffix
	}
}

var secretRegexes = []*regexp.Regexp{
	regexp.MustCompile(`(?i)ghp_[a-zA-Z0-9]{36}`),
	regexp.MustCompile(`(?i)github_pat_[a-zA-Z0-9_]{82}`),
	regexp.MustCompile(`(?i)sk-[a-zA-Z0-9]{48}`),
	regexp.MustCompile(`(?i)sk-proj-[a-zA-Z0-9-_]{150,}`),
	regexp.MustCompile(`(?i)sk-ant-[a-zA-Z0-9-_]{90,}`),
	regexp.MustCompile(`(?i)AIzaSy[a-zA-Z0-9-_]{33}`),
}

func redactSecrets(s string) string {
	for _, re := range secretRegexes {
		s = re.ReplaceAllString(s, "[REDACTED]")
	}
	return s
}

func (o *Orchestrator) writeLLMCallTrace(ctx context.Context, task *models.Task, agent *models.Agent, stepID string, messages []llm.Message, resp *llm.Response, parsed map[string]any) {
	ws := o.GetTaskWorkspace(task)
	stepTraceDir := filepath.Join(ws.Root, "logs", "llm", stepID)
	_ = os.MkdirAll(stepTraceDir, 0o755)

	callNumber := 1
	if files, err := os.ReadDir(stepTraceDir); err == nil {
		for _, f := range files {
			if f.IsDir() && strings.HasPrefix(f.Name(), "call-") {
				var n int
				if _, errScan := fmt.Sscanf(f.Name(), "call-%d", &n); errScan == nil {
					if n >= callNumber {
						callNumber = n + 1
					}
				}
			}
		}
	}

	callDirName := fmt.Sprintf("call-%d", callNumber)
	callPath := filepath.Join(stepTraceDir, callDirName)
	_ = os.MkdirAll(callPath, 0o755)

	reqJSON, _ := json.MarshalIndent(messages, "", "  ")
	_ = os.WriteFile(filepath.Join(callPath, "request.json"), []byte(redactSecrets(string(reqJSON))), 0o644)

	resJSON, _ := json.MarshalIndent(resp, "", "  ")
	_ = os.WriteFile(filepath.Join(callPath, "response.json"), []byte(redactSecrets(string(resJSON))), 0o644)

	var promptBuilder strings.Builder
	promptBuilder.WriteString("# LLM Request Prompt Reconstructed\n\n")
	for _, msg := range messages {
		promptBuilder.WriteString(fmt.Sprintf("## Role: %s\n\n%s\n\n---\n\n", msg.Role, msg.Content))
	}
	_ = os.WriteFile(filepath.Join(callPath, "prompt.md"), []byte(redactSecrets(promptBuilder.String())), 0o644)

	_ = os.WriteFile(filepath.Join(callPath, "output.md"), []byte(redactSecrets(resp.Content)), 0o644)

	if len(parsed) > 0 {
		parsedJSON, _ := json.MarshalIndent(parsed, "", "  ")
		_ = os.WriteFile(filepath.Join(callPath, "parsed.json"), []byte(redactSecrets(string(parsedJSON))), 0o644)
	}

	type TraceMetadata struct {
		Model        string    `json:"model"`
		PromptTokens int       `json:"prompt_tokens"`
		OutputTokens int       `json:"output_tokens"`
		AgentID      string    `json:"agent_id"`
		AgentName    string    `json:"agent_name"`
		Role         string    `json:"role"`
		Timestamp    time.Time `json:"timestamp"`
	}
	meta := TraceMetadata{
		Model:        resp.Model,
		PromptTokens: resp.PromptTokens,
		OutputTokens: resp.OutputTokens,
		AgentID:      agent.ID,
		AgentName:    agent.Name,
		Role:         agent.Role,
		Timestamp:    time.Now(),
	}
	metaJSON, _ := json.MarshalIndent(meta, "", "  ")
	_ = os.WriteFile(filepath.Join(callPath, "metadata.json"), metaJSON, 0o644)
}
