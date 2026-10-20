package runtimes

import (
	"testing"
)

func TestLoad(t *testing.T) {
	manifests, err := Load()
	if err != nil {
		t.Fatalf("Load() unexpected error: %v", err)
	}
	if len(manifests) != 5 {
		t.Fatalf("expected 5 manifests, got %d", len(manifests))
	}
	for i, expectedID := range RuntimeOrder {
		if manifests[i].ID != expectedID {
			t.Errorf("manifests[%d].ID = %q, want %q", i, manifests[i].ID, expectedID)
		}
	}
}
