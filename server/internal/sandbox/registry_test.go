package sandbox

import "testing"

func TestNewRegistryLoadsEmbeddedManifests(t *testing.T) {
	reg, err := NewRegistry()
	if err != nil {
		t.Fatalf("NewRegistry() error = %v", err)
	}

	wantIDs := []string{"node", "python", "java", "go", "flutter"}
	all := reg.All()
	if len(all) != len(wantIDs) {
		t.Fatalf("All() returned %d manifests, want %d", len(all), len(wantIDs))
	}
	for i, id := range wantIDs {
		if all[i].ID != id {
			t.Errorf("All()[%d].ID = %q, want %q (registry order must be deterministic)", i, all[i].ID, id)
		}
	}
}

func TestRegistryGet(t *testing.T) {
	reg, err := NewRegistry()
	if err != nil {
		t.Fatalf("NewRegistry() error = %v", err)
	}

	m, ok := reg.Get("python")
	if !ok {
		t.Fatal("Get(\"python\") ok = false, want true")
	}
	if m.Image == "" {
		t.Error("python manifest Image is empty")
	}
	if len(m.Detect) == 0 {
		t.Error("python manifest Detect is empty")
	}

	if _, ok := reg.Get("nonexistent"); ok {
		t.Error("Get(\"nonexistent\") ok = true, want false")
	}
}

func TestRegistryFlutterCacheUsesSandboxHomeDir(t *testing.T) {
	reg, err := NewRegistry()
	if err != nil {
		t.Fatalf("NewRegistry() error = %v", err)
	}
	m, ok := reg.Get("flutter")
	if !ok {
		t.Fatal("Get(\"flutter\") ok = false")
	}
	if len(m.Cache) == 0 {
		t.Fatal("flutter manifest has no cache entries")
	}
	for _, c := range m.Cache {
		if c.Container == "" || c.Host == "" {
			t.Errorf("flutter cache entry has empty field: %+v", c)
		}
	}
}
