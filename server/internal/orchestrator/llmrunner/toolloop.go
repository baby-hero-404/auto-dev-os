package llmrunner

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/auto-code-os/auto-code-os/server/pkg/llm"
)

// ToolExecutor executes a single tool call by name with JSON-encoded arguments and returns a
// string result (or an "Error: ..." string) to feed back to the LLM as a tool message. A
// non-nil error aborts the whole loop immediately instead of being fed back to the LLM — used
// for hard-stop conditions like a critical execution-boundary violation that must pause the
// task for human review rather than let the LLM retry.
type ToolExecutor func(ctx context.Context, name string, argumentsJSON string) (string, error)

// ToolLoopValidator inspects a parsed non-tool-call JSON response. Returning nil accepts the
// response and ends the loop. Returning an error appends its message as corrective feedback
// and continues the loop (until the iteration budget is exhausted).
type ToolLoopValidator func(parsed map[string]any) error

// ToolLoopHook is invoked after every LLM call in the loop (tool-call, parse-failure, or
// validated), so callers can trace iterations the same way llm_trace.WriteLLMCallTrace does.
type ToolLoopHook func(iteration int, messages []llm.Message, resp *llm.Response, parsed map[string]any, latency time.Duration)

// ToolLoopConfig parameterizes RunToolLoop.
type ToolLoopConfig struct {
	Messages      []llm.Message
	Tools         []llm.ToolDefinition
	MaxIterations int
	Chat          func(ctx context.Context, messages []llm.Message, opts llm.ChatOptions) (*llm.Response, error)
	ExecuteTool   ToolExecutor
	Validate      ToolLoopValidator
	OnCall        ToolLoopHook
}

// editToolNames are the tools whose successful calls represent a real workspace mutation —
// used to decide whether an exhausted loop has anything worth salvaging as a partial result.
var editToolNames = map[string]bool{"search_replace": true, "create_file": true}

// maxToolResultChars bounds how much of a single tool result gets appended to the loop's
// message history (~2000 tokens). Without this, a large run_tests/run_build/run_lint
// stdout+stderr blob (unbounded at the tool layer, e.g. tools/run_tests.go) gets re-sent to the
// LLM on every subsequent turn of the same loop, growing cost roughly quadratically with
// iteration count (Issue 7).
const maxToolResultChars = 8000

// truncateToolResult caps s at maxToolResultChars, appending a visible marker so the LLM (and
// anyone reading a trace) knows content was cut rather than mistaking it for the full output.
func truncateToolResult(s string) string {
	if len(s) <= maxToolResultChars {
		return s
	}
	return s[:maxToolResultChars] + fmt.Sprintf("\n... [truncated %d chars]", len(s)-maxToolResultChars)
}

// readFileMemoArgs mirrors the subset of tools.ReadFileArgs that determines what content a
// read_file call returns, used to key read-memoization within a single RunToolLoop run.
type readFileMemoArgs struct {
	Path       string `json:"path"`
	StartLine  int    `json:"start_line"`
	EndLine    int    `json:"end_line"`
	AroundLine int    `json:"around_line"`
	Radius     int    `json:"radius"`
	MaxLines   int    `json:"max_lines"`
}

// readFileMemoKey returns a discriminator for a read_file call's (path, line-range), or "" if
// the call has no path (in which case memoization is skipped rather than mis-keyed).
func readFileMemoKey(argumentsJSON string) string {
	var a readFileMemoArgs
	if err := json.Unmarshal([]byte(argumentsJSON), &a); err != nil || a.Path == "" {
		return ""
	}
	return fmt.Sprintf("%s|%d|%d|%d|%d|%d", a.Path, a.StartLine, a.EndLine, a.AroundLine, a.Radius, a.MaxLines)
}

// ToolLoopResult accompanies every RunToolLoop return (success, partial, or hard failure).
// Partial is true only on iteration-budget exhaustion where at least one edit tool call
// already succeeded, letting the caller salvage that work (e.g. checkpoint + targeted-test
// it) instead of discarding it (Issue 6). FilesRead is populated on every path so a caller
// that retries after this run can carry forward "already read" context instead of making the
// model re-discover file contents from scratch (Issue 6 retry carry-forward).
type ToolLoopResult struct {
	Partial      bool
	EditsApplied []string // discriminators (paths) touched by successful edit tool calls
	FilesRead    []string // discriminators (paths) touched by successful read_file calls
}

