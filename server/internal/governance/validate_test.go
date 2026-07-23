package governance

import "testing"

func TestValidateConfig_ValidPatchModeConfig(t *testing.T) {
	raw := []byte(`{
		"version": 1,
		"pipeline": {"extends": "api_native", "steps": [{"id": "dor_check", "enabled": false}]},
		"policies": {"routing": {"analyze": "balanced"}, "max_review_fix_cycles": 5}
	}`)
	cfg, errs, err := ValidateConfig(raw)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(errs) != 0 {
		t.Fatalf("expected no validation errors, got %v", errs)
	}
	if cfg == nil || cfg.Policies.Routing["analyze"] != "balanced" {
		t.Fatalf("expected parsed config with routing override, got %+v", cfg)
	}
}

func TestValidateConfig_RejectsWrongVersion(t *testing.T) {
	raw := []byte(`{"version": 2}`)
	_, errs, err := ValidateConfig(raw)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !hasMessage(errs, "unsupported config version") {
		t.Fatalf("expected version error, got %v", errs)
	}
}

func TestValidateConfig_RejectsUnknownField(t *testing.T) {
	raw := []byte(`{"version": 1, "policies": {"unknown_field": true}}`)
	_, errs, err := ValidateConfig(raw)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(errs) == 0 {
		t.Fatalf("expected schema validation error for unknown field")
	}
}

func TestValidateConfig_RejectsInvalidRoutingLevel(t *testing.T) {
	raw := []byte(`{"version": 1, "policies": {"routing": {"analyze": "ultra"}}}`)
	_, errs, err := ValidateConfig(raw)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(errs) == 0 {
		t.Fatalf("expected schema validation error for invalid routing level")
	}
}

func TestValidateConfig_FullCustomGraphRunsDAGChecks(t *testing.T) {
	raw := []byte(`{
		"version": 1,
		"pipeline": {"steps": [
			{"id": "a", "dependsOn": []},
			{"id": "b", "dependsOn": ["missing"]}
		]}
	}`)
	_, errs, err := ValidateConfig(raw)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !hasMessage(errs, "unresolved dependency") {
		t.Fatalf("expected DAG check to run and report unresolved dependency, got %v", errs)
	}
}

func TestValidateConfig_PatchModeSkipsDAGChecks(t *testing.T) {
	raw := []byte(`{
		"version": 1,
		"pipeline": {"extends": "api_native", "steps": [{"id": "review", "skip_when": {"label": "hotfix"}}]}
	}`)
	_, errs, err := ValidateConfig(raw)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(errs) != 0 {
		t.Fatalf("expected patch-mode config (no dependsOn) to skip DAG checks, got %v", errs)
	}
}

func TestValidateConfig_InvalidJSON(t *testing.T) {
	_, errs, err := ValidateConfig([]byte(`not json`))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(errs) == 0 {
		t.Fatalf("expected invalid JSON error")
	}
}

func TestPresets_LoadSuccessfully(t *testing.T) {
	for _, name := range PresetNames {
		raw, err := Preset(name)
		if err != nil {
			t.Fatalf("preset %s: %v", name, err)
		}
		if _, errs, err := ValidateConfig(raw); err != nil || len(errs) != 0 {
			t.Fatalf("preset %s failed validation: err=%v errs=%v", name, err, errs)
		}
	}
}
