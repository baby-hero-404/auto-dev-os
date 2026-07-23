package llmrunner

import (
	"strings"
	"testing"
)

func TestBuildProgressNudge_EmptyWhenNoFailures(t *testing.T) {
	if got := buildProgressNudge(15, map[string]int{}); got != "" {
		t.Errorf("expected no nudge when nothing failed, got %q", got)
	}
}

func TestBuildProgressNudge_SummarizesFailCounts(t *testing.T) {
	got := buildProgressNudge(15, map[string]int{"run_build:": 3, "search_replace:main.go": 2})
	if got == "" {
		t.Fatal("expected a nudge message")
	}
	if !strings.Contains(got, "iterations=15") {
		t.Errorf("expected iteration count in message, got %q", got)
	}
	if !strings.Contains(got, "run_build") || !strings.Contains(got, "search_replace") {
		t.Errorf("expected both failed tool names in message, got %q", got)
	}
}

func TestBuildProgressNudge_CallsOutRepeatFailer(t *testing.T) {
	got := buildProgressNudge(30, map[string]int{"search_replace:main.go": 3})
	if !strings.Contains(got, "change approach instead of retrying") {
		t.Errorf("expected repeat-fail callout for a call failing >=3 times, got %q", got)
	}
	if !strings.Contains(got, "main.go") {
		t.Errorf("expected the repeat-failing call's discriminator named, got %q", got)
	}
}

func TestBuildProgressNudge_NoCalloutBelowThreshold(t *testing.T) {
	got := buildProgressNudge(15, map[string]int{"search_replace:main.go": 2})
	if strings.Contains(got, "change approach instead of retrying") {
		t.Errorf("expected no repeat-fail callout below threshold, got %q", got)
	}
}