// RunToolLoop drives a native tool-calling agentic loop: call the LLM with tools, execute any
// requested tool calls and feed the results back, and repeat until the LLM returns a JSON
// response that passes Validate or the iteration budget is exhausted.
//
// This generalizes the pattern originally pioneered by AnalyzeStep's own hand-rolled loop
// (native tool_calls branch -> execute -> append tool result -> continue; JSON branch -> parse
// -> validate -> feedback -> continue) so every step reuses one implementation instead of a
// single-shot Chat() call with no tool support. AnalyzeStep itself now drives its
// spec-generation loop through this function (Task 4.2 / REQ-M08), with its analyze-specific
// checks (legacy "tool_use" convention, contract completeness, DAG/cost validation) wired in via
// Validate.
func RunToolLoop(ctx context.Context, cfg ToolLoopConfig) (map[string]any, []llm.Message, *ToolLoopResult, error) {
	maxIterations := cfg.MaxIterations
	if maxIterations <= 0 {
		maxIterations = 6
	}
	messages := cfg.Messages

	failureCounts := make(map[string]int)
	readMemo := make(map[string]int) // (path, line-range) discriminator -> turn first read at
	var editsApplied []string
	var filesRead []string

	for i := 0; i < maxIterations; i++ {
		start := time.Now()
		resp, err := cfg.Chat(ctx, messages, llm.ChatOptions{Tools: cfg.Tools, ToolChoice: "auto"})
		latency := time.Since(start)
		if err != nil {
			return nil, messages, &ToolLoopResult{FilesRead: filesRead}, fmt.Errorf("llm tool loop call failed: %w", err)
		}

		if len(resp.ToolCalls) > 0 {
			if cfg.OnCall != nil {
				cfg.OnCall(i+1, messages, resp, map[string]any{"tool_calls": resp.ToolCalls}, latency)
			}
			messages = append(messages, llm.Message{Role: "assistant", Content: resp.Content, ToolCalls: resp.ToolCalls})
			for _, call := range resp.ToolCalls {
				discriminator := extractCallKey(call.Arguments)
				key := call.Name + ":" + discriminator

				var readMemoKey string
				if call.Name == "read_file" {
					readMemoKey = readFileMemoKey(call.Arguments)
					if readMemoKey != "" {
						if turn, seen := readMemo[readMemoKey]; seen {
							result := fmt.Sprintf("Already read at turn %d for this path/range — reusing that content instead of re-sending it. Refer back to your earlier read_file result for this path/range.", turn)
							messages = append(messages, llm.Message{Role: "tool", ToolCallID: call.ID, ToolName: call.Name, Content: result})
							continue
						}
					}
				}

				if failureCounts[key] >= 2 {
					var result string
					if discriminator != "" {
						result = fmt.Sprintf("Error: You have called %s on %q multiple times without success. The file likely does not exist. Use create_file to create it first, then use search_replace to modify it.", call.Name, discriminator)
					} else {
						result = fmt.Sprintf("Error: You have called %s multiple times without success. Stop repeating this exact call — try a different approach (e.g. a narrower test target, or inspect the error output more carefully before retrying).", call.Name)
					}
					messages = append(messages, llm.Message{Role: "tool", ToolCallID: call.ID, ToolName: call.Name, Content: result})
					continue
				}

				result, toolErr := cfg.ExecuteTool(ctx, call.Name, call.Arguments)
				if toolErr != nil {
					return nil, messages, &ToolLoopResult{FilesRead: filesRead}, toolErr
				}

				if strings.HasPrefix(result, "Error:") {
					failureCounts[key]++
				} else {
					failureCounts[key] = 0
					if editToolNames[call.Name] && discriminator != "" {
						editsApplied = append(editsApplied, discriminator)
					}
					if call.Name == "read_file" && discriminator != "" {
						filesRead = append(filesRead, discriminator)
					}
					if readMemoKey != "" {
						readMemo[readMemoKey] = i + 1
					}
				}
				result = truncateToolResult(result)
				messages = append(messages, llm.Message{Role: "tool", ToolCallID: call.ID, ToolName: call.Name, Content: result})
			}
			// NOTE: a round where every call was blocked by the circuit breaker above
			// still counts toward maxIterations (no i--) — otherwise a model that keeps
			// repeating an already-blocked call never hits the iteration cap at all.
			continue
		}

		parsedJSON, parseErr := ParseJSONMarkdown(resp.Content)
		if parseErr != nil {
			if cfg.OnCall != nil {
				cfg.OnCall(i+1, messages, resp, map[string]any{"raw_content": resp.Content}, latency)
			}
			content := resp.Content
			if content == "" {
				content = "(empty response)"
			}
			messages = append(messages, llm.Message{Role: "assistant", Content: content})
			messages = append(messages, llm.Message{
				Role:    "user",
				Content: fmt.Sprintf("Your output was not valid JSON. Error: %v. Please correct the formatting/syntax and output strictly valid JSON matching the schema.", parseErr),
			})
			continue
		}

		if cfg.OnCall != nil {
			cfg.OnCall(i+1, messages, resp, parsedJSON, latency)
		}

		if cfg.Validate != nil {
			if validationErr := cfg.Validate(parsedJSON); validationErr != nil {
				content := resp.Content
				if content == "" {
					content = "(empty response)"
				}
				messages = append(messages, llm.Message{Role: "assistant", Content: content})
				messages = append(messages, llm.Message{Role: "user", Content: validationErr.Error()})
				continue
			}
		}

		return parsedJSON, messages, &ToolLoopResult{FilesRead: filesRead}, nil
	}

	if len(editsApplied) > 0 {
		// Exhausted without a valid final answer, but real edits were already applied to the
		// workspace — surface a partial result instead of discarding that work outright (Issue 6).
		return nil, messages, &ToolLoopResult{Partial: true, EditsApplied: editsApplied, FilesRead: filesRead}, nil
	}

	return nil, messages, &ToolLoopResult{FilesRead: filesRead}, fmt.Errorf("exceeded max iterations (%d)", maxIterations)
}

// extractCallKey returns a discriminator for the circuit breaker's per-call key.
// Prefers "path" (read_file, search_replace, create_file, run_lint); falls back to
// "command" for tools with no path concept (run_tests, run_build); returns "" if
// neither is present, which still throttles repeat no-argument calls to the same tool.
func extractCallKey(argumentsJSON string) string {
	var args struct {
		Path    string `json:"path"`
		Command string `json:"command"`
	}
	if err := json.Unmarshal([]byte(argumentsJSON), &args); err == nil {
		if args.Path != "" {
			return args.Path
		}
		if args.Command != "" {
			return args.Command
		}
	}
	return ""
}
