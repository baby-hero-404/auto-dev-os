package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/auto-code-os/auto-code-os/server/internal/middleware"
	"github.com/auto-code-os/auto-code-os/server/internal/service"
	"github.com/auto-code-os/auto-code-os/server/pkg/models"
	"github.com/go-chi/chi/v5"
)

// fakeTaskService implements the handler.TaskService interface, overriding
// only GetByID — every other method is unused by these two tests.
type fakeTaskService struct {
	TaskService
	task *models.Task
}

func (f *fakeTaskService) GetByID(ctx context.Context, id string) (*models.Task, error) {
	return f.task, nil
}

// TestTaskHandler_GetByID_IncludesAvailableActions closes tasks.md 2.2's
// "API test" gap: computeAvailableActions is unit-tested at the service
// layer, but nothing previously exercised GET /tasks/{id} over real HTTP to
// confirm the field survives serialization onto the wire.
func TestTaskHandler_GetByID_IncludesAvailableActions(t *testing.T) {
	task := &models.Task{
		ID:     "task-1",
		Status: models.TaskStatusFailed,
		AvailableActions: []models.AvailableAction{
			{ID: "retry_blocked", Label: "Retry"},
			{ID: "delete", Label: "Delete"},
		},
	}
	h := NewTaskHandler(&fakeTaskService{task: task}, nil)

	r := chi.NewRouter()
	r.Get("/tasks/{taskID}", h.GetByID)

	req := httptest.NewRequest(http.MethodGet, "/tasks/task-1", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var body struct {
		AvailableActions []models.AvailableAction `json:"available_actions"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(body.AvailableActions) != 2 {
		t.Fatalf("expected 2 available_actions in HTTP response, got %d: %s", len(body.AvailableActions), w.Body.String())
	}
}

// TestTaskHandler_GetByID_NonAdminSeesDisabledDelete closes tasks.md 2.4's
// "API test" gap: a caller without project.admin (role "member" in this
// codebase's two-role model) must see "delete" with disabled_reason set,
// not omitted — ApplyActionPolicy is unit-tested directly elsewhere, but
// this confirms GetByID actually calls it with the caller's real role from
// request context, not a hardcoded/omitted one.
func TestTaskHandler_GetByID_NonAdminSeesDisabledDelete(t *testing.T) {
	task := &models.Task{
		ID:     "task-1",
		Status: models.TaskStatusFailed,
		AvailableActions: []models.AvailableAction{
			{ID: "delete", Label: "Delete"},
		},
	}
	h := NewTaskHandler(&fakeTaskService{task: task}, nil)

	r := chi.NewRouter()
	r.Get("/tasks/{taskID}", h.GetByID)

	req := httptest.NewRequest(http.MethodGet, "/tasks/task-1", nil)
	claims := &service.TokenClaims{Subject: "user-1", Role: models.UserRoleMember}
	req = req.WithContext(middleware.WithVerifiedClaims(req.Context(), claims))
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var body struct {
		AvailableActions []models.AvailableAction `json:"available_actions"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(body.AvailableActions) != 1 {
		t.Fatalf("expected delete action to still be present (not omitted), got %d actions: %s", len(body.AvailableActions), w.Body.String())
	}
	if body.AvailableActions[0].ID != "delete" {
		t.Fatalf("expected delete action, got %q", body.AvailableActions[0].ID)
	}
	if body.AvailableActions[0].DisabledReason == "" {
		t.Fatalf("expected non-admin caller to see delete with a disabled_reason set, got empty")
	}
}
