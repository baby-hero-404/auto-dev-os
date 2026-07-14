package repository

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"database/sql"
	"hash/fnv"

	"github.com/auto-code-os/auto-code-os/server/pkg/models"
	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

var logMu sync.RWMutex

type LogHub struct {
	mu          sync.RWMutex
	subscribers map[string]map[chan models.TaskLog]struct{}
}

func NewLogHub() *LogHub {
	return &LogHub{
		subscribers: make(map[string]map[chan models.TaskLog]struct{}),
	}
}

func (h *LogHub) Subscribe(taskID string) chan models.TaskLog {
	h.mu.Lock()
	defer h.mu.Unlock()
	ch := make(chan models.TaskLog, 100)
	if h.subscribers[taskID] == nil {
		h.subscribers[taskID] = make(map[chan models.TaskLog]struct{})
	}
	h.subscribers[taskID][ch] = struct{}{}
	return ch
}

func (h *LogHub) Unsubscribe(taskID string, ch chan models.TaskLog) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if subs, ok := h.subscribers[taskID]; ok {
		delete(subs, ch)
		if len(subs) == 0 {
			delete(h.subscribers, taskID)
		}
	}
	close(ch)
}

func (h *LogHub) Broadcast(taskID string, log models.TaskLog) {
	h.mu.RLock()
	defer h.mu.RUnlock()
	subs, ok := h.subscribers[taskID]
	if !ok {
		return
	}
	for ch := range subs {
		select {
		case ch <- log:
		default:
			// slow subscriber, drop
		}
	}
}

type WorkflowRepo struct {
	db       *gorm.DB
	fileRoot string
	LogHub   *LogHub
}

func NewWorkflowRepo(db *gorm.DB) *WorkflowRepo {
	return &WorkflowRepo{
		db: db,
		LogHub: NewLogHub(),
	}
}

func (r *WorkflowRepo) SetLogFileRoot(root string) {
	r.fileRoot = root
}

func (r *WorkflowRepo) Enqueue(ctx context.Context, taskID string) (*models.WorkflowJob, error) {
	job := &models.WorkflowJob{TaskID: taskID, Status: models.WorkflowJobStatusQueued, Step: models.WorkflowStepAnalyze}
	if err := r.db.WithContext(ctx).Create(job).Error; err != nil {
		return nil, fmt.Errorf("enqueue workflow job: %w", err)
	}
	return job, nil
}

func (r *WorkflowRepo) ClaimNext(ctx context.Context) (*models.WorkflowJob, error) {
	var job models.WorkflowJob
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		res := tx.Clauses(clause.Locking{Strength: "UPDATE", Options: "SKIP LOCKED"}).
			Where("status = ?", models.WorkflowJobStatusQueued).
			Order("created_at ASC").
			Limit(1).
			Find(&job)
		if res.Error != nil {
			return res.Error
		}
		if res.RowsAffected == 0 {
			return gorm.ErrRecordNotFound
		}

		return tx.Model(&job).Updates(map[string]any{
			"status":   models.WorkflowJobStatusRunning,
			"attempts": gorm.Expr("attempts + 1"),
		}).Error
	})
	if err != nil {
		mapped := mapError(err)
		if errors.Is(mapped, ErrNotFound) {
			return nil, mapped
		}
		return nil, fmt.Errorf("claim workflow job: %w", mapped)
	}
	return &job, nil
}

func (r *WorkflowRepo) LatestByTaskID(ctx context.Context, taskID string) (*models.WorkflowJob, error) {
	var job models.WorkflowJob
	if err := r.db.WithContext(ctx).Where("task_id = ?", taskID).Order("created_at DESC").First(&job).Error; err != nil {
		return nil, fmt.Errorf("latest workflow job: %w", mapError(err))
	}
	return &job, nil
}

func (r *WorkflowRepo) UpdateJob(ctx context.Context, jobID string, updates map[string]any) (*models.WorkflowJob, error) {
	var job models.WorkflowJob
	if err := r.db.WithContext(ctx).Model(&models.WorkflowJob{}).Where("id = ?", jobID).Updates(updates).Error; err != nil {
		return nil, fmt.Errorf("update workflow job: %w", mapError(err))
	}
	if err := r.db.WithContext(ctx).First(&job, "id = ?", jobID).Error; err != nil {
		return nil, fmt.Errorf("reload workflow job: %w", mapError(err))
	}
	return &job, nil
}

func (r *WorkflowRepo) CreateCheckpoint(ctx context.Context, checkpoint models.WorkflowCheckpoint) error {
	if err := r.db.WithContext(ctx).Create(&checkpoint).Error; err != nil {
		return fmt.Errorf("create workflow checkpoint: %w", err)
	}
	return nil
}

