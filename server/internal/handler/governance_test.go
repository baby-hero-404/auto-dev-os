package handler

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
)

func TestGovernanceHandler_ListPresets(t *testing.T) {
	h := NewGovernanceHandler()

	r := chi.NewRouter()
	r.Get("/governance/presets", h.ListPresets)

	req := httptest.NewRequest(http.MethodGet, "/governance/presets", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var presets []GovernancePreset
	if err := json.NewDecoder(w.Body).Decode(&presets); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if len(presets) != 2 {
		t.Fatalf("expected 2 presets, got %d", len(presets))
	}

	presetMap := make(map[string]json.RawMessage)
	for _, p := range presets {
		presetMap[p.Name] = p.Config
	}

	if _, ok := presetMap["default_workflow"]; !ok {
		t.Errorf("missing default_workflow preset")
	}
	if _, ok := presetMap["strict_review"]; !ok {
		t.Errorf("missing strict_review preset")
	}
}
