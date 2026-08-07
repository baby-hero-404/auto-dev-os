package handler

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"sync"

	"github.com/auto-code-os/auto-code-os/server/internal/service"
	"github.com/auto-code-os/auto-code-os/server/pkg/models"
	"github.com/go-chi/chi/v5"
)

type TaskEventHandler struct {
	svc *service.TaskEventService
}

func NewTaskEventHandler(svc *service.TaskEventService) *TaskEventHandler {
	return &TaskEventHandler{svc: svc}
}

// List handles GET /tasks/{taskID}/events?before=&limit= — cursor-paginated
// event history, newest first (design.md's Event History API section).
func (h *TaskEventHandler) List(w http.ResponseWriter, r *http.Request) {
	taskID := chi.URLParam(r, "taskID")

	var before int64
	if raw := r.URL.Query().Get("before"); raw != "" {
		v, err := strconv.ParseInt(raw, 10, 64)
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid before cursor")
			return
		}
		before = v
	}

	limit := 100
	if raw := r.URL.Query().Get("limit"); raw != "" {
		v, err := strconv.Atoi(raw)
		if err != nil || v <= 0 {
			writeError(w, http.StatusBadRequest, "invalid limit")
			return
		}
		limit = v
	}

	events, err := h.svc.ListByTaskIDPaginated(r.Context(), taskID, before, limit)
	if err != nil {
		writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, events)
}

// Stream handles GET /tasks/{taskID}/events/stream (SSE). It mirrors
// WorkflowHandler.StreamLogs/streamLogsLoop's subscribe-first ordering so a
// reconnecting client (Last-Event-ID / ?after=) never misses an event
// broadcast between catch-up query and live-tail attachment.
func (h *TaskEventHandler) Stream(w http.ResponseWriter, r *http.Request) {
	taskID := chi.URLParam(r, "taskID")
	ctx := r.Context()

	var after int64
	if raw := r.Header.Get("Last-Event-ID"); raw != "" {
		if v, err := strconv.ParseInt(raw, 10, 64); err == nil {
			after = v
		}
	}
	if raw := r.URL.Query().Get("after"); raw != "" {
		if v, err := strconv.ParseInt(raw, 10, 64); err == nil {
			after = v
		}
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "Streaming unsupported", http.StatusInternalServerError)
		return
	}
	flusher.Flush()

	ch := h.svc.Subscribe(taskID)
	defer h.svc.Unsubscribe(taskID, ch)

	emit := func(event models.TaskEvent) {
		data, _ := json.Marshal(event)
		fmt.Fprintf(w, "id: %d\nevent: %s\ndata: %s\n\n", event.SequenceNumber, event.Type, string(data))
		flusher.Flush()
	}

	if err := streamEventsLoop(ctx, ch, func() ([]models.TaskEvent, error) {
		return h.svc.ListByTaskIDAfter(ctx, taskID, after)
	}, emit); err != nil {
		fmt.Fprintf(w, "event: error\ndata: %s\n\n", err.Error())
		flusher.Flush()
	}
}

// streamEventsLoop is streamLogsLoop's TaskEvent counterpart — see
// WorkflowHandler.streamLogsLoop for the full race-condition rationale
// (subscribe-first, buffer while catch-up query is in flight, drain the
// buffer only after the buffering goroutine has fully detached from ch).
func streamEventsLoop(ctx context.Context, ch chan models.TaskEvent, catchup func() ([]models.TaskEvent, error), emit func(models.TaskEvent)) error {
	var buffer []models.TaskEvent
	var bufMu sync.Mutex
	stopBuf := make(chan struct{})
	done := make(chan struct{})

	go func() {
		defer close(done)
		for {
			select {
			case <-stopBuf:
				// Go's select chooses pseudo-randomly among ready cases, so
				// stopBuf firing doesn't mean ch is empty — a broadcast can
				// have already landed in ch's buffer before stopBuf closed.
				// Drain whatever's already queued before returning, or it's
				// left for the trailing live-loop to emit with no dedup
				// against catchup()'s history.
				for {
					select {
					case event, ok := <-ch:
						if !ok {
							return
						}
						bufMu.Lock()
						buffer = append(buffer, event)
						bufMu.Unlock()
					default:
						return
					}
				}
			case event, ok := <-ch:
				if !ok {
					return
				}
				bufMu.Lock()
				buffer = append(buffer, event)
				bufMu.Unlock()
			}
		}
	}()

	history, err := catchup()
	if err != nil {
		close(stopBuf)
		<-done
		return err
	}

	var maxSeq int64 = -1
	for _, event := range history {
		emit(event)
		if event.SequenceNumber > maxSeq {
			maxSeq = event.SequenceNumber
		}
	}

	close(stopBuf)
	<-done

	// The buffer can contain events already returned by catchup() — a write
	// committed (and broadcast) between Subscribe and the catchup query
	// landing in both. Dedup by sequence_number, not identity, per
	// design.md's Ordering section.
	bufMu.Lock()
	for _, event := range buffer {
		if event.SequenceNumber > maxSeq {
			emit(event)
			maxSeq = event.SequenceNumber
		}
	}
	bufMu.Unlock()

	for {
		select {
		case <-ctx.Done():
			return nil
		case event, ok := <-ch:
			if !ok {
				return nil
			}
			emit(event)
		}
	}
}
