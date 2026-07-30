package prompts

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/auto-code-os/auto-code-os/server/pkg/models"
)

type fakeLearnedSkillsLister struct {
	skills []models.LearnedSkill
}

func (l fakeLearnedSkillsLister) SearchActiveByText(ctx context.Context, projectID string, query string, limit int) ([]models.LearnedSkill, error) {
	return l.skills, nil
}

func TestMaterializeCLIContext_ReturnsLayer1And2WhenAvailable(t *testing.T) {
	engine := &MockContextEngine{}
	learned := fakeLearnedSkillsLister{
		skills: []models.LearnedSkill{
			{ID: "ls-1", Title: "avoid flaky test", Content: "use real DB not mocks"},
		},
	}
	assembler := NewPromptAssembler(testBaseTools(), engine).
		WithLearnedSkillsLister(learned)

	task := models.Task{
		ID:        "task-1",
		ProjectID: "proj-1",
		Title:     "Fix bug",
		Analysis:  json.RawMessage(`{"task_rules":["always write tests"]}`),
	}

	files, err := assembler.MaterializeCLIContext(context.Background(), task, &models.Agent{Role: "backend"}, "code_backend")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	for _, want := range []string{"manifest.json", "README.md", "relevant/learned_skills.md", "relevant/task_rules.md"} {
		if _, ok := files[want]; !ok {
			t.Errorf("expected files to contain %q, got keys: %v", want, keysOf(files))
		}
	}
	if files["relevant/task_rules.md"] == "" || !strings.Contains(files["relevant/task_rules.md"], "always write tests") {
		t.Errorf("expected task_rules.md to contain the rule, got %q", files["relevant/task_rules.md"])
	}
	if !strings.Contains(files["relevant/learned_skills.md"], "use real DB not mocks") {
		t.Errorf("expected learned_skills.md to contain learned skill content, got %q", files["relevant/learned_skills.md"])
	}
}

func TestMaterializeCLIContext_EmptyWhenNothingAvailable(t *testing.T) {
	engine := &MockContextEngine{}
	assembler := NewPromptAssembler(testBaseTools(), engine)

	task := models.Task{ID: "task-2", ProjectID: "proj-2", Title: "No-op task"}

	files, err := assembler.MaterializeCLIContext(context.Background(), task, nil, "code_backend")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(files) != 0 {
		t.Errorf("expected empty map when nothing available, got: %v", keysOf(files))
	}
}

func TestSlugifySkillName_StripsPathTraversalCharacters(t *testing.T) {
	cases := map[string]string{
		"../../../../etc/cron.d/evil":                "etc-cron-d-evil",
		"../../../home/appuser/.ssh/authorized_keys": "home-appuser-ssh-authorized-keys",
		"API Patterns": "api-patterns",
		"api_patterns": "api-patterns",
		"...":          "skill",
		"":             "skill",
	}
	for in, want := range cases {
		got := slugifySkillName(in)
		if got != want {
			t.Errorf("slugifySkillName(%q) = %q, want %q", in, got, want)
		}
		if strings.Contains(got, "/") || strings.Contains(got, "..") {
			t.Errorf("slugifySkillName(%q) = %q still contains traversal-capable characters", in, got)
		}
	}
}

func TestMaterializeCLIContext_CollidingSkillSlugsDoNotClobberEachOther(t *testing.T) {
	tmpDir := t.TempDir()
	projectID := "proj-collide"
	skillsDir := filepath.Join(tmpDir, "projects", projectID, "skills")
	if err := os.MkdirAll(skillsDir, 0755); err != nil {
		t.Fatal(err)
	}

	// Two distinct skill files whose frontmatter names slugify to the same
	// string ("API Patterns" and "api_patterns" both -> "api-patterns").
	if err := os.WriteFile(filepath.Join(skillsDir, "a.md"), []byte("---\nname: API Patterns\ndescription: first\n---\nfirst skill content"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(skillsDir, "b.md"), []byte("---\nname: api_patterns\ndescription: second\n---\nsecond skill content"), 0644); err != nil {
		t.Fatal(err)
	}
	registry := `{
		"skills": {
			"custom": [
				{"id": "skill-a", "name": "API Patterns", "path": "a.md", "schema": {"allowed_tools": []}},
				{"id": "skill-b", "name": "api_patterns", "path": "b.md", "schema": {"allowed_tools": []}}
			]
		}
	}`
	if err := os.WriteFile(filepath.Join(skillsDir, "registry.json"), []byte(registry), 0644); err != nil {
		t.Fatal(err)
	}

	engine := &MockContextEngine{}
	assembler := NewPromptAssembler(testBaseTools(), engine).WithDataRoot(tmpDir)

	task := models.Task{
		ID:          "task-collide",
		ProjectID:   projectID,
		Title:       "api patterns task",
		Description: "needs api_patterns and API Patterns guidance",
	}

	files, err := assembler.MaterializeCLIContext(context.Background(), task, &models.Agent{Role: "backend"}, "code_backend")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var skillFiles []string
	for path := range files {
		if strings.HasPrefix(path, "relevant/skills/") {
			skillFiles = append(skillFiles, path)
		}
	}
	if len(skillFiles) != 2 {
		t.Fatalf("expected 2 distinct skill files for colliding slugs, got %d: %v", len(skillFiles), skillFiles)
	}
	if !strings.Contains(files["relevant/skills/api-patterns.md"], "first skill content") &&
		!strings.Contains(files["relevant/skills/api-patterns.md"], "second skill content") {
		t.Fatalf("expected api-patterns.md to retain one skill's content, got %q", files["relevant/skills/api-patterns.md"])
	}
}

func keysOf(m map[string]string) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	return keys
}
