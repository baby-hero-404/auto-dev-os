package tool

import (
	"encoding/json"
	"testing"

	"github.com/auto-code-os/auto-code-os/server/pkg/models"
)

func TestStepRequiresEditCaps(t *testing.T) {
	cases := []struct {
		stepID string
		want   bool
	}{
		{"fix", true},
		{"code_backend", true},
		{"code_backend_0", true},
		{"code_frontend", true},
		{"code_frontend_1", true},
		{"review", false},
		{"analyze", false},
		{"plan", false},
		{"context_load", false},
		{"test", false},
		{"merge", false},
		{"pr", false},
	}
	for _, tc := range cases {
		t.Run(tc.stepID, func(t *testing.T) {
			if got := StepRequiresEditCaps(tc.stepID); got != tc.want {
				t.Errorf("StepRequiresEditCaps(%q) = %v, want %v", tc.stepID, got, tc.want)
			}
		})
	}
}

func TestEffectiveRoleForStep(t *testing.T) {
	// Table-driven tests matching design.md § Role Resolution Matrix
	cases := []struct {
		name            string
		stepID          string
		agentRole       string
		primaryCategory string
		wantRole        string
	}{
		// Edit steps, coder roles -> unchanged
		{"edit step under backend", "fix", "backend", "backend", "backend"},
		{"edit step under frontend", "code_frontend", "frontend", "frontend", "frontend"},

		// Edit steps, non-coder roles -> remap to coder role
		{"edit step under reviewer, primary backend", "fix", "reviewer", "backend", "backend"},
		{"edit step under reviewer, primary frontend", "fix", "reviewer", "frontend", "frontend"},
		{"edit step under reviewer, primary ui", "fix", "reviewer", "ui", "frontend"},
		{"edit step under reviewer, primary ux", "fix", "reviewer", "ux", "frontend"},
		{"edit step under reviewer, empty analysis", "fix", "reviewer", "", "backend"},

		{"edit step under planner", "code_backend", "planner", "backend", "backend"},
		{"edit step under qa", "code_backend", "qa", "backend", "backend"},
		{"edit step under unknown role", "code_backend", "unknown_role", "backend", "backend"},

		// Read-only steps -> unchanged
		{"read-only step review under reviewer", "review", "reviewer", "backend", "reviewer"},
		{"read-only step review under planner", "review", "planner", "backend", "planner"},
		{"read-only step analyze under planner", "analyze", "planner", "backend", "planner"},
		{"read-only step plan under planner", "plan", "planner", "backend", "planner"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var analysis models.TaskAnalysis
			analysis.PrimaryCategory = tc.primaryCategory
			analysisBytes, _ := json.Marshal(analysis)
			task := &models.Task{
				Analysis: analysisBytes,
			}
			gotRole := EffectiveRoleForStep(tc.stepID, tc.agentRole, task)
			if gotRole != tc.wantRole {
				t.Errorf("EffectiveRoleForStep(%q, %q, Category=%q) = %q, want %q", tc.stepID, tc.agentRole, tc.primaryCategory, gotRole, tc.wantRole)
			}
		})
	}
}
