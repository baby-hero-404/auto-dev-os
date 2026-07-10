package steps

import (
	"context"
	"regexp"

	"github.com/auto-code-os/auto-code-os/server/pkg/models"
)

var compilerErrorRegex = regexp.MustCompile(`(?m)^([a-zA-Z0-9_\-\.\/]+\.(?:go|ts|tsx|js|jsx|py|cpp|h|c|java|rb|php|css|json|sql|sh|yaml|yml|toml)):(\d+)(?::(\d+))?:`)

func parseCompilerErrorFiles(output string) []string {
	matches := compilerErrorRegex.FindAllStringSubmatch(output, -1)
	if len(matches) == 0 {
		return nil
	}

	seen := make(map[string]bool)
	var files []string
	for _, m := range matches {
		if len(m) > 1 {
			filePath := m[1]
			if !seen[filePath] {
				seen[filePath] = true
				files = append(files, filePath)
			}
		}
	}
	return files
}

func updateAffectedFilesWithErrors(ctx context.Context, taskID string, tasks TaskRepository, rtTask *models.Task, err error) {
	if err == nil {
		return
	}
	parsedFiles := parseCompilerErrorFiles(err.Error())
	if len(parsedFiles) == 0 {
		return
	}
	_ = updateTaskAnalysis(ctx, taskID, tasks, rtTask, func(analysis *models.TaskAnalysis) bool {
		changed := false
		for _, pf := range parsedFiles {
			found := false
			for _, af := range analysis.AffectedFiles {
				if af.File == pf {
					found = true
					break
				}
			}
			if !found {
				analysis.AffectedFiles = append(analysis.AffectedFiles, models.AffectedFile{
					File:       pf,
					Confidence: 1.0,
					Reason:     "Extracted from compilation/test errors",
				})
				changed = true
			}
		}
		return changed
	})
}
