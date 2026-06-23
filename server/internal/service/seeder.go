package service

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/auto-code-os/auto-code-os/server/internal/repository"
	"github.com/auto-code-os/auto-code-os/server/pkg/models"
)

// SeederService seeds default rules and skills when a new project is created.
type SeederService struct {
	ruleRepo   *repository.RuleRepo
	skillRepo  *repository.SkillRepo
	skillsRoot string
}

type promptBaseRegistry struct {
	Skills map[string][]promptBaseSkill `json:"skills"`
}

type promptBaseSkill struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
	Path        string `json:"path"`
}

// NewSeederService creates a SeederService with the required repositories.
func NewSeederService(ruleRepo *repository.RuleRepo, skillRepo *repository.SkillRepo, skillsRoot string) *SeederService {
	return &SeederService{ruleRepo: ruleRepo, skillRepo: skillRepo, skillsRoot: skillsRoot}
}

// SeedProject inserts default rules and skills for a newly created project.
// Errors are logged but do not prevent project creation from succeeding.
func (s *SeederService) SeedProject(ctx context.Context, projectID string) {
	// Seeding of default rules and skills disabled as requested
	// s.seedRules(ctx, projectID)
	// s.seedSkills(ctx)
}

func loadPromptBaseSkills(skillsRoot string) ([]models.CreateSkillInput, error) {
	var registryPath string
	var err error
	if skillsRoot != "" {
		registryPath = filepath.Join(skillsRoot, "registry.min.json")
	} else {
		registryPath, err = promptBaseRegistryPath()
		if err != nil {
			return nil, err
		}
	}

	raw, err := os.ReadFile(registryPath)
	if err != nil {
		return nil, fmt.Errorf("read prompt base registry: %w", err)
	}

	var registry promptBaseRegistry
	if err := json.Unmarshal(raw, &registry); err != nil {
		return nil, fmt.Errorf("unmarshal prompt base registry: %w", err)
	}

	var defaults []models.CreateSkillInput
	for category, skills := range registry.Skills {
		for _, skill := range skills {
			if skill.Name == "" {
				continue
			}

			// Map legacy path format (e.g. "antigravity/skills/tech/react-patterns")
			// to the centralized system subfolders (e.g. "system/tech/react-patterns")
			relPath := skill.Path
			if strings.HasPrefix(relPath, "antigravity/skills/") {
				relPath = filepath.Join("system", strings.TrimPrefix(relPath, "antigravity/skills/"))
			} else {
				relPath = filepath.Join("system", category, skill.ID)
			}

			schema, err := json.Marshal(map[string]string{
				"source":   "prompt_base",
				"category": category,
				"registry": skill.ID,
				"path":     relPath,
			})
			if err != nil {
				return nil, fmt.Errorf("marshal prompt base skill schema for %q: %w", skill.Name, err)
			}

			defaults = append(defaults, models.CreateSkillInput{
				Name:        skill.Name,
				Description: skill.Description,
				Schema:      schema,
			})
		}
	}

	if len(defaults) == 0 {
		return nil, fmt.Errorf("prompt base registry did not contain any skills")
	}

	return defaults, nil
}

func promptBaseRegistryPath() (string, error) {
	_, filename, _, ok := runtime.Caller(0)
	if !ok {
		return "", fmt.Errorf("resolve runtime caller for seeder")
	}
	return filepath.Clean(filepath.Join(filepath.Dir(filename), "..", "..", "..", "resources", "prompt_base", "registry.min.json")), nil
}
