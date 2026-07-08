package patch

import (
	"path/filepath"
	"regexp"
	"strings"

	"github.com/auto-code-os/auto-code-os/server/pkg/models"
)

func DeriveChangeName(task *models.Task) string {
	slug := strings.ToLower(task.Title)
	reg := regexp.MustCompile(`[^a-z0-9]+`)
	slug = reg.ReplaceAllString(slug, "-")
	slug = strings.Trim(slug, "-")
	if len(slug) > 30 {
		slug = slug[:30]
	}
	slug = strings.Trim(slug, "-")
	if slug == "" {
		slug = "task-" + task.ID
		if len(slug) > 13 {
			slug = slug[:13]
		}
	}
	return slug
}

func repoNameFromURL(repoURL string) string {
	parts := strings.Split(repoURL, "/")
	repoName := parts[len(parts)-1]
	return strings.TrimSuffix(repoName, ".git")
}

func TaskReadyForExecution(task *models.Task) bool {
	switch task.SpecStatus {
	case models.TaskSpecStatusApproved, models.TaskSpecStatusAutoApproved:
		return true
	default:
		return false
	}
}

func MatchAffectedFile(pattern, file string) bool {
	pattern = strings.TrimSpace(pattern)
	file = strings.TrimSpace(file)
	if pattern == "" || file == "" {
		return false
	}

	cleanPattern := filepath.ToSlash(filepath.Clean(pattern))
	cleanFile := filepath.ToSlash(filepath.Clean(file))

	if cleanPattern == cleanFile {
		return true
	}

	// Implicitly allow test files if the base file is affected
	ext := filepath.Ext(cleanPattern)
	baseNoExt := strings.TrimSuffix(cleanPattern, ext)
	if ext == ".go" && cleanFile == baseNoExt+"_test.go" {
		return true
	}
	if ext == ".js" || ext == ".ts" || ext == ".jsx" || ext == ".tsx" {
		if cleanFile == baseNoExt+".test"+ext || cleanFile == baseNoExt+".spec"+ext {
			return true
		}
	}

	if strings.HasPrefix(cleanFile, cleanPattern+"/") {
		return true
	}

	if strings.HasSuffix(cleanFile, "/"+cleanPattern) {
		return true
	}

	if !strings.Contains(cleanPattern, "/") && filepath.Base(cleanFile) == cleanPattern {
		return true
	}

	if strings.ContainsAny(pattern, "*?[]") {
		if matched, err := filepath.Match(cleanPattern, cleanFile); err == nil && matched {
			return true
		}
	}

	return false
}

func IsUnderAffectedDir(file string, affectedFiles []models.AffectedFile) bool {
	file = strings.TrimSpace(file)
	if file == "" {
		return false
	}

	fileDir := filepath.ToSlash(filepath.Clean(filepath.Dir(file)))
	for _, pattern := range affectedFiles {
		patternStr := strings.TrimSpace(pattern.File)
		if patternStr == "" {
			continue
		}

		candidateDir := affectedPatternDir(patternStr)
		if candidateDir == "" {
			continue
		}
		if fileDir == candidateDir || strings.HasPrefix(fileDir, candidateDir+"/") {
			return true
		}
	}

	return false
}

func affectedPatternDir(pattern string) string {
	pattern = strings.TrimSpace(pattern)
	if pattern == "" {
		return ""
	}

	normalized := filepath.ToSlash(pattern)
	if strings.HasSuffix(normalized, "/") {
		dir := strings.TrimSuffix(filepath.ToSlash(filepath.Clean(normalized)), "/")
		if dir == "" {
			return "."
		}
		return dir
	}

	if strings.ContainsAny(normalized, "*?[]") {
		idx := strings.IndexAny(normalized, "*?[")
		if idx == -1 {
			idx = len(normalized)
		}
		base := strings.TrimSuffix(normalized[:idx], "/")
		if base == "" {
			return "."
		}
		return filepath.ToSlash(filepath.Clean(base))
	}

	dir := filepath.ToSlash(filepath.Clean(filepath.Dir(normalized)))
	if dir == "" {
		return "."
	}
	return dir
}
