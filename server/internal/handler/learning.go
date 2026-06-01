package handler

import (
	"net/http"
	"strconv"

	"github.com/auto-code-os/auto-code-os/server/internal/service"
	"github.com/go-chi/chi/v5"
)

// LearningHandler handles HTTP requests for learning suggestion review (HITL).
type LearningHandler struct {
	learningSvc LearningService
}

func NewLearningHandler(learningSvc LearningService) *LearningHandler {
	return &LearningHandler{learningSvc: learningSvc}
}

// ListSuggestions godoc — GET /api/v1/agents/{agentID}/suggestions
func (h *LearningHandler) ListSuggestions(w http.ResponseWriter, r *http.Request) {
	agentID := chi.URLParam(r, "agentID")
	if agentID == "" {
		writeError(w, http.StatusBadRequest, "agent_id is required")
		return
	}

	status := r.URL.Query().Get("status")
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))

	suggestions, err := h.learningSvc.ListSuggestions(r.Context(), agentID, status, limit)
	if err != nil {
		writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, envelope{"suggestions": suggestions})
}

// GetSuggestion godoc — GET /api/v1/suggestions/{suggestionID}
func (h *LearningHandler) GetSuggestion(w http.ResponseWriter, r *http.Request) {
	suggestionID := chi.URLParam(r, "suggestionID")

	suggestion, err := h.learningSvc.GetSuggestion(r.Context(), suggestionID)
	if err != nil {
		writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, envelope{"suggestion": suggestion})
}

// ApproveSuggestion godoc — POST /api/v1/suggestions/{suggestionID}/approve
func (h *LearningHandler) ApproveSuggestion(w http.ResponseWriter, r *http.Request) {
	suggestionID := chi.URLParam(r, "suggestionID")

	// Extract user ID from auth context (set by AuthMiddleware)
	claims, _ := r.Context().Value(authClaimsKey).(*service.TokenClaims)
	if claims == nil {
		writeError(w, http.StatusUnauthorized, "authentication required")
		return
	}

	suggestion, err := h.learningSvc.ApproveSuggestion(r.Context(), suggestionID, claims.Subject)
	if err != nil {
		writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, envelope{"suggestion": suggestion})
}

// RejectSuggestion godoc — POST /api/v1/suggestions/{suggestionID}/reject
func (h *LearningHandler) RejectSuggestion(w http.ResponseWriter, r *http.Request) {
	suggestionID := chi.URLParam(r, "suggestionID")

	claims, _ := r.Context().Value(authClaimsKey).(*service.TokenClaims)
	if claims == nil {
		writeError(w, http.StatusUnauthorized, "authentication required")
		return
	}

	var body struct {
		Feedback string `json:"feedback"`
	}
	if err := decodeJSON(r, &body); err != nil {
		body.Feedback = ""
	}

	suggestion, err := h.learningSvc.RejectSuggestion(r.Context(), suggestionID, claims.Subject, body.Feedback)
	if err != nil {
		writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, envelope{"suggestion": suggestion})
}
