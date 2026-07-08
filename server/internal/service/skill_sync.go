package service

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"os/exec"
	"strings"
	"time"

	"github.com/auto-code-os/auto-code-os/server/internal/gitops"
	"github.com/auto-code-os/auto-code-os/server/pkg/models"
)

func (s *SkillService) SyncSource(ctx context.Context, id string) (*models.SkillSource, error) {
	source, err := s.sourceRepo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	normalizedURL, err := validateSkillSourceURL(source.URL)
	if err != nil {
		return s.markSyncFailed(ctx, id, err)
	}
	source.URL = normalizedURL

	statusSyncing := "syncing"
	_, _ = s.sourceRepo.Update(ctx, id, models.UpdateSkillSourceInput{Status: &statusSyncing})

	repoName := getRepoNameFromURL(source.URL)
	targetDir := s.skillPaths.GitRepoRoot(repoName)

	var cmd *exec.Cmd
	if !s.fs.Exists(targetDir) {
		if err = s.fs.EnsureDir(s.skillPaths.GitSourceRoot()); err != nil {
			return s.markSyncFailed(ctx, id, fmt.Errorf("mkdir: %w", err))
		}
		cmd = exec.CommandContext(ctx, "git", "clone", source.URL, targetDir.String())
	} else {
		cmd = exec.CommandContext(ctx, "git", "-C", targetDir.String(), "pull")
	}

	output, err := cmd.CombinedOutput()
	if err != nil {
		return s.markSyncFailed(ctx, id, fmt.Errorf("git error: %s (err: %w)", string(output), err))
	}

	manifestPath := s.skillPaths.GitRegistryPath(repoName, false)
	if !s.fs.Exists(manifestPath) {
		manifestPath = s.skillPaths.GitRegistryPath(repoName, true)
	}

	if !s.fs.Exists(manifestPath) {
		return s.markSyncFailed(ctx, id, fmt.Errorf("git repo does not contain registry.json or registry.min.json"))
	}

	raw, err := s.fs.ReadFile(manifestPath)
	if err != nil {
		return s.markSyncFailed(ctx, id, fmt.Errorf("read git manifest: %w", err))
	}

	var gitReg skillRegistry
	if err := json.Unmarshal(raw, &gitReg); err != nil {
		return s.markSyncFailed(ctx, id, fmt.Errorf("unmarshal git manifest: %w", err))
	}

	reg, _ := s.loadRegistry()
	if reg.Skills == nil {
		reg.Skills = make(map[string][]registrySkill)
	}

	customNames := make(map[string]bool)
	for _, sk := range reg.Skills["custom"] {
		customNames[strings.ToLower(sk.Name)] = true
	}

	for cat, skills := range gitReg.Skills {
		if cat == "custom" {
			continue
		}
		var mergedSkills []registrySkill
		for _, sk := range skills {
			if customNames[strings.ToLower(sk.Name)] {
				continue
			}

			mappedPath := s.skillPaths.GitSkillPathRelative(repoName, sk.Path)
			schemaMap := map[string]any{
				"source":   "git",
				"repo":     repoName,
				"category": cat,
				"registry": sk.ID,
				"path":     mappedPath,
			}
			schemaRaw, _ := json.Marshal(schemaMap)

			mergedSkills = append(mergedSkills, registrySkill{
				ID:          sk.ID,
				Name:        sk.Name,
				Description: sk.Description,
				Path:        mappedPath,
				Schema:      schemaRaw,
			})
		}
		reg.Skills[cat] = mergedSkills
	}

	if err := s.saveRegistry(reg); err != nil {
		return s.markSyncFailed(ctx, id, fmt.Errorf("save merged registry: %w", err))
	}

	statusSynced := "synced"
	now := time.Now()
	emptyError := ""
	return s.sourceRepo.Update(ctx, id, models.UpdateSkillSourceInput{
		Status:       &statusSynced,
		Error:        &emptyError,
		LastSyncedAt: &now,
	})
}

func (s *SkillService) markSyncFailed(ctx context.Context, id string, err error) (*models.SkillSource, error) {
	statusFailed := "failed"
	errStr := err.Error()
	return s.sourceRepo.Update(ctx, id, models.UpdateSkillSourceInput{
		Status: &statusFailed,
		Error:  &errStr,
	})
}

func validateSkillSourceURL(raw string) (string, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "", ErrValidation("url is required")
	}

	normalized := raw
	if candidate, err := gitops.NormalizeGitURLToHTTPS(raw); err == nil && candidate != "" {
		normalized = candidate
	}

	parsed, err := url.Parse(normalized)
	if err != nil {
		return "", ErrValidation(fmt.Sprintf("invalid repository URL: %v", err))
	}

	switch parsed.Scheme {
	case "http", "https":
		if parsed.Host == "" || strings.TrimSpace(parsed.Path) == "" || parsed.Path == "/" {
			return "", ErrValidation("invalid repository URL: missing repository path")
		}
	case "file":
		if strings.TrimSpace(parsed.Path) == "" {
			return "", ErrValidation("invalid repository URL: missing local path")
		}
	default:
		return "", ErrValidation("invalid repository URL: unsupported scheme")
	}

	return normalized, nil
}
