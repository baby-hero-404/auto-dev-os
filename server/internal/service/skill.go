package service

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/auto-code-os/auto-code-os/server/internal/repository"
	"github.com/auto-code-os/auto-code-os/server/pkg/models"
	"github.com/auto-code-os/auto-code-os/server/pkg/paths"
)

type SkillService struct {
	repo       *repository.SkillRepo
	sourceRepo *repository.SkillSourceRepo
	skillPaths paths.SkillPaths
	fs         paths.FileSystem
}

func NewSkillService(repo *repository.SkillRepo, sourceRepo *repository.SkillSourceRepo, skillPaths paths.SkillPaths, fs paths.FileSystem) *SkillService {
	return &SkillService{
		repo:       repo,
		sourceRepo: sourceRepo,
		skillPaths: skillPaths,
		fs:         fs,
	}
}

type skillRegistry struct {
	Skills map[string][]registrySkill `json:"skills"`
}

type registrySkill struct {
	ID          string          `json:"id"`
	Name        string          `json:"name"`
	Description string          `json:"description"`
	Path        string          `json:"path"`
	Schema      json.RawMessage `json:"schema,omitempty"`
}

func (s *SkillService) loadRegistry() (skillRegistry, error) {
	var reg skillRegistry
	reg.Skills = make(map[string][]registrySkill)

	regPath := s.skillPaths.GlobalRegistryPath(false)
	if !s.fs.Exists(regPath) {
		regPath = s.skillPaths.GlobalRegistryPath(true)
	}

	if !s.fs.Exists(regPath) {
		return reg, nil
	}

	raw, err := s.fs.ReadFile(regPath)
	if err != nil {
		return reg, err
	}

	if err := json.Unmarshal(raw, &reg); err != nil {
		return reg, err
	}

	return reg, nil
}

func (s *SkillService) saveRegistry(reg skillRegistry) error {
	if err := s.fs.EnsureDir(s.skillPaths.Root()); err != nil {
		return err
	}
	regPath := s.skillPaths.GlobalRegistryPath(false)
	raw, err := json.MarshalIndent(reg, "", "  ")
	if err != nil {
		return err
	}
	return s.fs.WriteFile(regPath, raw, 0644)
}

func (s *SkillService) getActiveRepoNames(ctx context.Context) (map[string]bool, error) {
	sources, err := s.sourceRepo.List(ctx)
	if err != nil {
		return nil, err
	}
	activeRepos := make(map[string]bool)
	for _, source := range sources {
		repoName := getRepoNameFromURL(source.URL)
		activeRepos[strings.ToLower(repoName)] = true
	}
	return activeRepos, nil
}

func (s *SkillService) isSkillActive(ctx context.Context, skillSchema []byte, activeRepos map[string]bool) bool {
	if len(activeRepos) == 0 {
		return false
	}
	if len(skillSchema) == 0 {
		return false
	}
	var meta struct {
		Source string `json:"source"`
		Repo   string `json:"repo"`
	}
	if err := json.Unmarshal(skillSchema, &meta); err != nil {
		return false
	}
	return meta.Source == "git" && activeRepos[strings.ToLower(meta.Repo)]
}

func (s *SkillService) GetByID(ctx context.Context, id string) (*models.Skill, error) {
	activeRepos, err := s.getActiveRepoNames(ctx)
	if err != nil {
		return nil, fmt.Errorf("get active repos: %w", err)
	}

	reg, err := s.loadRegistry()
	if err != nil {
		return nil, fmt.Errorf("load registry: %w", err)
	}

	for _, skills := range reg.Skills {
		for _, sk := range skills {
			if sk.ID == id {
				if !s.isSkillActive(ctx, sk.Schema, activeRepos) {
					return nil, ErrNotFound
				}
				return &models.Skill{
					ID:          sk.ID,
					Name:        sk.Name,
					Description: sk.Description,
					Schema:      sk.Schema,
				}, nil
			}
		}
	}

	return nil, ErrNotFound
}

func (s *SkillService) List(ctx context.Context) ([]models.Skill, error) {
	activeRepos, err := s.getActiveRepoNames(ctx)
	if err != nil {
		return nil, fmt.Errorf("get active repos: %w", err)
	}
	if len(activeRepos) == 0 {
		return []models.Skill{}, nil
	}

	reg, err := s.loadRegistry()
	if err != nil {
		return nil, fmt.Errorf("load registry: %w", err)
	}

	var list []models.Skill
	for _, skills := range reg.Skills {
		for _, sk := range skills {
			if !s.isSkillActive(ctx, sk.Schema, activeRepos) {
				continue
			}
			list = append(list, models.Skill{
				ID:          sk.ID,
				Name:        sk.Name,
				Description: sk.Description,
				Schema:      sk.Schema,
			})
		}
	}

	return list, nil
}

func (s *SkillService) Test(ctx context.Context, id string, input map[string]any) (map[string]any, error) {
	skill, err := s.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	return map[string]any{
		"skill_id": skill.ID,
		"name":     skill.Name,
		"dry_run":  true,
		"input":    input,
		"schema":   json.RawMessage(skill.Schema),
	}, nil
}

