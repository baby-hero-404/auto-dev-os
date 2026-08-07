package repository

import (
	"context"
	"fmt"
	"sync"

	"github.com/auto-code-os/auto-code-os/server/pkg/models"
	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// EventHub is the TaskEvent pub/sub registry, mirroring LogHub.
type EventHub struct {
	mu          sync.RWMutex
	subscribers map[string]map[chan models.TaskEvent]struct{}
}

func NewEventHub() *EventHub {
	return &EventHub{
		subscribers: make(map[string]map[chan models.TaskEvent]struct{}),
	}
}

func (h *EventHub) Subscribe(taskID string) chan models.TaskEvent {
	h.mu.Lock()
	defer h.mu.Unlock()
	ch := make(chan models.TaskEvent, 100)
	if h.subscribers[taskID] == nil {
		h.subscribers[taskID] = make(map[chan models.TaskEvent]struct{})
	}
	h.subscribers[taskID][ch] = struct{}{}
	return ch
}

func (h *EventHub) Unsubscribe(taskID string, ch chan models.TaskEvent) {
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

func (h *EventHub) Broadcast(taskID string, event models.TaskEvent) {
	h.mu.RLock()
	defer h.mu.RUnlock()
	subs, ok := h.subscribers[taskID]
	if !ok {
		return
	}
	for ch := range subs {
		select {
		case ch <- event:
		default:
			// slow subscriber, drop
		}
	}
}

// TaskEventRepo persists task_events and broadcasts writes to live SSE subscribers.
type TaskEventRepo struct {
	db  *gorm.DB
	Hub *EventHub
}

func NewTaskEventRepo(db *gorm.DB) *TaskEventRepo {
	return &TaskEventRepo{db: db, Hub: NewEventHub()}
}

// CreateEvent assigns the next per-task SequenceNumber transactionally (row
// lock on the task's existing events) and inserts the event, then broadcasts
// it to live subscribers. See design.md's Ordering section: SequenceNumber,
// not CreatedAt, is the authoritative ordering/cursor key.
func (r *TaskEventRepo) CreateEvent(ctx context.Context, event *models.TaskEvent) error {
	if event.ID == "" {
		event.ID = uuid.New().String()
	}

	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var maxSeq int64
		if err := tx.Model(&models.TaskEvent{}).
			Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("task_id = ?", event.TaskID).
			Select("COALESCE(MAX(sequence_number), 0)").
			Scan(&maxSeq).Error; err != nil {
			return err
		}
		event.SequenceNumber = maxSeq + 1
		return tx.Create(event).Error
	})
	if err != nil {
		return fmt.Errorf("create task event: %w", mapError(err))
	}

	r.Hub.Broadcast(event.TaskID, *event)
	return nil
}

// ListByTaskID returns the full ordered event history for a task.
func (r *TaskEventRepo) ListByTaskID(ctx context.Context, taskID string) ([]models.TaskEvent, error) {
	var events []models.TaskEvent
	if err := r.db.WithContext(ctx).Where("task_id = ?", taskID).Order("sequence_number ASC").Find(&events).Error; err != nil {
		return nil, fmt.Errorf("list task events: %w", mapError(err))
	}
	return events, nil
}

// ListByTaskIDAfter returns events with sequence_number > after, ordered
// ascending — used for SSE reconnect catch-up (Last-Event-ID / ?after=).
func (r *TaskEventRepo) ListByTaskIDAfter(ctx context.Context, taskID string, after int64) ([]models.TaskEvent, error) {
	var events []models.TaskEvent
	if err := r.db.WithContext(ctx).
		Where("task_id = ? AND sequence_number > ?", taskID, after).
		Order("sequence_number ASC").
		Find(&events).Error; err != nil {
		return nil, fmt.Errorf("list task events after: %w", mapError(err))
	}
	return events, nil
}

// ListByTaskIDPaginated returns up to limit events with sequence_number <
// before (or the most recent limit events if before <= 0), ordered
// descending — backing GET /tasks/{taskID}/events?before=&limit=.
func (r *TaskEventRepo) ListByTaskIDPaginated(ctx context.Context, taskID string, before int64, limit int) ([]models.TaskEvent, error) {
	if limit <= 0 {
		limit = 100
	}
	query := r.db.WithContext(ctx).Where("task_id = ?", taskID)
	if before > 0 {
		query = query.Where("sequence_number < ?", before)
	}
	var events []models.TaskEvent
	if err := query.Order("sequence_number DESC").Limit(limit).Find(&events).Error; err != nil {
		return nil, fmt.Errorf("list task events paginated: %w", mapError(err))
	}
	return events, nil
}

// CountByTaskID backs the max_event_count execution guardrail.
func (r *TaskEventRepo) CountByTaskID(ctx context.Context, taskID string) (int64, error) {
	var count int64
	if err := r.db.WithContext(ctx).Model(&models.TaskEvent{}).Where("task_id = ?", taskID).Count(&count).Error; err != nil {
		return 0, fmt.Errorf("count task events: %w", mapError(err))
	}
	return count, nil
}

func (r *TaskEventRepo) Subscribe(taskID string) chan models.TaskEvent {
	return r.Hub.Subscribe(taskID)
}

func (r *TaskEventRepo) Unsubscribe(taskID string, ch chan models.TaskEvent) {
	r.Hub.Unsubscribe(taskID, ch)
}
