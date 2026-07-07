# Phase 17: Context Pruning & Harness Independence — Implementation Plan

> **For agentic workers:** Use `subagent-driven-development` or executing-plans to implement this plan task-by-task. Steps use checkbox syntax for tracking.

**Goal:** Implement intelligent Token Pruning using PageRank and Binary Search within the Context Engine, and enforce Harness Independence (Model Isolation) during the Review step with graceful fallbacks.

**Tech Stack:** Go 1.23.

---

## 1. Context Engine: PageRank & Task Dependency Buffing
- [x] **1.1** In `server/internal/context/provider/`, introduce a graph analysis utility (or leverage existing SQLite AST queries) to calculate basic PageRank-like importance scores for all AST nodes based on reference frequency.
- [x] **1.2** Add a method `SetTaskDependencies(files []string)` (or pass it as an argument to `GetRepoMap`) on the `ContextEngine` interface.
- [x] **1.3** Implement the buffing logic: Any file path included in the task dependencies must receive a massive multiplier (e.g., 50x) to its base importance score. This guarantees the orchestrator will prioritize these files when rendering the Repo Map.

## 2. Context Engine: Binary Search Token Pruning
- [x] **2.1** In the method that generates the final Repo Map string (e.g., `GetRepoMap`), introduce a `tokenBudget` parameter.
- [x] **2.2** Implement a lightweight Token Estimator (a simple heuristic like `len(str) / 4` is acceptable if a full tokenizer is too heavy, or integrate `tiktoken-go` if precise counting is required).
- [x] **2.3** Implement the **Binary Search Pruning Loop**:
  - Initialize `lower_bound = 0`, `upper_bound = total_nodes`.
  - Loop: Render the top `N` nodes (based on the buffed PageRank scores).
  - If `tokens > tokenBudget`, decrease `upper_bound`.
  - If `tokens < tokenBudget` (but not within a 10-15% acceptable margin), increase `lower_bound`.
  - Break the loop when the token count is within the acceptable margin or `lower_bound >= upper_bound`.
- [x] **2.4** Write unit tests in `provider_test.go` to verify that `GetRepoMap` strictly adheres to the `tokenBudget` and successfully includes the buffed task dependencies.

## 3. Orchestrator & Gateway: Harness Independence
- [x] **3.1** In `server/internal/orchestrator/steps/` (specifically the Human Review or AI Review step), retrieve the `ModelID` that was utilized during the preceding `Coding` step.
- [x] **3.2** Update the AI Gateway (`server/pkg/llm/gateway.go`) to accept an optional `ExcludeModelID` parameter when requesting an AI provider.
- [x] **3.3** Implement the **Graceful Fallback**: 
  - The Gateway attempts to find an active model where `model.ID != ExcludeModelID`.
  - If no other models are configured or available, the Gateway must catch the exclusion, log a warning (e.g., `"Harness Independence fallback: forcing review using the original coder model"`), and return the `ExcludeModelID` model to prevent the pipeline from halting.
- [x] **3.4** Update the relevant unit tests for the Review step and Gateway to assert that model isolation works when multiple models exist, and fallback succeeds when only one model exists.