func (s *SkillService) ListFiles(ctx context.Context, sourceID string, relativePath string) ([]models.FileItem, error) {
	if strings.HasPrefix(filepath.Clean(relativePath), "..") {
		return nil, fmt.Errorf("permission denied: path escapes boundary")
	}

	source, err := s.sourceRepo.GetByID(ctx, sourceID)
	if err != nil {
		return nil, err
	}

	repoName := getRepoNameFromURL(source.URL)
	repoRoot := s.skillPaths.GitRepoRoot(repoName)
	targetDir := repoRoot.Child(relativePath)

	absRepoRoot, err := filepath.Abs(repoRoot.String())
	if err != nil {
		return nil, fmt.Errorf("invalid repository path: %w", err)
	}
	absTargetDir, err := filepath.Abs(targetDir.String())
	if err != nil {
		return nil, fmt.Errorf("invalid target path: %w", err)
	}

	if !strings.HasPrefix(absTargetDir, absRepoRoot) {
		return nil, fmt.Errorf("permission denied: path escapes boundary")
	}

	entries, err := os.ReadDir(absTargetDir)
	if err != nil {
		return nil, fmt.Errorf("read directory: %w", err)
	}

	var files []models.FileItem
	for _, entry := range entries {
		info, err := entry.Info()
		if err != nil {
			continue
		}
		relPath, err := filepath.Rel(absRepoRoot, filepath.Join(absTargetDir, entry.Name()))
		if err != nil {
			continue
		}
		files = append(files, models.FileItem{
			Name:  entry.Name(),
			Path:  filepath.ToSlash(relPath),
			IsDir: entry.IsDir(),
			Size:  info.Size(),
		})
	}

	return files, nil
}

func (s *SkillService) GetFileContent(ctx context.Context, sourceID string, relativePath string) (*models.FileContent, error) {
	if strings.HasPrefix(filepath.Clean(relativePath), "..") {
		return nil, fmt.Errorf("permission denied: path escapes boundary")
	}

	source, err := s.sourceRepo.GetByID(ctx, sourceID)
	if err != nil {
		return nil, err
	}

	repoName := getRepoNameFromURL(source.URL)
	repoRoot := s.skillPaths.GitRepoRoot(repoName)
	targetFile := repoRoot.File(relativePath)

	absRepoRoot, err := filepath.Abs(repoRoot.String())
	if err != nil {
		return nil, fmt.Errorf("invalid repository path: %w", err)
	}
	absTargetFile, err := filepath.Abs(targetFile.String())
	if err != nil {
		return nil, fmt.Errorf("invalid target path: %w", err)
	}

	if !strings.HasPrefix(absTargetFile, absRepoRoot) {
		return nil, fmt.Errorf("permission denied: path escapes boundary")
	}

	info, err := os.Stat(absTargetFile)
	if err != nil {
		return nil, fmt.Errorf("stat file: %w", err)
	}
	if info.IsDir() {
		return nil, fmt.Errorf("cannot read a directory as a file")
	}
	if info.Size() > 2*1024*1024 {
		return nil, fmt.Errorf("file exceeds 2MB limit")
	}

	raw, err := s.fs.ReadFile(targetFile)
	if err != nil {
		return nil, fmt.Errorf("read file: %w", err)
	}

	relPath, err := filepath.Rel(absRepoRoot, absTargetFile)
	if err != nil {
		relPath = relativePath
	}

	return &models.FileContent{
		Content: string(raw),
		Path:    filepath.ToSlash(relPath),
	}, nil
}

func (s *SkillService) SeedDefaultSkills(ctx context.Context) ([]models.Skill, error) {
	// Seed default git source if empty
	sources, err := s.sourceRepo.List(ctx)
	if err == nil && len(sources) == 0 {
		_, _ = s.sourceRepo.Create(ctx, models.CreateSkillSourceInput{
			URL: "https://github.com/baby-hero-404/prompt_base.git",
		})
	}

	return s.List(ctx)
}

func (s *SkillService) ListSources(ctx context.Context) ([]models.SkillSource, error) {
	return s.sourceRepo.List(ctx)
}

func (s *SkillService) AddSource(ctx context.Context, input models.CreateSkillSourceInput) (*models.SkillSource, error) {
	normalizedURL, err := validateSkillSourceURL(input.URL)
	if err != nil {
		return nil, err
	}
	input.URL = normalizedURL
	src, err := s.sourceRepo.Create(ctx, input)
	if err != nil {
		return nil, err
	}
	syncedSrc, syncErr := s.SyncSource(ctx, src.ID)
	if syncErr != nil {
		// Return the updated failed source state rather than returning a hard error,
		// so that the record is visible to the user as failed.
		return syncedSrc, nil
	}
	return syncedSrc, nil
}

func (s *SkillService) DeleteSource(ctx context.Context, id string) error {
	return s.sourceRepo.Delete(ctx, id)
}

func getRepoNameFromURL(gitURL string) string {
	gitURL = strings.TrimSuffix(gitURL, ".git")
	parts := strings.Split(gitURL, "/")
	if len(parts) > 0 {
		return parts[len(parts)-1]
	}
	return "unknown"
}
