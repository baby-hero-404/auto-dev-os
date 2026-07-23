# Tasks: Cross-Harness Review

> Prerequisites: `pluggable-execution-engine`, `review-verdict-split` hoàn thành.

- [x] 1.1 Migration + model: `projects.review_harness_policy` (default deviates to `different_model`, see implementation-notes.md — the pre-existing Harness Independence feature already made different-model exclusion the unconditional default, so `same`-as-default would have been a regression)
- [x] 1.2 `ResolveReviewHarness` + degrade chain — implemented inline in `review.go`/`cross_review.go` as a policy switch rather than a standalone resolver function (no separate `available []ProviderModels` input existed to resolve against; the Gateway's provider list isn't exposed to the `steps` package)
- [x] 1.2b `underlying_provider` field trong cli_engine_config + `effectiveCLIHarness` logic + tests (REQ-001b)
- [x] 1.2c Adversarial directive inject khi sameHarness + test không inject khi provider khác (REQ-001c) — via the two-part heuristic in implementation-notes.md (policy==same, or prior-cycle self_review_fallback), not literal synchronous same-harness prediction
- [x] 1.3 Review step nhận model override; policy=same zero-regression test (REQ-M01)
- [x] 1.4 Metadata `coded_by`/`reviewed_by` vào step state + PR description footer (REQ-002)
- [x] 1.5 `cross_review` step cho cli_spec_first (diff + spec input, 2-verdict, re-dispatch loop via `ErrCrossReviewFixLoop`) (REQ-003)
- [x] 1.6 DAG: node optional theo policy (`CLISpecFirstWorkflow(runners, includeCrossReview)`) + `worker.go` call site updated + tests
- [x] 1.7 UI: project setting select + task detail hiển thị coded_by/reviewed_by — added Review Harness Policy select field to `project-profile.tsx` and `coded_by`/`reviewed_by` metadata badges in `TaskSidebar.tsx`.
- [ ] 1.8 Integration: CLI task với policy different_provider chạy đủ vòng review-fail-fix — **deferred**; covered instead by unit tests on `CrossReviewStep` (pass/fail/cycle-limit) and the DAG-shape test, not a full engine-driven end-to-end run
- [x] 1.9 Update specs.md status

## Docs sync

- [x] Update corresponding `docs/features/` as specified in feature-docs-sync/design.md — done 2026-07-23: product/09, product/01
