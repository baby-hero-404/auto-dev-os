package steps

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/auto-code-os/auto-code-os/server/internal/workflow"
	"github.com/auto-code-os/auto-code-os/server/pkg/models"
)

// cliAnalysisCapturePath is the file the CLI agent is instructed to write
// its analysis to. It lives under .autocode/, the CLI engine's ephemeral
// working directory, so it must be captured via CLIStepRunner's
// captureFiles before that directory is cleaned up post-run.
const cliAnalysisCapturePath = ".autocode/analysis.md"

// cliAnalysisPayload is the shape persisted into task.Analysis by
// cli_analyze — distinct from models.TaskAnalysis (the API-native flow's
// richer analysis shape), since a black-box CLI agent's report is
// unstructured markdown that we only parse best-effort.
type cliAnalysisPayload struct {
	RawMarkdown string   `json:"raw_markdown"`
	TechStack   string   `json:"tech_stack,omitempty"`
	Files       []string `json:"files,omitempty"`
	Risks       []string `json:"risks,omitempty"`
}

// CLIAnalyzeStep implements Step for the first stage of the CLI spec-first
// flow: spawn the CLI agent to analyze the repo/task and persist its report.
type CLIAnalyzeStep struct {
	rt      StepRuntime
	tasks   TaskUpdater
	runner  CLIStepRunner
	prompts StepPromptLoader
	log     Logger
}

func NewCLIAnalyzeStep(rt StepRuntime, tasks TaskUpdater, runner CLIStepRunner, prompts StepPromptLoader, log Logger) *CLIAnalyzeStep {
	return &CLIAnalyzeStep{rt: rt, tasks: tasks, runner: runner, prompts: prompts, log: log}
}

func (s *CLIAnalyzeStep) ID() string { return workflow.StepCLIAnalyze }

func (s *CLIAnalyzeStep) StatusOnResume(_ StepResult) string { return models.TaskStatusAnalyzing }

func (s *CLIAnalyzeStep) Execute(ctx context.Context, stepCtx workflow.StepContext) (StepResult, error) {
	base, err := s.prompts.LoadStepPrompt(workflow.StepCLIAnalyze)
	if err != nil {
		return nil, fmt.Errorf("cli_analyze: load prompt: %w", err)
	}
	instruction := fmt.Sprintf("%s\n\n## Task\n\n### %s\n\n%s\n", base, s.rt.Task.Title, s.rt.Task.Description)

	out, err := s.runner.RunCLIStep(ctx, s.rt.Task, s.rt.Agent, s.rt.JobID, s.ID(), instruction, []string{cliAnalysisCapturePath})
	if err != nil {
		return nil, fmt.Errorf("cli_analyze: %w", err)
	}

	raw, ok := out.Files[cliAnalysisCapturePath]
	if !ok || strings.TrimSpace(raw) == "" {
		return nil, fmt.Errorf("cli_analyze: expected %s to be written by the CLI agent, but it was missing or empty", cliAnalysisCapturePath)
	}

	payload := cliAnalysisPayload{RawMarkdown: raw}
	payload.TechStack = extractMDSection(raw, "Tech Stack")
	payload.Files = extractMDListItems(raw, "Affected Files")
	payload.Risks = extractMDListItems(raw, "Risks")

	analysisJSON, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("cli_analyze: marshal analysis: %w", err)
	}

	if s.tasks != nil {
		if _, err := s.tasks.Update(ctx, s.rt.Task.ID, models.UpdateTaskInput{Analysis: analysisJSON}); err != nil {
			return nil, fmt.Errorf("cli_analyze: save analysis: %w", err)
		}
	}
	s.rt.Task.Analysis = analysisJSON

	return StepResult{"status": "success"}, nil
}

// extractMDSection returns the trimmed body text of a "## <heading>" section
// (everything up to the next "## " heading or end of document). Best-effort:
// returns "" when the heading isn't found.
func extractMDSection(md, heading string) string {
	marker := "## " + heading
	idx := strings.Index(md, marker)
	if idx < 0 {
		return ""
	}
	rest := md[idx+len(marker):]
	if next := strings.Index(rest, "\n## "); next >= 0 {
		rest = rest[:next]
	}
	return strings.TrimSpace(rest)
}

// extractMDListItems returns the "- " bullet items under a "## <heading>"
// section, stripped of a leading "path: description" separator if present.
func extractMDListItems(md, heading string) []string {
	section := extractMDSection(md, heading)
	if section == "" {
		return nil
	}
	var items []string
	for _, line := range strings.Split(section, "\n") {
		trimmed := strings.TrimSpace(line)
		trimmed = strings.TrimPrefix(trimmed, "- ")
		trimmed = strings.TrimPrefix(trimmed, "* ")
		if trimmed == "" {
			continue
		}
		items = append(items, trimmed)
	}
	return items
}
