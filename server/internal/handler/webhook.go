package handler

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"

	"github.com/auto-code-os/auto-code-os/server/internal/service"
	"github.com/auto-code-os/auto-code-os/server/pkg/models"
)

type WebhookHandler struct {
	taskSvc *service.TaskService
}

func NewWebhookHandler(taskSvc *service.TaskService) *WebhookHandler {
	return &WebhookHandler{taskSvc: taskSvc}
}

func (h *WebhookHandler) GitHub(w http.ResponseWriter, r *http.Request) {
	if secret := os.Getenv("WEBHOOK_SECRET"); secret != "" && r.Header.Get("X-AutoCodeOS-Webhook-Token") != secret {
		writeError(w, http.StatusUnauthorized, "invalid webhook token")
		return
	}

	projectID := r.URL.Query().Get("project_id")
	if projectID == "" {
		writeJSON(w, http.StatusAccepted, envelope{"status": "ignored", "reason": "project_id query parameter is required to create tasks"})
		return
	}

	event := r.Header.Get("X-GitHub-Event")
	var payload map[string]any
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		writeError(w, http.StatusBadRequest, "invalid webhook payload")
		return
	}

	input, ok := githubPayloadToTask(event, payload)
	if !ok {
		writeJSON(w, http.StatusAccepted, envelope{"status": "ignored", "event": event})
		return
	}
	task, err := h.taskSvc.Create(r.Context(), projectID, input)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, task)
}

func githubPayloadToTask(event string, payload map[string]any) (models.CreateTaskInput, bool) {
	switch event {
	case "issues":
		if payload["action"] != "opened" {
			return models.CreateTaskInput{}, false
		}
		issue, _ := payload["issue"].(map[string]any)
		title, _ := issue["title"].(string)
		body, _ := issue["body"].(string)
		htmlURL, _ := issue["html_url"].(string)
		if title == "" {
			return models.CreateTaskInput{}, false
		}
		return models.CreateTaskInput{
			Title:       "GitHub issue: " + title,
			Description: fmt.Sprintf("%s\n\nSource: %s", body, htmlURL),
			Complexity:  models.TaskComplexityMedium,
			Labels:      []string{"github", "issue"},
		}, true
	case "workflow_run", "check_suite":
		action, _ := payload["action"].(string)
		if action == "" {
			action = "completed"
		}
		return models.CreateTaskInput{
			Title:       "GitHub CI event: " + action,
			Description: "A GitHub CI event was received. Review the run logs and create a fix if needed.",
			Complexity:  models.TaskComplexityMedium,
			Labels:      []string{"github", "ci"},
		}, true
	default:
		return models.CreateTaskInput{}, false
	}
}
