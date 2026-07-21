# Tasks: Smart LLM Router

> Prerequisite: `llm-prompt-caching` (usage fields đã parse).

- [ ] 1.1 Migration `token_usage` + repository (insert, aggregate queries) + tests
- [ ] 1.2 `config/model_prices.yaml` + loader + cost calc
- [ ] 1.3 Ghi usage async từ llmrunner call-site (best-effort) (REQ-001)
- [ ] 1.4 `ResolveStepModelLevel` + matrix + complexity/retry rules + unit test matrix (REQ-002/003/004)
- [ ] 1.5 Project setting `smart_routing` (default true) + off-path test (REQ-M01)
- [ ] 1.6 Wire resolver vào các step call-sites (grep DefaultModelLevel)
- [ ] 1.7 `GET /projects/{id}/usage` aggregate + tests (REQ-005)
- [ ] 1.8 UI usage card (pattern stat cards hiện có)
- [ ] 1.9 Update specs.md status
