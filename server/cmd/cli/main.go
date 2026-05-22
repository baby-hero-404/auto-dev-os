package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/auto-code-os/auto-code-os/server/pkg/config"
	"github.com/auto-code-os/auto-code-os/server/pkg/llm"
)

const version = "0.1.0-poc"

// Global rules injected into the System Prompt (immutable, cannot be overridden).
const systemPrompt = `You are an expert software engineer working as part of the Auto Code OS platform.
You follow these IMMUTABLE rules at all times:

1. All agents share the same memory server.
2. The system architecture should follow a modular and plugin-oriented design.
3. All code execution must occur within isolated, sandboxed environments.
4. Employ Progressive Disclosure — only load context relevant to the immediate sub-task.
5. Author comprehensive automated tests for every feature or bug fix.
6. For Medium/Hard tasks, produce an implementation plan before writing code.

When given a task, respond with clean, production-ready code.
Include file paths as comments at the top of each code block.
If the task is complex, break it into steps and explain your approach first.`

func main() {
	// CLI flags
	taskFile := flag.String("file", "", "Path to a task description file")
	taskInline := flag.String("task", "", "Inline task description")
	output := flag.String("output", "", "Path to save the generated output (default: stdout)")
	showVersion := flag.Bool("version", false, "Show version")
	flag.Parse()

	if *showVersion {
		fmt.Printf("auto-code-os CLI v%s\n", version)
		os.Exit(0)
	}

	// Determine the task description.
	task, err := resolveTask(*taskFile, *taskInline)
	if err != nil {
		fmt.Fprintf(os.Stderr, "❌ Error: %v\n", err)
		fmt.Fprintf(os.Stderr, "\nUsage:\n")
		fmt.Fprintf(os.Stderr, "  auto-code-os --task \"Create a Go HTTP handler for user registration\"\n")
		fmt.Fprintf(os.Stderr, "  auto-code-os --file task.md\n")
		fmt.Fprintf(os.Stderr, "  auto-code-os --file task.md --output result.md\n")
		os.Exit(1)
	}

	// Load configuration from environment.
	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintf(os.Stderr, "❌ Config error: %v\n", err)
		fmt.Fprintf(os.Stderr, "\nSet environment variables:\n")
		fmt.Fprintf(os.Stderr, "  export LLM_PROVIDER=openai        # openai | anthropic | gemini\n")
		fmt.Fprintf(os.Stderr, "  export OPENAI_API_KEY=sk-xxx       # your API key\n")
		os.Exit(1)
	}

	// Create the LLM provider.
	provider, err := llm.NewProvider(cfg)
	if err != nil {
		fmt.Fprintf(os.Stderr, "❌ Provider error: %v\n", err)
		os.Exit(1)
	}

	fmt.Fprintf(os.Stderr, "🚀 Auto Code OS CLI v%s\n", version)
	fmt.Fprintf(os.Stderr, "📡 Provider: %s (%s)\n", provider.Name(), cfg.LLMModel)
	fmt.Fprintf(os.Stderr, "📝 Task: %s\n", truncate(task, 80))
	fmt.Fprintf(os.Stderr, "⏳ Generating...\n\n")

	// Build the message chain: System (global rules) + User (task context).
	messages := []llm.Message{
		{Role: "system", Content: systemPrompt},
		{Role: "user", Content: task},
	}

	// Call the LLM.
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	start := time.Now()
	resp, err := provider.Chat(ctx, messages)
	elapsed := time.Since(start)

	if err != nil {
		fmt.Fprintf(os.Stderr, "❌ LLM error: %v\n", err)
		os.Exit(1)
	}

	// Output the result.
	if *output != "" {
		if err := os.WriteFile(*output, []byte(resp.Content), 0644); err != nil {
			fmt.Fprintf(os.Stderr, "❌ Failed to write output: %v\n", err)
			os.Exit(1)
		}
		fmt.Fprintf(os.Stderr, "✅ Output saved to: %s\n", *output)
	} else {
		fmt.Println(resp.Content)
	}

	// Print stats.
	fmt.Fprintf(os.Stderr, "\n──────────────────────────────────\n")
	fmt.Fprintf(os.Stderr, "📊 Model:         %s\n", resp.Model)
	fmt.Fprintf(os.Stderr, "📊 Input tokens:   %d\n", resp.PromptTokens)
	fmt.Fprintf(os.Stderr, "📊 Output tokens:  %d\n", resp.OutputTokens)
	fmt.Fprintf(os.Stderr, "📊 Duration:       %s\n", elapsed.Round(time.Millisecond))
	fmt.Fprintf(os.Stderr, "──────────────────────────────────\n")
}

// resolveTask determines the task from either a file or inline argument.
func resolveTask(filePath, inline string) (string, error) {
	if filePath != "" {
		data, err := os.ReadFile(filePath)
		if err != nil {
			return "", fmt.Errorf("read task file %q: %w", filePath, err)
		}
		return strings.TrimSpace(string(data)), nil
	}

	if inline != "" {
		return inline, nil
	}

	return "", fmt.Errorf("no task provided. Use --task or --file")
}

// truncate shortens a string for display purposes.
func truncate(s string, maxLen int) string {
	s = strings.ReplaceAll(s, "\n", " ")
	if len(s) > maxLen {
		return s[:maxLen] + "..."
	}
	return s
}
