package sandbox

import (
	"embed"
	"fmt"

	"gopkg.in/yaml.v3"
)

// runtimeManifestsFS embeds every runtimes/<id>/manifest.yaml at build time
// (matching the go:embed convention already used elsewhere in this repo,
// e.g. pkg/config/config.go's embedded config.yaml) so the server binary
// never depends on a runtime filesystem path for these — a container image
// or a `go run` from a different working directory would otherwise silently
// find zero manifests.
//
//go:embed runtimes/*/manifest.yaml
var runtimeManifestsFS embed.FS

// CacheMapping is one entry of a manifest's cache: list — a single host
// directory (may use "~" for the invoking user's home dir, expanded by the
// caller, e.g. SandboxManager) bind-mounted at Container inside the
// sandbox.
type CacheMapping struct {
	Host      string `yaml:"host"`
	Container string `yaml:"container"`
}

// Manifest describes one supported project runtime: how to detect it (via
// marker files in the workspace root), which image to run it in, which
// caches to mount, and which setup/healthcheck commands to run before
// handing the container off to the agent's real command.
type Manifest struct {
	ID          string         `yaml:"id"`
	Image       string         `yaml:"image"`
	Detect      []string       `yaml:"detect"`
	Cache       []CacheMapping `yaml:"cache"`
	Setup       []string       `yaml:"setup"`
	Healthcheck string         `yaml:"healthcheck"`
}

// runtimeOrder is the explicit, documented load/detection order: when a
// workspace matches more than one manifest's marker files (rare, but e.g. a
// polyglot repo with both package.json and go.mod), the first manifest in
// this list wins. This is a deliberately dumb, predictable tie-break — not
// an attempt to intelligently guess the "primary" language of a repo — so
// detection results are reproducible across runs instead of depending on
// map/filesystem iteration order.
var runtimeOrder = []string{"node", "python", "java", "go", "flutter"}

// Registry holds every embedded runtime manifest, loaded once at startup.
type Registry struct {
	manifests []*Manifest
	byID      map[string]*Manifest
}

// NewRegistry loads and parses every embedded runtimes/*/manifest.yaml,
// returning an error if any manifest is missing or malformed — a broken
// manifest should fail server startup, not silently disable one runtime's
// detection.
func NewRegistry() (*Registry, error) {
	reg := &Registry{
		byID: make(map[string]*Manifest, len(runtimeOrder)),
	}
	for _, id := range runtimeOrder {
		data, err := runtimeManifestsFS.ReadFile(fmt.Sprintf("runtimes/%s/manifest.yaml", id))
		if err != nil {
			return nil, fmt.Errorf("read runtime manifest %q: %w", id, err)
		}
		var m Manifest
		if err := yaml.Unmarshal(data, &m); err != nil {
			return nil, fmt.Errorf("parse runtime manifest %q: %w", id, err)
		}
		if m.ID == "" {
			m.ID = id
		}
		if m.ID != id {
			return nil, fmt.Errorf("runtime manifest %q declares mismatched id %q", id, m.ID)
		}
		reg.manifests = append(reg.manifests, &m)
		reg.byID[m.ID] = &m
	}
	return reg, nil
}

// Get returns the manifest for runtimeID, if registered.
func (r *Registry) Get(runtimeID string) (*Manifest, bool) {
	m, ok := r.byID[runtimeID]
	return m, ok
}

// All returns every registered manifest in detection-priority order (see
// runtimeOrder).
func (r *Registry) All() []*Manifest {
	return r.manifests
}
