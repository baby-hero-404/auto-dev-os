# Implementation Notes: smart-llm-router

## Pre-existing infrastructure discovered

Before writing any code, an Explore pass over usage-logging/routing code found that a large
fraction of the spec was already implemented under an "analytics" naming rather than the
spec's literal "token_usage router" framing:

- `models.TokenUsage` + `repository/analytics.go` — full persistence and aggregate-query
  layer (REQ-001's storage half, REQ-005's query half).
- `handler/analytics.go` + `web/src/app/analytics/*` — the `/projects/{id}/usage`-style API
  and its frontend stat-card page (REQ-005).
- `pkg/llm/pricing.go` — static in-code model pricing table (`inputCostPer1K`/
  `outputCostPer1K`/`MetadataForModel`/`EstimateCost`), substituting for design.md's proposed
  `config/model_prices.yaml` file. Kept as-is: it already provides full cost estimation and
  changing the storage format would be pure churn.

This let the actual new work scope down to: step-aware routing (REQ-002/003/004), the
`smart_routing` toggle (REQ-M01), and completing cache-token persistence (a gap in REQ-001 —
the `Response` struct already carried `CacheReadTokens`/`CacheWriteTokens` but they were
dropped before reaching the `token_usage` table).

## Step routing matrix (REQ-002/003/004)

`internal/orchestrator/llmrunner/step_routing.go`'s `ResolveStepModelLevel` composes three
rules in order:

1. A `stepBaseLevel` matrix gives cheap steps (`context_load`, `analyze`, `cli_analyze` →
   fast; `plan`, `review`, `cross_review`, `cli_spec` → balanced) a lower level than the
   project default, clamped so it never *exceeds* the project's chosen level. Steps outside
   the matrix (`code_*`, `fix`, `cli_implement`, ...) pass through the project level
   unchanged — those are the steps that actually write code.
2. `Complexity == "easy"` downgrades the resolved level by one more tier.
3. A retry escape hatch restores the pre-downgrade level, so a cheap model doesn't loop
   forever on a failure it can't fix.

REQ-004's "retry attempt ≥ 2" was mapped onto the existing `prompts.IsRetry(ctx)` boolean
(set by `patch_retry_loop.go` at `attempt >= 2`) rather than introducing a new numeric
retry-counter context key — that flag already means exactly the right thing.

Placed in `internal/orchestrator/llmrunner` (not `pkg/llm`) to avoid a `pkg` → `internal`
import-direction inversion, since the step-ID constants live in `internal/workflow`.

## `smart_routing` project toggle (REQ-M01)

`models.Project.SmartRouting bool` (gorm `default:true`, `not null`) gates all of the above:
`smart_routing=false` makes every step behave exactly as it did before this feature (pure
pass-through of `DefaultModelLevel`). Wired through `CreateProjectInput`/`UpdateProjectInput`
and the repository Create/Update paths, mirroring the existing `ReviewHarnessPolicy` pattern.

**Deferred**: a `smart_routing` on/off UI toggle in `project-profile.tsx` was not added. The
project-settings UI only has a `<Select>` dropdown primitive (no boolean checkbox component
exists in `web/src/components/ui`), and building one purely for this single field is out of
scope for this pass. The field defaults to `true` and is settable today via the
update-project API. Matches the precedent set by `cross-harness-review`'s UI deferral.

## Cache-token persistence completion

`pkg/llm/anthropic.go`'s `Response` already carried `CacheReadTokens`/`CacheWriteTokens`, but
`Gateway.record()` in `pkg/llm/router.go` wasn't forwarding them into `UsageRecord`, so they
never reached `token_usage`. Added the fields to `UsageRecord`, `models.TokenUsage`, and the
`RecordLLMUsage` insert, plus migration `000015_add_smart_routing` (which also adds
`projects.smart_routing`).

## Testing

`internal/orchestrator/llmrunner/step_routing_test.go` — 7 cases covering matrix resolution,
project-level clamping, code-step pass-through, easy-complexity downgrade, retry restore,
`smart_routing=false` no-op, and empty-project-level no-op.

`internal/repository/project_test.go`'s existing `TestProjectRepo_CreatePersistsMaxReviewFixCycles`
needed its expected INSERT SQL/args updated for the new `smart_routing` column (GORM
substitutes the Go zero-value default-tag literal `true` into the INSERT for a bool field
with `default:true`, the same mechanism seen with `review_harness_policy` in a prior pass).

All packages pass `go build ./...` and `go test ./...`.