func (r *WorkflowRepo) ListCheckpoints(ctx context.Context, taskID string) ([]models.WorkflowCheckpoint, error) {
	var checkpoints []models.WorkflowCheckpoint
	if err := r.db.WithContext(ctx).Where("task_id = ?", taskID).Order("created_at ASC").Find(&checkpoints).Error; err != nil {
		return nil, fmt.Errorf("list workflow checkpoints: %w", err)
	}
	for i := range checkpoints {
		checkpoints[i].CommitHash = checkpoints[i].GetCommitHash()
	}
	return checkpoints, nil
}

func (r *WorkflowRepo) DeleteCheckpoints(ctx context.Context, taskID string, steps []string) error {
	if len(steps) == 0 {
		return nil
	}

	query := r.db.WithContext(ctx).Where("task_id = ?", taskID)

	var conditions []string
	var args []any
	for _, s := range steps {
		conditions = append(conditions, "(step = ? OR step LIKE ?)")
		args = append(args, s, s+"_%")
	}

	query = query.Where(strings.Join(conditions, " OR "), args...)

	if err := query.Delete(&models.WorkflowCheckpoint{}).Error; err != nil {
		return fmt.Errorf("delete workflow checkpoints: %w", err)
	}
	return nil
}

func (r *WorkflowRepo) DeleteByTaskID(ctx context.Context, taskID string) error {
	if err := r.db.WithContext(ctx).Where("task_id = ?", taskID).Delete(&models.WorkflowCheckpoint{}).Error; err != nil {
		return fmt.Errorf("delete workflow checkpoints by task: %w", err)
	}
	return nil
}

func (r *WorkflowRepo) ResetStuckJobs(ctx context.Context) error {
	staleBefore := time.Now().Add(-10 * time.Minute)
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var stuckJobs []models.WorkflowJob
		if err := tx.Where("status = ? AND updated_at < ?", models.WorkflowJobStatusRunning, staleBefore).Find(&stuckJobs).Error; err != nil {
			return err
		}
		if len(stuckJobs) == 0 {
			return nil
		}

		var agentIDs []string
		var jobIDs []string
		for _, job := range stuckJobs {
			jobIDs = append(jobIDs, job.ID)
			if job.AgentID != nil && *job.AgentID != "" {
				agentIDs = append(agentIDs, *job.AgentID)
			}
		}

		// Reset jobs to queued
		if err := tx.Model(&models.WorkflowJob{}).Where("id IN ?", jobIDs).Update("status", models.WorkflowJobStatusQueued).Error; err != nil {
			return err
		}

		// Reset agents to idle
		if len(agentIDs) > 0 {
			if err := tx.Table("agents").Where("id IN ?", agentIDs).Update("status", models.AgentStatusIdle).Error; err != nil {
				return err
			}
		}

		return nil
	})
	if err != nil {
		return fmt.Errorf("reset stuck jobs: %w", err)
	}
	return nil
}

func (r *WorkflowRepo) ResetAllRunningJobs(ctx context.Context) error {
	err := r.db.WithContext(ctx).Model(&models.WorkflowJob{}).
		Where("status = ?", models.WorkflowJobStatusRunning).
		Update("status", models.WorkflowJobStatusQueued).Error
	if err != nil {
		return fmt.Errorf("reset all running jobs: %w", err)
	}
	return nil
}

func (r *WorkflowRepo) CreateLog(ctx context.Context, log models.TaskLog) error {
	if log.ID == "" {
		log.ID = uuid.New().String()
	}
	if log.CreatedAt.IsZero() {
		log.CreatedAt = time.Now()
	}

	if r.fileRoot == "" {
		if err := r.db.WithContext(ctx).Create(&log).Error; err != nil {
			return fmt.Errorf("create task log: %w", err)
		}
		r.LogHub.Broadcast(log.TaskID, log)
		return nil
	}

	logPath := filepath.Join(r.fileRoot, log.TaskID+".jsonl")
	if err := os.MkdirAll(filepath.Dir(logPath), 0755); err != nil {
		return fmt.Errorf("create log dir: %w", err)
	}

	logMu.Lock()
	defer logMu.Unlock()

	f, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return fmt.Errorf("open log file: %w", err)
	}
	defer f.Close()

	data, err := json.Marshal(log)
	if err != nil {
		return fmt.Errorf("marshal log: %w", err)
	}

	if _, err := f.Write(append(data, '\n')); err != nil {
		return fmt.Errorf("write log: %w", err)
	}

	r.LogHub.Broadcast(log.TaskID, log)

	return nil
}

