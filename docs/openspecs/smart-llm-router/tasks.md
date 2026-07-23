# Tasks: Smart LLM Router

> Prerequisite: `llm-prompt-caching` (usage fields đã parse).

- [x] 1.1 Migration `token_usage` + repository (insert, aggregate queries) + tests — pre-existing infra found already implemented (analytics.go repository); this pass only added `cache_read_tokens`/`cache_write_tokens` columns/fields since those were parsed by the LLM response but dropped before persistence.
- [x] 1.2 `config/model_prices.yaml` + loader + cost calc — deviation: implemented as static in-code table (`pkg/llm/pricing.go`: `inputCostPer1K`/`outputCostPer1K`/`MetadataForModel`/`EstimateCost`), pre-existing before this session. Fully functional; not re-implemented as YAML since it already covers the requirement.
- [x] 1.3 Ghi usage async từ llmrunner call-site (best-effort) (REQ-001) — pre-existing (`Gateway.record()` in `pkg/llm/router.go`); this pass extended it to populate `CacheReadTokens`/`CacheWriteTokens`.
- [x] 1.4 `ResolveStepModelLevel` + matrix + complexity/retry rules + unit test matrix (REQ-002/003/004) — new this pass: `internal/orchestrator/llmrunner/step_routing.go` + `step_routing_test.go` (7 tests, all passing).
- [x] 1.5 Project setting `smart_routing` (default true) + off-path test (REQ-M01) — new this pass: `models.Project.SmartRouting`, migration `000015_add_smart_routing`, repository Create/Update wiring, `TestResolveStepModelLevel_SmartRoutingOffIsNoOp`.
- [x] 1.6 Wire resolver vào các step call-sites (grep DefaultModelLevel) — wired in `llmrunner.Runner.routeName` (used by all steps routed through the runner) and directly in `steps/analyze.go`'s `buildAnalyzeRouteOptions`.
- [x] 1.7 `GET /projects/{id}/usage` aggregate + tests (REQ-005) — pre-existing (`handler/analytics.go` + `repository/analytics.go`), predates this session.
- [x] 1.8 UI usage card (pattern stat cards hiện có) — pre-existing (`web/src/app/analytics/*`), predates this session. Deferred: a `smart_routing` on/off *toggle* in the project-settings UI (`project-profile.tsx`) was not added — the codebase has no boolean-checkbox primitive (only `<Select>` dropdowns are used for project settings), and adding one is out of scope for this pass; the field defaults to `true` and can be set today via the update-project API directly.
- [x] 1.9 Update specs.md status

## Docs sync

- [ ] Update corresponding `docs/features/` as specified in feature-docs-sync/design.md
