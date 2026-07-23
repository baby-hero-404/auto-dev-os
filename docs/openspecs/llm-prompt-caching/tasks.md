# Tasks: Anthropic Prompt Caching

- [x] 1.1 Audit cache partition — verified: `internal/prompts/builder.go` only puts stable sections (Base/Role/Step Prompt, Global/Project Rules, Available Tools) at `Destination: "system"`; RepoMap, diffs, memories, task requirement are all `"user"`. System also gets per-job-stable metadata only (`project_id`, `task_id`, `assigned_role`, `task_rules`) via `appendSystemPrompt` in `assembler.go:194-211`. No per-turn-dynamic content (timestamps/nonces/repomap) leaks into the cached prefix.
- [x] 1.2 `server/pkg/llm/anthropic.go:87-104`: system → array-of-blocks with `cache_control` on last system block + last tool
- [x] 1.3 Parse `cache_creation_input_tokens`/`cache_read_input_tokens` into Usage struct + usage log — `anthropic.go:165-179,241-242`
- [x] 1.4 Guard: `cache_control` only in `anthropic.go`, structurally never rendered for other providers (REQ-004)
- [ ] 1.5 Unit tests: request body snapshot (breakpoints ≤4), non-Anthropic without field — NOT done, no `*_test.go` exists for `pkg/llm/anthropic.go`
- [ ] 1.6 Integration smoke (cache_read > 0 on 2nd tool-loop turn) — NOT done, no test/log evidence found; specs.md still shows "Not Started" for REQ-002/003

## Docs sync

- [x] Update corresponding `docs/features/` as specified in feature-docs-sync/design.md — done 2026-07-23: product/01-unified-ai-gateway.md