func (r *WorkflowRepo) TailLogs(ctx context.Context, taskID string, n int) ([]models.TaskLog, error) {
	if r.fileRoot == "" {
		var logs []models.TaskLog
		if err := r.db.WithContext(ctx).Where("task_id = ?", taskID).Order("created_at DESC").Limit(n).Find(&logs).Error; err != nil {
			return nil, fmt.Errorf("tail task logs: %w", err)
		}
		for i, j := 0, len(logs)-1; i < j; i, j = i+1, j-1 {
			logs[i], logs[j] = logs[j], logs[i]
		}
		return logs, nil
	}

	logPath := filepath.Join(r.fileRoot, taskID+".jsonl")

	logMu.RLock()
	defer logMu.RUnlock()

	f, err := os.Open(logPath)
	if err != nil {
		if os.IsNotExist(err) {
			return []models.TaskLog{}, nil
		}
		return nil, fmt.Errorf("open log file: %w", err)
	}
	defer f.Close()

	stat, err := f.Stat()
	if err != nil {
		return nil, fmt.Errorf("stat log file: %w", err)
	}

	var logs []models.TaskLog
	var lines [][]byte

	chunkSize := int64(32 * 1024)
	fileSize := stat.Size()
	var offset = fileSize
	var leftover []byte

	for offset > 0 && len(lines) < n {
		readSize := chunkSize
		if offset < chunkSize {
			readSize = offset
		}
		offset -= readSize

		buf := make([]byte, readSize)
		if _, err := f.ReadAt(buf, offset); err != nil {
			return nil, fmt.Errorf("read log file at offset: %w", err)
		}

		if len(leftover) > 0 {
			buf = append(buf, leftover...)
		}

		parts := strings.Split(string(buf), "\n")
		leftover = []byte(parts[0])

		for i := len(parts) - 1; i >= 1; i-- {
			line := strings.TrimSpace(parts[i])
			if line != "" {
				lines = append(lines, []byte(line))
				if len(lines) == n {
					break
				}
			}
		}
	}

	if offset == 0 && len(lines) < n {
		line := strings.TrimSpace(string(leftover))
		if line != "" {
			lines = append(lines, []byte(line))
		}
	}

	for _, line := range lines {
		var log models.TaskLog
		if err := json.Unmarshal(line, &log); err == nil {
			logs = append(logs, log)
		}
	}

	for i, j := 0, len(logs)-1; i < j; i, j = i+1, j-1 {
		logs[i], logs[j] = logs[j], logs[i]
	}

	return logs, nil
}

func (r *WorkflowRepo) SubscribeLogs(taskID string) chan models.TaskLog {
	return r.LogHub.Subscribe(taskID)
}

func (r *WorkflowRepo) UnsubscribeLogs(taskID string, ch chan models.TaskLog) {
	r.LogHub.Unsubscribe(taskID, ch)
}

func (r *WorkflowRepo) ListLogs(ctx context.Context, taskID string) ([]models.TaskLog, error) {
	if r.fileRoot == "" {
		var logs []models.TaskLog
		if err := r.db.WithContext(ctx).Where("task_id = ?", taskID).Order("created_at ASC").Find(&logs).Error; err != nil {
			return nil, fmt.Errorf("list task logs: %w", err)
		}
		return logs, nil
	}

	logPath := filepath.Join(r.fileRoot, taskID+".jsonl")

	logMu.RLock()
	defer logMu.RUnlock()

	f, err := os.Open(logPath)
	if err != nil {
		if os.IsNotExist(err) {
			return []models.TaskLog{}, nil
		}
		return nil, fmt.Errorf("open log file: %w", err)
	}
	defer f.Close()

	var logs []models.TaskLog
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		var log models.TaskLog
		if err := json.Unmarshal(scanner.Bytes(), &log); err != nil {
			continue
		}
		logs = append(logs, log)
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("scan log file: %w", err)
	}

	return logs, nil
}

func (r *WorkflowRepo) AcquireAdvisoryLock(ctx context.Context, taskID string) (any, bool, error) {
	if r.db.Dialector.Name() != "postgres" {
		return "mock-conn", true, nil
	}

	h := fnv.New64a()
	h.Write([]byte(taskID))
	key := int64(h.Sum64())

	sqlDB, err := r.db.DB()
	if err != nil {
		return nil, false, fmt.Errorf("failed to get sql db: %w", err)
	}

	conn, err := sqlDB.Conn(ctx)
	if err != nil {
		return nil, false, fmt.Errorf("failed to check out standard connection: %w", err)
	}

	var locked bool
	row := conn.QueryRowContext(ctx, "SELECT pg_try_advisory_lock($1)", key)
	if err := row.Scan(&locked); err != nil {
		_ = conn.Close()
		return nil, false, fmt.Errorf("failed to scan pg_try_advisory_lock: %w", err)
	}

	if !locked {
		_ = conn.Close()
		return nil, false, nil
	}

	return conn, true, nil
}

func (r *WorkflowRepo) ReleaseAdvisoryLock(ctx context.Context, lockConn any, taskID string) error {
	if lockConn == nil || lockConn == "mock-conn" {
		return nil
	}
	conn, ok := lockConn.(*sql.Conn)
	if !ok {
		return fmt.Errorf("invalid lock connection type: %T", lockConn)
	}
	defer conn.Close()

	h := fnv.New64a()
	h.Write([]byte(taskID))
	key := int64(h.Sum64())

	var unlocked bool
	row := conn.QueryRowContext(ctx, "SELECT pg_advisory_unlock($1)", key)
	_ = row.Scan(&unlocked)

	return nil
}
