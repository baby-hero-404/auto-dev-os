# Tasks: Cross-Harness Review

> Prerequisites: `pluggable-execution-engine`, `review-verdict-split` hoàn thành.

- [ ] 1.1 Migration + model: `projects.review_harness_policy` (default `same`)
- [ ] 1.2 `ResolveReviewHarness` + degrade chain + warnings + unit tests (matrix policy × available providers)
- [ ] 1.2b `underlying_provider` field trong cli_engine_config + effectiveProvider logic + tests (Claude-CLI + chỉ có Anthropic key → sameHarness=true) (REQ-001b)
- [ ] 1.2c Adversarial directive inject khi sameHarness + test không inject khi provider khác (REQ-001c)
- [ ] 1.3 Review step nhận model override; policy=same zero-regression test (REQ-M01)
- [ ] 1.4 Metadata `coded_by`/`reviewed_by` vào step state + PR description footer (REQ-002)
- [ ] 1.5 `cross_review` step cho cli_spec_first (diff + spec input, 2-verdict, re-dispatch loop) (REQ-003)
- [ ] 1.6 DAG: node optional theo policy + BuildWorkflow signature update + tests
- [ ] 1.7 UI: project setting select + task detail hiển thị coded_by/reviewed_by
- [ ] 1.8 Integration: CLI task với policy different_provider chạy đủ vòng review-fail-fix
- [ ] 1.9 Update specs.md status
