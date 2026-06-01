package handler

import (
	"net/http"
	"strconv"

	"github.com/auto-code-os/auto-code-os/server/pkg/models"
	"github.com/go-chi/chi/v5"
)

// MemoryHandler handles HTTP requests for episodic memory browsing and search.
type MemoryHandler struct {
	memorySvc MemoryService
}

func NewMemoryHandler(memorySvc MemoryService) *MemoryHandler {
	return &MemoryHandler{memorySvc: memorySvc}
}

// ListByAgent godoc — GET /api/v1/agents/{agentID}/memories
func (h *MemoryHandler) ListByAgent(w http.ResponseWriter, r *http.Request) {
	agentID := chi.URLParam(r, "agentID")
	if agentID == "" {
		writeError(w, http.StatusBadRequest, "agent_id is required")
		return
	}

	tier := r.URL.Query().Get("tier")
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	offset, _ := strconv.Atoi(r.URL.Query().Get("offset"))

	memories, err := h.memorySvc.ListByAgent(r.Context(), agentID, tier, limit, offset)
	if err != nil {
		writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, envelope{"memories": memories})
}

// Search godoc — GET /api/v1/agents/{agentID}/memories/search?q=...
func (h *MemoryHandler) Search(w http.ResponseWriter, r *http.Request) {
	agentID := chi.URLParam(r, "agentID")
	if agentID == "" {
		writeError(w, http.StatusBadRequest, "agent_id is required")
		return
	}

	query := r.URL.Query().Get("q")
	if query == "" {
		writeError(w, http.StatusBadRequest, "query parameter 'q' is required")
		return
	}

	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	if limit <= 0 {
		limit = 10
	}

	results, err := h.memorySvc.Search(r.Context(), models.MemorySearchInput{
		Query:   query,
		AgentID: agentID,
		Limit:   limit,
	})
	if err != nil {
		writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, envelope{"results": results})
}

// GetByID godoc — GET /api/v1/memories/{memoryID}
func (h *MemoryHandler) GetByID(w http.ResponseWriter, r *http.Request) {
	memoryID := chi.URLParam(r, "memoryID")

	memory, err := h.memorySvc.GetByID(r.Context(), memoryID)
	if err != nil {
		writeServiceError(w, err)
		return
	}

	edges, err := h.memorySvc.GetEdgesByMemory(r.Context(), memoryID)
	if err != nil {
		edges = nil // non-fatal
	}

	writeJSON(w, http.StatusOK, envelope{"memory": memory, "edges": edges})
}

// Delete godoc — DELETE /api/v1/memories/{memoryID}
func (h *MemoryHandler) Delete(w http.ResponseWriter, r *http.Request) {
	memoryID := chi.URLParam(r, "memoryID")

	if err := h.memorySvc.Delete(r.Context(), memoryID); err != nil {
		writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, envelope{"deleted": true})
}
