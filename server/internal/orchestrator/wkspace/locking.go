package wkspace

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/auto-code-os/auto-code-os/server/pkg/models"
)

type localWorkspaceLock struct {
	TaskID      string    `json:"task_id"`
	JobID       string    `json:"job_id"`
	WorkerPID   int       `json:"worker_pid"`
	Hostname    string    `json:"hostname"`
	AcquiredAt  time.Time `json:"acquired_at"`
	HeartbeatAt time.Time `json:"heartbeat_at"`
	TTLSeconds  int       `json:"ttl_seconds"`
}

func (m *Manager) AcquireWorkspaceLock(ctx context.Context, task *models.Task, jobID string) error {
	ws := m.GetTaskWorkspace(task)
	lockPath := filepath.Join(ws.Root, ".workspace.lock")

	// 1. Acquire DB advisory lock first (Postgres level authority)
	if m.Workflows != nil {
		lockConn, locked, err := m.Workflows.AcquireAdvisoryLock(ctx, task.ID)
		if err != nil {
			return fmt.Errorf("failed to acquire DB advisory lock: %w", err)
		}
		if !locked {
			return fmt.Errorf("workspace is locked in DB by another active process (advisory lock check failed)")
		}
		m.LockConns.Store(task.ID, lockConn)
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
					if m.Workflows != nil {
						if lockConn, loaded := m.LockConns.LoadAndDelete(task.ID); loaded {
							if err := m.Workflows.ReleaseAdvisoryLock(ctx, lockConn, task.ID); err != nil {
								m.Log(ctx, task.ID, nil, "warn", fmt.Sprintf("failed to release advisory lock: %v", err))
							}
						}
					}
					return fmt.Errorf("workspace is locked by active job %s on %s (last heartbeat %v ago)", existingLock.JobID, existingLock.Hostname, timeSinceLastHeartbeat)
				}
				// Stale lock: remove it before trying to acquire
				if err := os.Remove(lockPath); err != nil && !os.IsNotExist(err) {
					m.Log(ctx, task.ID, nil, "warn", fmt.Sprintf("failed to remove stale lock file: %v", err))
				}
			} else {
				// Already locked by this job
				return nil
			}
		}
	}

	if err := os.MkdirAll(ws.Root, 0o755); err != nil {
		// Clean up DB lock on failure
		if m.Workflows != nil {
			if lockConn, loaded := m.LockConns.LoadAndDelete(task.ID); loaded {
				if err := m.Workflows.ReleaseAdvisoryLock(ctx, lockConn, task.ID); err != nil {
					m.Log(ctx, task.ID, nil, "warn", fmt.Sprintf("failed to release advisory lock: %v", err))
				}
			}
		}
		return err
	}

	// 3. Atomic O_EXCL check
	f, err := os.OpenFile(lockPath, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o644)
	if err != nil {
		// Clean up DB lock on failure
		if m.Workflows != nil {
			if lockConn, loaded := m.LockConns.LoadAndDelete(task.ID); loaded {
				if err := m.Workflows.ReleaseAdvisoryLock(ctx, lockConn, task.ID); err != nil {
					m.Log(ctx, task.ID, nil, "warn", fmt.Sprintf("failed to release advisory lock: %v", err))
				}
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
	// Finding 7: Align heartbeat lifecycle with actual task/job execution context
	hbCtx, hbCancel := context.WithCancel(ctx)
	m.LockCancels.Store(task.ID, hbCancel)

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

func (m *Manager) ReleaseWorkspaceLock(taskID string) {
	weHoldLock := false
	if cancelVal, loaded := m.LockCancels.LoadAndDelete(taskID); loaded {
		weHoldLock = true
		if cancel, ok := cancelVal.(context.CancelFunc); ok {
			cancel()
		}
	}
	if m.Workflows != nil {
		if lockConn, loaded := m.LockConns.LoadAndDelete(taskID); loaded {
			weHoldLock = true
			if err := m.Workflows.ReleaseAdvisoryLock(context.Background(), lockConn, taskID); err != nil {
				m.Log(context.Background(), taskID, nil, "warn", fmt.Sprintf("failed to release advisory lock: %v", err))
			}
		}
	}
	if weHoldLock {
		root := filepath.Join(m.WorkspaceRoot, taskID)
		lockPath := filepath.Join(root, ".workspace.lock")
		if err := os.Remove(lockPath); err != nil && !os.IsNotExist(err) {
			m.Log(context.Background(), taskID, nil, "warn", fmt.Sprintf("failed to remove workspace lock file: %v", err))
		}
	}
}
