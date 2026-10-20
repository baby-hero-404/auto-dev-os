package runtimes

import (
	"embed"
	"fmt"

	"github.com/baby-hero-404/aicli/sandbox"
)

//go:embed */manifest.yaml
var manifestsFS embed.FS

// RuntimeOrder defines the detection priority order when a repo contains multiple language markers.
var RuntimeOrder = []string{"node", "python", "java", "go", "flutter"}

// Load loads all embedded runtime manifests ordered by RuntimeOrder priority.
func Load() ([]sandbox.Manifest, error) {
	manifests, err := sandbox.LoadManifests(manifestsFS)
	if err != nil {
		return nil, fmt.Errorf("load embedded runtime manifests: %w", err)
	}
	byID := make(map[string]sandbox.Manifest, len(manifests))
	for _, m := range manifests {
		byID[m.ID] = m
	}
	ordered := make([]sandbox.Manifest, 0, len(RuntimeOrder))
	for _, id := range RuntimeOrder {
		if m, ok := byID[id]; ok {
			ordered = append(ordered, m)
		}
	}
	return ordered, nil
}
