package governance

import "testing"

func TestConfig_NilConfigIsNoOp(t *testing.T) {
	var c *Config
	if c.IsDorDisabled() {
		t.Error("expected nil config to not disable DoR")
	}
	if c.ShouldSkipStepForLabels("review", []string{"hotfix"}) {
		t.Error("expected nil config to not skip any step")
	}
	if c.IsStepDisabled("dor_check") {
		t.Error("expected nil config to not disable any step")
	}
	if _, ok := c.RoutingOverride("analyze"); ok {
		t.Error("expected nil config to have no routing override")
	}
}

func TestConfig_ShouldSkipStepForLabels_MatchesHotfix(t *testing.T) {
	c := &Config{Pipeline: &Pipeline{Steps: []StepOverride{
		{ID: "review", SkipWhen: &SkipWhen{Label: "hotfix"}},
	}}}
	if !c.ShouldSkipStepForLabels("review", []string{"backend", "Hotfix"}) {
		t.Error("expected case-insensitive label match to skip review")
	}
	if c.ShouldSkipStepForLabels("review", []string{"backend"}) {
		t.Error("expected no skip without matching label")
	}
	if c.ShouldSkipStepForLabels("merge", []string{"hotfix"}) {
		t.Error("expected no skip for an unrelated step id")
	}
}

func TestConfig_IsStepDisabled(t *testing.T) {
	disabled := false
	c := &Config{Pipeline: &Pipeline{Steps: []StepOverride{
		{ID: "dor_check", Enabled: &disabled},
	}}}
	if !c.IsStepDisabled("dor_check") {
		t.Error("expected dor_check to be disabled")
	}
	if c.IsStepDisabled("review") {
		t.Error("expected review to not be disabled")
	}
}

func TestConfig_RoutingOverride(t *testing.T) {
	c := &Config{Policies: &Policies{Routing: map[string]string{"analyze": "balanced"}}}
	level, ok := c.RoutingOverride("analyze")
	if !ok || level != "balanced" {
		t.Fatalf("expected override balanced, got %q ok=%v", level, ok)
	}
	if _, ok := c.RoutingOverride("plan"); ok {
		t.Error("expected no override for unlisted step")
	}
}

func TestConfig_IsDorDisabled(t *testing.T) {
	c := &Config{Policies: &Policies{Dor: &DorPolicy{Disabled: true}}}
	if !c.IsDorDisabled() {
		t.Error("expected DoR to be disabled")
	}
}
