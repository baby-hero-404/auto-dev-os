package handler

import (
	"encoding/json"
	"net/http"

	"github.com/auto-code-os/auto-code-os/server/internal/governance"
)

type GovernancePreset struct {
	Name   string          `json:"name"`
	Config json.RawMessage `json:"config"`
}

type GovernanceHandler struct{}

func NewGovernanceHandler() *GovernanceHandler {
	return &GovernanceHandler{}
}

// ListPresets implements GET /api/v1/governance/presets (REQ-002).
func (h *GovernanceHandler) ListPresets(w http.ResponseWriter, _ *http.Request) {
	presets := make([]GovernancePreset, 0, len(governance.PresetNames))
	for _, name := range governance.PresetNames {
		raw, err := governance.Preset(name)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "failed to load preset "+name)
			return
		}
		presets = append(presets, GovernancePreset{
			Name:   name,
			Config: json.RawMessage(raw),
		})
	}
	writeJSON(w, http.StatusOK, presets)
}
