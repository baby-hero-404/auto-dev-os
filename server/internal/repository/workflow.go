package repository

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
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

type WorkflowRepo struct {
	db       *gorm.DB
	fileRoot string
}

func NewWorkflowRepo(db *gorm.DB) *WorkflowRepo {
	return &WorkflowRepo{db: db}
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
	return checkpoints, nil
}

func (r *WorkflowRepo) DeleteCheckpoints(ctx context.Context, taskID string, steps []string) error {
	if err := r.db.WithContext(ctx).Where("task_id = ? AND step IN ?", taskID, steps).Delete(&models.WorkflowCheckpoint{}).Error; err != nil {
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
	err := r.db.WithContext(ctx).Model(&models.WorkflowJob{}).
		Where("status = ? AND updated_at < ?", models.WorkflowJobStatusRunning, staleBefore).
		Update("status", models.WorkflowJobStatusQueued).Error
	if err != nil {
		return fmt.Errorf("reset stuck jobs: %w", err)
	}
	return nil
}

func (r *WorkflowRepo) CreateLog(ctx context.Context, log models.TaskLog) error {
	if r.fileRoot == "" {
		if err := r.db.WithContext(ctx).Create(&log).Error; err != nil {
			return fmt.Errorf("create task log: %w", err)
		}
		return nil
	}

	if log.ID == "" {
		log.ID = uuid.New().String()
	}
	if log.CreatedAt.IsZero() {
		log.CreatedAt = time.Now()
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

	return nil
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
