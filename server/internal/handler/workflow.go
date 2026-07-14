package handler

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sync"

	"github.com/auto-code-os/auto-code-os/server/internal/orchestrator"
	"github.com/auto-code-os/auto-code-os/server/pkg/models"
	"github.com/go-chi/chi/v5"
)

type WorkflowHandler struct {
	orch *orchestrator.Orchestrator
}

func NewWorkflowHandler(orch *orchestrator.Orchestrator) *WorkflowHandler {
	return &WorkflowHandler{orch: orch}
}

func (h *WorkflowHandler) Execute(w http.ResponseWriter, r *http.Request) {
	taskID := chi.URLParam(r, "taskID")
	job, err := h.orch.Execute(r.Context(), taskID)
	if err != nil {
		writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusAccepted, job)
}

func (h *WorkflowHandler) Status(w http.ResponseWriter, r *http.Request) {
	taskID := chi.URLParam(r, "taskID")
	status, err := h.orch.WorkflowStatus(r.Context(), taskID)
	if err != nil {
		writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, status)
}

func (h *WorkflowHandler) Logs(w http.ResponseWriter, r *http.Request) {
	taskID := chi.URLParam(r, "taskID")
	logs, err := h.orch.Logs(r.Context(), taskID)
	if err != nil {
		writeServiceError(w, err)
		return
	}

	if r.URL.Query().Get("stream") != "true" {
		writeJSON(w, http.StatusOK, logs)
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	for _, log := range logs {
		_, _ = fmt.Fprintf(w, "event: log\ndata: %s\n\n", log.Message)
	}
}

func (h *WorkflowHandler) StreamLogs(w http.ResponseWriter, r *http.Request) {
	taskID := chi.URLParam(r, "taskID")
	ctx := r.Context()

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "Streaming unsupported", http.StatusInternalServerError)
		return
	}
	flusher.Flush()

	ch := h.orch.SubscribeLogs(taskID)
	defer h.orch.UnsubscribeLogs(taskID, ch)

	emit := func(log models.TaskLog) {
		data, _ := json.Marshal(log)
		fmt.Fprintf(w, "event: log\ndata: %s\n\n", string(data))
		flusher.Flush()
	}

	if err := streamLogsLoop(ctx, ch, func() ([]models.TaskLog, error) {
		return h.orch.TailLogs(ctx, taskID, 500)
	}, emit); err != nil {
		fmt.Fprintf(w, "event: error\ndata: %s\n\n", err.Error())
		flusher.Flush()
	}
}

// streamLogsLoop drains the tail snapshot then live logs from ch onto emit. It is deliberately
// decoupled from http.ResponseWriter/chi so the subscribe-first race can be exercised directly
// in a unit test without wiring up a full Orchestrator.
//
// Ordering is subscribe-first: the caller has already subscribed to ch before calling this
// function, so nothing broadcast after subscription is missed. A background goroutine buffers
// anything that arrives on ch while tail() is in flight; once tail() returns and the historical
// snapshot has been emitted, we must be certain that goroutine has fully detached from ch before
// this function starts reading ch directly — otherwise the two would race as concurrent
// receivers on the same channel and a value could be delivered to whichever one the Go runtime
// happens to pick, silently dropping it if that's the goroutine (its buffer is never read again
// after the flush below). Closing stopBuf alone does not guarantee that detachment happens
// before this function proceeds; <-done does.
func streamLogsLoop(ctx context.Context, ch chan models.TaskLog, tail func() ([]models.TaskLog, error), emit func(models.TaskLog)) error {
	var buffer []models.TaskLog
	var bufMu sync.Mutex
	stopBuf := make(chan struct{})
	done := make(chan struct{})

	go func() {
		defer close(done)
		for {
			select {
			case <-stopBuf:
				return
			case log, ok := <-ch:
				if !ok {
					return
				}
				bufMu.Lock()
				buffer = append(buffer, log)
				bufMu.Unlock()
			}
		}
	}()

	history, err := tail()
	if err != nil {
		close(stopBuf)
		<-done
		return err
	}

	for _, log := range history {
		emit(log)
	}

	close(stopBuf)
	<-done // guarantees the goroutine above has fully detached from ch before we read it directly

	bufMu.Lock()
	for _, log := range buffer {
		emit(log)
	}
	bufMu.Unlock()

	for {
		select {
		case <-ctx.Done():
			return nil
		case log, ok := <-ch:
			if !ok {
				return nil
			}
			emit(log)
		}
	}
}

func (h *WorkflowHandler) Approve(w http.ResponseWriter, r *http.Request) {
	taskID := chi.URLParam(r, "taskID")
	task, err := h.orch.ApproveMerge(r.Context(), taskID)
	if err != nil {
		writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, task)
}

// Retry re-enqueues a failed task, resuming from the last successful checkpoint.
// POST /api/v1/tasks/:taskID/retry
func (h *WorkflowHandler) Retry(w http.ResponseWriter, r *http.Request) {
	taskID := chi.URLParam(r, "taskID")
	job, err := h.orch.RetryFromLastStep(r.Context(), taskID)
	if err != nil {
		writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusAccepted, job)
}

func (h *WorkflowHandler) Pause(w http.ResponseWriter, r *http.Request) {
	taskID := chi.URLParam(r, "taskID")
	if err := h.orch.PauseJob(r.Context(), taskID); err != nil {
		writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, envelope{"status": "paused"})
}

func (h *WorkflowHandler) Cancel(w http.ResponseWriter, r *http.Request) {
	taskID := chi.URLParam(r, "taskID")
	if err := h.orch.CancelJob(r.Context(), taskID); err != nil {
		writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, envelope{"status": "cancelled"})
}

func (h *WorkflowHandler) Artifacts(w http.ResponseWriter, r *http.Request) {
	jobID := chi.URLParam(r, "jobID")
	artifacts, err := h.orch.ListArtifacts(r.Context(), jobID)
	if err != nil {
		writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, artifacts)
}
