package engine

import (
	"testing"

	"github.com/auto-code-os/auto-code-os/server/pkg/models"
)

func TestResolveEngine(t *testing.T) {
	cli := models.ExecutionEngineCLI
	empty := ""

	cases := []struct {
		name          string
		taskEngine    *string
		projectEngine string
		want          string
	}{
		{"task override wins", &cli, models.ExecutionEngineAPINative, models.ExecutionEngineCLI},
		{"nil task falls back to project", nil, models.ExecutionEngineCLI, models.ExecutionEngineCLI},
		{"empty task string falls back to project", &empty, models.ExecutionEngineCLI, models.ExecutionEngineCLI},
		{"nothing set falls back to api_native", nil, "", models.ExecutionEngineAPINative},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := ResolveEngine(tc.taskEngine, tc.projectEngine)
			if got != tc.want {
				t.Errorf("ResolveEngine() = %q, want %q", got, tc.want)
			}
		})
	}
}
