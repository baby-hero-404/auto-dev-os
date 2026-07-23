package handler

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"

	"github.com/auto-code-os/auto-code-os/server/internal/orchestrator"
	"github.com/auto-code-os/auto-code-os/server/pkg/models"
)

type WebhookHandler struct {
	taskSvc TaskService
	orch    *orchestrator.Orchestrator
}

func NewWebhookHandler(taskSvc TaskService, orch *orchestrator.Orchestrator) *WebhookHandler {
	return &WebhookHandler{taskSvc: taskSvc, orch: orch}
}

// verifyGitHubSignature checks the X-Hub-Signature-256 header (GitHub's
// HMAC-SHA256 of the raw request body, keyed with the shared webhook
// secret) in constant time. GitHub cannot send custom headers, so a
// signature over the body is the only authentication mechanism it supports.
func verifyGitHubSignature(secret string, body []byte, signatureHeader string) bool {
	sig := strings.TrimPrefix(signatureHeader, "sha256=")
	if sig == signatureHeader || sig == "" {
		return false
	}
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(body)
	expected := hex.EncodeToString(mac.Sum(nil))
	return hmac.Equal([]byte(expected), []byte(sig))
}

func (h *WebhookHandler) GitHub(w http.ResponseWriter, r *http.Request) {
	body, err := io.ReadAll(io.LimitReader(r.Body, 10<<20))
	if err != nil {
		writeError(w, http.StatusBadRequest, "failed to read webhook body")
		return
	}

	if secret := os.Getenv("WEBHOOK_SECRET"); secret != "" {
		if !verifyGitHubSignature(secret, body, r.Header.Get("X-Hub-Signature-256")) {
			writeError(w, http.StatusUnauthorized, "invalid webhook signature")
			return
		}
	}

	event := r.Header.Get("X-GitHub-Event")
	var payload map[string]any
	if err := json.Unmarshal(body, &payload); err != nil {
		writeError(w, http.StatusBadRequest, "invalid webhook payload")
		return
	}

	if event == "pull_request" {
		action, _ := payload["action"].(string)
		pr, _ := payload["pull_request"].(map[string]any)
		if pr != nil && action == "closed" {
			merged, _ := pr["merged"].(bool)
			if merged {
				htmlURL, _ := pr["html_url"].(string)
				if htmlURL != "" {
					if h.orch == nil {
						writeError(w, http.StatusInternalServerError, "orchestrator not configured")
						return
					}
					task, err := h.orch.SyncPRMerged(r.Context(), htmlURL)
					if err != nil {
						writeError(w, http.StatusInternalServerError, err.Error())
						return
					}
					writeJSON(w, http.StatusOK, task)
					return
				}
			}
		}
		writeJSON(w, http.StatusAccepted, envelope{"status": "ignored", "reason": "not a merged pull request event"})
		return
	}

	projectID := r.URL.Query().Get("project_id")
	if projectID == "" {
		writeJSON(w, http.StatusAccepted, envelope{"status": "ignored", "reason": "project_id query parameter is required to create tasks"})
		return
	}

	input, ok := githubPayloadToTask(event, payload)
	if !ok {
		writeJSON(w, http.StatusAccepted, envelope{"status": "ignored", "event": event})
		return
	}
	task, err := h.taskSvc.Create(r.Context(), projectID, input)
	if err != nil {
		writeServiceError(w, err)
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
