package llm

import "fmt"

// CheckBudget and CheckActualUsage are the shared budget circuit breaker
// logic — used by Gateway.checkBudget/checkActualUsage (below) and by
// internal/gateway.AIGateway, the primary org-scoped call path, which
// previously had no budget enforcement at all despite LLM_CIRCUIT_MAX_TOKENS/
// LLM_CIRCUIT_MAX_COST_USD being documented as an active safeguard.
// Extracted to package-level functions (taking explicit limits instead of
// reading Gateway's own fields) so both callers share one implementation.

// CheckBudget rejects a call before it's made if the estimated input tokens
// or estimated cost (input + a caller-supplied output ceiling) exceed the
// given limits. A zero limit means "no limit" for that dimension.
func CheckBudget(inputTokens, outputLimit int, maxTokens int, maxCostUSD float64, meta ProviderMetadata) error {
	if maxTokens > 0 && inputTokens > maxTokens {
		return fmt.Errorf("%w: estimated input tokens %d exceed limit %d", ErrCircuitOpen, inputTokens, maxTokens)
	}
	if maxCostUSD > 0 {
		cost := estimateCost(inputTokens, outputLimit, meta)
		if cost > maxCostUSD {
			return fmt.Errorf("%w: estimated cost %.6f exceeds limit %.6f", ErrCircuitOpen, cost, maxCostUSD)
		}
	}
	return nil
}

// CheckActualUsage re-checks the same limits against a response's real
// token counts/cost, since actual usage can exceed the pre-call estimate.
func CheckActualUsage(resp *Response, cost float64, maxTokens int, maxOutputTokens int, maxCostUSD float64) error {
	if maxTokens > 0 && resp.PromptTokens > maxTokens {
		return fmt.Errorf("%w: prompt tokens %d exceed limit %d", ErrCircuitOpen, resp.PromptTokens, maxTokens)
	}
	if maxOutputTokens > 0 && resp.OutputTokens > maxOutputTokens {
		return fmt.Errorf("%w: output tokens %d exceed limit %d", ErrCircuitOpen, resp.OutputTokens, maxOutputTokens)
	}
	if maxCostUSD > 0 && cost > maxCostUSD {
		return fmt.Errorf("%w: actual cost %.6f exceeds limit %.6f", ErrCircuitOpen, cost, maxCostUSD)
	}
	return nil
}
