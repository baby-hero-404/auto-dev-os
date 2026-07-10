package llmrunner

import (
	"context"
	"fmt"
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

// RunToolLoop drives a native tool-calling agentic loop: call the LLM with tools, execute any
// requested tool calls and feed the results back, and repeat until the LLM returns a JSON
// response that passes Validate or the iteration budget is exhausted.
//
// This generalizes the pattern pioneered by AnalyzeStep.runAnalyzeLLMLoop (native tool_calls
// branch -> execute -> append tool result -> continue; JSON branch -> parse -> validate ->
// feedback -> continue) so review/coding steps can reuse it instead of only ever making a
// single-shot Chat() call with no tool support (Issue 1+2).
func RunToolLoop(ctx context.Context, cfg ToolLoopConfig) (map[string]any, []llm.Message, error) {
	maxIterations := cfg.MaxIterations
	if maxIterations <= 0 {
		maxIterations = 6
	}
	messages := cfg.Messages

	for i := 0; i < maxIterations; i++ {
		start := time.Now()
		resp, err := cfg.Chat(ctx, messages, llm.ChatOptions{Tools: cfg.Tools, ToolChoice: "auto"})
		latency := time.Since(start)
		if err != nil {
			return nil, messages, fmt.Errorf("llm tool loop call failed: %w", err)
		}

		if len(resp.ToolCalls) > 0 {
			if cfg.OnCall != nil {
				cfg.OnCall(i+1, messages, resp, map[string]any{"tool_calls": resp.ToolCalls}, latency)
			}
			messages = append(messages, llm.Message{Role: "assistant", Content: resp.Content, ToolCalls: resp.ToolCalls})
			for _, call := range resp.ToolCalls {
				result, toolErr := cfg.ExecuteTool(ctx, call.Name, call.Arguments)
				if toolErr != nil {
					return nil, messages, toolErr
				}
				messages = append(messages, llm.Message{Role: "tool", ToolCallID: call.ID, ToolName: call.Name, Content: result})
			}
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

		return parsedJSON, messages, nil
	}

	return nil, messages, fmt.Errorf("exceeded max iterations (%d)", maxIterations)
}
