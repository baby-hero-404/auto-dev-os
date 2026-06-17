package orchestrator

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/auto-code-os/auto-code-os/server/pkg/models"
)

type FallbackSkillLister struct {
	primary  SkillLister
	fallback SkillLister
}

func NewFallbackSkillLister(primary, fallback SkillLister) *FallbackSkillLister {
	return &FallbackSkillLister{primary: primary, fallback: fallback}
}

func (l *FallbackSkillLister) List(ctx context.Context) ([]models.Skill, error) {
	if l == nil {
		return nil, nil
	}
	if l.primary != nil {
		skills, err := l.primary.List(ctx)
		if err != nil {
			return nil, err
		}
		if len(skills) > 0 {
			return skills, nil
		}
	}
	if l.fallback == nil {
		return nil, nil
	}
	return l.fallback.List(ctx)
}

type FilesystemSkillLister struct {
	root string
}

func NewFilesystemSkillLister(root string) *FilesystemSkillLister {
	return &FilesystemSkillLister{root: root}
}

type filesystemSkillRegistry struct {
	Skills map[string][]filesystemSkill `json:"skills"`
}

type filesystemSkill struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
	Path        string `json:"path"`
}

func (l *FilesystemSkillLister) List(context.Context) ([]models.Skill, error) {
	root := strings.TrimSpace(l.root)
	if root == "" {
		return nil, nil
	}

	raw, err := os.ReadFile(filepath.Join(root, "registry.min.json"))
	if err != nil {
		return nil, fmt.Errorf("read skills registry: %w", err)
	}

	var registry filesystemSkillRegistry
	if err := json.Unmarshal(raw, &registry); err != nil {
		return nil, fmt.Errorf("unmarshal skills registry: %w", err)
	}

	skills := make([]models.Skill, 0)
	for category, entries := range registry.Skills {
		for _, entry := range entries {
			name := strings.TrimSpace(entry.Name)
			if name == "" {
				name = strings.TrimSpace(entry.ID)
			}
			if name == "" {
				continue
			}
			relPath := skillRegistryPath(category, entry)
			schema, err := json.Marshal(map[string]string{
				"source":   "filesystem",
				"category": category,
				"registry": entry.ID,
				"path":     relPath,
			})
			if err != nil {
				return nil, fmt.Errorf("marshal skill schema for %q: %w", name, err)
			}
			skills = append(skills, models.Skill{
				ID:          entry.ID,
				Name:        name,
				Description: entry.Description,
				Schema:      schema,
			})
		}
	}
	return skills, nil
}

func skillRegistryPath(category string, skill filesystemSkill) string {
	relPath := strings.TrimSpace(skill.Path)
	if strings.HasPrefix(relPath, "antigravity/skills/") {
		return filepath.ToSlash(filepath.Join("system", strings.TrimPrefix(relPath, "antigravity/skills/")))
	}
	if relPath != "" {
		return filepath.ToSlash(filepath.Join("system", relPath))
	}
	return filepath.ToSlash(filepath.Join("system", category, skill.ID))
}
